//go:build !windows

package storage

import (
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
