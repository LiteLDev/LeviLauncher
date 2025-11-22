//go:build linux

package winegdk

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

func FindWineBin() string {
    base := strings.TrimSpace(utils.GetLauncherConfigRoot())
    if base != "" {
        alt := filepath.Join(base, "wine", "files", "bin", "wine64")
        if fi, err := os.Stat(alt); err == nil && fi.Mode().IsRegular() {
            return alt
        }
        p := filepath.Join(base, "wine", "files", "bin", "wine")
        if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
            return p
        }
    }
    return ""
}
