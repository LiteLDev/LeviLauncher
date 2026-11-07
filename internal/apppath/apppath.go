// Package apppath names every file and directory LeviLauncher owns. The
// layout and its naming rules are specified in README.md next to this file;
// a path the launcher writes is added there first.
//
// Directories are PascalCase, data files lowercase kebab-case. The getters
// only compute paths; EnsureDirs creates the fixed directories once at
// startup, and callers that write below a directory create it themselves.
package apppath

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// AppDirName is the product directory below each known folder.
	AppDirName = "LeviLauncher"

	ConfigFileName = "config.json"

	LogsDirName        = "Logs"
	StartupLogFileName = "startup.log"
	AccountsDirName    = "Accounts"
	WebView2DirName    = "WebView2"

	InstancesDirName       = "Instances"
	BackupsDirName         = "Backups"
	WorldBackupsDirName    = "Worlds"
	InstanceBackupsDirName = "Instances"

	CacheDirName               = "Cache"
	InstallersDirName          = "Installers"
	UpdatesDirName             = "Updates"
	DownloadsDirName           = "Downloads"
	ImportsDirName             = "Imports"
	GDKPendingVersionsFileName = "gdk-pending-versions.json"
	GamertagsFileName          = "gamertags.json"

	// MigrationFileName records a completed migration from the legacy layout
	// (internal/storage); MigrationJournalFileName journals one in progress.
	MigrationFileName        = "storage-migration.json"
	MigrationJournalFileName = "storage-migration.journal"

	// InstanceStateDirName holds the launcher's state inside an instance, as
	// .lip holds lip's.
	InstanceStateDirName  = ".levilauncher"
	InstanceMetaFileName  = "instance.json"
	InstanceIconFileName  = "icon.ico"
	InstanceAuthKeyMarker = "xbox-auth-key-v1"
)

// Names of the legacy layout, read by the migration and by
// legacy mode.
const (
	LegacyDefaultExeName         = "levilauncher.exe"
	LegacyLogsDirName            = "logs"
	LegacyAccountsDirName        = "microsoft-account"
	LegacyWebViewDirName         = "EBWebView"
	LegacyVersionsDirName        = "versions"
	LegacyBackupsDirName         = "backups"
	LegacyWorldBackupsDirName    = "worlds"
	LegacyInstanceBackupsDirName = "instances"
	LegacyInstallersDirName      = "installers"
	LegacyUpdatesDirName         = "updates"
	LegacyGDKPendingFileName     = "gdk_pending_versions.json"
	LegacyGamertagsFileName      = "user_gamertag_map.json"
	LegacyInstanceMetaFileName   = "version.json"
	LegacyInstanceIconFileName   = "LargeLogo.ico"
	LegacyInstanceAuthKeyMarker  = ".levilauncher-xbox-auth-key-v1"
)

var (
	BaseRootOverrideProvider func() string

	mu               sync.RWMutex
	baseRootOverride string
	// legacyDir is set when the launcher runs on the legacy layout because
	// the migration could not complete (see internal/storage).
	legacyDir string
)

func SetBaseRootOverride(path string) {
	mu.Lock()
	baseRootOverride = path
	mu.Unlock()
}

// SetLegacyMode makes every getter return the legacy layout rooted at dir,
// %APPDATA%\<exe name>. An empty dir restores the current layout.
func SetLegacyMode(dir string) {
	mu.Lock()
	legacyDir = strings.TrimSpace(dir)
	mu.Unlock()
}

// LegacyMode returns the legacy directory the launcher runs on, or "".
func LegacyMode() string {
	mu.RLock()
	defer mu.RUnlock()
	return legacyDir
}

func LauncherDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

// AppData is the roaming application data folder (FOLDERID_RoamingAppData).
func AppData() string { return roamingAppData() }

// LocalAppData is the local application data folder (FOLDERID_LocalAppData).
func LocalAppData() string { return localAppData() }

// LegacyDir is the directory of the legacy layout: %APPDATA%\<exe name>, the
// exe name lowercased, as WebView2 also used it for its user data.
func LegacyDir() string {
	exeName := LegacyDefaultExeName
	if exe, err := os.Executable(); err == nil {
		if base := strings.TrimSpace(filepath.Base(exe)); base != "" {
			exeName = strings.ToLower(base)
		}
	}
	return filepath.Join(AppData(), exeName)
}

// ConfigDir holds config.json: %APPDATA%\LeviLauncher, small and roamable.
func ConfigDir() string {
	if d := LegacyMode(); d != "" {
		return d
	}
	return filepath.Join(AppData(), AppDirName)
}

func ConfigPath() string { return filepath.Join(ConfigDir(), ConfigFileName) }

// LocalDir holds what belongs to this machine: %LOCALAPPDATA%\LeviLauncher.
// It is also the default data root.
func LocalDir() string {
	return filepath.Join(LocalAppData(), AppDirName)
}

// LogsDir is LocalDir\Logs, also in legacy mode: the log is opened before the
// migration decides the layout.
func LogsDir() string { return filepath.Join(LocalDir(), LogsDirName) }

func StartupLogPath() string { return filepath.Join(LogsDir(), StartupLogFileName) }

// AccountsDir holds the DPAPI-protected Microsoft account and device records.
func AccountsDir() string {
	if d := LegacyMode(); d != "" {
		return filepath.Join(d, LegacyAccountsDirName)
	}
	return filepath.Join(LocalDir(), AccountsDirName)
}

// WebView2Dir is the WebView2 user data folder; WebView2 creates EBWebView
// below it.
func WebView2Dir() string {
	if d := LegacyMode(); d != "" {
		return d
	}
	return filepath.Join(LocalDir(), WebView2DirName)
}

// MigrationPath and MigrationJournalPath are in LocalDir, which the
// migration never moves.
func MigrationPath() string        { return filepath.Join(LocalDir(), MigrationFileName) }
func MigrationJournalPath() string { return filepath.Join(LocalDir(), MigrationJournalFileName) }

// DefaultBaseRoot is the data root when config.json names none.
func DefaultBaseRoot() string {
	if d := LegacyMode(); d != "" {
		return d
	}
	return LocalDir()
}

// BaseRoot is the data root: config.json's base_root, or DefaultBaseRoot.
func BaseRoot() string {
	mu.RLock()
	override := strings.TrimSpace(baseRootOverride)
	mu.RUnlock()
	if override != "" {
		return override
	}
	if BaseRootOverrideProvider != nil {
		if v := strings.TrimSpace(BaseRootOverrideProvider()); v != "" {
			return v
		}
	}
	return DefaultBaseRoot()
}

func ensure(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// InstancesDir holds one directory per game instance, created on demand.
func InstancesDir() (string, error) {
	name := InstancesDirName
	if LegacyMode() != "" {
		name = LegacyVersionsDirName
	}
	return ensure(filepath.Join(BaseRoot(), name))
}

// BackupsDir is <BaseRoot>\Backups; Windows paths ignore case, so it is also
// the legacy backups directory.
func BackupsDir() string { return filepath.Join(BaseRoot(), BackupsDirName) }

func WorldBackupsDir() string { return filepath.Join(BackupsDir(), WorldBackupsDirName) }

func InstanceBackupsDir() string { return filepath.Join(BackupsDir(), InstanceBackupsDirName) }

// CacheDir holds what can be deleted at any time without losing anything.
func CacheDir() string { return filepath.Join(BaseRoot(), CacheDirName) }

// InstallersDir holds downloaded game packages and redistributables.
func InstallersDir() (string, error) {
	if LegacyMode() != "" {
		return ensure(filepath.Join(BaseRoot(), LegacyInstallersDirName))
	}
	return ensure(filepath.Join(CacheDir(), InstallersDirName))
}

// UpdatesDir holds a downloaded launcher update until it is installed.
func UpdatesDir() string {
	if LegacyMode() != "" {
		return filepath.Join(BaseRoot(), LegacyUpdatesDirName)
	}
	return filepath.Join(CacheDir(), UpdatesDirName)
}

// DownloadsDir holds files downloaded for an import, removed once imported.
func DownloadsDir() string { return filepath.Join(CacheDir(), DownloadsDirName) }

// ImportsDir holds files the frontend hands over for an import.
func ImportsDir() string { return filepath.Join(CacheDir(), ImportsDirName) }

func GDKPendingVersionsPath() string {
	if d := LegacyMode(); d != "" {
		return filepath.Join(d, LegacyGDKPendingFileName)
	}
	return filepath.Join(CacheDir(), GDKPendingVersionsFileName)
}

func GamertagsPath() string {
	if d := LegacyMode(); d != "" {
		return filepath.Join(d, LegacyGamertagsFileName)
	}
	return filepath.Join(CacheDir(), GamertagsFileName)
}

// InstanceStateDir is <instance>\.levilauncher.
func InstanceStateDir(instanceDir string) string {
	return filepath.Join(instanceDir, InstanceStateDirName)
}

// EnsureDirs creates the fixed directories of the layout.
func EnsureDirs() error {
	for _, dir := range []string{ConfigDir(), LocalDir(), LogsDir(), AccountsDir(), WebView2Dir(), BaseRoot()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func MinecraftData(isPreview bool) string {
	suffix := "Minecraft Bedrock"
	if isPreview {
		suffix = "Minecraft Bedrock Preview"
	}
	return filepath.Join(AppData(), suffix)
}
