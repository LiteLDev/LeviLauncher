package explorer

import (
	"log"

	"os/exec"

	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

func SelectFile(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" || !utils.FileExists(p) {
		return false
	}
	cmd := exec.Command("explorer", "/select,\""+p+"\"")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		log.Println("explorer.SelectFile error:", err)
		return false
	}
	return true
}
