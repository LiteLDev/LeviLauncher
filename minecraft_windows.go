//go:build windows

package main

import (
    "path/filepath"
    "strings"
    "unsafe"
    "golang.org/x/sys/windows"
)

func isProcessRunningAtPath(exePath string) bool {
    p := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
    if p == "" { return false }
    snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
    if err != nil { return false }
    defer windows.CloseHandle(snap)
    var pe windows.ProcessEntry32
    pe.Size = uint32(unsafe.Sizeof(pe))
    if err := windows.Process32First(snap, &pe); err != nil { return false }
    for {
        pid := pe.ProcessID
        h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
        if err == nil {
            buf := make([]uint16, 1024)
            size := uint32(len(buf))
            if e := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e == nil && size > 0 {
                path := windows.UTF16ToString(buf[:size])
                _ = windows.CloseHandle(h)
                norm := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
                norm = strings.TrimPrefix(norm, `\\?\`)
                norm = strings.TrimPrefix(norm, `\??\`)
                if norm == p { return true }
            } else { _ = windows.CloseHandle(h) }
        }
        if err := windows.Process32Next(snap, &pe); err != nil { break }
    }
    return false
}