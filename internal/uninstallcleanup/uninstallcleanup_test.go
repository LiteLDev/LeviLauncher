package uninstallcleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	json "github.com/goccy/go-json"
	"golang.org/x/sys/windows"
)

func TestParseArgs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		want      Options
		requested bool
	}{
		{"no args", nil, Options{}, false},
		{"other flags only", []string{"--launch=1.21", "--delete-versions=1"}, Options{DeleteVersions: true}, false},
		{"cleanup alone", []string{"--uninstall-cleanup"}, Options{}, true},
		{"all true", []string{"--uninstall-cleanup", "--delete-installers=1", "--delete-backups=1", "--delete-versions=1"},
			Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true}, true},
		{"empty values", []string{"--uninstall-cleanup", "--delete-installers=", "--delete-backups=", "--delete-versions="},
			Options{}, true},
		{"mixed", []string{"--delete-installers=1", "--uninstall-cleanup", "--delete-backups=0", "--delete-versions="},
			Options{DeleteInstallers: true}, true},
		{"non-1 values are false", []string{"--uninstall-cleanup", "--delete-installers=true", "--delete-backups=yes", "--delete-versions=2"},
			Options{}, true},
		{"quoted and spaced 1", []string{"--uninstall-cleanup", `--delete-installers="1"`, "--delete-versions= 1 "},
			Options{DeleteInstallers: true, DeleteVersions: true}, true},
		{"flag without value is ignored", []string{"--uninstall-cleanup", "--delete-versions"}, Options{}, true},
		{"cleanup flag must match exactly", []string{"--uninstall-cleanup=1"}, Options{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, requested := ParseArgs(tc.args)
			if got != tc.want || requested != tc.requested {
				t.Fatalf("ParseArgs(%q) = (%+v, %v), want (%+v, %v)", tc.args, got, requested, tc.want, tc.requested)
			}
		})
	}
}

// env fakes %APPDATA% and %LOCALAPPDATA%, which apppath reads first.
type env struct {
	configDir string // %APPDATA%\LeviLauncher
	localDir  string // %LOCALAPPDATA%\LeviLauncher, the default data root
	legacyDir string // %APPDATA%\levilauncher.exe
}

func newEnv(t *testing.T) env {
	t.Helper()
	appData, localAppData := t.TempDir(), t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)
	return env{
		configDir: filepath.Join(appData, "LeviLauncher"),
		localDir:  filepath.Join(localAppData, "LeviLauncher"),
		legacyDir: filepath.Join(appData, "levilauncher.exe"),
	}
}

func j(parts ...string) string { return filepath.Join(parts...) }

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeConfig writes config.json in dir, with base_root unless it is "".
func writeConfig(t *testing.T, dir, baseRoot string) {
	t.Helper()
	c := map[string]any{"window_width": 1024}
	if baseRoot != "" {
		c["base_root"] = baseRoot
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, j(dir, "config.json"), string(b))
}

// seedCurrent fakes a launcher on the current layout with its data in baseRoot.
func seedCurrent(t *testing.T, e env, baseRoot string) {
	t.Helper()
	for _, p := range []string{
		`Instances\a\level.dat`, `Backups\Worlds\w.mcworld`, `Cache\Installers\x.msixvc`,
		`Cache\gamertags.json`, `Keep\x`,
	} {
		writeFile(t, j(baseRoot, p), "x")
	}
	for _, p := range []string{`Logs\startup.log`, `Accounts\a`, `WebView2\EBWebView\x`, `storage-migration.json`} {
		writeFile(t, j(e.localDir, p), "x")
	}
}

// seedLegacy fakes a launcher on the legacy layout with its data in baseRoot.
func seedLegacy(t *testing.T, e env, baseRoot string) {
	t.Helper()
	for _, p := range []string{`versions\a\level.dat`, `backups\w.mcworld`, `installers\x.msixvc`, `keep\x`} {
		writeFile(t, j(baseRoot, p), "x")
	}
	for _, p := range []string{`user_gamertag_map.json`, `EBWebView\x`, `bin\x`, `logs\x`} {
		writeFile(t, j(e.legacyDir, p), "x")
	}
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
	return false
}

func assertExists(t *testing.T, want bool, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if got := exists(t, path); got != want {
			t.Errorf("exists(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestRunCurrentLayoutDefaultDataRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"nothing selected", Options{}},
		{"installers", Options{DeleteInstallers: true}},
		{"backups", Options{DeleteBackups: true}},
		{"versions", Options{DeleteVersions: true}},
		{"all", Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			writeConfig(t, e.configDir, "")
			seedCurrent(t, e, e.localDir)

			Run(tc.opts)

			assertExists(t, false, e.configDir, j(e.localDir, "WebView2"), j(e.localDir, "storage-migration.json"),
				j(e.localDir, "Cache", "gamertags.json"))
			assertExists(t, true, j(e.localDir, "Logs", "startup.log"), j(e.localDir, "Accounts", "a"), j(e.localDir, "Keep", "x"))
			assertExists(t, !tc.opts.DeleteInstallers, j(e.localDir, "Cache", "Installers", "x.msixvc"))
			assertExists(t, !tc.opts.DeleteBackups, j(e.localDir, "Backups", "Worlds", "w.mcworld"))
			assertExists(t, !tc.opts.DeleteVersions, j(e.localDir, "Instances", "a", "level.dat"))
		})
	}
}

func TestRunCurrentLayoutCustomDataRoot(t *testing.T) {
	e := newEnv(t)
	baseRoot := j(t.TempDir(), "Games", "LeviLauncher")
	writeConfig(t, e.configDir, baseRoot)
	seedCurrent(t, e, baseRoot)
	// Folders of the same name in the default data root are not the user's data.
	writeFile(t, j(e.localDir, "Instances", "other", "level.dat"), "x")

	Run(Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

	assertExists(t, false, j(baseRoot, "Instances"), j(baseRoot, "Backups"), j(baseRoot, "Cache"), e.configDir)
	assertExists(t, true, j(baseRoot, "Keep", "x"), j(e.localDir, "Instances", "other", "level.dat"))
}

func TestRunLegacyLayout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		custom bool
	}{
		{"default data root", false},
		{"custom data root", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			baseRoot := e.legacyDir
			if tc.custom {
				baseRoot = j(t.TempDir(), "Data")
				writeConfig(t, e.legacyDir, baseRoot)
			} else {
				writeConfig(t, e.legacyDir, "")
			}
			seedLegacy(t, e, baseRoot)

			Run(Options{DeleteInstallers: true, DeleteVersions: true})

			assertExists(t, false, j(baseRoot, "installers"), j(baseRoot, "versions"),
				j(e.legacyDir, "config.json"), j(e.legacyDir, "user_gamertag_map.json"),
				j(e.legacyDir, "EBWebView"), j(e.legacyDir, "bin"))
			assertExists(t, true, j(baseRoot, "backups", "w.mcworld"), j(baseRoot, "keep", "x"), j(e.legacyDir, "logs", "x"))
		})
	}
}

// After the migration both config.json files exist; the current one decides
// the data root, and the copy the lossless migration kept in the legacy
// folder is left alone.
func TestRunPrefersCurrentConfig(t *testing.T) {
	e := newEnv(t)
	writeConfig(t, e.configDir, "")
	writeConfig(t, e.legacyDir, "")
	seedCurrent(t, e, e.localDir)
	seedLegacy(t, e, e.legacyDir)

	Run(Options{DeleteVersions: true})

	assertExists(t, false, j(e.localDir, "Instances"), e.configDir, j(e.legacyDir, "config.json"))
	assertExists(t, true, j(e.legacyDir, "versions", "a", "level.dat"))
}

func TestRunRemovesEmptyDirectories(t *testing.T) {
	e := newEnv(t)
	writeConfig(t, e.configDir, "")
	writeFile(t, j(e.localDir, "WebView2", "EBWebView", "x"), "x")
	writeFile(t, j(e.localDir, "Cache", "gamertags.json"), "x")
	writeConfig(t, e.legacyDir, "")
	writeFile(t, j(e.legacyDir, "EBWebView", "x"), "x")

	Run(Options{})

	assertExists(t, false, e.configDir, e.localDir, e.legacyDir)
}

func TestRunWithoutConfigUsesDefaultDataRoot(t *testing.T) {
	e := newEnv(t)
	seedCurrent(t, e, e.localDir)

	Run(Options{DeleteBackups: true})

	assertExists(t, false, j(e.localDir, "Backups"))
	assertExists(t, true, j(e.localDir, "Instances", "a", "level.dat"))
}

func TestRunUnreadableConfigKeepsData(t *testing.T) {
	e := newEnv(t)
	seedCurrent(t, e, e.localDir)
	writeFile(t, j(e.configDir, "config.json"), "{not json")

	Run(Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

	assertExists(t, true, j(e.localDir, "Instances", "a", "level.dat"), j(e.localDir, "Backups", "Worlds", "w.mcworld"),
		j(e.localDir, "Cache", "Installers", "x.msixvc"))
	assertExists(t, false, j(e.configDir, "config.json"))
}

func TestRunIgnoresUnrelatedMalformedConfigFields(t *testing.T) {
	e := newEnv(t)
	baseRoot := j(t.TempDir(), "Data")
	seedCurrent(t, e, baseRoot)
	quoted, err := json.Marshal(baseRoot)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, j(e.configDir, "config.json"), `{"base_root": `+string(quoted)+`, "window_width": "wide"}`)

	Run(Options{DeleteBackups: true})

	assertExists(t, false, j(baseRoot, "Backups"))
}

func TestRunRefusesUnsafeDataRoot(t *testing.T) {
	e := newEnv(t)
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	writeConfig(t, e.configDir, profile)
	seedCurrent(t, e, profile)

	Run(Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

	assertExists(t, true, j(profile, "Instances", "a", "level.dat"), j(profile, "Backups", "Worlds", "w.mcworld"),
		j(profile, "Cache", "Installers", "x.msixvc"), j(profile, "Cache", "gamertags.json"))
	assertExists(t, false, e.configDir)
}

func TestIsSafeBaseRoot(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	appData := filepath.Join(profile, "AppData", "Roaming")
	t.Setenv("APPDATA", appData)
	sysRoot := filepath.Join(t.TempDir(), "Windows")
	t.Setenv("SystemRoot", sysRoot)
	programFiles := filepath.Join(t.TempDir(), "Program Files")
	t.Setenv("ProgramFiles", programFiles)

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"", false},
		{"   ", false},
		{`relative\dir`, false},
		{`C:\`, false},
		{`C:`, false},
		{`D:\`, false},
		{`\\server\share`, false},
		{`\\server\share\`, false},
		{profile, false},
		{filepath.Dir(profile), false},
		{appData, false},
		{sysRoot, false},
		{filepath.Join(sysRoot, "System32"), false},
		{programFiles, false},
		{strings.ToLower(programFiles), false},
		{filepath.Join(appData, "levilauncher.exe"), true},
		{filepath.Join(profile, "AppData", "Local", "LeviLauncher"), true},
		{filepath.Join(profile, "Games", "LeviLauncher"), true},
		{`D:\LeviLauncher`, true},
		{`\\server\share\LeviLauncher`, true},
	} {
		if got := IsSafeBaseRoot(tc.path); got != tc.want {
			t.Errorf("IsSafeBaseRoot(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsSafeBaseRootRejectsShellFolders(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	for _, sub := range []string{"Desktop", "Documents", "Downloads"} {
		if IsSafeBaseRoot(filepath.Join(profile, sub)) {
			t.Errorf("IsSafeBaseRoot(%s) = true, want false", sub)
		}
	}
	if docs, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DONT_VERIFY); err == nil {
		if IsSafeBaseRoot(docs) {
			t.Errorf("IsSafeBaseRoot(%q) = true, want false", docs)
		}
	}
}
