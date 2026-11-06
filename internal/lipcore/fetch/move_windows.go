//go:build windows

package fetch

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

// moveNoReplace moves src to dst, failing with fs.ErrExist when dst exists,
// like .NET File.Move(src, dst, overwrite: false) (MoveFileEx with
// MOVEFILE_COPY_ALLOWED and without MOVEFILE_REPLACE_EXISTING).
func moveNoReplace(src, dst string) error {
	from, err := windows.UTF16PtrFromString(fixLongPath(src))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(fixLongPath(dst))
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_COPY_ALLOWED)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return fs.ErrExist
	}
	if err != nil {
		return &os.LinkError{Op: "move", Old: src, New: dst, Err: err}
	}
	return nil
}

// fixLongPath prefixes long absolute paths with \\?\ so MoveFileEx accepts
// them (os does this internally for its own calls).
func fixLongPath(p string) string {
	if len(p) < 248 || len(p) >= 2 && p[0] == '\\' && p[1] == '\\' {
		return p
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		b := []byte(p)
		for i := range b {
			if b[i] == '/' {
				b[i] = '\\'
			}
		}
		return `\\?\` + string(b)
	}
	return p
}
