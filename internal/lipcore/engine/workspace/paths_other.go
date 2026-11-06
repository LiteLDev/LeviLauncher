//go:build !windows

package workspace

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// getFullPath ports Path.GetFullPath on Unix with base as the working
// directory: combine, collapse "//", "." and ".."; a trailing separator is
// kept.
func getFullPath(base, p string) string {
	in := p
	if !strings.HasPrefix(p, "/") {
		in = strings.TrimRight(base, "/") + "/" + p
	}
	full := path.Clean(in)
	if strings.HasSuffix(in, "/") && full != "/" {
		full += "/"
	}
	return full
}

// expandShortName returns p: there are no 8.3 short names on Unix.
func expandShortName(p string) string { return p }

// fileExists ports File.Exists: the path exists (dangling symlinks
// included) and is not a directory.
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		_, lerr := os.Lstat(p)
		return lerr == nil
	}
	return !fi.IsDir()
}

// dirExists ports Directory.Exists.
func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// deleteFile ports File.Delete: a missing file is not an error; a
// directory is.
func deleteFile(p string) error {
	err := syscall.Unlink(p)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.ENOENT):
		return nil
	case errors.Is(err, syscall.EISDIR), errors.Is(err, syscall.EPERM) && dirExists(p):
		return &os.PathError{Op: "remove", Path: p, Err: fmt.Errorf("Access to the path '%s' is denied.: %w", p, err)}
	}
	return &os.PathError{Op: "remove", Path: p, Err: err}
}

// isLink reports whether p exists and whether it is a symbolic link,
// without following it.
func isLink(p string) (link, exists bool) {
	fi, err := os.Lstat(p)
	if err != nil {
		return false, false
	}
	return fi.Mode()&os.ModeSymlink != 0, true
}

// realPath returns the existing p with every symbolic link resolved.
func realPath(p string) (string, error) {
	return filepath.EvalSymlinks(p)
}
