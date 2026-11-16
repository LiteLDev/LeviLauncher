//go:build !windows

package apppath

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func roamingAppData() string {
	if v := strings.TrimSpace(os.Getenv("APPDATA")); v != "" {
		return v
	}
	v, _ := os.UserConfigDir()
	return v
}

func localAppData() string {
	if v := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		v, _ := os.UserConfigDir()
		return v
	}
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}
