//go:build windows

package fetch

import (
	"os"

	"golang.org/x/sys/windows"
)

// localAppData mirrors .NET Environment.GetFolderPath(
// SpecialFolder.LocalApplicationData) on Windows (FOLDERID_LocalAppData).
func localAppData() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, windows.KF_FLAG_DONT_VERIFY); err == nil && p != "" {
		return p
	}
	return os.Getenv("LOCALAPPDATA")
}
