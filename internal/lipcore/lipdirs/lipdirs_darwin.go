package lipdirs

import (
	"os"
	"path/filepath"
)

// libraryDir is ~/Library/<name>.
func libraryDir(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", name)
}

// applicationData is ~/Library/Application Support, the folder macOS gives
// what Windows keeps in FOLDERID_RoamingAppData, as the launcher's own
// configuration does (internal/apppath).
func applicationData() string { return libraryDir("Application Support") }

// localAppData is .NET SpecialFolder.LocalApplicationData on macOS (.NET 8+):
// ~/Library/Application Support.
func localAppData() string { return libraryDir("Application Support") }

// cacheDir is ~/Library/Caches/lip, which Time Machine skips and macOS may
// purge when space runs low.
func cacheDir() string { return filepath.Join(libraryDir("Caches"), appName) }
