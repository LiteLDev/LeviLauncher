//go:build darwin || linux

package mcservice

import (
	"os"
	"path/filepath"
)

func userDesktopDir() string {
	if value := os.Getenv("XDG_DESKTOP_DIR"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}
