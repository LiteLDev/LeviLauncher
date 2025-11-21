//go:build windows

package peeditor

import (
    "context"
    "os/exec"
    "path/filepath"
    "strings"
    "github.com/wailsapp/wails/v3/pkg/application"
    "github.com/liteldev/LeviLauncher/internal/procutil"
)

func RunForVersion(ctx context.Context, versionDir string) bool {
    dir := strings.TrimSpace(versionDir)
    if dir == "" { application.Get().Event.Emit(EventEnsureDone, false); return false }
    exe := filepath.Join(dir, "Minecraft.Windows.exe")
    tool := filepath.Join(dir, "PeEditor.exe")
    bak := filepath.Join(dir, "Minecraft.Windows.exe.bak")
    if fileExists(bak) { return true }
    if !fileExists(tool) || !fileExists(exe) { application.Get().Event.Emit(EventEnsureDone, false); return false }
    application.Get().Event.Emit(EventEnsureStart, struct{}{})
    cmd := exec.Command(tool, "-m", "-b", "--inplace", "--exe", "./Minecraft.Windows.exe")
    cmd.Dir = dir
    procutil.NoWindow(cmd)
    _ = cmd.Run()
    application.Get().Event.Emit(EventEnsureDone, true)
    return true
}