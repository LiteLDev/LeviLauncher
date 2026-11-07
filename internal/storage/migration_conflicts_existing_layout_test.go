package storage

// Regression tests for the group "conflicts-existing-layout": legacy data
// migrated into a current layout that already exists, name collisions, and
// files where the layout has directories (or the other way round). The
// expected outcomes are those of internal/apppath/README.md, section
// "从旧布局迁移".

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// ---- helpers (prefixed "cel" to stay clear of migrate_test.go) -------------

// celSnapshot maps every entry below root (relative path) to its content,
// "<dir>" for a directory.
func celSnapshot(root string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			out[rel] = "<unreadable>"
			return nil
		}
		out[rel] = string(b)
		return nil
	})
	return out
}

func celLogTree(t *testing.T, label, root string) {
	t.Helper()
	snap := celSnapshot(root)
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := snap[k]
		if len(v) > 40 {
			v = v[:40] + "..."
		}
		b.WriteString("  " + k + " = " + strings.ReplaceAll(v, "\n", " ") + "\n")
	}
	t.Logf("tree %s (%s):\n%s", label, root, b.String())
}

// celSameSnapshot fails when anything below the snapshot root changed.
func celSameSnapshot(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if got, ok := after[k]; !ok {
			t.Errorf("%s: %s disappeared", what, k)
		} else if got != v {
			t.Errorf("%s: %s changed %q -> %q", what, k, v, got)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s appeared", what, k)
		}
	}
}

// celNoContentLost fails when the content of a file in before is found in
// fewer files after: the migration moves files, it never deletes one. Files
// whose relative path is in rewritten (config.json the migration merges) are
// not counted.
func celNoContentLost(t *testing.T, before, after map[string]string, rewritten ...string) {
	t.Helper()
	skip := map[string]bool{}
	for _, r := range rewritten {
		skip[strings.ToLower(r)] = true
	}
	count := func(s map[string]string, honourSkip bool) map[string]int {
		c := map[string]int{}
		for k, v := range s {
			if v == "<dir>" || v == "<unreadable>" || (honourSkip && skip[strings.ToLower(k)]) {
				continue
			}
			c[v]++
		}
		return c
	}
	want, got := count(before, true), count(after, false)
	for v, n := range want {
		if got[v] < n {
			t.Errorf("file content %q: %d before the migration, %d after (deleted)", v, n, got[v])
		}
	}
}

func celResetRecord() {
	recordMu.Lock()
	recordLoaded = false
	recordMu.Unlock()
}

// celRoot is the directory holding Roaming and Local of the test.
func celRoot(e env) string { return filepath.Dir(e.roaming) }

// celStartup does what main.go does after Prepare: EnsureDirs, config.Load
// (a fresh read of apppath.ConfigPath(), which sets the data root).
func celStartup(t *testing.T) config.AppConfig {
	t.Helper()
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	if err := apppath.EnsureDirs(); err != nil {
		t.Errorf("apppath.EnsureDirs failed (the launcher would not start): %v", err)
	}
	c, err := config.Reload()
	if err != nil {
		t.Errorf("config.Reload failed (the launcher would not start): %v", err)
	}
	return c
}

// celListNames lists instances as mcservice.ListVersionMetas does.
func celListNames(t *testing.T) map[string]versions.VersionMeta {
	t.Helper()
	vdir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatalf("apppath.InstancesDir: %v", err)
	}
	metas, err := versions.ScanVersions(vdir)
	if err != nil {
		t.Fatalf("versions.ScanVersions(%s): %v", vdir, err)
	}
	out := map[string]versions.VersionMeta{}
	for _, m := range metas {
		if _, dup := out[m.Name]; dup {
			t.Errorf("two listed instances are named %q", m.Name)
		}
		out[m.Name] = m
	}
	return out
}

// celCheckE1Reachable checks that every item seedLegacy wrote is readable,
// byte-identical, where the launcher's getters look for it in this run.
func celCheckE1Reachable(t *testing.T) {
	t.Helper()
	vdir, err := apppath.InstancesDir()
	if err != nil {
		t.Errorf("InstancesDir() failed: %v", err)
	} else {
		a := filepath.Join(vdir, "A")
		if m, err := versions.ReadMeta(a); err != nil || m.Name != "A" {
			t.Errorf("ReadMeta(%s) = %+v, %v", a, m, err)
		}
		if got, err := os.ReadFile(filepath.Join(a, "Minecraft.Windows.exe")); err != nil || string(got) != "exe" {
			t.Errorf("A\\Minecraft.Windows.exe = %q, %v", got, err)
		}
		if names := celListNames(t); len(names) == 0 {
			t.Errorf("no instance listed in %s", vdir)
		} else if _, ok := names["A"]; !ok {
			t.Errorf("instance A not listed: %v", names)
		}
	}
	inst, err := apppath.InstallersDir()
	if err != nil {
		t.Errorf("InstallersDir() failed: %v", err)
	}
	for p, want := range map[string]string{
		filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"):                             "world",
		filepath.Join(apppath.InstanceBackupsDir(), "A", "A_1.zip"):                              "zip",
		filepath.Join(inst, "1.21.msixvc"):                                                       "pkg",
		filepath.Join(apppath.UpdatesDir(), "1.0.8.zip"):                                         "update",
		apppath.GamertagsPath():                                                                  `{"1":"Steve"}`,
		apppath.GDKPendingVersionsPath():                                                         `[]`,
		filepath.Join(apppath.AccountsDir(), "device.dpapi"):                                     "device",
		filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
	} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != want {
			t.Errorf("not reachable through the getters: %s = %q, %v; want %q", p, b, err, want)
		}
	}
}

// celSecondPrepareNoop runs Prepare as the next start would and checks that
// it finds the current layout and changes nothing.
func celSecondPrepareNoop(t *testing.T, e env) {
	t.Helper()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	celResetRecord()
	root := celRoot(e)
	before := celSnapshot(root)
	outcome, err := Prepare()
	if err != nil || outcome != Current {
		t.Errorf("second Prepare = %v, %v; want Current, nil", outcome, err)
	}
	celSameSnapshot(t, "second Prepare", before, celSnapshot(root))
}

func celSeedE1(t *testing.T, e env, cfg string) {
	t.Helper()
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), cfg)
}

// celTranslatesTo checks that TranslateLegacyPath maps legacy to an existing
// path holding want in file (relative to the translated directory).
func celTranslatesTo(t *testing.T, legacy, wantDir, file, want string) {
	t.Helper()
	celResetRecord()
	got := TranslateLegacyPath(legacy)
	if !samePath(got, wantDir) {
		t.Errorf("TranslateLegacyPath(%s) = %s, want %s", legacy, got, wantDir)
	}
	if file != "" {
		if b, err := os.ReadFile(filepath.Join(got, file)); err != nil || string(b) != want {
			t.Errorf("translated %s\\%s = %q, %v; want %q", got, file, b, err, want)
		}
	}
}

// ---- K1: a current layout made by an earlier run, no migration record -----

func TestMigration_K1_NewLayoutFromEarlierRun(t *testing.T) {
	t.Run("logs-webview-accounts-cache", func(t *testing.T) {
		e := setup(t)
		celSeedE1(t, e, `{"window_width":900,"legacy_only":"kept"}`)
		local := filepath.Join(e.local, "LeviLauncher")
		// A start that ran on the legacy layout, or a tool, made these.
		write(t, filepath.Join(local, "Logs", "startup.log"), "new log")
		write(t, filepath.Join(local, "WebView2", "EBWebView", "Default", "x"), "fresh profile")
		write(t, filepath.Join(local, "Accounts", "device.dpapi"), "fresh device")
		write(t, filepath.Join(local, "Cache", "gdk-pending-versions.json"), `["fresh"]`)
		write(t, apppath.ConfigPath(), `{"window_width":1024,"current_only":"kept"}`)
		before := celSnapshot(celRoot(e))

		outcome, err := Prepare()
		celLogTree(t, "after Prepare", celRoot(e))
		if err != nil || outcome != Migrated {
			t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
		}
		celNoContentLost(t, before, celSnapshot(celRoot(e)), `Roaming\LeviLauncher\config.json`, `Local\LeviLauncher\storage-migration.json`)
		for p, want := range map[string]string{
			filepath.Join(local, "WebView2", "EBWebView-before-migration", "Default", "x"): "fresh profile",
			filepath.Join(local, "Accounts-before-migration", "device.dpapi"):              "fresh device",
			filepath.Join(local, "Cache", "gdk-pending-versions-before-migration.json"):    `["fresh"]`,
			filepath.Join(local, "Logs", "startup.log"):                                    "new log",
			filepath.Join(local, "Logs", "startup-legacy.log"):                             "old log",
			filepath.Join(e.legacy, "config.migrated.json"):                                `{"window_width":900,"legacy_only":"kept"}`,
		} {
			if got := read(t, p); got != want {
				t.Errorf("%s = %q, want %q", p, got, want)
			}
		}
		// No migration record before: the legacy settings win, keys of
		// either side are kept.
		cfg := readJSON(t, apppath.ConfigPath())
		if cfg["window_width"] != float64(900) || cfg["legacy_only"] != "kept" || cfg["current_only"] != "kept" {
			t.Errorf("config = %v", cfg)
		}
		if _, ok := cfg["base_root"]; ok {
			t.Errorf("default data root got base_root: %v", cfg)
		}
		c := celStartup(t)
		if c.WindowWidth != 900 {
			t.Errorf("config.Load window_width = %d, want 900", c.WindowWidth)
		}
		celCheckE1Reachable(t)
		celTranslatesTo(t, filepath.Join(e.legacy, "versions", "A"), filepath.Join(local, "Instances", "A"), "Minecraft.Windows.exe", "exe")
		celSecondPrepareNoop(t, e)
	})
	t.Run("legacy-mode-logs-only", func(t *testing.T) {
		e := setup(t)
		celSeedE1(t, e, `{}`)
		local := filepath.Join(e.local, "LeviLauncher")
		write(t, filepath.Join(local, "Logs", "startup.log"), "new log")
		before := celSnapshot(celRoot(e))

		outcome, err := Prepare()
		celLogTree(t, "after Prepare", celRoot(e))
		if err != nil || outcome != Migrated {
			t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
		}
		celNoContentLost(t, before, celSnapshot(celRoot(e)))
		if got := read(t, filepath.Join(local, "Logs", "startup-legacy.log")); got != "old log" {
			t.Errorf("legacy log = %q", got)
		}
		if got := read(t, filepath.Join(local, "Logs", "startup.log")); got != "new log" {
			t.Errorf("current log = %q", got)
		}
		for _, p := range []string{
			filepath.Join(local, "WebView2", "EBWebView-before-migration"),
			filepath.Join(local, "Accounts-before-migration"),
		} {
			if exists(p) {
				t.Errorf("%s without a conflict", p)
			}
		}
		celStartup(t)
		celCheckE1Reachable(t)
		celSecondPrepareNoop(t, e)
	})
}

// ---- K2: an instance name taken in the current layout -----------------------

func TestMigration_K2_InstancesNameCollision(t *testing.T) {
	e := setup(t)
	local := filepath.Join(e.local, "LeviLauncher")
	instances := filepath.Join(local, "Instances")
	// First start of the new version: migrates A.
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.21.0","type":"release"}`)
	write(t, filepath.Join(e.legacy, "versions", "A", "x"), "new")
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("first Prepare = %v, %v", outcome, err)
	}
	// Downgrade: the old version installs an instance of the same name, once
	// per round.
	for round, wantDir := range []string{"A-legacy", "A-legacy-2"} {
		payload := "old" + string(rune('0'+round))
		write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":1024,"base_root":""}`)
		write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A","gameVersion":"1.20.0","type":"release"}`)
		write(t, filepath.Join(e.legacy, "versions", "A", "x"), payload)
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
		celResetRecord()
		before := celSnapshot(celRoot(e))

		outcome, err := Prepare()
		celLogTree(t, "after Prepare "+wantDir, celRoot(e))
		if err != nil || outcome != Migrated {
			t.Fatalf("Prepare = %v, %v", outcome, err)
		}
		celNoContentLost(t, before, celSnapshot(celRoot(e)), `Roaming\LeviLauncher\config.json`, `Local\LeviLauncher\storage-migration.json`)
		if got := read(t, filepath.Join(instances, "A", "x")); got != "new" {
			t.Errorf("Instances\\A\\x = %q, want new", got)
		}
		if got := read(t, filepath.Join(instances, wantDir, "x")); got != payload {
			t.Errorf("Instances\\%s\\x = %q, want %q", wantDir, got, payload)
		}
		// A record existed: the current settings win.
		if cfg := readJSON(t, apppath.ConfigPath()); cfg["window_width"] != float64(900) {
			t.Errorf("config = %v", cfg)
		}
		celStartup(t)
		names := celListNames(t)
		t.Logf("listed: %v", names)
		if len(names) != round+2 {
			t.Errorf("listed %d instances, want %d", len(names), round+2)
		}
		if m, ok := names["A"]; !ok || m.GameVersion != "1.21.0" {
			t.Errorf("A listed as %+v (%v), want gameVersion 1.21.0", m, ok)
		}
		if m, ok := names[wantDir]; !ok || m.GameVersion != "1.20.0" {
			t.Errorf("%s listed as %+v (%v), want gameVersion 1.20.0", wantDir, m, ok)
		}
		// Looking an instance up by its listed name opens its own folder.
		if m, err := versions.ReadMeta(filepath.Join(instances, wantDir)); err != nil || m.Name != wantDir || m.GameVersion != "1.20.0" {
			t.Errorf("ReadMeta(%s) = %+v, %v", wantDir, m, err)
		}
		if m, err := versions.ReadMeta(filepath.Join(instances, "A")); err != nil || m.Name != "A" || m.GameVersion != "1.21.0" {
			t.Errorf("ReadMeta(A) = %+v, %v", m, err)
		}
		// The registration of the old version's A was at versions\A: it
		// now names the -legacy folder.
		celTranslatesTo(t, filepath.Join(e.legacy, "versions", "A"), filepath.Join(instances, wantDir), "x", payload)
	}
	if got := read(t, filepath.Join(e.legacy, "config.migrated.json")); got != `{"window_width":900}` {
		t.Errorf("config.migrated.json = %q", got)
	}
	if got := read(t, filepath.Join(e.legacy, "config.migrated-3.json")); got != `{"window_width":1024,"base_root":""}` {
		t.Errorf("config.migrated-3.json = %q", got)
	}
	celSecondPrepareNoop(t, e)
}

// ---- K3: world and instance backups merge into existing ones ----------------

func TestMigration_K3_BackupsWorldsCollision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		top    string // existing backups directory in the local directory
		worlds string // existing world backups directory below it
	}{
		{"current-Worlds", "Backups", "Worlds"},
		// Launchers up to 0.0.9 kept their data in %LOCALAPPDATA%\LeviLauncher
		// under the legacy lowercase names.
		{"eraA-lowercase", "backups", "worlds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(local, tc.top, tc.worlds, "W1", "W1_a.mcworld"), "w1")
			if err := os.MkdirAll(filepath.Join(local, tc.top, "Instances"), 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
			write(t, filepath.Join(e.legacy, "backups", "worlds", "W2", "W2_b.mcworld"), "w2")
			write(t, filepath.Join(e.legacy, "backups", "instances", "A", "A.zip"), "azip")
			before := celSnapshot(celRoot(e))

			outcome, err := Prepare()
			celLogTree(t, "after Prepare", celRoot(e))
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v", outcome, err)
			}
			celNoContentLost(t, before, celSnapshot(celRoot(e)))
			c := celStartup(t)
			if c.WindowWidth != 900 {
				t.Errorf("window_width = %d", c.WindowWidth)
			}
			for p, want := range map[string]string{
				filepath.Join(apppath.WorldBackupsDir(), "W1", "W1_a.mcworld"): "w1",
				filepath.Join(apppath.WorldBackupsDir(), "W2", "W2_b.mcworld"): "w2",
				filepath.Join(apppath.InstanceBackupsDir(), "A", "A.zip"):      "azip",
			} {
				if b, err := os.ReadFile(p); err != nil || string(b) != want {
					t.Errorf("backup not visible at %s: %q, %v", p, b, err)
				}
			}
			for p, want := range map[string]string{
				filepath.Join(local, "Backups"):              "Backups",
				filepath.Join(local, "Backups", "Worlds"):    "Worlds",
				filepath.Join(local, "Backups", "Instances"): "Instances",
			} {
				if got := actualName(p); got != want {
					t.Errorf("%s is named %q, want %q", p, got, want)
				}
			}
			ents, _ := os.ReadDir(filepath.Join(local, "Backups"))
			for _, en := range ents {
				if strings.Contains(strings.ToLower(en.Name()), "-legacy") {
					t.Errorf("legacy backups landed in Backups\\%s, which the launcher does not read", en.Name())
				}
			}
			if exists(filepath.Join(e.legacy, "backups")) {
				t.Error("legacy backups directory still marks the legacy layout as not migrated")
			}
			celTranslatesTo(t, filepath.Join(e.legacy, "backups", "worlds", "W2"), filepath.Join(local, "Backups", "Worlds", "W2"), "W2_b.mcworld", "w2")
			celSecondPrepareNoop(t, e)
		})
	}
}

// ---- K4: instances named like directories of the layout -------------------

var celLayoutLikeNames = []string{
	"Instances", "Cache", "Backups", ".levilauncher", "Instances-legacy",
	".uwp-install-123", ".msixvc-staging-9", "versions", "Logs", "WebView2",
	"Accounts", "backups",
}

func TestMigration_K4_InstanceNamedLikeLayoutDirs(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "fresh"
		if existing {
			name = "into-existing-layout"
		}
		t.Run(name, func(t *testing.T) {
			e := setup(t)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(e.legacy, "config.json"), `{}`)
			for _, n := range celLayoutLikeNames {
				if n == "backups" {
					// Same name as "Backups" up to case: one folder on Windows.
					continue
				}
				write(t, filepath.Join(e.legacy, "versions", n, "version.json"), `{"name":`+quote(n)+`}`)
				write(t, filepath.Join(e.legacy, "versions", n, "payload"), "data-"+n)
			}
			if existing {
				write(t, filepath.Join(local, "Instances", "Other", "version.json"), `{"name":"Other"}`)
				write(t, filepath.Join(local, "Cache", "Installers", "keep.msixvc"), "cache-keep")
				write(t, filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"), "backup-keep")
			}
			layoutBefore := map[string]map[string]string{}
			for _, d := range []string{"Cache", "Backups"} {
				layoutBefore[d] = celSnapshot(filepath.Join(local, d))
			}
			before := celSnapshot(celRoot(e))

			outcome, err := Prepare()
			celLogTree(t, "after Prepare", celRoot(e))
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v", outcome, err)
			}
			celNoContentLost(t, before, celSnapshot(celRoot(e)))
			// Nothing of the instances landed in the layout directories.
			for d, snap := range layoutBefore {
				celSameSnapshot(t, d, snap, celSnapshot(filepath.Join(local, d)))
			}
			celStartup(t)
			vdir, err := apppath.InstancesDir()
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range celLayoutLikeNames {
				if n == "backups" {
					continue
				}
				p := filepath.Join(vdir, n, "payload")
				if got, err := os.ReadFile(p); err != nil || string(got) != "data-"+n {
					t.Errorf("%s = %q, %v", p, got, err)
				}
				if n != "Instances" && exists(filepath.Join(vdir, n+"-legacy")) {
					t.Errorf("unexpected %s-legacy", n)
				}
				celTranslatesTo(t, filepath.Join(e.legacy, "versions", n), filepath.Join(local, "Instances", n), "payload", "data-"+n)
			}
			names := celListNames(t)
			want := len(celLayoutLikeNames) - 1
			if existing {
				want++
				if _, ok := names["Other"]; !ok {
					t.Error("existing instance Other not listed")
				}
			}
			if len(names) != want {
				t.Errorf("listed %d instances, want %d: %v", len(names), want, names)
			}
			for _, n := range celLayoutLikeNames {
				if n == "backups" {
					continue
				}
				if _, ok := names[n]; !ok {
					t.Errorf("instance %q not listed", n)
				}
			}
			celSecondPrepareNoop(t, e)
		})
	}
}

// ---- K5: a file where the layout has a directory ---------------------------

func TestMigration_K5_FileWhereDirExpected(t *testing.T) {
	for _, stray := range []string{"Instances", "Cache", "WebView2", "Backups", "Accounts"} {
		t.Run(stray, func(t *testing.T) {
			e := setup(t)
			local := filepath.Join(e.local, "LeviLauncher")
			celSeedE1(t, e, `{"window_width":900}`)
			write(t, filepath.Join(local, stray), "stray file")
			before := celSnapshot(celRoot(e))

			outcome, err := Prepare()
			celLogTree(t, "after Prepare", celRoot(e))
			// README: the file is kept as "<name>-before-migration" and the
			// legacy data takes its place.
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v (legacy mode %q); want Migrated", outcome, err, apppath.LegacyMode())
			}
			celNoContentLost(t, before, celSnapshot(celRoot(e)))
			if got := read(t, filepath.Join(local, stray+"-before-migration")); got != "stray file" {
				t.Errorf("stray file = %q, want it kept as %s-before-migration", got, stray)
			}
			if !isDir(filepath.Join(local, stray)) {
				t.Errorf("%s is not a directory after the migration", stray)
			}
			if c := celStartup(t); c.WindowWidth != 900 {
				t.Errorf("window_width = %d", c.WindowWidth)
			}
			celCheckE1Reachable(t)
			celSecondPrepareNoop(t, e)
		})
	}
}

// ---- K6: a legacy data directory that is a file ---------------------------

func TestMigration_K6_LegacyEntryIsFile(t *testing.T) {
	for _, entry := range []string{"versions", "backups", "installers", "updates"} {
		t.Run(entry, func(t *testing.T) {
			e := setup(t)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
			write(t, filepath.Join(e.legacy, entry), "odd "+entry)
			if entry != "versions" {
				write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A"}`)
			}
			before := celSnapshot(celRoot(e))

			outcome, err := Prepare()
			celLogTree(t, "after Prepare", celRoot(e))
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			celNoContentLost(t, before, celSnapshot(celRoot(e)))
			// README: a file of a data directory's name is not legacy data;
			// it stays where it is.
			if got := read(t, filepath.Join(e.legacy, entry)); got != "odd "+entry {
				t.Errorf("legacy %s file = %q", entry, got)
			}
			if got := read(t, filepath.Join(e.legacy, "config.migrated.json")); got != `{"window_width":900}` {
				t.Errorf("config.migrated.json = %q", got)
			}
			if c := celStartup(t); c.WindowWidth != 900 {
				t.Errorf("window_width = %d", c.WindowWidth)
			}
			names := celListNames(t)
			if entry == "versions" {
				if len(names) != 0 {
					t.Errorf("listed %v", names)
				}
			} else {
				if _, ok := names["A"]; !ok || len(names) != 1 {
					t.Errorf("instance A not listed: %v", names)
				}
				celTranslatesTo(t, filepath.Join(e.legacy, "versions", "A"), filepath.Join(local, "Instances", "A"), "", "")
			}
			if _, err := apppath.InstallersDir(); err != nil {
				t.Errorf("InstallersDir: %v", err)
			}
			celSecondPrepareNoop(t, e)
		})
	}
}

// ---- K7: a record exists; an old version wrote config.json again ----------

func TestMigration_K7_RecordPresentCurrentWins(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		e := setup(t)
		local := filepath.Join(e.local, "LeviLauncher")
		write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
		write(t, filepath.Join(e.legacy, "versions", "Old", "version.json"), `{"name":"Old"}`)
		if outcome, err := Prepare(); err != nil || outcome != Migrated {
			t.Fatalf("first Prepare = %v, %v", outcome, err)
		}
		write(t, apppath.ConfigPath(), `{"window_width":1500,"x":1}`)
		write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":1024,"base_root":""}`)
		write(t, filepath.Join(e.legacy, "versions", "New", "version.json"), `{"name":"New"}`)
		celResetRecord()
		before := celSnapshot(celRoot(e))

		outcome, err := Prepare()
		celLogTree(t, "after Prepare", celRoot(e))
		if err != nil || outcome != Migrated {
			t.Fatalf("Prepare = %v, %v", outcome, err)
		}
		celNoContentLost(t, before, celSnapshot(celRoot(e)), `Roaming\LeviLauncher\config.json`, `Local\LeviLauncher\storage-migration.json`)
		cfg := readJSON(t, apppath.ConfigPath())
		if cfg["window_width"] != float64(1500) || cfg["x"] != float64(1) {
			t.Errorf("config = %v", cfg)
		}
		if br, ok := cfg["base_root"]; ok && br != "" {
			t.Errorf("base_root = %v, want the default", br)
		}
		if got := read(t, filepath.Join(e.legacy, "config.migrated.json")); got != `{"window_width":900}` {
			t.Errorf("config.migrated.json = %q", got)
		}
		if got := read(t, filepath.Join(e.legacy, "config.migrated-2.json")); got != `{"window_width":1024,"base_root":""}` {
			t.Errorf("config.migrated-2.json = %q", got)
		}
		c := celStartup(t)
		if c.WindowWidth != 1500 {
			t.Errorf("config.Load window_width = %d", c.WindowWidth)
		}
		names := celListNames(t)
		for _, n := range []string{"Old", "New"} {
			if _, ok := names[n]; !ok {
				t.Errorf("instance %s not listed: %v", n, names)
			}
		}
		if !exists(filepath.Join(local, "Instances", "New", "version.json")) && !exists(filepath.Join(local, "Instances", "New", ".levilauncher", "instance.json")) {
			t.Error("New not in Instances")
		}
		celTranslatesTo(t, filepath.Join(e.legacy, "versions", "New"), filepath.Join(local, "Instances", "New"), "", "")
		celTranslatesTo(t, filepath.Join(e.legacy, "versions", "Old"), filepath.Join(local, "Instances", "Old"), "", "")
		// The record keeps the moves of both migrations.
		rec := readRecord()
		if rec == nil || len(rec.Moves) < 2 {
			t.Errorf("record = %+v", rec)
		}
		celSecondPrepareNoop(t, e)
	})
	// The user moved the data root after the migration; the old version then
	// writes base_root "" (its default).
	t.Run("current-custom-base_root", func(t *testing.T) {
		e := setup(t)
		local := filepath.Join(e.local, "LeviLauncher")
		custom := filepath.Join(celRoot(e), "Custom")
		write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
		write(t, filepath.Join(e.legacy, "versions", "Old", "version.json"), `{"name":"Old"}`)
		if outcome, err := Prepare(); err != nil || outcome != Migrated {
			t.Fatalf("first Prepare = %v, %v", outcome, err)
		}
		write(t, filepath.Join(custom, "Instances", "Cur", "version.json"), `{"name":"Cur"}`)
		write(t, apppath.ConfigPath(), `{"window_width":1500,"x":1,"base_root":`+quote(custom)+`}`)
		write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":1024,"base_root":""}`)
		write(t, filepath.Join(e.legacy, "versions", "New", "version.json"), `{"name":"New"}`)
		celResetRecord()
		before := celSnapshot(celRoot(e))

		outcome, err := Prepare()
		celLogTree(t, "after Prepare", celRoot(e))
		if err != nil || outcome != Migrated {
			t.Fatalf("Prepare = %v, %v", outcome, err)
		}
		celNoContentLost(t, before, celSnapshot(celRoot(e)), `Roaming\LeviLauncher\config.json`, `Local\LeviLauncher\storage-migration.json`)
		cfg := readJSON(t, apppath.ConfigPath())
		if cfg["window_width"] != float64(1500) || cfg["x"] != float64(1) {
			t.Errorf("config = %v", cfg)
		}
		if cfg["base_root"] != custom {
			t.Errorf("base_root = %v, want %s (the user's current data root)", cfg["base_root"], custom)
		}
		c := celStartup(t)
		if !samePath(c.BaseRoot, custom) || !samePath(apppath.BaseRoot(), custom) {
			t.Errorf("data root = %q / %q, want %s", c.BaseRoot, apppath.BaseRoot(), custom)
		}
		names := celListNames(t)
		for _, n := range []string{"Cur", "New"} {
			if _, ok := names[n]; !ok {
				t.Errorf("instance %q not visible after the migration: %v", n, names)
			}
		}
		// The instance of the first migration is where that migration put it.
		if !exists(filepath.Join(local, "Instances", "Old")) {
			t.Error("Old deleted")
		}
		celTranslatesTo(t, filepath.Join(e.legacy, "versions", "New"), filepath.Join(custom, "Instances", "New"), "", "")
		celSecondPrepareNoop(t, e)
	})
}

var _ = json.Marshal
