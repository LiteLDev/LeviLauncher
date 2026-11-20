//go:build darwin

package mcservice

import (
	"os"
	"path/filepath"
)

func userDesktopDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}
