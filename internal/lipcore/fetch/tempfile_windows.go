//go:build windows

package fetch

import (
	"os"

	"golang.org/x/sys/windows"
)

// createDeleteOnClose creates path (failing if it exists) for reading and
// writing, unshared, deleted when its handle is closed: .NET FileStream
// with FileMode.CreateNew, FileShare.None and FileOptions.DeleteOnClose.
func createDeleteOnClose(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(fixLongPath(path))
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p,
		windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE,
		0, nil, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
