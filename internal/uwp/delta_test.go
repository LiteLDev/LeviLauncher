package uwp

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type deltaTestFile struct {
	name    string // zip name ("/", percent-encoded like MakeAppx)
	content []byte
	deflate bool
}

const deltaTestSignature = "PKCX-test-signature"

// buildDeltaPackage writes an Appx the way MakeAppx does: deflated files are
// separately flushed 64 KiB segments followed by an empty final block, and
// the block map records each segment.
func buildDeltaPackage(t *testing.T, files []deltaTestFile) ([]byte, appxDigests) {
	t.Helper()
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	var bm strings.Builder
	bm.WriteString(`<?xml version="1.0" encoding="UTF-8"?><BlockMap xmlns="http://schemas.microsoft.com/appx/2010/blockmap" HashMethod="http://www.w3.org/2001/04/xmlenc#sha256">`)
	for _, f := range files {
		var stored bytes.Buffer
		var blocks strings.Builder
		for off := 0; off < len(f.content); off += deltaBlockSize {
			end := off + deltaBlockSize
			if end > len(f.content) {
				end = len(f.content)
			}
			chunk := f.content[off:end]
			sum := sha256.Sum256(chunk)
			if f.deflate {
				var seg bytes.Buffer
				fw, _ := flate.NewWriter(&seg, flate.DefaultCompression)
				fw.Write(chunk)
				fw.Flush()
				fmt.Fprintf(&blocks, `<Block Hash="%s" Size="%d"/>`, base64.StdEncoding.EncodeToString(sum[:]), seg.Len())
				stored.Write(seg.Bytes())
			} else {
				fmt.Fprintf(&blocks, `<Block Hash="%s"/>`, base64.StdEncoding.EncodeToString(sum[:]))
				stored.Write(chunk)
			}
		}
		method := zip.Store
		if f.deflate {
			method = zip.Deflate
			stored.Write([]byte{0x03, 0x00}) // empty final block
		}
		hw, err := w.CreateRaw(&zip.FileHeader{
			Name:               f.name,
			Method:             method,
			CRC32:              crc32.ChecksumIEEE(f.content),
			CompressedSize64:   uint64(stored.Len()),
			UncompressedSize64: uint64(len(f.content)),
		})
		if err != nil {
			t.Fatal(err)
		}
		hw.Write(stored.Bytes())
		bmName := strings.ReplaceAll(strings.ReplaceAll(f.name, "%20", " "), "/", `\`)
		fmt.Fprintf(&bm, `<File Name="%s" Size="%d" LfhSize="%d">%s</File>`, bmName, len(f.content), 30+len(f.name), blocks.String())
	}
	bm.WriteString(`</BlockMap>`)
	contentTypes := []byte(`<?xml version="1.0"?><Types/>`)
	for name, data := range map[string][]byte{"AppxBlockMap.xml": []byte(bm.String()), "[Content_Types].xml": contentTypes, "AppxSignature.p7x": []byte(deltaTestSignature)} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(data)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	bmSum := sha256.Sum256([]byte(bm.String()))
	ctSum := sha256.Sum256(contentTypes)
	return archive.Bytes(), appxDigests{"AXBM": bmSum[:], "AXCT": ctSum[:]}
}

func stubDeltaSignature(t *testing.T, digests appxDigests) {
	t.Helper()
	prev := verifyDeltaSignature
	verifyDeltaSignature = func(p7x []byte) (appxDigests, *x509.Certificate, error) {
		if string(p7x) != deltaTestSignature {
			return nil, nil, errors.New("bad test signature")
		}
		return digests, nil, nil
	}
	t.Cleanup(func() { verifyDeltaSignature = prev })
}

// serveArchive serves data with Range support and counts the bytes sent.
func serveArchive(t *testing.T, data []byte, ranges bool) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var sent atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ranges {
			r.Header.Del("Range")
		}
		cw := &countingWriter{ResponseWriter: w, n: &sent}
		http.ServeContent(cw, r, "test.appx", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, &sent
}

type countingWriter struct {
	http.ResponseWriter
	n *atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n.Add(int64(len(p)))
	return c.ResponseWriter.Write(p)
}

func patternBytes(n int, seed byte) []byte {
	b := make([]byte, n)
	x := uint32(seed) + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func writeTree(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInstallDeltaReusesUnchangedBlocks(t *testing.T) {
	big := patternBytes(5*deltaBlockSize+123, 1)
	bigNew := append([]byte(nil), big...)
	copy(bigNew[2*deltaBlockSize+10:], []byte("changed in the new version"))
	same := patternBytes(3*deltaBlockSize, 2)
	stored := patternBytes(2*deltaBlockSize+7, 3)
	storedNew := append([]byte(nil), stored...)
	storedNew[deltaBlockSize+1] ^= 0xFF
	added := patternBytes(deltaBlockSize/2, 4)
	manifest := []byte(testManifest("neutral"))
	exe := []byte("test-executable")

	archive, digests := buildDeltaPackage(t, []deltaTestFile{
		{"AppxManifest.xml", manifest, true},
		{"Minecraft.Windows.exe", exe, true},
		{"data/big.bin", bigNew, true},
		{"data/same.bin", same, true},
		{"data/stored.png", storedNew, false},
		{"data/new%20file.json", added, true},
		{"data/empty.txt", nil, true},
	})
	stubDeltaSignature(t, digests)
	srv, sent := serveArchive(t, archive, true)

	base := t.TempDir()
	writeTree(t, base, map[string][]byte{
		"AppxManifest.xml":      manifest,
		"Minecraft.Windows.exe": exe,
		"data/big.bin":          big,
		"data/same.bin":         same,
		"data/stored.png":       stored,
		// Installed by an older launcher under its encoded part name.
		"data/new%20file.json":              added,
		"Minecraft Bedrock/Users/world.dat": []byte("user data is not part of the package"),
	})
	target := filepath.Join(t.TempDir(), "new")
	var last DeltaStats
	_, err := InstallDelta(context.Background(), srv.URL+"/pkg.appx", base, target, DeltaOptions{
		Client: srv.Client(),
		Stats:  func(s DeltaStats) { last = s },
	})
	if err != nil {
		t.Fatalf("InstallDelta: %v", err)
	}
	want := map[string][]byte{
		"AppxManifest.xml":      manifest,
		"Minecraft.Windows.exe": exe,
		"data/big.bin":          bigNew,
		"data/same.bin":         same,
		"data/stored.png":       storedNew,
		"data/new file.json":    added,
		"data/empty.txt":        {},
	}
	for name, data := range want {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("%s differs (err=%v, len %d want %d)", name, err, len(got), len(data))
		}
	}
	for _, absent := range []string{"AppxSignature.p7x", "Minecraft Bedrock", "data/new%20file.json"} {
		if _, err := os.Stat(filepath.Join(target, absent)); !os.IsNotExist(err) {
			t.Errorf("%s should not be in the new build", absent)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "AppxBlockMap.xml")); err != nil {
		t.Errorf("block map missing: %v", err)
	}
	// Unchanged: all of same.bin, 5 of 6 blocks of big.bin, 2 of 3 of
	// stored.png, the manifest, the executable and, found under its legacy
	// encoded name, new file.json.
	wantReused := int64(len(same)) + int64(len(big)) - deltaBlockSize + int64(len(stored)) - deltaBlockSize + int64(len(manifest)+len(exe)+len(added))
	if last.Phase != "finish" || last.Reused != wantReused {
		t.Errorf("stats = %+v, want reused %d", last, wantReused)
	}
	if sent.Load() >= int64(len(archive))/2 {
		t.Errorf("sent %d of %d bytes; delta should transfer far less", sent.Load(), len(archive))
	}
	t.Logf("archive %d bytes, transferred %d, reused %d of %d", len(archive), sent.Load(), last.Reused, last.Total)
}

func TestInstallDeltaRejectsTamperedBlock(t *testing.T) {
	content := patternBytes(2*deltaBlockSize, 5)
	archive, digests := buildDeltaPackage(t, []deltaTestFile{
		{"AppxManifest.xml", []byte(testManifest("neutral")), false},
		{"Minecraft.Windows.exe", []byte("test-executable"), false},
		{"data/blob.bin", content, false},
	})
	stubDeltaSignature(t, digests)
	// Corrupt the stored bytes of data/blob.bin in transit.
	i := bytes.Index(archive, content[deltaBlockSize:deltaBlockSize+64])
	if i < 0 {
		t.Fatal("block not found in archive")
	}
	archive[i+3] ^= 0xFF
	srv, _ := serveArchive(t, archive, true)
	target := filepath.Join(t.TempDir(), "new")
	_, err := InstallDelta(context.Background(), srv.URL, t.TempDir(), target, DeltaOptions{Client: srv.Client()})
	if ErrorCode(err) != "ERR_UWP_INTEGRITY" {
		t.Fatalf("err = %v, want ERR_UWP_INTEGRITY", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatal("a corrupt build was published")
	}
}

func TestInstallDeltaNeedsRangeRequests(t *testing.T) {
	archive, digests := buildDeltaPackage(t, []deltaTestFile{{"AppxManifest.xml", []byte(testManifest("neutral")), true}})
	stubDeltaSignature(t, digests)
	srv, _ := serveArchive(t, archive, false)
	_, err := InstallDelta(context.Background(), srv.URL, t.TempDir(), filepath.Join(t.TempDir(), "new"), DeltaOptions{Client: srv.Client()})
	if ErrorCode(err) != "ERR_UWP_DELTA_UNAVAILABLE" {
		t.Fatalf("err = %v, want ERR_UWP_DELTA_UNAVAILABLE", err)
	}
}

func TestInstallDeltaRejectsUnsignedBlockMap(t *testing.T) {
	archive, digests := buildDeltaPackage(t, []deltaTestFile{{"AppxManifest.xml", []byte(testManifest("neutral")), true}})
	wrong := appxDigests{"AXBM": make([]byte, 32), "AXCT": digests["AXCT"]}
	stubDeltaSignature(t, wrong)
	srv, _ := serveArchive(t, archive, true)
	_, err := InstallDelta(context.Background(), srv.URL, t.TempDir(), filepath.Join(t.TempDir(), "new"), DeltaOptions{Client: srv.Client()})
	if ErrorCode(err) != "ERR_UWP_INTEGRITY" {
		t.Fatalf("err = %v, want ERR_UWP_INTEGRITY", err)
	}
}
