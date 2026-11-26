package apppath

import (
	"os"
	"path/filepath"
	"strings"
)

// APPDATA and LOCALAPPDATA override the folders, as on Windows, for tests
// and portable setups. Without them the launcher follows Apple's File System
// Programming Guide: data in ~/Library/Application Support, caches in
// ~/Library/Caches and logs in ~/Library/Logs.

func roamingAppData() string {
	if v := strings.TrimSpace(os.Getenv("APPDATA")); v != "" {
		return v
	}
	v, _ := os.UserConfigDir()
	return v
}

// localOverride is LOCALAPPDATA when set. The launcher then keeps its logs
// and caches inside it as well, so that an override never writes to ~/Library.
func localOverride() string { return strings.TrimSpace(os.Getenv("LOCALAPPDATA")) }

func localAppData() string {
	if v := localOverride(); v != "" {
		return v
	}
	v, _ := os.UserConfigDir()
	return v
}

// libraryDir is ~/Library/<name>.
func libraryDir(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", name)
}

// logsDir is ~/Library/Logs/LeviLauncher, where Console finds the logs.
func logsDir() string {
	if localOverride() != "" {
		return filepath.Join(LocalDir(), LogsDirName)
	}
	return filepath.Join(libraryDir("Logs"), AppDirName)
}

// cacheDir is ~/Library/Caches/LeviLauncher, which Time Machine skips and
// macOS may purge when space runs low. It does not follow base_root. The
// product directory is named as everywhere else (README N1) rather than after
// the bundle identifier, whose cache directory WebKit uses for itself.
func cacheDir() string {
	if localOverride() != "" {
		return filepath.Join(LocalDir(), CacheDirName)
	}
	if v, err := os.UserCacheDir(); err == nil && v != "" {
		return filepath.Join(v, AppDirName)
	}
	return filepath.Join(libraryDir("Caches"), AppDirName)
}

// fixedDirs leaves out WebView2Dir: WKWebView keeps its data in
// ~/Library/WebKit/<bundle identifier> itself.
func fixedDirs() []string {
	return []string{ConfigDir(), LocalDir(), LogsDir(), AccountsDir(), BaseRoot()}
}
