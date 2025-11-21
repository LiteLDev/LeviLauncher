//go:build windows

package mcservice

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "unsafe"
    "github.com/liteldev/LeviLauncher/internal/icons"
    "github.com/liteldev/LeviLauncher/internal/utils"
    "github.com/liteldev/LeviLauncher/internal/versions"
    "golang.org/x/sys/windows"
)

func IsProcessRunningAtPath(exePath string) bool {
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

func CreateDesktopShortcut(name string) string {
    n := strings.TrimSpace(name)
    if n == "" { return "ERR_NAME_REQUIRED" }
    exePath, _ := os.Executable()
    exePath = strings.TrimSpace(exePath)
    if exePath == "" { return "ERR_SHORTCUT_CREATE_FAILED" }
    home, _ := os.UserHomeDir()
    if strings.TrimSpace(home) == "" { home = os.Getenv("USERPROFILE") }
    if strings.TrimSpace(home) == "" { return "ERR_SHORTCUT_CREATE_FAILED" }
    desktop := filepath.Join(home, "Desktop")
    if fi, err := os.Stat(desktop); err != nil || !fi.IsDir() { return "ERR_SHORTCUT_CREATE_FAILED" }
    safeName := n
    lnk := filepath.Join(desktop, "Minecraft - "+safeName+".lnk")
    args := "--launch=" + n
    workdir := filepath.Dir(exePath)
    iconPath := exePath
    if vdir, err := utils.GetVersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
        dir := filepath.Join(vdir, n)
        e := filepath.Join(dir, "Minecraft.Windows.exe")
        isPreview := false
        if m, er := versions.ReadMeta(dir); er == nil {
            isPreview = strings.EqualFold(strings.TrimSpace(m.Type), "preview")
        }
        if p := icons.EnsureVersionIcon(dir, isPreview); strings.TrimSpace(p) != "" {
            lp := strings.ToLower(p)
            if strings.HasSuffix(lp, ".ico") || strings.HasSuffix(lp, ".exe") { iconPath = p } else { if utils.FileExists(e) { iconPath = e } }
        } else if utils.FileExists(e) { iconPath = e }
    }
    esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
    script := "$WshShell = New-Object -ComObject WScript.Shell; " +
        "$Shortcut = $WshShell.CreateShortcut('" + esc(lnk) + "'); " +
        "$Shortcut.TargetPath = '" + esc(exePath) + "'; " +
        "$Shortcut.Arguments = '" + esc(args) + "'; " +
        "$Shortcut.WorkingDirectory = '" + esc(workdir) + "'; " +
        "$Shortcut.IconLocation = '" + esc(iconPath) + "'; " +
        "$Shortcut.Save()"
    cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
    if err := cmd.Run(); err != nil { return "ERR_SHORTCUT_CREATE_FAILED" }
    return ""
}