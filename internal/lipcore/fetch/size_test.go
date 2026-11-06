package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceSizes(t *testing.T) {
	dir := t.TempDir()

	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, _ := zw.Create("a/b.txt")
	w.Write([]byte("hello"))
	zw.Close()
	zipPath := filepath.Join(dir, "a.zip")
	os.WriteFile(zipPath, zb.Bytes(), 0o644)

	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	tw.WriteHeader(&tar.Header{Name: "c.txt", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
	tw.Write([]byte("abc"))
	tw.Close()
	tarPath := filepath.Join(dir, "a.tar")
	os.WriteFile(tarPath, tb.Bytes(), 0o644)

	single := filepath.Join(dir, "one.bin")
	os.WriteFile(single, make([]byte, 7), 0o644)

	type sizer interface {
		Size(string) (int64, bool)
	}
	check := func(name string, src AssetSource, key string, want int64) {
		t.Helper()
		s, ok := src.(sizer)
		if !ok {
			t.Fatalf("%s: source %T has no Size", name, src)
		}
		if got, ok := s.Size(key); !ok || got != want {
			t.Errorf("%s: Size(%q) = %d, %v; want %d, true", name, key, got, ok, want)
		}
		if _, ok := s.Size("missing"); ok {
			t.Errorf("%s: Size of a missing key reported ok", name)
		}
	}

	zs, err := OpenArchive("zip", zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zs.Close()
	check("zip", zs, "a/b.txt", 5)

	ts, err := OpenArchive("tar", tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	check("tar", ts, "c.txt", 3)

	check("single", NewSingleFileSource(single), "", 7)
}
