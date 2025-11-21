//go:build linux

package explorer

import (
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "github.com/liteldev/LeviLauncher/internal/utils"
)

func OpenPath(dir string) bool {
    d := strings.TrimSpace(dir)
    if d == "" {
        return false
    }
    if !utils.DirExists(d) {
        if err := os.MkdirAll(d, 0755); err != nil {
            return false
        }
    }
    cmd := exec.Command("xdg-open", d)
    if err := cmd.Run(); err != nil {
        log.Println("explorer.OpenPath error:", err)
        return false
    }
    return true
}

func SelectFile(path string) bool {
    p := strings.TrimSpace(path)
    if p == "" || !utils.FileExists(p) {
        return false
    }
    dir := filepath.Dir(p)
    cmd := exec.Command("xdg-open", dir)
    if err := cmd.Run(); err != nil {
        log.Println("explorer.SelectFile error:", err)
        return false
    }
    return true
}

func OpenMods(name string) bool {
    n := strings.TrimSpace(name)
    if n == "" {
        return false
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return false
    }
    dir := filepath.Join(vdir, n, "mods")
    return OpenPath(dir)
}

func OpenWorlds(isPreview bool) bool {
    dir := filepath.Join(utils.GetMinecraftGDKDataPath(isPreview), "worlds")
    return OpenPath(dir)
}

func OpenInstallers() bool {
    dir, err := utils.GetInstallerDir()
    if err != nil || dir == "" {
        return false
    }
    return OpenPath(dir)
}