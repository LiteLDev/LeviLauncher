package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

type env struct {
	roaming, local, legacy string
}

func setup(t *testing.T) env {
	t.Helper()
	root := t.TempDir()
	e := env{roaming: filepath.Join(root, "Roaming"), local: filepath.Join(root, "Local")}
	for _, d := range []string{e.roaming, e.local} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("APPDATA", e.roaming)
	t.Setenv("LOCALAPPDATA", e.local)
	e.legacy = apppath.LegacyDir()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	recordMu.Lock()
	recordLoaded = false
	recordMu.Unlock()
	t.Cleanup(func() {
		apppath.SetLegacyMode("")
		recordMu.Lock()
		recordLoaded = false
		recordMu.Unlock()
	})
	return e
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func seedLegacy(t *testing.T, dir, base string) {
	t.Helper()
	write(t, filepath.Join(base, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(base, "versions", "A", "Minecraft.Windows.exe"), "exe")
	write(t, filepath.Join(base, "backups", "worlds", "W", "W_1.mcworld"), "world")
	write(t, filepath.Join(base, "backups", "instances", "A", "A_1.zip"), "zip")
	write(t, filepath.Join(base, "installers", "1.21.msixvc"), "pkg")
	write(t, filepath.Join(base, "updates", "1.0.8.zip"), "update")
	write(t, filepath.Join(dir, "microsoft-account", "device.dpapi"), "device")
	write(t, filepath.Join(dir, "EBWebView", "Default", "Local Storage", "leveldb"), "storage")
	write(t, filepath.Join(dir, "logs", "startup.log"), "old log")
	write(t, filepath.Join(dir, "user_gamertag_map.json"), `{"1":"Steve"}`)
	write(t, filepath.Join(dir, "gdk_pending_versions.json"), `[]`)
}

func TestPrepareWithoutLegacyLayout(t *testing.T) {
	e := setup(t)
	outcome, err := Prepare()
	if err != nil || outcome != Current {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	if exists(apppath.ConfigPath()) || exists(apppath.MigrationPath()) {
		t.Fatal("a fresh start must not write config.json or a migration record")
	}
	_ = e
}

func TestPrepareMigratesDefaultLayout(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseRoot func(env) string
	}{
		{"no base_root", func(env) string { return "" }},
		// ResetBaseRoot used to write the default data root explicitly.
		{"base_root is the legacy directory", func(e env) string { return e.legacy }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			seedLegacy(t, e.legacy, e.legacy)
			cfg, _ := json.Marshal(map[string]any{"base_root": tc.baseRoot(e), "window_width": 900, "future_key": "kept"})
			write(t, filepath.Join(e.legacy, "config.json"), string(cfg))
			// The current log is already open when the migration runs.
			write(t, filepath.Join(e.local, "LeviLauncher", "Logs", "startup.log"), "new log")

			outcome, err := Prepare()
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v", outcome, err)
			}
			local := filepath.Join(e.local, "LeviLauncher")
			for path, want := range map[string]string{
				filepath.Join(local, "Instances", "A", "version.json"):                               `{"name":"A"}`,
				filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"):                        "world",
				filepath.Join(local, "Backups", "Instances", "A", "A_1.zip"):                         "zip",
				filepath.Join(local, "Cache", "Installers", "1.21.msixvc"):                           "pkg",
				filepath.Join(local, "Cache", "Updates", "1.0.8.zip"):                                "update",
				filepath.Join(local, "Cache", "gamertags.json"):                                      `{"1":"Steve"}`,
				filepath.Join(local, "Cache", "gdk-pending-versions.json"):                           `[]`,
				filepath.Join(local, "Accounts", "device.dpapi"):                                     "device",
				filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
				filepath.Join(local, "Logs", "startup.log"):                                          "new log",
				filepath.Join(local, "Logs", "startup-legacy.log"):                                   "old log",
			} {
				if got := read(t, path); got != want {
					t.Errorf("%s = %q, want %q", path, got, want)
				}
			}
			if got := actualName(filepath.Join(local, "Backups", "worlds")); got != "Worlds" {
				t.Errorf("backups\\worlds kept its name %q", got)
			}

			var migrated map[string]any
			if err := json.Unmarshal([]byte(read(t, filepath.Join(e.roaming, "LeviLauncher", "config.json"))), &migrated); err != nil {
				t.Fatal(err)
			}
			if _, ok := migrated["base_root"]; ok {
				t.Errorf("default data root kept base_root: %v", migrated)
			}
			if migrated["future_key"] != "kept" || migrated["window_width"] != float64(900) {
				t.Errorf("config members lost: %v", migrated)
			}
			// Nothing of the legacy layout is deleted.
			if !exists(filepath.Join(e.legacy, "config.json")) {
				t.Error("legacy config.json was removed")
			}
			if exists(apppath.MigrationJournalPath()) {
				t.Error("journal left after a committed migration")
			}
			if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "A")); !samePath(got, filepath.Join(local, "Instances", "A")) {
				t.Errorf("TranslateLegacyPath = %s", got)
			}
			if apppath.LegacyMode() != "" {
				t.Error("legacy mode after a migration")
			}

			// The next start finds the current layout.
			recordLoaded = false
			if outcome, err := Prepare(); err != nil || outcome != Current {
				t.Fatalf("second Prepare = %v, %v", outcome, err)
			}
		})
	}
}

func TestPrepareRenamesCustomDataRootInPlace(t *testing.T) {
	e := setup(t)
	base := filepath.Join(filepath.Dir(e.roaming), "Data")
	seedLegacy(t, e.legacy, base)
	cfg, _ := json.Marshal(map[string]any{"base_root": base})
	write(t, filepath.Join(e.legacy, "config.json"), string(cfg))

	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	if got := read(t, filepath.Join(base, "Instances", "A", "version.json")); got != `{"name":"A"}` {
		t.Errorf("instance = %q", got)
	}
	if got := read(t, filepath.Join(base, "Cache", "Installers", "1.21.msixvc")); got != "pkg" {
		t.Errorf("installer = %q", got)
	}
	if got := actualName(filepath.Join(base, "backups")); got != "Backups" {
		t.Errorf("backups kept its name %q", got)
	}
	var migrated map[string]any
	_ = json.Unmarshal([]byte(read(t, apppath.ConfigPath())), &migrated)
	if migrated["base_root"] != base {
		t.Errorf("base_root = %v, want %s", migrated["base_root"], base)
	}
}

func TestPrepareUndoesEverythingWhenAnInstanceIsInUse(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	// A directory with an open file cannot be renamed on Windows, as when the
	// game runs from an instance. Hold one of the last entries the migration
	// moves, so that the entries before it are moved and must be undone.
	f, err := os.Open(filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb"))
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := Prepare()
	f.Close()
	if outcome != Legacy || err == nil {
		t.Fatalf("Prepare = %v, %v; want Legacy with an error", outcome, err)
	}
	if apppath.LegacyMode() == "" || !samePath(apppath.ConfigDir(), e.legacy) {
		t.Fatalf("legacy mode not set: %q", apppath.LegacyMode())
	}
	for _, p := range []string{
		filepath.Join(e.legacy, "versions", "A", "version.json"),
		filepath.Join(e.legacy, "backups", "worlds", "W", "W_1.mcworld"),
		filepath.Join(e.legacy, "installers", "1.21.msixvc"),
		filepath.Join(e.legacy, "microsoft-account", "device.dpapi"),
		filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb"),
		filepath.Join(e.legacy, "user_gamertag_map.json"),
	} {
		if !exists(p) {
			t.Errorf("%s was not restored", p)
		}
	}
	apppath.SetLegacyMode("")
	if exists(apppath.ConfigPath()) || exists(apppath.MigrationJournalPath()) || exists(filepath.Join(e.local, "LeviLauncher", "Accounts")) {
		t.Error("an undone migration left files of the current layout")
	}

	// Once the game is closed, the next start migrates.
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("retry Prepare = %v, %v", outcome, err)
	}
}

func TestPrepareUndoesAnInterruptedMigration(t *testing.T) {
	e := setup(t)
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{}`)
	from := filepath.Join(e.legacy, "versions")
	to := filepath.Join(e.local, "LeviLauncher", "Instances")
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	journal := `{"op":"Begin"}` + "\n" +
		`{"op":"Move","path":` + quote(to) + `,"from":` + quote(from) + `}` + "\n" +
		`{"op":"Cre` // cut off by the crash
	write(t, apppath.MigrationJournalPath(), journal)

	outcome, err := Prepare()
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	// Undone, then migrated again from the restored legacy layout.
	if got := read(t, filepath.Join(to, "A", "version.json")); got != `{}` {
		t.Errorf("instance = %q", got)
	}
	if exists(from) {
		t.Error("legacy versions left behind")
	}
}

func TestPrepareKeepsBothEntriesOnConflict(t *testing.T) {
	e := setup(t)
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"legacy":true}`)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	instances := filepath.Join(e.local, "LeviLauncher", "Instances")
	write(t, filepath.Join(instances, "A", "version.json"), `{"current":true}`)

	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	if got := read(t, filepath.Join(instances, "A", "version.json")); got != `{"current":true}` {
		t.Errorf("existing instance overwritten: %q", got)
	}
	if got := read(t, filepath.Join(instances, "A-legacy", "version.json")); got != `{"legacy":true}` {
		t.Errorf("legacy instance = %q", got)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestPrepareWaitsForAnUnavailableDataRoot(t *testing.T) {
	e := setup(t)
	base := filepath.Join(filepath.Dir(e.roaming), "Unplugged")
	cfg, _ := json.Marshal(map[string]any{"base_root": base})
	write(t, filepath.Join(e.legacy, "config.json"), string(cfg))

	if outcome, err := Prepare(); outcome != Legacy || err == nil {
		t.Fatalf("Prepare = %v, %v; want Legacy", outcome, err)
	}
	apppath.SetLegacyMode("")
	if exists(apppath.ConfigPath()) {
		t.Fatal("migrated without the data root")
	}
	// The drive is back.
	write(t, filepath.Join(base, "versions", "A", "version.json"), `{}`)
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	if !exists(filepath.Join(base, "Instances", "A", "version.json")) {
		t.Fatal("instance not renamed")
	}
}
