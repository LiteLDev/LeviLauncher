package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// testSecondVolume is a directory on another NTFS volume than the temporary
// directory, for the tests that migrate across volumes; they are skipped
// without it. Tests create unique directories in it and remove them.
var testSecondVolume = os.Getenv("LEVILAUNCHER_TEST_SECOND_VOLUME")

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
		releaseLock()
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
	if exists(apppath.ConfigPath()) {
		t.Fatal("a fresh start must not write config.json")
	}
	// It records that the current layout is in use, so that legacy data an
	// older launcher writes later does not take precedence over it.
	rec := readRecord()
	if rec == nil || len(rec.Moves) != 0 {
		t.Fatalf("record = %+v, want one without moves", rec)
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
			// Nothing of the legacy layout is deleted; its config.json is
			// renamed so that the next start does not migrate again.
			if exists(filepath.Join(e.legacy, "config.json")) {
				t.Error("legacy config.json still marks the legacy layout as not migrated")
			}
			if !exists(filepath.Join(e.legacy, "config.migrated.json")) {
				t.Error("legacy config.json was not kept as config.migrated.json")
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

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(read(t, path)), &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

// A current layout that exists without a migration record was made without
// the user's data, for example by a start that skipped the migration: the
// legacy data is migrated into it and its settings, WebView2 profile and
// accounts take precedence.
func TestPrepareMigratesIntoAnExistingCurrentLayout(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	cfg, _ := json.Marshal(map[string]any{"base_root": e.legacy, "window_width": 1200, "legacy_only": "kept"})
	write(t, filepath.Join(e.legacy, "config.json"), string(cfg))
	local := filepath.Join(e.local, "LeviLauncher")
	write(t, apppath.ConfigPath(), `{"base_root":"","window_width":1024,"current_only":"kept"}`)
	write(t, filepath.Join(local, "WebView2", "EBWebView", "Default", "fresh"), "fresh profile")
	write(t, filepath.Join(local, "Accounts", "device.dpapi"), "fresh device")
	write(t, filepath.Join(local, "Cache", "gamertags.json"), `{}`)
	if err := os.MkdirAll(filepath.Join(local, "Instances"), 0o755); err != nil {
		t.Fatal(err)
	}

	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	for path, want := range map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):                               `{"name":"A"}`,
		filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(local, "WebView2", "EBWebView-before-migration", "Default", "fresh"):   "fresh profile",
		filepath.Join(local, "Accounts", "device.dpapi"):                                     "device",
		filepath.Join(local, "Accounts-before-migration", "device.dpapi"):                    "fresh device",
		filepath.Join(local, "Cache", "gamertags.json"):                                      `{"1":"Steve"}`,
		filepath.Join(local, "Cache", "gamertags-before-migration.json"):                     `{}`,
		filepath.Join(e.legacy, "config.migrated.json"):                                      string(cfg),
	} {
		if got := read(t, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	migrated := readJSON(t, apppath.ConfigPath())
	if _, ok := migrated["base_root"]; ok {
		t.Errorf("default data root kept base_root: %v", migrated)
	}
	if migrated["window_width"] != float64(1200) || migrated["legacy_only"] != "kept" || migrated["current_only"] != "kept" {
		t.Errorf("config not merged with the legacy members winning: %v", migrated)
	}
	if exists(filepath.Join(e.roaming, "LeviLauncher", "config-before-migration.json")) {
		t.Error("the replaced config.json was left behind after the commit")
	}

	recordLoaded = false
	if outcome, err := Prepare(); err != nil || outcome != Current {
		t.Fatalf("second Prepare = %v, %v", outcome, err)
	}
}

// After a migration, legacy data appears again only when an old version runs
// again: it is migrated too, but the current data takes precedence.
func TestPrepareMigratesLegacyDataThatReappears(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v", outcome, err)
	}
	write(t, apppath.ConfigPath(), `{"window_width":1500}`)

	// The old version runs again and writes its defaults and a new instance.
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":800,"old_only":true}`)
	write(t, filepath.Join(e.legacy, "versions", "B", "version.json"), `{"name":"B"}`)
	write(t, filepath.Join(e.legacy, "EBWebView", "Default", "old"), "old profile")
	recordLoaded = false
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("second Prepare = %v, %v", outcome, err)
	}
	local := filepath.Join(e.local, "LeviLauncher")
	migrated := readJSON(t, apppath.ConfigPath())
	if migrated["window_width"] != float64(1500) || migrated["old_only"] != true {
		t.Errorf("config not merged with the current members winning: %v", migrated)
	}
	for path, want := range map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):                               `{"name":"A"}`,
		filepath.Join(local, "Instances", "B", "version.json"):                               `{"name":"B"}`,
		filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(e.legacy, "EBWebView", "Default", "old"):                               "old profile",
		filepath.Join(e.legacy, "config.migrated.json"):                                      `{"window_width":900}`,
		filepath.Join(e.legacy, "config.migrated-2.json"):                                    `{"window_width":800,"old_only":true}`,
	} {
		if got := read(t, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	if exists(filepath.Join(local, "WebView2", "EBWebView-before-migration")) {
		t.Error("the current WebView2 profile was replaced by the legacy one")
	}
	// The current profile wins: the old version's profile is not merged into
	// it (and copied again at every such start), it stays where it was.
	if exists(filepath.Join(local, "WebView2", "EBWebView", "Default-legacy")) {
		t.Error("the old version's WebView2 profile was merged into the current one")
	}
	recordLoaded = false
	for _, legacyPath := range []string{
		filepath.Join(e.legacy, "versions", "A"),
		filepath.Join(e.legacy, "versions", "B"),
	} {
		if got := TranslateLegacyPath(legacyPath); samePath(got, legacyPath) {
			t.Errorf("TranslateLegacyPath(%s) lost its move", legacyPath)
		}
	}
}

// What a migration leaves in the legacy directory does not start another
// one: config.migrated.json, entries copied across volumes, and a data root
// kept in place, whose backups were renamed to Backups.
func TestPrepareIgnoresAMigratedLegacyDirectory(t *testing.T) {
	e := setup(t)
	write(t, filepath.Join(e.legacy, "config.migrated.json"), `{}`)
	write(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device")
	write(t, filepath.Join(e.legacy, "EBWebView", "Default", "x"), "x")
	write(t, filepath.Join(e.legacy, "logs", "startup.log"), "log")
	write(t, filepath.Join(e.legacy, "Instances", "A", "version.json"), `{}`)
	write(t, filepath.Join(e.legacy, "Backups", "Worlds", "W", "W_1.mcworld"), "world")
	// The data root was kept in place, across volumes from %LOCALAPPDATA%.
	write(t, apppath.ConfigPath(), `{"base_root":`+quote(e.legacy)+`}`)
	rec, _ := json.Marshal(Record{Format: recordFormat, LegacyDir: e.legacy, BaseRoot: e.legacy})
	write(t, apppath.MigrationPath(), string(rec))
	before := read(t, apppath.ConfigPath())

	if outcome, err := Prepare(); err != nil || outcome != Current {
		t.Fatalf("Prepare = %v, %v; want Current", outcome, err)
	}
	if read(t, apppath.ConfigPath()) != before || read(t, apppath.MigrationPath()) != string(rec) {
		t.Error("a migrated legacy directory was migrated again")
	}
}

// An undone migration restores a current config.json, a migration record and
// entries it moved aside.
func TestPrepareUndoRestoresTheExistingCurrentLayout(t *testing.T) {
	for _, withRecord := range []bool{false, true} {
		name := "without record"
		if withRecord {
			name = "with record"
		}
		t.Run(name, func(t *testing.T) {
			e := setup(t)
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
			local := filepath.Join(e.local, "LeviLauncher")
			const currentConfig = `{"window_width":1500}`
			write(t, apppath.ConfigPath(), currentConfig)
			write(t, filepath.Join(local, "Accounts", "device.dpapi"), "fresh device")
			write(t, filepath.Join(local, "WebView2", "EBWebView", "Default", "fresh"), "fresh profile")
			record := ""
			if withRecord {
				rec, _ := json.Marshal(Record{Format: recordFormat, LegacyDir: e.legacy, Moves: []Move{{From: `C:\old`, To: `C:\new`}}})
				record = string(rec)
				write(t, apppath.MigrationPath(), record)
			}
			// An instance in use: its directory cannot be renamed.
			f, err := os.Open(filepath.Join(e.legacy, "versions", "A", "Minecraft.Windows.exe"))
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := Prepare()
			f.Close()
			// Without a record the user's data is in the legacy layout, which
			// the launcher then runs on. After a migration it is in the
			// current one, which stays in use.
			want := Legacy
			if withRecord {
				want = Deferred
			}
			if outcome != want || err == nil {
				t.Fatalf("Prepare = %v, %v; want %v with an error", outcome, err, want)
			}
			if withRecord && apppath.LegacyMode() != "" {
				t.Fatal("legacy mode although the current layout holds the data")
			}
			apppath.SetLegacyMode("")
			for path, want := range map[string]string{
				apppath.ConfigPath():                                              currentConfig,
				filepath.Join(local, "Accounts", "device.dpapi"):                  "fresh device",
				filepath.Join(local, "WebView2", "EBWebView", "Default", "fresh"): "fresh profile",
				filepath.Join(e.legacy, "config.json"):                            `{"window_width":900}`,
				filepath.Join(e.legacy, "microsoft-account", "device.dpapi"):      "device",
				filepath.Join(e.legacy, "versions", "A", "version.json"):          `{"name":"A"}`,
			} {
				if got := read(t, path); got != want {
					t.Errorf("%s = %q, want %q", path, got, want)
				}
			}
			if withRecord && read(t, apppath.MigrationPath()) != record {
				t.Error("the migration record was not restored")
			}
			if !withRecord && exists(apppath.MigrationPath()) {
				t.Error("an undone migration left a record")
			}
			for _, p := range []string{
				filepath.Join(local, "Accounts-before-migration"),
				filepath.Join(local, "WebView2", "EBWebView-before-migration"),
				filepath.Join(e.roaming, "LeviLauncher", "config-before-migration.json"),
				filepath.Join(local, "storage-migration-before-migration.json"),
				filepath.Join(e.legacy, "config.migrated.json"),
			} {
				if exists(p) {
					t.Errorf("%s left after the undo", p)
				}
			}
		})
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
