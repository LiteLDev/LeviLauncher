//go:build windows

package apppath

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// knownFolder prefers the environment variable, which tests and portable
// setups override, then the known folder itself.
func knownFolder(env string, id *windows.KNOWNFOLDERID) string {
	if v := strings.TrimSpace(os.Getenv(env)); v != "" {
		return v
	}
	if p, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
		return p
	}
	return ""
}

func roamingAppData() string { return knownFolder("APPDATA", windows.FOLDERID_RoamingAppData) }

func localAppData() string { return knownFolder("LOCALAPPDATA", windows.FOLDERID_LocalAppData) }

func logsDir() string { return filepath.Join(LocalDir(), LogsDirName) }

func cacheDir() string { return filepath.Join(BaseRoot(), CacheDirName) }

func fixedDirs() []string {
	return []string{ConfigDir(), LocalDir(), LogsDir(), AccountsDir(), WebView2Dir(), BaseRoot()}
}
