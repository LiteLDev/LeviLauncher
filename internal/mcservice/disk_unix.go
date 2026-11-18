//go:build darwin

package mcservice

import (
	"golang.org/x/sys/unix"
)

func GetDriveStats(root string) map[string]uint64 {
	r := map[string]uint64{"total": 0, "free": 0}
	var s unix.Statfs_t
	if unix.Statfs(root, &s) == nil {
		r["total"] = s.Blocks * uint64(s.Bsize)
		r["free"] = s.Bavail * uint64(s.Bsize)
	}
	return r
}
