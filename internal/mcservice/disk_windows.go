package mcservice

import (
	"strings"

	"golang.org/x/sys/windows"
)

func GetDriveStats(root string) map[string]uint64 {
	res := map[string]uint64{"total": 0, "free": 0}
	r := strings.TrimSpace(root)
	if r == "" {
		return res
	}
	if !strings.HasSuffix(r, "\\") {
		r += "\\"
	}
	p, err := windows.UTF16PtrFromString(r)
	if err != nil {
		return res
	}
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err == nil {
		res["total"] = totalNumberOfBytes
		res["free"] = totalNumberOfFreeBytes
	}
	return res
}
