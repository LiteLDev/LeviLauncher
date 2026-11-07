// Package uninstallcleanup removes launcher data when the MSI uninstalls the
// launcher. The MSI runs `LeviLauncher.exe --uninstall-cleanup` from a
// deferred, impersonated custom action, so this code runs as the user whose
// %APPDATA% holds the launcher config. It must stay headless and quick.
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

	// storageDirName is the per-user folder under %APPDATA%. The launcher names
	// it after its executable; the installer always ships LeviLauncher.exe, and
	// a fixed name keeps a renamed or temp copy from cleaning the wrong folder.
	storageDirName = "levilauncher.exe"

	deleteInstallersFlag = "--delete-installers="
	deleteBackupsFlag    = "--delete-backups="
	deleteVersionsFlag   = "--delete-versions="
)

// Options selects the optional base_root folders to remove. Per-user launcher
// files are always removed.
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

// Run cleans up the launcher data of the current user. Errors are ignored:
// the uninstaller cannot act on them and must not be blocked.
func Run(opts Options) {
	Cleanup(filepath.Join(apppath.AppData(), storageDirName), opts)
}

// Cleanup removes the selected base_root folders and the per-user launcher
// files under storageRoot (%APPDATA%\levilauncher.exe). The base root is
// resolved before anything is deleted, because config.json is one of the
// files removed.
func Cleanup(storageRoot string, opts Options) {
	if strings.TrimSpace(storageRoot) == "" {
		return
	}
	storageRoot = filepath.Clean(storageRoot)

	if opts.DeleteInstallers || opts.DeleteBackups || opts.DeleteVersions {
		if baseRoot, ok := ResolveBaseRoot(storageRoot); ok && IsSafeBaseRoot(baseRoot) {
			if opts.DeleteInstallers {
				_ = os.RemoveAll(filepath.Join(baseRoot, "installers"))
			}
			if opts.DeleteBackups {
				_ = os.RemoveAll(filepath.Join(baseRoot, "backups"))
			}
			if opts.DeleteVersions {
				_ = os.RemoveAll(filepath.Join(baseRoot, "versions"))
			}
		}
	}

	_ = os.Remove(filepath.Join(storageRoot, "config.json"))
	_ = os.Remove(filepath.Join(storageRoot, "user_gamertag_map.json"))
	_ = os.RemoveAll(filepath.Join(storageRoot, "EBWebView"))
	_ = os.RemoveAll(filepath.Join(storageRoot, "bin"))
	// Remove only succeeds on an empty directory; logs and other data keep it,
	// as with the Inno uninstaller.
	_ = os.Remove(storageRoot)
}

// ResolveBaseRoot returns the base root the launcher uses: base_root from
// config.json, or storageRoot when it is unset (see apppath.BaseRoot). It
// reports false when the config exists but cannot be read or parsed, since the
// launcher's data location is then unknown.
func ResolveBaseRoot(storageRoot string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(storageRoot, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return storageRoot, true
		}
		return "", false
	}
	// Decode only base_root so an unrelated malformed field cannot hide it.
	var c struct {
		BaseRoot string `json:"base_root"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return "", false
	}
	if v := strings.TrimSpace(c.BaseRoot); v != "" {
		return v, true
	}
	return storageRoot, true
}

// IsSafeBaseRoot rejects base roots whose installers/backups/versions children
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
