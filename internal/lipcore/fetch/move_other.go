//go:build !windows

package fetch

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// moveNoReplace moves src to dst, failing with fs.ErrExist when dst exists,
// like .NET File.Move(src, dst, overwrite: false) on Unix (link + unlink,
// copying across file systems).
func moveNoReplace(src, dst string) error {
	err := os.Link(src, dst)
	if err == nil {
		os.Remove(src)
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return fs.ErrExist
	}
	if !errors.Is(err, syscall.EXDEV) && !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.ENOTSUP) {
		return err
	}
	// Cross-device (or no hard links): copy next to dst, then link.
	staged := dst + "." + newGUID() + ".tmp"
	if cerr := copyFile(src, staged); cerr != nil {
		os.Remove(staged)
		return cerr
	}
	defer os.Remove(staged)
	if lerr := os.Link(staged, dst); lerr != nil {
		if errors.Is(lerr, fs.ErrExist) {
			return fs.ErrExist
		}
		return lerr
	}
	os.Remove(src)
	return nil
}
