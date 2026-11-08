package uwp

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

// Delta installation builds a new game version from the blocks of an
// existing instance plus the blocks that changed, fetched with HTTP range
// requests, instead of downloading the whole package.
//
// An Appx package lists every payload file in AppxBlockMap.xml as 64 KiB
// blocks of uncompressed data, each with its SHA-256 and, for deflated files,
// the size of its independently compressed bytes. A block whose hash matches
// the same block of the same file in the old instance is copied locally;
// every other block is downloaded, inflated on its own and checked against
// its hash. The block map is authenticated by the package signature (see
// verifyAppxSignature), so every written byte is verified.

const deltaBlockSize = 64 << 10

// DeltaStats reports a delta installation as it runs.
type DeltaStats struct {
	// Phase is "verify", "compare", "download" or "finish".
	Phase string `json:"phase"`
	// Total is the uncompressed size of the new build.
	Total int64 `json:"total"`
	// Reused is how much of it was taken from the old instance.
	Reused int64 `json:"reused"`
	// ToDownload and Downloaded count compressed bytes fetched over HTTP.
	ToDownload int64 `json:"toDownload"`
	Downloaded int64 `json:"downloaded"`
}

type DeltaOptions struct {
	Options
	Client *http.Client
	Stats  func(DeltaStats)
}

type blockMapXML struct {
	HashMethod string `xml:"HashMethod,attr"`
	Files      []struct {
		Name    string `xml:"Name,attr"`
		Size    int64  `xml:"Size,attr"`
		LfhSize int64  `xml:"LfhSize,attr"`
		Blocks  []struct {
			Hash string `xml:"Hash,attr"`
			Size *int64 `xml:"Size,attr"`
		} `xml:"Block"`
	} `xml:"File"`
}

type deltaBlock struct {
	hash   [sha256.Size]byte
	offset int64 // uncompressed offset in the file
	length int64 // uncompressed length
	coff   int64 // offset of its stored bytes from the file data start
	csize  int64 // size of its stored bytes
	need   bool
}

type deltaFile struct {
	rel string
	// legacyRel is the still percent-encoded name under which launchers
	// before part-name decoding installed the file; "" when it is rel.
	legacyRel  string
	size       int64
	method     uint16
	dataOffset int64
	blocks     []deltaBlock
}

func deltaUnavailable(err error) error { return failure("ERR_UWP_DELTA_UNAVAILABLE", err) }

// blockMapKey maps a zip entry name (percent-encoded, "/") and a block map
// name (decoded, "\") to the same key.
func blockMapKey(name string) string {
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return strings.ToLower(strings.ReplaceAll(name, "/", `\`))
}

// InstallDelta installs the package at packageURL into targetDir, reusing
// identical blocks from the instance in baseDir. It publishes the same
// layout as Install and never modifies baseDir.
func InstallDelta(ctx context.Context, packageURL, baseDir, targetDir string, opts DeltaOptions) (Manifest, error) {
	var manifest Manifest
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Client == nil {
		return manifest, errors.New("InstallDelta: no HTTP client")
	}
	stats := DeltaStats{Phase: "verify"}
	var statsMu sync.Mutex
	// Workers report concurrently; the callback runs under the lock so it
	// sees one update at a time.
	report := func(update func(*DeltaStats)) {
		statsMu.Lock()
		defer statsMu.Unlock()
		update(&stats)
		if opts.Stats != nil {
			opts.Stats(stats)
		}
	}
	report(func(*DeltaStats) {})
	if _, err := os.Lstat(targetDir); !os.IsNotExist(err) {
		return manifest, failure("ERR_TARGET_EXISTS", err)
	}
	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return manifest, err
	}
	stage, err := fsx.MkdirTemp(parent, filepath.Base(targetDir))
	if err != nil {
		return manifest, err
	}
	defer os.RemoveAll(stage)

	remote, err := newHTTPRangeReader(ctx, opts.Client, packageURL)
	if err != nil {
		if ctx.Err() != nil {
			return manifest, failure("ERR_CANCELED", ctx.Err())
		}
		return manifest, deltaUnavailable(err)
	}
	var data io.ReaderAt = remote
	size := remote.Size()
	meta := newChunkCache(data, size)
	dir, err := readZipDirectory(meta, size)
	if err != nil {
		return manifest, failure("ERR_UWP_ARCHIVE", err)
	}
	if findZipEntry(dir, "AppxManifest.xml") == nil {
		// A bundle: read the application package for this machine in place.
		zr, err := zip.NewReader(meta, size)
		if err != nil {
			return manifest, failure("ERR_UWP_ARCHIVE", err)
		}
		selected, err := selectBundlePackage(zr, runtime.GOARCH)
		if err != nil {
			return manifest, err
		}
		if selected.Method != zip.Store {
			return manifest, deltaUnavailable(errors.New("bundled package is compressed"))
		}
		off, err := selected.DataOffset()
		if err != nil {
			return manifest, failure("ERR_UWP_ARCHIVE", err)
		}
		size = int64(selected.CompressedSize64)
		data = io.NewSectionReader(remote, off, size)
		meta = newChunkCache(data, size)
		if dir, err = readZipDirectory(meta, size); err != nil {
			return manifest, failure("ERR_UWP_ARCHIVE", err)
		}
	}

	// Footprint files: authenticate the block map through the signature.
	sigEntry := findZipEntry(dir, "AppxSignature.p7x")
	bmEntry := findZipEntry(dir, "AppxBlockMap.xml")
	ctEntry := findZipEntry(dir, "[Content_Types].xml")
	ciEntry := findZipEntry(dir, "AppxMetadata/CodeIntegrity.cat")
	if sigEntry == nil || bmEntry == nil || ctEntry == nil {
		return manifest, deltaUnavailable(errors.New("package is not signed"))
	}
	sig, err := readZipSmall(meta, *sigEntry, 1<<20)
	if err != nil {
		return manifest, failure("ERR_UWP_ARCHIVE", err)
	}
	digests, _, err := verifyDeltaSignature(sig)
	if err != nil {
		return manifest, deltaUnavailable(fmt.Errorf("signature: %w", err))
	}
	footprint := map[*zipEntry][]byte{}
	for _, part := range []struct {
		entry *zipEntry
		tag   string
		limit int64
	}{{bmEntry, "AXBM", 128 << 20}, {ctEntry, "AXCT", 1 << 20}, {ciEntry, "AXCI", 16 << 20}} {
		if part.entry == nil {
			continue
		}
		content, err := readZipSmall(meta, *part.entry, part.limit)
		if err != nil {
			return manifest, failure("ERR_UWP_ARCHIVE", err)
		}
		sum := sha256.Sum256(content)
		if want := digests[part.tag]; len(want) != sha256.Size || !bytes.Equal(want, sum[:]) {
			return manifest, failure("ERR_UWP_INTEGRITY", fmt.Errorf("%s does not match the signature", part.entry.Name))
		}
		footprint[part.entry] = content
	}
	var bm blockMapXML
	if err := xml.Unmarshal(footprint[bmEntry], &bm); err != nil {
		return manifest, failure("ERR_UWP_ARCHIVE", err)
	}
	if bm.HashMethod != "http://www.w3.org/2001/04/xmlenc#sha256" {
		return manifest, deltaUnavailable(fmt.Errorf("block map hash %q", bm.HashMethod))
	}

	files, err := planDeltaFiles(dir, &bm, map[*zipEntry]bool{sigEntry: true, bmEntry: true, ctEntry: true, ciEntry: true})
	if err != nil {
		return manifest, err
	}
	for _, f := range files {
		stats.Total += f.size
	}

	// Write the footprint files and allocate the payload files.
	for entry, content := range footprint {
		rel, err := packagePath(entry.Name)
		if err != nil {
			return manifest, failure("ERR_UWP_UNSAFE_ARCHIVE", err)
		}
		target := filepath.Join(stage, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return manifest, err
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return manifest, err
		}
	}
	for _, f := range files {
		target := filepath.Join(stage, f.rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return manifest, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return manifest, err
		}
		err = out.Truncate(f.size)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return manifest, err
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var firstErr error
	var errOnce sync.Once
	fail := func(err error) {
		errOnce.Do(func() { firstErr = err; cancel() })
	}
	var done int64
	var progressMu sync.Mutex
	total := stats.Total
	progress := func(f *deltaFile, n int64) {
		progressMu.Lock()
		defer progressMu.Unlock()
		done += n
		if opts.Progress != nil {
			opts.Progress(Progress{Current: done, Total: total, File: f.rel})
		}
	}

	// Compare with the old instance and copy matching blocks.
	report(func(s *DeltaStats) { s.Phase = "compare" })
	work := make(chan *deltaFile)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, deltaBlockSize)
			for f := range work {
				reused, err := reuseBlocks(runCtx, baseDir, stage, f, buf, progress)
				if err != nil {
					fail(err)
					continue
				}
				if reused > 0 {
					report(func(s *DeltaStats) { s.Reused += reused })
				}
			}
		}()
	}
	for _, f := range files {
		if runCtx.Err() != nil {
			break
		}
		work <- f
	}
	close(work)
	wg.Wait()
	if firstErr != nil {
		return manifest, firstErr
	}
	if err := ctx.Err(); err != nil {
		return manifest, failure("ERR_CANCELED", err)
	}

	// Download the remaining blocks in coalesced ranges.
	ranges := planDeltaRanges(files)
	var toDownload int64
	for _, r := range ranges {
		toDownload += r.end - r.start
	}
	startDownloaded := remote.downloaded.Load()
	report(func(s *DeltaStats) { s.Phase = "download"; s.ToDownload = toDownload })
	rangeWork := make(chan deltaRange)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range rangeWork {
				if err := fetchDeltaRange(runCtx, data, stage, r, progress); err != nil {
					fail(err)
					continue
				}
				downloaded := remote.downloaded.Load() - startDownloaded
				report(func(s *DeltaStats) { s.Downloaded = downloaded })
			}
		}()
	}
	for _, r := range ranges {
		if runCtx.Err() != nil {
			break
		}
		rangeWork <- r
	}
	close(rangeWork)
	wg.Wait()
	if firstErr != nil {
		if ctx.Err() != nil {
			return manifest, failure("ERR_CANCELED", ctx.Err())
		}
		return manifest, firstErr
	}
	if err := ctx.Err(); err != nil {
		return manifest, failure("ERR_CANCELED", err)
	}
	report(func(s *DeltaStats) { s.Phase = "finish" })
	return publishStage(ctx, stage, targetDir, opts.Options)
}

func findZipEntry(dir *zipDirectory, name string) *zipEntry {
	for i := range dir.Entries {
		if strings.EqualFold(strings.ReplaceAll(dir.Entries[i].Name, `\`, "/"), name) {
			return &dir.Entries[i]
		}
	}
	return nil
}

// readZipSmall reads and inflates a small entry and checks its CRC.
func readZipSmall(r io.ReaderAt, e zipEntry, limit int64) ([]byte, error) {
	if e.CompressedSize > limit || e.UncompressedSize > limit {
		return nil, fmt.Errorf("%s is too large", e.Name)
	}
	off, err := e.dataOffset(r)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, e.CompressedSize)
	if _, err := r.ReadAt(raw, off); err != nil && err != io.EOF {
		return nil, err
	}
	var content []byte
	switch e.Method {
	case zip.Store:
		content = raw
	case zip.Deflate:
		content, err = io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), limit+1))
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%s uses compression method %d", e.Name, e.Method)
	}
	if int64(len(content)) != e.UncompressedSize || crc32.ChecksumIEEE(content) != e.CRC32 {
		return nil, fmt.Errorf("%s is corrupt", e.Name)
	}
	return content, nil
}

// planDeltaFiles pairs every payload entry with its block map record.
func planDeltaFiles(dir *zipDirectory, bm *blockMapXML, footprint map[*zipEntry]bool) ([]*deltaFile, error) {
	byKey := make(map[string]int, len(bm.Files))
	for i, f := range bm.Files {
		key := blockMapKey(f.Name)
		if _, dup := byKey[key]; dup {
			return nil, failure("ERR_UWP_UNSAFE_ARCHIVE", fmt.Errorf("duplicate block map file %q", f.Name))
		}
		byKey[key] = i
	}
	seen := make(map[string]bool, len(dir.Entries))
	var files []*deltaFile
	var total int64
	for i := range dir.Entries {
		e := &dir.Entries[i]
		if footprint[e] {
			continue
		}
		rel, err := packagePath(e.Name)
		if err != nil {
			return nil, failure("ERR_UWP_UNSAFE_ARCHIVE", err)
		}
		if strings.HasSuffix(e.Name, "/") {
			return nil, failure("ERR_UWP_UNSAFE_ARCHIVE", fmt.Errorf("directory entry %q", e.Name))
		}
		key := blockMapKey(e.Name)
		if seen[key] {
			return nil, failure("ERR_UWP_UNSAFE_ARCHIVE", fmt.Errorf("duplicate archive path %q", e.Name))
		}
		seen[key] = true
		idx, ok := byKey[key]
		if !ok {
			return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("%q is not in the block map", e.Name))
		}
		rec := bm.Files[idx]
		if rec.Size != e.UncompressedSize || rec.Size < 0 || rec.LfhSize < 30 || rec.LfhSize > 64<<10 {
			return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("%q does not match the block map", e.Name))
		}
		if e.Method != zip.Store && e.Method != zip.Deflate {
			return nil, deltaUnavailable(fmt.Errorf("%q uses compression method %d", e.Name, e.Method))
		}
		total += rec.Size
		if total > maxExtractBytes {
			return nil, failure("ERR_UWP_ARCHIVE", fmt.Errorf("archive exceeds size limit"))
		}
		want := (rec.Size + deltaBlockSize - 1) / deltaBlockSize
		if int64(len(rec.Blocks)) != want {
			return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("%q has %d blocks, want %d", e.Name, len(rec.Blocks), want))
		}
		f := &deltaFile{rel: rel, size: rec.Size, method: e.Method, dataOffset: e.HeaderOffset + rec.LfhSize}
		if legacy, err := safeRelativePath(e.Name); err == nil && legacy != rel {
			f.legacyRel = legacy
		}
		var coff int64
		for b, blk := range rec.Blocks {
			hash, err := base64.StdEncoding.DecodeString(blk.Hash)
			if err != nil || len(hash) != sha256.Size {
				return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("bad block hash in %q", e.Name))
			}
			length := int64(deltaBlockSize)
			if rest := rec.Size - int64(b)*deltaBlockSize; rest < length {
				length = rest
			}
			csize := length
			if e.Method == zip.Deflate {
				if blk.Size == nil || *blk.Size <= 0 || *blk.Size > 2*deltaBlockSize {
					return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("bad block size in %q", e.Name))
				}
				csize = *blk.Size
			}
			db := deltaBlock{offset: int64(b) * deltaBlockSize, length: length, coff: coff, csize: csize}
			copy(db.hash[:], hash)
			f.blocks = append(f.blocks, db)
			coff += csize
		}
		// MakeAppx ends each deflated file with an empty final block (2 bytes)
		// after the last separately flushed block.
		if coff != e.CompressedSize && !(e.Method == zip.Deflate && coff+2 == e.CompressedSize) {
			return nil, failure("ERR_UWP_INTEGRITY", fmt.Errorf("%q compressed size does not match its blocks", e.Name))
		}
		files = append(files, f)
	}
	if len(files) != len(bm.Files) {
		return nil, failure("ERR_UWP_INTEGRITY", errors.New("block map lists files missing from the archive"))
	}
	return files, nil
}

// reuseBlocks copies the blocks of f that are unchanged in the old instance
// and marks the others as needed. It returns the reused byte count.
func reuseBlocks(ctx context.Context, baseDir, stage string, f *deltaFile, buf []byte, progress func(*deltaFile, int64)) (int64, error) {
	for i := range f.blocks {
		f.blocks[i].need = true
	}
	if len(f.blocks) == 0 {
		return 0, nil
	}
	old, err := os.Open(filepath.Join(baseDir, f.rel))
	if err != nil && f.legacyRel != "" {
		old, err = os.Open(filepath.Join(baseDir, f.legacyRel))
	}
	if err != nil {
		return 0, nil
	}
	defer old.Close()
	if info, err := old.Stat(); err != nil || !info.Mode().IsRegular() {
		return 0, nil
	}
	var out *os.File
	defer func() {
		if out != nil {
			out.Close()
		}
	}()
	var reused int64
	for i := range f.blocks {
		if err := ctx.Err(); err != nil {
			return reused, failure("ERR_CANCELED", err)
		}
		b := &f.blocks[i]
		chunk := buf[:b.length]
		if _, err := old.ReadAt(chunk, b.offset); err != nil {
			continue
		}
		if sha256.Sum256(chunk) != b.hash {
			continue
		}
		if out == nil {
			if out, err = os.OpenFile(filepath.Join(stage, f.rel), os.O_WRONLY, 0); err != nil {
				return reused, err
			}
		}
		if _, err := out.WriteAt(chunk, b.offset); err != nil {
			return reused, err
		}
		b.need = false
		reused += b.length
		progress(f, b.length)
	}
	if out != nil {
		err := out.Close()
		out = nil
		if err != nil {
			return reused, err
		}
	}
	return reused, nil
}

type deltaItem struct {
	file  *deltaFile
	block *deltaBlock
	at    int64 // absolute offset of the stored bytes
}

type deltaRange struct {
	start, end int64
	items      []deltaItem
}

// planDeltaRanges groups the needed blocks into few range requests, joining
// neighbours separated by small gaps.
func planDeltaRanges(files []*deltaFile) []deltaRange {
	var items []deltaItem
	for _, f := range files {
		for i := range f.blocks {
			if f.blocks[i].need {
				items = append(items, deltaItem{file: f, block: &f.blocks[i], at: f.dataOffset + f.blocks[i].coff})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].at < items[j].at })
	const maxGap, maxRange = 128 << 10, 4 << 20
	var ranges []deltaRange
	for _, it := range items {
		end := it.at + it.block.csize
		if n := len(ranges); n > 0 && it.at-ranges[n-1].end <= maxGap && end-ranges[n-1].start <= maxRange && it.at >= ranges[n-1].end {
			ranges[n-1].end = end
			ranges[n-1].items = append(ranges[n-1].items, it)
			continue
		}
		ranges = append(ranges, deltaRange{start: it.at, end: end, items: []deltaItem{it}})
	}
	return ranges
}

func fetchDeltaRange(ctx context.Context, src io.ReaderAt, stage string, r deltaRange, progress func(*deltaFile, int64)) error {
	if err := ctx.Err(); err != nil {
		return failure("ERR_CANCELED", err)
	}
	buf := make([]byte, r.end-r.start)
	if _, err := src.ReadAt(buf, r.start); err != nil && err != io.EOF {
		if ctx.Err() != nil {
			return failure("ERR_CANCELED", ctx.Err())
		}
		return failure("ERR_UWP_DELTA_DOWNLOAD", err)
	}
	plain := make([]byte, deltaBlockSize)
	for _, it := range r.items {
		stored := buf[it.at-r.start : it.at-r.start+it.block.csize]
		block := plain[:it.block.length]
		if it.file.method == zip.Deflate {
			// Each block is a separately flushed deflate segment without a
			// final block, so the reader stops with io.ErrUnexpectedEOF
			// after producing exactly the block's bytes.
			if _, err := io.ReadFull(flate.NewReader(bytes.NewReader(stored)), block); err != nil {
				return failure("ERR_UWP_INTEGRITY", fmt.Errorf("inflate %s: %v", it.file.rel, err))
			}
		} else {
			copy(block, stored)
		}
		if sha256.Sum256(block) != it.block.hash {
			return failure("ERR_UWP_INTEGRITY", fmt.Errorf("block of %s does not match the block map", it.file.rel))
		}
		out, err := os.OpenFile(filepath.Join(stage, it.file.rel), os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, err = out.WriteAt(block, it.block.offset)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		progress(it.file, it.block.length)
	}
	return nil
}

// verifyDeltaSignature is replaced in tests, which cannot produce a
// Microsoft signature.
var verifyDeltaSignature = verifyAppxSignature
