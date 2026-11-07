//go:build windows

package fsx

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// A staging file is renamed into place with its attributes, so it must not
// be hidden.
func TestWriteFileAtomicResultIsNotHidden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.json")
	if err := WriteFileAtomic(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(path)
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		t.Fatal(err)
	}
	if attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0 {
		t.Fatal("the written file is hidden")
	}
}
