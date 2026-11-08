//go:build windows

package install

import (
	"path/filepath"
	"syscall"
)

// shortName is the 8.3 short name of an existing file or directory, or ""
// when it cannot be read.
func shortName(path string) string {
	query := path
	if len(path) >= 260 {
		query = `\\?\` + path
	}
	p, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		return ""
	}
	buf := make([]uint16, len(query)+260)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) >= len(buf) {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:n]))
}
