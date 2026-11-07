//go:build windows

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// sameVolume reports whether a and b, or their nearest existing ancestors,
// are on one volume, so that a rename between them is possible.
func sameVolume(a, b string) bool {
	va, vb := volumeOf(a), volumeOf(b)
	return va != "" && strings.EqualFold(va, vb)
}

func volumeOf(p string) string {
	cur := filepath.Clean(p)
	for {
		if _, err := os.Stat(cur); err == nil {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
	ptr, err := windows.UTF16PtrFromString(cur)
	if err != nil {
		return ""
	}
	buf := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(ptr, &buf[0], uint32(len(buf))); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// isTransient reports an error that another process holding a file causes.
func isTransient(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
