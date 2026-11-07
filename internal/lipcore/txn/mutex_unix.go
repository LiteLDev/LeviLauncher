//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package txn

import (
	"errors"
	"os"
	"syscall"
)

// acquire opens the lock file and takes an exclusive flock on it, as .NET
// does for FileShare.None. Every failure but a missing directory or a denied
// access counts as another process holding it (WorkspaceMutex.IsLocked).
func acquire(path string) (*os.File, bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, !notFound(err) && !denied(err), err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, true, &os.PathError{Op: "flock", Path: path, Err: err}
	}
	return f, false, nil
}

// denied is UnauthorizedAccessException.
func denied(err error) bool {
	return errors.Is(err, os.ErrPermission)
}
