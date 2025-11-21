//go:build linux

package winegdk

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "github.com/liteldev/LeviLauncher/internal/utils"
)

func FindWineBin() string {
    app := strings.TrimSpace(utils.GetAppDataPath())
    if app != "" {
        p := filepath.Join(app, "wine", "files", "bin", "wine")
        if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() { return p }
        wow := filepath.Join(app, "wine", "files", "bin-wow64", "wine")
        if fi, err := os.Stat(wow); err == nil && fi.Mode().IsRegular() { return wow }
        alt := filepath.Join(app, "wine", "files", "bin", "wine64")
        if fi, err := os.Stat(alt); err == nil && fi.Mode().IsRegular() { return alt }
    }
    if sys, err := exec.LookPath("wine"); err == nil { return sys }
    return ""
}