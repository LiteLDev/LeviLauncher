//go:build darwin

package downloader

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func lockDownload(dest string) (func(), error) {
	path := dest + ".lock"
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			f.Close()
			if errors.Is(err, unix.EWOULDBLOCK) {
				return nil, ErrBusy
			}
			return nil, err
		}
		// A previous owner may have removed this inode after we opened it.
		opened, err1 := f.Stat()
		current, err2 := os.Stat(path)
		if err1 != nil || err2 != nil || !os.SameFile(opened, current) {
			unix.Flock(int(f.Fd()), unix.LOCK_UN)
			f.Close()
			continue
		}
		return func() { os.Remove(path); unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
	}
}
