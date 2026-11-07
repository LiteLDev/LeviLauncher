package archive

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// testdata/dotnet_inflate_truncated.json was produced by a C# harness over
// lip v0.38.0-beta.1's ZipSource: Deflate streams (dynamic, stored and
// Huffman-only blocks; one of ~32 KiB, so several 8 KiB input buffers) are
// cut after each of a set of byte counts and stored as method 8 (Deflate,
// zlib in .NET) and method 9 (Deflate64, .NET's managed decoder) with the
// full uncompressed size and with a third of it. For each case (in the
// order cut, method 8 then 9, full size then a third) it records how many
// bytes ZipSource.OpenRead serves (-1: it throws). Neither decoder reports
// data that ends early; zlib serves every complete symbol, the managed
// decoder only what it handed out before the input ran out.
func TestZipSource_TruncatedDeflateMatchesDotnet(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "dotnet_inflate_truncated.json"))
	if err != nil {
		t.Fatal(err)
	}
	var streams []struct {
		Name   string
		Stream string
		Size   uint64
		Cuts   []int
		Served []int64
	}
	if err := json.Unmarshal(b, &streams); err != nil {
		t.Fatal(err)
	}
	if len(streams) == 0 {
		t.Fatal("no streams")
	}
	for _, s := range streams {
		comp, _ := base64.StdEncoding.DecodeString(s.Stream)
		full, err := io.ReadAll(newDeflateReader(bytes.NewReader(comp)))
		if err != nil || uint64(len(full)) != s.Size {
			t.Fatalf("%s: decoded %d bytes, %v", s.Name, len(full), err)
		}
		if len(s.Served) != 4*len(s.Cuts) {
			t.Fatalf("%s: %d results for %d cuts", s.Name, len(s.Served), len(s.Cuts))
		}
		k := 0
		for _, cut := range s.Cuts {
			for _, method := range []uint16{zipDeflate, zipDeflate64} {
				for _, usize := range []uint64{s.Size, s.Size / 3} {
					want := s.Served[k]
					k++
					got, err := readAll(rawZipSource(t, method, comp[:cut], usize), "f")
					switch {
					case want < 0:
						if err == nil {
							t.Errorf("%s cut %d method %d size %d: %d bytes, want an error", s.Name, cut, method, usize, len(got))
						}
					case err != nil || int64(len(got)) != want || !bytes.Equal(got, full[:len(got)]):
						t.Errorf("%s cut %d method %d size %d: %d bytes, %v; want %d", s.Name, cut, method, usize, len(got), err, want)
					}
				}
			}
		}
	}
}

// testdata/dotnet_inflate_corrupt.json: the same harness over Deflate
// streams (dynamic, fixed, Huffman-only and stored blocks) with one to
// three bits flipped and sometimes cut, as method 8 and 9 with an
// uncompressed size of 1000000: whether ZipSource.OpenRead throws, and
// otherwise the length and CRC-32 of what it serves. zlib and .NET's own
// Deflate64 decoder treat broken codes differently; both are matched.
func TestZipSource_CorruptDeflateMatchesDotnet(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "dotnet_inflate_corrupt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Bases []string
		Cases []struct {
			Base   int
			Method uint16
			Flips  []int
			Cut    int
			Served int64
			CRC    uint32
		}
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Cases) == 0 {
		t.Fatal("no cases")
	}
	for i, c := range g.Cases {
		comp, _ := base64.StdEncoding.DecodeString(g.Bases[c.Base])
		for _, bit := range c.Flips {
			comp[bit/8] ^= 1 << (bit % 8)
		}
		if c.Cut >= 0 {
			comp = comp[:c.Cut]
		}
		got, err := readAll(rawZipSource(t, c.Method, comp, 1000000), "f")
		switch {
		case c.Served < 0:
			var de *DamagedError
			if !errors.As(err, &de) {
				t.Errorf("case %d (method %d): %d bytes, %v; want a damaged entry", i, c.Method, len(got), err)
			}
		case err != nil || int64(len(got)) != c.Served || crc32.ChecksumIEEE(got) != c.CRC:
			t.Errorf("case %d (method %d): %d bytes, %v; want %d", i, c.Method, len(got), err, c.Served)
		}
	}
}

func rawZipSource(t *testing.T, method uint16, comp []byte, usize uint64) *ZipSource {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "f", Method: method,
		CompressedSize64: uint64(len(comp)), UncompressedSize64: usize})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(comp)
	zw.Close()
	s, err := OpenZipBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
