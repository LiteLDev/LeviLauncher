//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package txn

import (
	"errors"
	"os"
)

// acquire opens the lock file; this platform has no flock, so the lock is
// not enforced between processes.
func acquire(path string) (*os.File, bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, !notFound(err) && !errors.Is(err, os.ErrPermission), err
	}
	return f, false, nil
}
