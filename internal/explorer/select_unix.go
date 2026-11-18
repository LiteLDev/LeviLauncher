//go:build darwin

package explorer

import (
	"os/exec"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

func SelectFile(path string) bool {
	if !utils.FileExists(path) {
		return false
	}
	return exec.Command("/usr/bin/open", "-R", path).Run() == nil
}
