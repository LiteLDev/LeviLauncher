//go:build !windows

package fetch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// localAppData mirrors .NET Environment.GetFolderPath(
// SpecialFolder.LocalApplicationData) on Unix: $XDG_DATA_HOME when it is an
// absolute path, else $HOME/.local/share; on macOS (.NET 8+)
// $HOME/Library/Application Support.
func localAppData() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support")
	}
	if data := os.Getenv("XDG_DATA_HOME"); strings.HasPrefix(data, "/") {
		return data
	}
	return filepath.Join(home, ".local", "share")
}
