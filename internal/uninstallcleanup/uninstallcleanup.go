// Package uninstallcleanup removes launcher data when the MSI uninstalls the
// launcher, as the Inno Setup uninstaller did. The MSI runs
// `LeviLauncher.exe --uninstall-cleanup` from a deferred, impersonated custom
// action, so this code runs as the user whose %APPDATA% and %LOCALAPPDATA%
// hold the launcher data. It must stay headless and quick.
package uninstallcleanup

import (
	"os"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"
	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

const (
	Flag = "--uninstall-cleanup"

	deleteInstallersFlag = "--delete-installers="
	deleteBackupsFlag    = "--delete-backups="
	deleteVersionsFlag   = "--delete-versions="
)

// Options selects the optional data root folders to remove. The launcher's
// own per-user files are always removed.
type Options struct {
	DeleteInstallers bool
	DeleteBackups    bool
	DeleteVersions   bool
}

// ParseArgs reports whether args (without the program name) request the
// uninstall cleanup. A delete flag is true only for the value "1": the MSI
// passes unset properties as an empty string.
func ParseArgs(args []string) (Options, bool) {
	var opts Options
	requested := false
	for _, arg := range args {
		switch {
		case arg == Flag:
			requested = true
		case strings.HasPrefix(arg, deleteInstallersFlag):
			opts.DeleteInstallers = isTrue(strings.TrimPrefix(arg, deleteInstallersFlag))
		case strings.HasPrefix(arg, deleteBackupsFlag):
			opts.DeleteBackups = isTrue(strings.TrimPrefix(arg, deleteBackupsFlag))
		case strings.HasPrefix(arg, deleteVersionsFlag):
			opts.DeleteVersions = isTrue(strings.TrimPrefix(arg, deleteVersionsFlag))
		}
	}
	return opts, requested
}

func isTrue(v string) bool {
	return strings.Trim(strings.TrimSpace(v), `"'`) == "1"
}

// layout is where the launcher keeps its files (internal/apppath/README.md):
// config.json in configDir, machine data in localDir, which is also the
// default data root, and everything in legacyDir before the storage migration.
type layout struct {
	configDir string
	localDir  string
	legacyDir string
}

func currentLayout() layout {
	return layout{
		configDir: filepath.Join(apppath.AppData(), apppath.AppDirName),
		localDir:  apppath.LocalDir(),
		// The installer always ships LeviLauncher.exe, and a fixed name keeps
		// a renamed copy from cleaning the wrong folder.
		legacyDir: filepath.Join(apppath.AppData(), apppath.LegacyDefaultExeName),
	}
}

// Run cleans up the launcher data of the current user, in both the current
// and the legacy layout. Errors are ignored: the uninstaller cannot act on
// them and must not be blocked.
func Run(opts Options) {
	currentLayout().cleanup(opts)
}

func (l layout) cleanup(opts Options) {
	// Resolved first, because config.json is one of the files removed.
	baseRoot, ok := l.resolveBaseRoot()
	if ok && !IsSafeBaseRoot(baseRoot) {
		ok = false
	}
	if ok {
		if opts.DeleteInstallers {
			_ = os.RemoveAll(filepath.Join(baseRoot, apppath.CacheDirName))
			_ = os.RemoveAll(filepath.Join(baseRoot, apppath.LegacyInstallersDirName))
		}
		if opts.DeleteVersions {
			_ = os.RemoveAll(filepath.Join(baseRoot, apppath.InstancesDirName))
			_ = os.RemoveAll(filepath.Join(baseRoot, apppath.LegacyVersionsDirName))
		}
		if opts.DeleteBackups {
			// Also the legacy "backups": Windows paths are case-insensitive.
			_ = os.RemoveAll(filepath.Join(baseRoot, apppath.BackupsDirName))
		}
		_ = os.Remove(filepath.Join(baseRoot, apppath.CacheDirName, apppath.GamertagsFileName))
		_ = os.Remove(filepath.Join(baseRoot, apppath.CacheDirName))
	}

	// The current layout. Logs and Accounts stay, as logs and
	// microsoft-account do in the legacy layout; os.Remove only removes an
	// empty directory.
	_ = os.Remove(filepath.Join(l.configDir, apppath.ConfigFileName))
	_ = os.Remove(l.configDir)
	_ = os.RemoveAll(filepath.Join(l.localDir, apppath.WebView2DirName))
	_ = os.Remove(filepath.Join(l.localDir, apppath.MigrationFileName))
	_ = os.Remove(l.localDir)

	// The legacy layout.
	_ = os.Remove(filepath.Join(l.legacyDir, apppath.ConfigFileName))
	_ = os.Remove(filepath.Join(l.legacyDir, apppath.LegacyGamertagsFileName))
	_ = os.RemoveAll(filepath.Join(l.legacyDir, apppath.LegacyWebViewDirName))
	_ = os.RemoveAll(filepath.Join(l.legacyDir, "bin"))
	_ = os.Remove(l.legacyDir)
}

// resolveBaseRoot returns the data root the launcher uses: base_root from the
// current config.json, else from the legacy one, defaulting to the data root
// of that layout. Without any config.json it is the current default. It
// reports false when a config.json exists but cannot be read or parsed, since
// the data root is then unknown.
func (l layout) resolveBaseRoot() (string, bool) {
	if root, found, ok := readBaseRoot(l.configDir, l.localDir); found {
		return root, ok
	}
	if root, found, ok := readBaseRoot(l.legacyDir, l.legacyDir); found {
		return root, ok
	}
	return l.localDir, true
}

func readBaseRoot(configDir, defaultRoot string) (root string, found, ok bool) {
	b, err := os.ReadFile(filepath.Join(configDir, apppath.ConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, false
		}
		return "", true, false
	}
	// Decode only base_root so an unrelated malformed field cannot hide it.
	var c struct {
		BaseRoot string `json:"base_root"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return "", true, false
	}
	if v := strings.TrimSpace(c.BaseRoot); v != "" {
		return v, true, true
	}
	return defaultRoot, true, true
}

// IsSafeBaseRoot rejects data roots whose Instances, Backups and Cache children
// must never be deleted wholesale: relative paths, volume roots and
// well-known system, profile and shell folders.
func IsSafeBaseRoot(baseRoot string) bool {
	baseRoot = strings.TrimSpace(baseRoot)
	if baseRoot == "" || !filepath.IsAbs(baseRoot) {
		return false
	}
	clean := filepath.Clean(baseRoot)
	if filepath.Dir(clean) == clean {
		return false
	}
	for _, protected := range protectedDirs() {
		if strings.EqualFold(clean, protected) {
			return false
		}
	}
	return true
}

func protectedDirs() []string {
	var dirs []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || !filepath.IsAbs(p) {
			return
		}
		dirs = append(dirs, filepath.Clean(p))
	}
	for _, name := range []string{
		"SystemRoot", "windir", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432",
		"ProgramData", "ALLUSERSPROFILE", "PUBLIC", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
		"TEMP", "TMP", "OneDrive",
	} {
		add(os.Getenv(name))
	}
	if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
		add(filepath.Join(sysRoot, "System32"))
		add(filepath.Join(sysRoot, "SysWOW64"))
	}
	if profile := strings.TrimSpace(os.Getenv("USERPROFILE")); profile != "" && filepath.IsAbs(profile) {
		profile = filepath.Clean(profile)
		add(filepath.Dir(profile)) // the Users folder
		for _, sub := range []string{"Desktop", "Documents", "Downloads"} {
			add(filepath.Join(profile, sub))
		}
	}
	// Shell folders may be redirected (e.g. to OneDrive) away from USERPROFILE.
	for _, id := range []*windows.KNOWNFOLDERID{
		windows.FOLDERID_Profile, windows.FOLDERID_Desktop,
		windows.FOLDERID_Documents, windows.FOLDERID_Downloads,
	} {
		if p, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
			add(p)
		}
	}
	return dirs
}
