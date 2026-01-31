package explorer

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/apppath"
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
	cmd := exec.Command("powershell", "explorer \""+d+"\"")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
	cmd := exec.Command("explorer", "/select,\""+p+"\"")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
	vdir, err := apppath.VersionsDir()
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
	dir, err := apppath.InstallersDir()
	if err != nil || dir == "" {
		return false
	}
	return OpenPath(dir)
}

func OpenVersionsDir() string {
	vdir, err := apppath.VersionsDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return vdir
}
