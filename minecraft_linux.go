//go:build linux

package main

import (
    "os"
    "path/filepath"
    "strings"
)

func isProcessRunningAtPath(exePath string) bool {
    target := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
    if target == "" { return false }
    entries, _ := os.ReadDir("/proc")
    for _, e := range entries {
        if !e.IsDir() { continue }
        name := e.Name()
        for i := 0; i < len(name); i++ { if name[i] < '0' || name[i] > '9' { name = ""; break } }
        if name == "" { continue }
        exe := filepath.Join("/proc", e.Name(), "exe")
        p, err := os.Readlink(exe)
        if err != nil || strings.TrimSpace(p) == "" { continue }
        lp := strings.ToLower(filepath.Clean(p))
        if lp == target { return true }
    }
    return false
}