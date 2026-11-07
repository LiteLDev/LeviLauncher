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

type fixture struct {
	appData     string
	storageRoot string
	baseRoot    string
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, storageRoot string, baseRoot string) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"base_root": baseRoot, "window_width": 1024})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageRoot, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedBaseRoot(t *testing.T, baseRoot string) {
	t.Helper()
	for _, dir := range []string{"installers", "backups", "versions", "keep"} {
		writeFile(t, filepath.Join(baseRoot, dir, "nested", "file.bin"))
	}
}

func seedStorage(t *testing.T, storageRoot string, withLogs bool) {
	t.Helper()
	writeFile(t, filepath.Join(storageRoot, "user_gamertag_map.json"))
	writeFile(t, filepath.Join(storageRoot, "EBWebView", "Default", "x"))
	writeFile(t, filepath.Join(storageRoot, "bin", "x"))
	if withLogs {
		writeFile(t, filepath.Join(storageRoot, "logs", "x"))
	}
}

// newFixture fakes %APPDATA% and a separate base_root with launcher data.
func newFixture(t *testing.T) fixture {
	t.Helper()
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	f := fixture{
		appData:     appData,
		storageRoot: filepath.Join(appData, "levilauncher.exe"),
		baseRoot:    filepath.Join(t.TempDir(), "LeviLauncherData"),
	}
	writeConfig(t, f.storageRoot, f.baseRoot)
	seedStorage(t, f.storageRoot, true)
	seedBaseRoot(t, f.baseRoot)
	return f
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

func assertExists(t *testing.T, path string, want bool) {
	t.Helper()
	if got := exists(t, path); got != want {
		t.Fatalf("exists(%s) = %v, want %v", path, got, want)
	}
}

func assertUserFilesRemoved(t *testing.T, storageRoot string) {
	t.Helper()
	for _, name := range []string{"config.json", "user_gamertag_map.json", "EBWebView", "bin"} {
		assertExists(t, filepath.Join(storageRoot, name), false)
	}
}

func TestCleanupSelectedBaseRootFolders(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"none", Options{}},
		{"installers", Options{DeleteInstallers: true}},
		{"backups", Options{DeleteBackups: true}},
		{"versions", Options{DeleteVersions: true}},
		{"all", Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			Cleanup(f.storageRoot, tc.opts)

			assertExists(t, filepath.Join(f.baseRoot, "installers"), !tc.opts.DeleteInstallers)
			assertExists(t, filepath.Join(f.baseRoot, "backups"), !tc.opts.DeleteBackups)
			assertExists(t, filepath.Join(f.baseRoot, "versions"), !tc.opts.DeleteVersions)
			assertExists(t, filepath.Join(f.baseRoot, "keep", "nested", "file.bin"), true)
			assertExists(t, f.baseRoot, true)

			assertUserFilesRemoved(t, f.storageRoot)
			// logs keeps the storage folder, matching the Inno uninstaller.
			assertExists(t, filepath.Join(f.storageRoot, "logs", "x"), true)
		})
	}
}

func TestCleanupRemovesEmptyStorageRoot(t *testing.T) {
	appData := t.TempDir()
	storageRoot := filepath.Join(appData, "levilauncher.exe")
	baseRoot := filepath.Join(t.TempDir(), "data")
	writeConfig(t, storageRoot, baseRoot)
	seedStorage(t, storageRoot, false)
	seedBaseRoot(t, baseRoot)

	Cleanup(storageRoot, Options{DeleteVersions: true})

	assertExists(t, storageRoot, false)
	assertExists(t, filepath.Join(baseRoot, "versions"), false)
	assertExists(t, filepath.Join(baseRoot, "installers"), true)
}

func TestCleanupDefaultBaseRootIsStorageRoot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config func(t *testing.T, storageRoot string)
	}{
		{"no config", func(t *testing.T, storageRoot string) {}},
		{"empty base_root", func(t *testing.T, storageRoot string) { writeConfig(t, storageRoot, "") }},
		{"blank base_root", func(t *testing.T, storageRoot string) { writeConfig(t, storageRoot, "   ") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storageRoot := filepath.Join(t.TempDir(), "levilauncher.exe")
			tc.config(t, storageRoot)
			seedStorage(t, storageRoot, false)
			seedBaseRoot(t, storageRoot)

			Cleanup(storageRoot, Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

			for _, dir := range []string{"installers", "backups", "versions"} {
				assertExists(t, filepath.Join(storageRoot, dir), false)
			}
			assertExists(t, filepath.Join(storageRoot, "keep", "nested", "file.bin"), true)
			assertUserFilesRemoved(t, storageRoot)
		})
	}
}

func TestCleanupCorruptConfigKeepsBaseRootData(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "levilauncher.exe")
	seedStorage(t, storageRoot, false)
	seedBaseRoot(t, storageRoot)
	if err := os.WriteFile(filepath.Join(storageRoot, "config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	Cleanup(storageRoot, Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

	for _, dir := range []string{"installers", "backups", "versions"} {
		assertExists(t, filepath.Join(storageRoot, dir), true)
	}
	assertUserFilesRemoved(t, storageRoot)
}

func TestCleanupRefusesUnsafeBaseRoot(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("USERPROFILE", profile)
	seedBaseRoot(t, profile)

	for _, tc := range []struct {
		name     string
		baseRoot string
	}{
		{"user profile", profile},
		{"user profile with trailing separator", profile + string(filepath.Separator)},
		{"user profile different case", strings.ToUpper(profile)},
		{"relative", "relative" + string(filepath.Separator) + "data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storageRoot := filepath.Join(t.TempDir(), "levilauncher.exe")
			writeConfig(t, storageRoot, tc.baseRoot)

			Cleanup(storageRoot, Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

			for _, dir := range []string{"installers", "backups", "versions"} {
				assertExists(t, filepath.Join(profile, dir, "nested", "file.bin"), true)
			}
			assertExists(t, filepath.Join(storageRoot, "config.json"), false)
		})
	}
}

func TestCleanupEmptyStorageRootIsNoop(t *testing.T) {
	Cleanup("", Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})
	Cleanup("   ", Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})
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

func TestCleanupIgnoresUnrelatedMalformedConfigFields(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "levilauncher.exe")
	baseRoot := filepath.Join(t.TempDir(), "data")
	seedStorage(t, storageRoot, false)
	seedBaseRoot(t, baseRoot)
	quoted, err := json.Marshal(baseRoot)
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"base_root": ` + string(quoted) + `, "window_width": "wide", "launcher_launch_count": -1}`
	if err := os.WriteFile(filepath.Join(storageRoot, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	Cleanup(storageRoot, Options{DeleteBackups: true})

	assertExists(t, filepath.Join(baseRoot, "backups"), false)
	assertExists(t, filepath.Join(baseRoot, "versions"), true)
}

func TestRunUsesAppDataStorageFolder(t *testing.T) {
	f := newFixture(t)
	// A sibling folder named after another executable must be left alone.
	other := filepath.Join(f.appData, "other.exe")
	writeFile(t, filepath.Join(other, "config.json"))

	Run(Options{DeleteInstallers: true, DeleteBackups: true, DeleteVersions: true})

	for _, dir := range []string{"installers", "backups", "versions"} {
		assertExists(t, filepath.Join(f.baseRoot, dir), false)
	}
	assertUserFilesRemoved(t, f.storageRoot)
	assertExists(t, filepath.Join(f.storageRoot, "logs", "x"), true)
	assertExists(t, filepath.Join(other, "config.json"), true)
}
