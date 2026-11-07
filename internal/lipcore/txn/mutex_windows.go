//go:build windows

package txn

import (
	"errors"
	"os"
	"syscall"
)

// acquire opens the lock file without sharing (FileMode.OpenOrCreate,
// FileShare.None). Only a sharing or lock violation (HRESULT 32 or 33) means
// another process holds it.
func acquire(path string) (*os.File, bool, error) {
	p, err := syscall.UTF16PtrFromString(longPath(path))
	if err != nil {
		return nil, false, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		var errno syscall.Errno
		busy := errors.As(err, &errno) && (errno == 32 || errno == 33)
		return nil, busy, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), false, nil
}
