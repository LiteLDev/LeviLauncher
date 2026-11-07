//go:build windows

package lipdirs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// knownFolder mirrors .NET Environment.GetFolderPath(folder,
// SpecialFolderOption.DoNotVerify), falling back to the environment
// variable env when the known folder cannot be resolved.
func knownFolder(id *windows.KNOWNFOLDERID, env string) string {
	if p, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil && p != "" {
		return p
	}
	return os.Getenv(env)
}

// applicationData is SpecialFolder.ApplicationData (FOLDERID_RoamingAppData).
func applicationData() string { return knownFolder(windows.FOLDERID_RoamingAppData, "APPDATA") }

// localAppData is SpecialFolder.LocalApplicationData (FOLDERID_LocalAppData).
func localAppData() string { return knownFolder(windows.FOLDERID_LocalAppData, "LOCALAPPDATA") }

func cacheDir() string { return filepath.Join(localAppData(), appName, "Cache") }
