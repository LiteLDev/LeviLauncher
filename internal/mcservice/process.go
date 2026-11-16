package mcservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

func normalizePath(p string) string {
	return utils.NormalizeWindowsPath(p)
}

// linkedVersionRoots maps the resolved location of every junction-backed
// version folder to its version name. Windows reports the resolved image path
// for a running process, so a linked version's game does not appear under the
// versions directory.
func linkedVersionRoots(versionsDir string) map[string]string {
	if strings.TrimSpace(versionsDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil
	}
	roots := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dir := filepath.Join(versionsDir, e.Name())
		if !utils.ResolvesToDir(dir) {
			continue
		}
		if resolved := canonicalPath(dir); resolved != "" && resolved != normalizePath(dir) {
			roots[resolved] = e.Name()
		}
	}
	return roots
}

func matchLinkedVersion(roots map[string]string, cleanPath string) (string, bool) {
	for root, name := range roots {
		if strings.HasPrefix(cleanPath, root+string(filepath.Separator)) {
			return name, true
		}
	}
	return "", false
}

func KillAllMinecraftProcesses() error {
	var firstErr error
	found := false
	for _, p := range ListMinecraftProcesses() {
		found = true
		if err := KillProcess(p.Pid); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if !found {
		return fmt.Errorf("no Minecraft process found")
	}
	return nil
}
