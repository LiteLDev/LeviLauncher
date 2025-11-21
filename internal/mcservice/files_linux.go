package mcservice

import (
    "bufio"
    "os"
    "strings"
    "syscall"
)

func ListDrives() []string {
    mounts := []string{}
    f, err := os.Open("/proc/mounts")
    if err != nil {
        return []string{"/"}
    }
    defer f.Close()
    seen := map[string]struct{}{}
    sc := bufio.NewScanner(f)
    for sc.Scan() {
        line := strings.TrimSpace(sc.Text())
        if line == "" { continue }
        parts := strings.Fields(line)
        if len(parts) < 2 { continue }
        m := parts[1]
        if m == "/" { seen[m] = struct{}{}; continue }
        if _, ok := seen[m]; ok { continue }
        seen[m] = struct{}{}
        mounts = append(mounts, m)
    }
    if len(mounts) == 0 {
        return []string{"/"}
    }
    return mounts
}

func GetDriveStats(root string) map[string]uint64 {
    res := map[string]uint64{"total": 0, "free": 0}
    r := strings.TrimSpace(root)
    if r == "" { r = "/" }
    var st syscall.Statfs_t
    if err := syscall.Statfs(r, &st); err != nil {
        return res
    }
    total := uint64(st.Blocks) * uint64(st.Bsize)
    free := uint64(st.Bavail) * uint64(st.Bsize)
    res["total"] = total
    res["free"] = free
    return res
}