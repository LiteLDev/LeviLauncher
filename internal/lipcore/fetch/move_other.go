//go:build !windows

package fetch

import (
	"errors"
	"fmt"
	"io"
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// newGUID returns a random version-4 GUID in .NET's "D" format.
func newGUID() string {
	b := newGUIDBytes()
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
