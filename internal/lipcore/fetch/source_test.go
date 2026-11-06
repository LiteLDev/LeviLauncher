package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func gzipBytes(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Name = name
	zw.Write(data)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func tarBytes(t *testing.T, entries []tar.Header, contents []string) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for i, h := range entries {
		c := contents[i]
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(c))
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write([]byte(c))
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// probeTar mirrors an archive whose keys were listed with SharpCompress
// 0.50.5 ArchiveFactory.OpenArchive (lip ArchiveSource).
func probeTar(t *testing.T) []byte {
	long := "dir/" + strings.Repeat("x", 120) + ".txt"
	gnu := "gnu/" + strings.Repeat("y", 120) + ".txt"
	return tarBytes(t, []tar.Header{
		{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "./a.txt"},
		{Name: "plain.txt"},
		{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: long, Format: tar.FormatPAX},
		{Name: gnu, Format: tar.FormatGNU},
		{Name: "ünï.txt", Format: tar.FormatPAX},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "a.txt", Mode: 0o777},
		{Name: "hard", Typeflag: tar.TypeLink, Linkname: "plain.txt"},
		{Name: "./dot/./x.txt"},
	}, []string{"", "A", "P", "", "PAXLONG", "GNULONG", "U", "", "", "X"})
}

var probeTarWant = []struct{ key, content string }{
	{"./a.txt", "A"},
	{"plain.txt", "P"},
	{"dir/" + strings.Repeat("x", 120) + ".txt", "PAXLONG"},
	{"gnu/" + strings.Repeat("y", 120) + ".txt", "GNULONG"},
	{"ünï.txt", "U"},
	{"link", ""},
	{"hard", ""},
	{"./dot/./x.txt", "X"},
}

func checkSource(t *testing.T, s AssetSource, want []struct{ key, content string }) {
	t.Helper()
	keys := s.Keys()
	var wantKeys []string
	for _, w := range want {
		wantKeys = append(wantKeys, w.key)
	}
	if !equal(keys, wantKeys) {
		t.Fatalf("keys = %q\nwant   %q", keys, wantKeys)
	}
	for _, w := range want {
		rc, err := s.Open(w.key)
		if err != nil {
			t.Fatalf("Open(%q): %v", w.key, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %q: %v", w.key, err)
		}
		if string(b) != w.content {
			t.Errorf("content of %q = %q, want %q", w.key, b, w.content)
		}
	}
	if _, err := s.Open("does/not/exist"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Open(missing) err = %v", err)
	}
	if err := s.Prefetch(context.Background(), s.Keys()); err != nil {
		t.Errorf("Prefetch: %v", err)
	}
}

func TestZipSource(t *testing.T) {
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	for _, n := range []string{"dir/", "dir/a.txt", "./dot.txt", "back\\slash.txt", "ünï.txt", "empty/", "dir/sub/b.bin", "dup.txt", "dup.txt"} {
		f, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(n, "/") {
			f.Write([]byte("content of " + n))
		}
	}
	zw.Close()
	// The asset type does not matter; the format is sniffed.
	for _, typ := range []string{"zip", "tar", "tgz"} {
		s, err := OpenArchive(typ, writeTemp(t, "a.zip", zb.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		checkSource(t, s, []struct{ key, content string }{
			{"dir/a.txt", "content of dir/a.txt"},
			{"./dot.txt", "content of ./dot.txt"},
			{"back\\slash.txt", "content of back\\slash.txt"},
			{"ünï.txt", "content of ünï.txt"},
			{"dir/sub/b.bin", "content of dir/sub/b.bin"},
			{"dup.txt", "content of dup.txt"},
			{"dup.txt", "content of dup.txt"},
		})
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}
}

func TestTarSource(t *testing.T) {
	s, err := OpenArchive("tar", writeTemp(t, "a.tar", probeTar(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, probeTarWant)
}

func TestTgzSource(t *testing.T) {
	s, err := OpenArchive("tgz", writeTemp(t, "a.tgz", gzipBytes(t, probeTar(t), "")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, probeTarWant)
}

func TestTarPAXGlobalHeaderSkipped(t *testing.T) {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "abc"}})
	tw.WriteHeader(&tar.Header{Name: "proj/", Typeflag: tar.TypeDir, Mode: 0o755})
	tw.WriteHeader(&tar.Header{Name: "proj/f.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
	tw.Write([]byte("F"))
	tw.Close()
	for name, data := range map[string][]byte{"g.tar": b.Bytes(), "g.tgz": gzipBytes(t, b.Bytes(), "")} {
		s, err := OpenArchive("tgz", writeTemp(t, name, data))
		if err != nil {
			t.Fatal(err)
		}
		checkSource(t, s, []struct{ key, content string }{{"proj/f.txt", "F"}})
		s.Close()
	}
}

func TestTgzTempRemovedOnClose(t *testing.T) {
	s, err := OpenArchive("tgz", writeTemp(t, "a.tgz", gzipBytes(t, probeTar(t), "")))
	if err != nil {
		t.Fatal(err)
	}
	tmp := s.(*tarSource).path
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("temp tar missing: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp tar not removed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestTarSparseEntry(t *testing.T) {
	// A PAX 0.1 sparse file (archive/tar cannot write one): map "0,1,4,1"
	// (data at offsets 0 and 4), real size 5.
	var b bytes.Buffer
	pax := paxRecord("GNU.sparse.name", "sp") + paxRecord("GNU.sparse.size", "5") +
		paxRecord("GNU.sparse.numblocks", "2") + paxRecord("GNU.sparse.map", "0,1,4,1")
	b.Write(rawTarHeader("PaxHeaders/sp", 'x', len(pax)))
	b.Write(padBlock([]byte(pax)))
	b.Write(rawTarHeader("GNUSparseFile.0/sp", '0', 2))
	b.Write(padBlock([]byte("ab")))
	b.Write(rawTarHeader("after.txt", '0', 1))
	b.Write(padBlock([]byte("Z")))
	b.Write(make([]byte, 1024))
	s, err := OpenArchive("tar", writeTemp(t, "sp.tar", b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{{"sp", "a\x00\x00\x00b"}, {"after.txt", "Z"}})
}

func paxRecord(k, v string) string {
	rec := " " + k + "=" + v + "\n"
	n := len(rec)
	for len(strconv.Itoa(n))+len(rec) != n {
		n = len(strconv.Itoa(n)) + len(rec)
	}
	return strconv.Itoa(n) + rec
}

func padBlock(b []byte) []byte {
	return append(b, make([]byte, (512-len(b)%512)%512)...)
}

// rawTarHeader builds a ustar header block by hand.
func rawTarHeader(name string, typ byte, size int) []byte {
	h := make([]byte, 512)
	copy(h[0:], name)
	copy(h[100:], "0000644\x00")
	copy(h[108:], "0000000\x00")
	copy(h[116:], "0000000\x00")
	copy(h[124:], fmt.Sprintf("%011o\x00", size))
	copy(h[136:], "00000000000\x00")
	h[156] = typ
	copy(h[257:], "ustar\x0000")
	copy(h[148:], "        ")
	sum := 0
	for _, c := range h {
		sum += int(c)
	}
	copy(h[148:], fmt.Sprintf("%06o\x00 ", sum))
	return h
}

func TestDirectorySourceLinkCycle(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	os.WriteFile(filepath.Join(d, "sub", "f.txt"), []byte("f"), 0o644)
	link := filepath.Join(d, "sub", "loop")
	if err := os.Symlink(d, link); err != nil {
		// Unprivileged Windows: use a directory junction instead.
		if runtime.GOOS != "windows" {
			t.Skipf("cannot create symlink: %v", err)
		}
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, d).CombinedOutput(); err != nil {
			t.Skipf("cannot create junction: %v %s", err, out)
		}
	}
	keys := NewDirectorySource(d).Keys()
	if !equal(keys, []string{"sub/f.txt"}) {
		t.Errorf("keys = %q", keys)
	}
}

func TestPlainGzipSource(t *testing.T) {
	text := "just text, not a tar archive at all"
	s, err := OpenArchive("tgz", writeTemp(t, "p.gz", gzipBytes(t, []byte(text), "inner.txt")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{{"inner.txt", text}})
}

func TestSingleFileSource(t *testing.T) {
	s, err := OpenArchive("uncompressed", writeTemp(t, "x.dll", []byte("MZ")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{{"", "MZ"}})
}

func TestOpenArchiveErrors(t *testing.T) {
	if _, err := OpenArchive("zip", writeTemp(t, "junk", []byte("not an archive"))); err == nil {
		t.Error("junk opened as archive")
	}
	if _, err := OpenArchive("self", writeTemp(t, "x", nil)); err == nil {
		t.Error("unsupported type accepted")
	}
	if _, err := OpenArchive("zip", filepath.Join(t.TempDir(), "missing.zip")); err == nil {
		t.Error("missing file opened")
	}
}

func TestDirectoryAndCompositeSource(t *testing.T) {
	d := t.TempDir()
	for p, c := range map[string]string{
		"b.txt":         "b",
		"a/x.txt":       "x",
		"a/.git":        "gitfile",
		".git/config":   "cfg",
		"a/c/deep.txt":  "deep",
		"z.txt":         "z",
		"a/b/.gitkeep":  "",
		"a/b/inner.bin": "in",
	} {
		full := filepath.Join(d, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	ds := NewDirectorySource(d)
	checkSource(t, ds, []struct{ key, content string }{
		{"b.txt", "b"},
		{"z.txt", "z"},
		{"a/x.txt", "x"},
		{"a/b/.gitkeep", ""},
		{"a/b/inner.bin", "in"},
		{"a/c/deep.txt", "deep"},
	})
	for _, bad := range []string{".git/config", "a/../b.txt", "./b.txt", "a//x.txt", "a/.git", "a"} {
		if _, err := ds.Open(bad); err == nil {
			t.Errorf("Open(%q) succeeded", bad)
		}
	}
	if rc, err := ds.Open("a\\x.txt"); err != nil {
		t.Errorf("backslash key: %v", err)
	} else {
		rc.Close()
	}

	single := NewSingleFileSource(filepath.Join(d, "b.txt"))
	other := NewDirectorySource(filepath.Join(d, "a"))
	cs := NewCompositeSource(other, ds, single)
	keys := cs.Keys()
	want := []string{"x.txt", "b/.gitkeep", "b/inner.bin", "c/deep.txt", "b.txt", "z.txt", "a/x.txt", "a/b/.gitkeep", "a/b/inner.bin", "a/c/deep.txt", ""}
	if !equal(keys, want) {
		t.Errorf("composite keys = %q\nwant %q", keys, want)
	}
	rc, err := cs.Open("")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "b" {
		t.Errorf("content = %q", b)
	}
	if err := cs.Prefetch(context.Background(), []string{"x.txt", "nope"}); err != nil {
		t.Error(err)
	}
	if err := cs.Close(); err != nil {
		t.Error(err)
	}
}
