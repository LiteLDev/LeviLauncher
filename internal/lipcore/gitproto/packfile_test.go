package gitproto

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteOnCloseTemp(t *testing.T) {
	f, err := createDeleteOnCloseFile(t.TempDir(), "lip-", ".pack")
	if err != nil {
		t.Fatal(err)
	}
	p := &PackFile{f: f}
	if _, err := f.Write([]byte("PACK")); err != nil {
		t.Fatal(err)
	}
	if err := p.finish(); err != nil || p.Size() != 4 {
		t.Fatalf("size %d: %v", p.Size(), err)
	}
	var b [4]byte
	if _, err := p.ReadAt(b[:], 0); err != nil || string(b[:]) != "PACK" {
		t.Fatalf("ReadAt %q: %v", b, err)
	}
	if !strings.HasPrefix(filepath.Base(f.Name()), "lip-") || filepath.Ext(f.Name()) != ".pack" {
		t.Errorf("name %s", f.Name())
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pack still exists: %v", err)
	}
}
