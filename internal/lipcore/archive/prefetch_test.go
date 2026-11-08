package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// Ports of the tests ZipSourceTests.cs gained with extracting ZIP archives
// before hook commands run (lip cfaa7a1).

const localHeaderSignature = 0x04034b50

// storedZip is a ZIP archive of uncompressed (stored) entries.
func storedZip(t *testing.T, pairs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i+1 < len(pairs); i += 2 {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: pairs[i], Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(pairs[i+1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// setCompressionMethod sets the compression method of every entry, in the
// local and the central headers.
func setCompressionMethod(z []byte, method uint16) {
	for i := 0; i+4 <= len(z); i++ {
		switch binary.LittleEndian.Uint32(z[i:]) {
		case localHeaderSignature:
			binary.LittleEndian.PutUint16(z[i+8:], method)
		case 0x02014b50:
			binary.LittleEndian.PutUint16(z[i+10:], method)
		}
	}
}

// corruptData damages the data of a Deflate-compressed entry: its first
// block gets the reserved block type, which the archive's directory does not
// reveal, only extracting the entry.
func corruptData(t *testing.T, z []byte, name string) {
	t.Helper()
	for i := 0; i+30 <= len(z); i++ {
		if binary.LittleEndian.Uint32(z[i:]) == localHeaderSignature &&
			int(binary.LittleEndian.Uint16(z[i+26:])) == len(name) && string(z[i+30:i+30+len(name)]) == name {
			extra := int(binary.LittleEndian.Uint16(z[i+28:]))
			z[i+30+len(name)+extra] = 0xFF
			return
		}
	}
	t.Fatalf("no local header for %s", name)
}

func TestPrefetchValidEntriesOpenServesThem(t *testing.T) {
	s, err := NewZipBytesSource(storedZip(t, "a.txt", "a", "sub/b.txt", "b"), "the ZIP archive")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Prefetch(context.Background(), []string{"a.txt", "sub/b.txt"}); err != nil {
		t.Fatal(err)
	}

	if readKey(t, s, "a.txt") != "a" || readKey(t, s, "sub/b.txt") != "b" {
		t.Fatal("content")
	}
}

func TestPrefetchUnsupportedCompressionMethodFails(t *testing.T) {
	// 14 is LZMA, which lip does not support.
	z := storedZip(t, "a.txt", "a")
	setCompressionMethod(z, 14)
	s, err := NewCachedZipSource(z, "the download of https://example.com/a.zip")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Prefetch(context.Background(), []string{"a.txt"})

	var de *DamagedError
	if !errors.As(err, &de) || !strings.Contains(de.Message, "'a.txt'") ||
		!strings.Contains(de.Message, "lip cache clean") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrefetchDamagedEntryFails(t *testing.T) {
	z := makeZip(t, file("a.txt", strings.Repeat("content ", 100)))
	corruptData(t, z, "a.txt")
	s, err := NewZipBytesSource(z, "the ZIP archive")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Prefetch(context.Background(), []string{"a.txt"})

	var de *DamagedError
	if !errors.As(err, &de) || !strings.Contains(de.Message, "'a.txt'") {
		t.Fatalf("err = %v", err)
	}
}
