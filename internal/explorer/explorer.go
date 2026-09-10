package explorer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func OpenPath(dir string) error {
	app := application.Get()
	if app == nil {
		return errors.New("application is not ready to open directories")
	}
	return openPath(dir, app.Browser.OpenFile)
}

func openPath(dir string, open func(string) error) error {
	d := strings.TrimSpace(dir)
	if d == "" {
		return errors.New("directory path is empty")
	}
	// MkdirAll also rejects existing files: OpenFile must only receive directories here.
	if err := os.MkdirAll(d, 0755); err != nil {
		return fmt.Errorf("prepare directory %s: %w", d, err)
	}
	if err := open(d); err != nil {
		return fmt.Errorf("open directory %s: %w", d, err)
	}
	return nil
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

func OpenMods(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return errors.New("version name is empty")
	}
	vdir, err := apppath.VersionsDir()
	if err != nil {
		return fmt.Errorf("resolve versions directory: %w", err)
	}
	if strings.TrimSpace(vdir) == "" {
		return errors.New("versions directory is empty")
	}
	dir := filepath.Join(vdir, n, "mods")
	return OpenPath(dir)
}
