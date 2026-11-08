//go:build windows

package archive

import (
	"path/filepath"
	"syscall"
	"testing"
)

// makeSpecialFiles: Windows has no FIFOs or socket files (shortTempDir
// skips the tests first).
func makeSpecialFiles(t *testing.T, _, _ string) { t.Skip("Windows has no FIFOs or socket files.") }

// shortName is the 8.3 short name of an existing file, or "" when it cannot
// be read.
func shortName(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	buf := make([]uint16, len(path)+260)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) >= len(buf) {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:n]))
}
