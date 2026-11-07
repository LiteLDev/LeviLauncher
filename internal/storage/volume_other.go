//go:build !windows

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func sameVolume(a, b string) bool {
	da, ok1 := deviceOf(a)
	db, ok2 := deviceOf(b)
	return ok1 && ok2 && da == db
}

func deviceOf(p string) (uint64, bool) {
	cur := filepath.Clean(p)
	for {
		if info, err := os.Stat(cur); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				return uint64(st.Dev), true
			}
			return 0, false
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return 0, false
		}
		cur = parent
	}
}

func isTransient(error) bool { return false }

func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }

func secureDir(dir string) { _ = os.Chmod(dir, 0o700) }

func lockFileNow(f *os.File, exclusive bool) bool {
	how := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		how = syscall.LOCK_EX | syscall.LOCK_NB
	}
	return syscall.Flock(int(f.Fd()), how) == nil
}

func lockFileShared(f *os.File) bool { return syscall.Flock(int(f.Fd()), syscall.LOCK_SH) == nil }

func unlockFile(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
}
