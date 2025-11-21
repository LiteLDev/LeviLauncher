package mcservice

import (
    "os"
    "strings"
    "golang.org/x/sys/windows"
)

func ListDrives() []string {
    drives := []string{}
    for l := 'A'; l <= 'Z'; l++ {
        root := string(l) + ":\\"
        if fi, err := os.Stat(root); err == nil && fi.IsDir() {
            drives = append(drives, root)
        }
    }
    return drives
}

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