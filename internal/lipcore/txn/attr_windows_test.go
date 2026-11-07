//go:build windows

package txn

import (
	"syscall"
	"testing"
)

func setAttributes(t *testing.T, path string, attrs uint32) {
	t.Helper()
	p, _ := syscall.UTF16PtrFromString(path)
	if err := syscall.SetFileAttributes(p, attrs); err != nil {
		t.Fatal(err)
	}
}

func makeReadOnly(t *testing.T, path string) {
	setAttributes(t, path, syscall.FILE_ATTRIBUTE_READONLY)
}

func makeHidden(t *testing.T, path string) {
	setAttributes(t, path, syscall.FILE_ATTRIBUTE_HIDDEN)
}

func makeNormal(t *testing.T, path string) {
	setAttributes(t, path, syscall.FILE_ATTRIBUTE_NORMAL)
}

// attrString lists the hidden and read-only attributes of a file.
func attrString(t *testing.T, path string) string {
	t.Helper()
	a, _, err := attributes(path)
	if err != nil {
		t.Fatal(err)
	}
	s := ""
	if a&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		s += "R"
	}
	if a&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		s += "H"
	}
	return s
}

const hiddenBackups = true
