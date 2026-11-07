//go:build !windows

package lipdirs

import (
	"os"
	"path/filepath"
	"runtime"
)

// xdgHome returns $env when it is an absolute path (the XDG specification
// says to ignore an empty or relative value), else $HOME/fallback.
func xdgHome(env, fallback string) string {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(userProfile(), fallback)
}

// userProfile is .NET SpecialFolder.UserProfile on Unix: $HOME.
func userProfile() string {
	home, _ := os.UserHomeDir()
	return home
}

// applicationData is .NET SpecialFolder.ApplicationData on Unix (macOS
// included): $XDG_CONFIG_HOME if absolute, else ~/.config.
func applicationData() string { return xdgHome("XDG_CONFIG_HOME", ".config") }

// localAppData is .NET SpecialFolder.LocalApplicationData on Unix:
// $XDG_DATA_HOME if absolute, else ~/.local/share; on macOS (.NET 8+)
// ~/Library/Application Support.
func localAppData() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(userProfile(), "Library", "Application Support")
	}
	return xdgHome("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func cacheDir() string { return filepath.Join(xdgHome("XDG_CACHE_HOME", ".cache"), appName) }
