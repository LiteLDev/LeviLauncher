//go:build linux

package mcservice

import (
    "os"
    "path/filepath"
    "strings"
)

func IsProcessRunningAtPath(exePath string) bool {
    target := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
    if target == "" { return false }
    entries, _ := os.ReadDir("/proc")
    for _, e := range entries {
        if !e.IsDir() { continue }
        name := e.Name()
        isNum := true
        for i := 0; i < len(name); i++ { if name[i] < '0' || name[i] > '9' { isNum = false; break } }
        if !isNum { continue }
        exe := filepath.Join("/proc", e.Name(), "exe")
        p, err := os.Readlink(exe)
        if err != nil || strings.TrimSpace(p) == "" { continue }
        lp := strings.ToLower(filepath.Clean(p))
        if lp == target { return true }
    }
    return false
}

func CreateDesktopShortcut(name string) string {
    n := strings.TrimSpace(name)
    if n == "" { return "ERR_NAME_REQUIRED" }
    exePath, _ := os.Executable()
    exePath = strings.TrimSpace(exePath)
    if exePath == "" { return "ERR_SHORTCUT_CREATE_FAILED" }
    home, _ := os.UserHomeDir()
    if strings.TrimSpace(home) == "" { return "ERR_SHORTCUT_CREATE_FAILED" }
    desktop := filepath.Join(home, "Desktop")
    if fi, err := os.Stat(desktop); err != nil || !fi.IsDir() { return "ERR_SHORTCUT_CREATE_FAILED" }
    safeName := n
    lnk := filepath.Join(desktop, "Minecraft - "+safeName+".desktop")
    iconPath := exePath
    content := "[Desktop Entry]\nType=Application\nName=Minecraft - " + safeName + "\nExec=\"" + exePath + "\" --launch=\"" + n + "\"\nIcon=\"" + iconPath + "\"\nTerminal=false\n"
    if err := os.WriteFile(lnk, []byte(content), 0644); err != nil { return "ERR_SHORTCUT_CREATE_FAILED" }
    return ""
}