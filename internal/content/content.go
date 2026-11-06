package content

import (
	"archive/zip"
	"bytes"
	crand "crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func findManifestDir(dir string) string {
	if utils.FileExists(filepath.Join(dir, "manifest.json")) {
		return dir
	}
	queue := []string{dir}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ents, er := os.ReadDir(cur)
		if er != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			sub := filepath.Join(cur, e.Name())
			if utils.FileExists(filepath.Join(sub, "manifest.json")) {
				return sub
			}
			queue = append(queue, sub)
		}
	}
	return ""
}

func findPackPathsByUuid(uuid string, dirs ...string) []string {
	if uuid == "" {
		return nil
	}
	var paths []string
	for _, root := range dirs {
		if strings.TrimSpace(root) == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(root, e.Name())
			mfPath := filepath.Join(p, "manifest.json")
			b, err := os.ReadFile(mfPath)
			if err != nil {
				continue
			}
			var mf packages.RawManifest
			if err := json.Unmarshal(utils.JsonCompatBytes(b), &mf); err != nil {
				continue
			}
			if mf.Header.UUID == uuid {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func generateRandomPackName() string {
	b := make([]byte, 8)
	_, _ = crand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func normalizeZipEntryName(name string) string {
	n := strings.TrimSpace(name)
	n = strings.TrimPrefix(n, "./")
	n = strings.ReplaceAll(n, "\\", "/")
	n = strings.TrimPrefix(n, "/")
	for strings.Contains(n, "//") {
		n = strings.ReplaceAll(n, "//", "/")
	}
	return n
}

func ImportMcpackToDirs(data []byte, resDir, bpDir, skinDir string, overwrite bool) string {
	if strings.TrimSpace(resDir) == "" && strings.TrimSpace(bpDir) == "" && strings.TrimSpace(skinDir) == "" {
		return "ERR_OPEN_ZIP"
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	manifestDir := ""
	var manifest packages.RawManifest
	for _, f := range zr.File {
		nameInZip := normalizeZipEntryName(f.Name)
		if strings.HasSuffix(nameInZip, "/") {
			continue
		}
		if strings.EqualFold(path.Base(nameInZip), "manifest.json") {
			dir := path.Dir(nameInZip)
			if parsed, err := readPackManifest(f); err == nil {
				manifest = parsed
			}
			if dir != "." && strings.TrimSpace(dir) != "" {
				manifestDir = dir
			}
			break
		}
	}
	if manifestDir == "" && len(manifest.Modules) == 0 {
		return "ERR_MANIFEST_NOT_FOUND"
	}
	return installPack(zr, manifestDir, manifest, resDir, bpDir, skinDir, overwrite)
}

func readPackManifest(f *zip.File) (packages.RawManifest, error) {
	var manifest packages.RawManifest
	r, err := f.Open()
	if err != nil {
		return manifest, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(utils.JsonCompatBytes(b), &manifest)
	return manifest, err
}

// Different import entry points can install the same UUID into the same roots.
var packInstallMu sync.Mutex

func installPack(zr *zip.Reader, manifestDir string, manifest packages.RawManifest, resDir, bpDir, skinDir string, overwrite bool) string {
	packInstallMu.Lock()
	defer packInstallMu.Unlock()

	existing := findPackPathsByUuid(manifest.Header.UUID, resDir, bpDir, skinDir)
	if len(existing) > 0 && !overwrite {
		return "ERR_DUPLICATE_UUID"
	}

	var roots []string
	hadSkin := false
	for _, module := range manifest.Modules {
		var dir string
		switch strings.ToLower(strings.TrimSpace(module.Type)) {
		case "resources":
			dir = resDir
		case "data", "script":
			dir = bpDir
		case "skin_pack":
			hadSkin = true
			dir = skinDir
		}
		if strings.TrimSpace(dir) != "" {
			dir = filepath.Clean(dir)
			if !slices.Contains(roots, dir) {
				roots = append(roots, dir)
			}
		}
	}
	if len(roots) == 0 {
		if hadSkin {
			return "ERR_NO_PLAYER"
		}
		return "ERR_INVALID_PACKAGE"
	}

	// Keep the installed copies until every module of the replacement has been extracted.
	name := generateRandomPackName()
	var created []string
	defer func() {
		for _, dir := range created {
			_ = os.RemoveAll(dir)
		}
	}()
	for _, dir := range roots {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "ERR_CREATE_TARGET_DIR"
		}
		target := filepath.Join(dir, name)
		if err := os.Mkdir(target, 0755); err != nil {
			if os.IsExist(err) {
				return "ERR_DUPLICATE_FOLDER"
			}
			return "ERR_CREATE_TARGET_DIR"
		}
		created = append(created, target)
		if result := extractContentArchive(zr, manifestDir, target); result != "" {
			return result
		}
	}
	created = nil
	for _, dir := range existing {
		if err := os.RemoveAll(dir); err != nil {
			return "ERR_WRITE_FILE"
		}
	}
	return ""
}

func extractContentArchive(zr *zip.Reader, archiveDir, targetRoot string) string {
	for _, f := range zr.File {
		name := normalizeZipEntryName(f.Name)
		if archiveDir != "" {
			if name == archiveDir || !strings.HasPrefix(name, archiveDir+"/") {
				continue
			}
			name = strings.TrimPrefix(name, archiveDir+"/")
		}
		if !filepath.IsLocal(name) {
			continue
		}
		target := filepath.Join(targetRoot, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "ERR_CREATE_TARGET_DIR"
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "ERR_CREATE_TARGET_DIR"
		}
		if result := extractContentFile(f, target); result != "" {
			return result
		}
	}
	return ""
}

func extractContentFile(f *zip.File, target string) string {
	in, err := f.Open()
	if err != nil {
		return "ERR_READ_ZIP_ENTRY"
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return "ERR_WRITE_FILE"
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func ImportMcaddonToDirs(data []byte, resDir, bpDir, skinDir string, overwrite bool) string {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	imported := false
	hadSkin := false
	for _, f := range zr.File {
		name := strings.TrimSpace(f.Name)
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".mcpack") {
			rc, er := f.Open()
			if er != nil {
				return "ERR_OPEN_ZIP"
			}
			b, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "ERR_OPEN_ZIP"
			}
			err2 := ImportMcpackToDirs(b, resDir, bpDir, skinDir, overwrite)
			if err2 != "" {
				return err2
			}
			imported = true
		}
	}

	for _, f := range zr.File {
		name := normalizeZipEntryName(f.Name)
		if strings.HasSuffix(name, "/") || !strings.EqualFold(path.Base(name), "manifest.json") {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			continue
		}
		manifest, err := readPackManifest(f)
		if err != nil {
			continue
		}
		switch result := installPack(zr, dir, manifest, resDir, bpDir, skinDir, overwrite); result {
		case "":
			imported = true
		case "ERR_NO_PLAYER":
			hadSkin = true
		case "ERR_INVALID_PACKAGE":
			continue
		default:
			return result
		}
	}
	if imported {
		return ""
	}
	if hadSkin {
		return "ERR_NO_PLAYER"
	}
	return "ERR_INVALID_PACKAGE"
}

func ImportMcworldToDir(data []byte, worldsDir string) string {
	if strings.TrimSpace(worldsDir) == "" {
		return "ERR_OPEN_ZIP"
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	levelDir := ""
	for _, f := range zr.File {
		nameInZip := normalizeZipEntryName(f.Name)
		if strings.HasSuffix(nameInZip, "/") {
			continue
		}
		if strings.EqualFold(path.Base(nameInZip), "level.dat") {
			d := path.Dir(nameInZip)
			if d != "." && strings.TrimSpace(d) != "" {
				levelDir = d
			}
			break
		}
	}
	targetRoot := filepath.Join(worldsDir, generateRandomPackName())
	return extractContentArchive(zr, levelDir, targetRoot)
}

func ReadPackInfoFromDir(dir string) types.PackInfo {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return types.PackInfo{}
	}
	target := findManifestDir(dir)
	if target == "" {
		target = dir
	}
	var mf packages.RawManifest
	if b, err := os.ReadFile(filepath.Join(target, "manifest.json")); err == nil {
		_ = json.Unmarshal(utils.JsonCompatBytes(b), &mf)
	}
	return mf.ReadPackInfo(target)
}

func IsMcpackSkinPack(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		nameInZip := normalizeZipEntryName(f.Name)
		if strings.EqualFold(path.Base(nameInZip), "manifest.json") {
			mf, err := readPackManifest(f)
			if err != nil {
				continue
			}
			for _, m := range mf.Modules {
				tp := strings.ToLower(strings.TrimSpace(m.Type))
				if tp == "skin_pack" {
					return true
				}
			}
		}
	}
	return false
}
