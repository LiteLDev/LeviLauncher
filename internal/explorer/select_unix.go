//go:build darwin || linux

package explorer

import (
	"github.com/liteldev/LeviLauncher/internal/utils"
	"os/exec"
	"path/filepath"
	"runtime"
)

func SelectFile(path string) bool {
	if !utils.FileExists(path) {
		return false
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("/usr/bin/open", "-R", path)
	} else {
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	return cmd.Run() == nil
}
