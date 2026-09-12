package uwp

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const maxExtractBytes int64 = 20 << 30

type Progress struct {
	Current     int64
	Total       int64
	File        string
	FileCurrent int64
	FileTotal   int64
}

type Options struct {
	Progress func(Progress)
	Prepare  func(staging string, manifest Manifest) error
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func safeRelativePath(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.ContainsAny(normalized, ":\x00") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	for _, component := range strings.Split(strings.TrimSuffix(normalized, "/"), "/") {
		if component == "" || component == "." || component == ".." || strings.TrimRight(component, " .") != component || strings.ContainsAny(component, "<>\"|?*") {
			return "", fmt.Errorf("unsafe archive path %q", name)
		}
		for _, r := range component {
			if r < 32 {
				return "", fmt.Errorf("control character in archive path")
			}
		}
		base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", fmt.Errorf("reserved archive path %q", name)
		}
	}
	return filepath.FromSlash(path.Clean(normalized)), nil
}

func architectureRank(candidate, target string) int {
	candidate = strings.ToLower(candidate)
	var preferred []string
	switch target {
	case "amd64", "x64":
		preferred = []string{"x64", "x86", "neutral"}
	case "arm64":
		preferred = []string{"arm64", "x64", "x86", "neutral"}
	case "386", "x86":
		preferred = []string{"x86", "neutral"}
	default:
		preferred = []string{target, "neutral"}
	}
	for i, arch := range preferred {
		if candidate == arch {
			return i
		}
	}
	return -1
}

func selectBundlePackage(reader *zip.Reader, architecture string) (*zip.File, error) {
	var bundle struct {
		Packages []struct {
			Type         string `xml:"Type,attr"`
			Architecture string `xml:"Architecture,attr"`
			FileName     string `xml:"FileName,attr"`
		} `xml:"Packages>Package"`
	}
	var bundleManifest *zip.File
	for _, entry := range reader.File {
		if strings.EqualFold(strings.ReplaceAll(entry.Name, "\\", "/"), "AppxMetadata/AppxBundleManifest.xml") {
			bundleManifest = entry
			break
		}
	}
	if bundleManifest == nil {
		return nil, failure("ERR_UWP_MANIFEST", fmt.Errorf("AppxManifest.xml or bundle manifest missing"))
	}
	f, err := bundleManifest.Open()
	if err != nil {
		return nil, err
	}
	err = xml.NewDecoder(io.LimitReader(f, 4<<20)).Decode(&bundle)
	_ = f.Close()
	if err != nil {
		return nil, failure("ERR_UWP_MANIFEST", err)
	}
	for _, pkg := range bundle.Packages {
		if strings.EqualFold(pkg.Type, "resource") {
			return nil, failure("ERR_UWP_RESOURCE_BUNDLE", fmt.Errorf("bundle contains an independent resource package %q", pkg.FileName))
		}
	}
	bestRank := 100
	var selected *zip.File
	for _, pkg := range bundle.Packages {
		if !strings.EqualFold(pkg.Type, "application") {
			continue
		}
		rank := architectureRank(pkg.Architecture, architecture)
		if rank < 0 || rank >= bestRank {
			continue
		}
		if _, err := safeRelativePath(pkg.FileName); err != nil {
			return nil, failure("ERR_UWP_UNSAFE_ARCHIVE", err)
		}
		for _, entry := range reader.File {
			if strings.EqualFold(entry.Name, pkg.FileName) {
				selected = entry
				bestRank = rank
				break
			}
		}
	}
	if selected == nil {
		return nil, failure("ERR_UWP_ARCHITECTURE", fmt.Errorf("bundle has no compatible application package"))
	}
	return selected, nil
}

// Install extracts into a sibling staging directory and publishes only a fully
// validated Minecraft package. It never overwrites an existing instance.
func Install(ctx context.Context, archivePath, targetDir string, opts Options) (Manifest, error) {
	var manifest Manifest
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return manifest, failure("ERR_CANCELED", err)
	}
	if _, err := os.Lstat(targetDir); !os.IsNotExist(err) {
		return manifest, failure("ERR_TARGET_EXISTS", err)
	}
	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return manifest, err
	}
	stage, err := os.MkdirTemp(parent, ".uwp-install-")
	if err != nil {
		return manifest, err
	}
	defer os.RemoveAll(stage)
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return manifest, failure("ERR_UWP_ARCHIVE", err)
	}
	defer reader.Close()
	packageReader := &reader.Reader
	hasManifest := false
	for _, entry := range reader.File {
		if strings.EqualFold(entry.Name, "AppxManifest.xml") {
			hasManifest = true
			break
		}
	}
	if !hasManifest {
		selected, err := selectBundlePackage(packageReader, runtime.GOARCH)
		if err != nil {
			return manifest, err
		}
		if selected.UncompressedSize64 > uint64(maxExtractBytes) {
			return manifest, failure("ERR_UWP_ARCHIVE", fmt.Errorf("package exceeds size limit"))
		}
		nested, err := os.CreateTemp(parent, ".uwp-package-*.appx")
		if err != nil {
			return manifest, err
		}
		defer os.Remove(nested.Name())
		src, err := selected.Open()
		if err != nil {
			nested.Close()
			return manifest, err
		}
		nestedSize, copyErr := io.Copy(nested, io.LimitReader(contextReader{ctx: ctx, reader: src}, maxExtractBytes+1))
		_ = src.Close()
		closeErr := nested.Close()
		if copyErr != nil {
			if ctx.Err() != nil {
				return manifest, failure("ERR_CANCELED", ctx.Err())
			}
			return manifest, failure("ERR_UWP_ARCHIVE", copyErr)
		}
		if nestedSize > maxExtractBytes {
			return manifest, failure("ERR_UWP_ARCHIVE", fmt.Errorf("package exceeds size limit"))
		}
		if closeErr != nil {
			return manifest, closeErr
		}
		inner, err := zip.OpenReader(nested.Name())
		if err != nil {
			return manifest, failure("ERR_UWP_ARCHIVE", err)
		}
		defer inner.Close()
		packageReader = &inner.Reader
	}
	if err := extract(ctx, packageReader, stage, opts.Progress); err != nil {
		return manifest, err
	}
	manifest, err = ReadManifest(stage)
	if err != nil {
		return manifest, err
	}
	if architectureRank(manifest.Identity.Architecture, runtime.GOARCH) < 0 {
		return manifest, failure("ERR_UWP_ARCHITECTURE", nil)
	}
	exeRel, _ := safeRelativePath(manifest.Applications[0].Executable)
	if info, err := os.Stat(filepath.Join(stage, exeRel)); err != nil || !info.Mode().IsRegular() {
		return manifest, failure("ERR_NOT_FOUND_EXE", err)
	}
	if opts.Prepare != nil {
		if err := opts.Prepare(stage, manifest); err != nil {
			return manifest, err
		}
	}
	if err := ctx.Err(); err != nil {
		return manifest, failure("ERR_CANCELED", err)
	}
	if _, err := os.Lstat(targetDir); !os.IsNotExist(err) {
		return manifest, failure("ERR_TARGET_EXISTS", err)
	}
	if err := os.Rename(stage, targetDir); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func extract(ctx context.Context, reader *zip.Reader, dir string, progress func(Progress)) error {
	if len(reader.File) > 100000 {
		return failure("ERR_UWP_ARCHIVE", fmt.Errorf("too many archive entries"))
	}
	var total int64
	seen := make(map[string]bool, len(reader.File))
	for _, entry := range reader.File {
		rel, err := safeRelativePath(entry.Name)
		if err != nil {
			return failure("ERR_UWP_UNSAFE_ARCHIVE", err)
		}
		if entry.Mode()&os.ModeSymlink != 0 || (!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
			return failure("ERR_UWP_UNSAFE_ARCHIVE", fmt.Errorf("non-regular entry %q", entry.Name))
		}
		key := strings.ToLower(rel)
		if seen[key] {
			return failure("ERR_UWP_UNSAFE_ARCHIVE", fmt.Errorf("duplicate archive path %q", entry.Name))
		}
		seen[key] = true
		if entry.UncompressedSize64 > uint64(maxExtractBytes-total) {
			return failure("ERR_UWP_ARCHIVE", fmt.Errorf("archive exceeds size limit"))
		}
		total += int64(entry.UncompressedSize64)
	}
	var current int64
	buffer := make([]byte, 256<<10)
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return failure("ERR_CANCELED", err)
		}
		rel, _ := safeRelativePath(entry.Name)
		target := filepath.Join(dir, rel)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractFile(ctx, entry, target, buffer, func(n int64) {
			if progress != nil {
				progress(Progress{Current: current + n, Total: total, File: entry.Name, FileCurrent: n, FileTotal: int64(entry.UncompressedSize64)})
			}
		}); err != nil {
			return err
		}
		current += int64(entry.UncompressedSize64)
	}
	return nil
}

func extractFile(ctx context.Context, entry *zip.File, target string, buffer []byte, progress func(int64)) error {
	src, err := entry.Open()
	if err != nil {
		return failure("ERR_UWP_ARCHIVE", err)
	}
	defer src.Close()
	dest, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer dest.Close()
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return failure("ERR_CANCELED", err)
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			written += int64(n)
			if written > int64(entry.UncompressedSize64) {
				return failure("ERR_UWP_ARCHIVE", fmt.Errorf("entry exceeds declared size"))
			}
			if _, err := dest.Write(buffer[:n]); err != nil {
				return err
			}
			progress(written)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return failure("ERR_UWP_ARCHIVE", readErr)
		}
	}
	if written != int64(entry.UncompressedSize64) {
		return failure("ERR_UWP_ARCHIVE", io.ErrUnexpectedEOF)
	}
	return dest.Close()
}
