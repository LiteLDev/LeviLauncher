//go:build windows

package storage

// Regression tests for the migration of a legacy layout whose data root is
// not the legacy directory: a custom base_root (internal/apppath/README.md,
// "从旧布局迁移"). Every test simulates whole launcher starts as main.go runs
// them: storage.Prepare, apppath.EnsureDirs, config.Load.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

// cbrStart runs one launcher start in this process: what main.go does before
// the UI. Each start is a new process, so the lock and caches are dropped.
func cbrStart(t *testing.T) (Outcome, error, config.AppConfig) {
	t.Helper()
	releaseLock()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
	outcome, err := Prepare()
	if err != nil && outcome == Current {
		return outcome, err, config.AppConfig{}
	}
	if derr := apppath.EnsureDirs(); derr != nil {
		t.Fatalf("EnsureDirs: %v", derr)
	}
	c, cerr := config.Reload()
	if cerr != nil {
		t.Fatalf("config.Reload: %v", cerr)
	}
	resetRecordCache()
	return outcome, err, c
}

func cbrTree(t *testing.T, roots ...string) {
	t.Helper()
	for _, r := range roots {
		var b strings.Builder
		_ = filepath.WalkDir(r, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(r, p)
			if d.IsDir() {
				b.WriteString("  " + rel + "\\\n")
				return nil
			}
			c, _ := os.ReadFile(p)
			if len(c) > 80 {
				c = c[:80]
			}
			b.WriteString("  " + rel + " = " + string(c) + "\n")
			return nil
		})
		t.Logf("tree %s:\n%s", r, b.String())
	}
}

// cbrWant checks that every path holds exactly its content.
func cbrWant(t *testing.T, files map[string]string) bool {
	t.Helper()
	ok := true
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, p := range keys {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			ok = false
			continue
		}
		if string(b) != files[p] {
			t.Errorf("%s = %q, want %q", p, b, files[p])
			ok = false
		}
	}
	return ok
}

func cbrAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if exists(p) {
			t.Errorf("%s still exists", p)
		}
	}
}

// cbrInstances lists the instances as the app does: the instances directory
// of apppath, scanned by versions.ScanVersions.
func cbrInstances(t *testing.T) []string {
	t.Helper()
	dir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatalf("InstancesDir: %v", err)
	}
	metas, err := versions.ScanVersions(dir)
	if err != nil {
		t.Fatalf("ScanVersions(%s): %v", dir, err)
	}
	var names []string
	for _, m := range metas {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}

func cbrWantInstances(t *testing.T, want ...string) {
	t.Helper()
	got := cbrInstances(t)
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("instances = %v, want %v (InstancesDir under %s)", got, want, apppath.BaseRoot())
	}
}

func cbrLegacyConfig(t *testing.T, dir, base string, extra map[string]any) string {
	t.Helper()
	m := map[string]any{"base_root": base}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	write(t, filepath.Join(dir, "config.json"), string(b))
	return string(b)
}

func cbrLocal(e env) string { return filepath.Join(e.local, "LeviLauncher") }

// cbrMigratedFiles is where seedLegacy's items are once a migration kept the
// data root base in place.
func cbrMigratedFiles(e env, base string) map[string]string {
	local := cbrLocal(e)
	return map[string]string{
		filepath.Join(base, "Instances", "A", "version.json"):                                `{"name":"A"}`,
		filepath.Join(base, "Instances", "A", "Minecraft.Windows.exe"):                       "exe",
		filepath.Join(base, "Backups", "Worlds", "W", "W_1.mcworld"):                         "world",
		filepath.Join(base, "Backups", "Instances", "A", "A_1.zip"):                          "zip",
		filepath.Join(base, "Cache", "Installers", "1.21.msixvc"):                            "pkg",
		filepath.Join(base, "Cache", "Updates", "1.0.8.zip"):                                 "update",
		filepath.Join(base, "Cache", "gamertags.json"):                                       `{"1":"Steve"}`,
		filepath.Join(base, "Cache", "gdk-pending-versions.json"):                            `[]`,
		filepath.Join(local, "Accounts", "device.dpapi"):                                     "device",
		filepath.Join(local, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(local, "Logs", "startup.log"):                                          "old log",
	}
}

// cbrGetters checks that the getters the app uses resolve into base and the
// local directory.
func cbrGetters(t *testing.T, e env, base string) {
	t.Helper()
	inst, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	installers, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	local := cbrLocal(e)
	for _, c := range []struct{ name, got, want string }{
		{"BaseRoot", apppath.BaseRoot(), base},
		{"InstancesDir", inst, filepath.Join(base, "Instances")},
		{"BackupsDir", apppath.BackupsDir(), filepath.Join(base, "Backups")},
		{"WorldBackupsDir", apppath.WorldBackupsDir(), filepath.Join(base, "Backups", "Worlds")},
		{"InstanceBackupsDir", apppath.InstanceBackupsDir(), filepath.Join(base, "Backups", "Instances")},
		{"InstallersDir", installers, filepath.Join(base, "Cache", "Installers")},
		{"UpdatesDir", apppath.UpdatesDir(), filepath.Join(base, "Cache", "Updates")},
		{"GamertagsPath", apppath.GamertagsPath(), filepath.Join(base, "Cache", "gamertags.json")},
		{"GDKPendingVersionsPath", apppath.GDKPendingVersionsPath(), filepath.Join(base, "Cache", "gdk-pending-versions.json")},
		{"AccountsDir", apppath.AccountsDir(), filepath.Join(local, "Accounts")},
		{"WebView2Dir", apppath.WebView2Dir(), filepath.Join(local, "WebView2")},
		{"ConfigPath", apppath.ConfigPath(), filepath.Join(e.roaming, "LeviLauncher", "config.json")},
	} {
		if !samePath(c.got, c.want) {
			t.Errorf("%s = %s, want %s", c.name, c.got, c.want)
		}
	}
}

// cbrTranslate checks TranslateLegacyPath for legacy paths.
func cbrTranslate(t *testing.T, pairs map[string]string) {
	t.Helper()
	resetRecordCache()
	for from, want := range pairs {
		if got := TranslateLegacyPath(from); !samePath(got, want) {
			t.Errorf("TranslateLegacyPath(%s) = %s, want %s", from, got, want)
		}
	}
}

// cbrIdempotent runs further starts and checks that they change nothing.
func cbrIdempotent(t *testing.T, starts int, wantBase string) {
	t.Helper()
	cfg := read(t, apppath.ConfigPath())
	rec := read(t, apppath.MigrationPath())
	for i := 0; i < starts; i++ {
		outcome, err, c := cbrStart(t)
		if err != nil || outcome != Current {
			t.Errorf("later start %d = %v, %v; want Current", i+2, outcome, err)
		}
		if c.BaseRoot != wantBase {
			t.Errorf("later start %d: base_root = %q, want %q", i+2, c.BaseRoot, wantBase)
		}
	}
	if got := read(t, apppath.ConfigPath()); got != cfg {
		t.Errorf("a later start rewrote config.json:\n%s\nwas\n%s", got, cfg)
	}
	if got := read(t, apppath.MigrationPath()); got != rec {
		t.Errorf("a later start rewrote the migration record")
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Error("journal left behind")
	}
}

// B1: an absolute custom data root on the volume of the legacy directory.
func TestMigration_B1_AbsoluteSameVolume(t *testing.T) {
	e := setup(t)
	base := filepath.Join(filepath.Dir(e.roaming), "LLroot")
	seedLegacy(t, e.legacy, base)
	legacyCfg := cbrLegacyConfig(t, e.legacy, base, map[string]any{"window_width": 900, "future_key": "kept"})

	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		cbrTree(t, filepath.Dir(e.roaming))
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, cbrMigratedFiles(e, base)) {
		cbrTree(t, filepath.Dir(e.roaming))
	}
	cbrWant(t, map[string]string{filepath.Join(e.legacy, "config.migrated.json"): legacyCfg})
	cbrAbsent(t,
		filepath.Join(e.legacy, "config.json"),
		filepath.Join(e.legacy, "microsoft-account"),
		filepath.Join(e.legacy, "EBWebView"),
		filepath.Join(e.legacy, "user_gamertag_map.json"),
		filepath.Join(e.legacy, "gdk_pending_versions.json"),
	)
	for _, n := range []string{"versions", "installers", "updates"} {
		cbrAbsent(t, filepath.Join(base, n))
	}
	for p, want := range map[string]string{
		filepath.Join(base, "Backups"):              "Backups",
		filepath.Join(base, "Backups", "Worlds"):    "Worlds",
		filepath.Join(base, "Backups", "Instances"): "Instances",
	} {
		if got := actualName(p); got != want {
			t.Errorf("%s is named %q, want %q", p, got, want)
		}
	}
	if c.BaseRoot != base {
		t.Errorf("base_root = %q, want %q verbatim", c.BaseRoot, base)
	}
	m := readJSON(t, apppath.ConfigPath())
	if m["future_key"] != "kept" || m["window_width"] != float64(900) || m["base_root"] != base {
		t.Errorf("config members lost: %v", m)
	}
	cbrGetters(t, e, base)
	cbrWantInstances(t, "A")
	cbrTranslate(t, map[string]string{
		filepath.Join(base, "versions", "A"):             filepath.Join(base, "Instances", "A"),
		filepath.Join(base, "installers", "1.21.msixvc"): filepath.Join(base, "Cache", "Installers", "1.21.msixvc"),
		filepath.Join(e.legacy, "microsoft-account"):     filepath.Join(cbrLocal(e), "Accounts"),
	})
	cbrIdempotent(t, 2, base)
}

// B2: a custom data root and legacy data left in the legacy directory itself:
// from the default root used before base_root was set (the legacy launcher
// never moved data), or from the VC++/WebView2 installers and update
// downloads that launchers wrote there before reading config.json.
func TestMigration_B2_CustomRootPlusOrphanedDefaultData(t *testing.T) {
	type tc struct {
		name string
		// orphans are files relative to the legacy directory, and where
		// they belong relative to the data root.
		orphans map[string][2]string
		extra   []string // instances besides Live
	}
	for _, c := range []tc{
		{"versions", map[string][2]string{
			`versions\OldDefault\version.json`: {`{"name":"OldDefault"}`, `Instances\OldDefault\version.json`},
		}, []string{"OldDefault"}},
		{"installers", map[string][2]string{
			`installers\vc_redist.x64.exe`: {"vcredist", `Cache\Installers\vc_redist.x64.exe`},
		}, nil},
		{"updates-part", map[string][2]string{
			`updates\x.zip.part`: {"partial", `Cache\Updates\x.zip.part`},
		}, nil},
		{"all", map[string][2]string{
			`versions\OldDefault\version.json`: {`{"name":"OldDefault"}`, `Instances\OldDefault\version.json`},
			`backups\worlds\W\W_1.mcworld`:     {"world", `Backups\Worlds\W\W_1.mcworld`},
			`installers\vc_redist.x64.exe`:     {"vcredist", `Cache\Installers\vc_redist.x64.exe`},
			`updates\x.zip.part`:               {"partial", `Cache\Updates\x.zip.part`},
		}, []string{"OldDefault"}},
		{"same-instance-name", map[string][2]string{
			`versions\Live\version.json`: {`{"name":"Live","where":"default root"}`, `Instances\Live-legacy\version.json`},
		}, []string{"Live-legacy"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := setup(t)
			base := filepath.Join(filepath.Dir(e.roaming), "LLroot")
			write(t, filepath.Join(base, "versions", "Live", "version.json"), `{"name":"Live"}`)
			write(t, filepath.Join(e.legacy, "EBWebView", "Default", "x"), "profile")
			for rel, v := range c.orphans {
				write(t, filepath.Join(e.legacy, rel), v[0])
			}
			cbrLegacyConfig(t, e.legacy, base, nil)

			outcome, err, cfg := cbrStart(t)
			if err != nil || outcome != Migrated {
				cbrTree(t, filepath.Dir(e.roaming))
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			want := map[string]string{
				filepath.Join(base, "Instances", "Live", "version.json"):            `{"name":"Live"}`,
				filepath.Join(cbrLocal(e), "WebView2", "EBWebView", "Default", "x"): "profile",
			}
			translate := map[string]string{}
			for rel, v := range c.orphans {
				want[filepath.Join(base, v[1])] = v[0]
				translate[filepath.Join(e.legacy, rel)] = filepath.Join(base, v[1])
			}
			if !cbrWant(t, want) {
				cbrTree(t, filepath.Dir(e.roaming))
			}
			for _, n := range []string{"versions", "backups", "installers", "updates"} {
				cbrAbsent(t, filepath.Join(e.legacy, n))
			}
			if cfg.BaseRoot != base {
				t.Errorf("base_root = %q, want %q", cfg.BaseRoot, base)
			}
			cbrGetters(t, e, base)
			cbrWantInstances(t, append([]string{"Live"}, c.extra...)...)
			cbrTranslate(t, translate)
			cbrIdempotent(t, 2, base)
			cbrWantInstances(t, append([]string{"Live"}, c.extra...)...)
		})
	}

	// After the migration an older launcher runs again: it finds no
	// config.json in the legacy directory, writes its default one (base_root
	// "") and downloads the VC++ redistributable into the legacy directory
	// before reading it. The next start must keep the custom data root.
	t.Run("old-version-runs-after-migration", func(t *testing.T) {
		e := setup(t)
		base := filepath.Join(filepath.Dir(e.roaming), "LLroot")
		write(t, filepath.Join(base, "versions", "Live", "version.json"), `{"name":"Live"}`)
		write(t, filepath.Join(e.legacy, "versions", "OldDefault", "version.json"), `{"name":"OldDefault"}`)
		cbrLegacyConfig(t, e.legacy, base, map[string]any{"window_width": 900})
		if outcome, err, _ := cbrStart(t); err != nil || outcome != Migrated {
			t.Fatalf("first start = %v, %v", outcome, err)
		}

		write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":"","window_width":800,"old_only":true}`)
		write(t, filepath.Join(e.legacy, "installers", "vc_redist.x64.exe"), "vcredist")
		write(t, filepath.Join(e.legacy, "versions", "New", "version.json"), `{"name":"New"}`)

		outcome, err, cfg := cbrStart(t)
		if err != nil || outcome != Migrated {
			cbrTree(t, filepath.Dir(e.roaming))
			t.Fatalf("second start = %v, %v; want Migrated", outcome, err)
		}
		if !cbrWant(t, map[string]string{
			filepath.Join(base, "Instances", "Live", "version.json"):        `{"name":"Live"}`,
			filepath.Join(base, "Instances", "OldDefault", "version.json"):  `{"name":"OldDefault"}`,
			filepath.Join(base, "Instances", "New", "version.json"):         `{"name":"New"}`,
			filepath.Join(base, "Cache", "Installers", "vc_redist.x64.exe"): "vcredist",
			filepath.Join(e.legacy, "config.migrated-2.json"):               `{"base_root":"","window_width":800,"old_only":true}`,
		}) {
			cbrTree(t, filepath.Dir(e.roaming))
		}
		if cfg.BaseRoot != base || cfg.WindowWidth != 900 {
			t.Errorf("config = base_root %q, window_width %d; want %q, 900", cfg.BaseRoot, cfg.WindowWidth, base)
		}
		if readJSON(t, apppath.ConfigPath())["old_only"] != true {
			t.Error("a member only the old version's config.json has was lost")
		}
		cbrGetters(t, e, base)
		cbrWantInstances(t, "Live", "New", "OldDefault")
		cbrTranslate(t, map[string]string{
			filepath.Join(e.legacy, "versions", "OldDefault"): filepath.Join(base, "Instances", "OldDefault"),
			filepath.Join(e.legacy, "versions", "New"):        filepath.Join(base, "Instances", "New"),
		})
		cbrIdempotent(t, 2, base)
	})
}

func cbrShortPath(p string) string {
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return ""
	}
	buf := make([]uint16, 1024)
	n, err := windows.GetShortPathName(ptr, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// B3: base_root names the legacy directory under another spelling, as
// ResetBaseRoot wrote it: it is the default data root.
func TestMigration_B3_BaseRootEqualsLegacySpelling(t *testing.T) {
	for _, v := range []struct {
		name  string
		spell func(string) string
	}{
		{"exact", func(s string) string { return s }},
		{"trailing-backslash", func(s string) string { return s + `\` }},
		{"forward-slashes", func(s string) string { return strings.ReplaceAll(s, `\`, "/") + "/" }},
		{"other-case", func(s string) string { return filepath.Join(filepath.Dir(s), strings.ToUpper(filepath.Base(s))) }},
		{"doubled-separators", func(s string) string { return filepath.Dir(s) + `\\` + filepath.Base(s) }},
		{"extended-length-prefix", func(s string) string { return `\\?\` + s }},
		{"whitespace-padded", func(s string) string { return "  " + s + " \t" }},
		{"short-8.3-name", cbrShortPath},
	} {
		t.Run(v.name, func(t *testing.T) {
			e := setup(t)
			seedLegacy(t, e.legacy, e.legacy)
			spelled := v.spell(e.legacy)
			if v.name == "short-8.3-name" && (spelled == "" || strings.EqualFold(spelled, e.legacy)) {
				t.Skipf("no distinct 8.3 name for %s", e.legacy)
			}
			t.Logf("base_root = %q", spelled)
			legacyCfg := cbrLegacyConfig(t, e.legacy, spelled, map[string]any{"window_width": 900})

			outcome, err, c := cbrStart(t)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			local := cbrLocal(e)
			if !cbrWant(t, cbrMigratedFiles(e, local)) {
				cbrTree(t, filepath.Dir(e.roaming))
			}
			cbrWant(t, map[string]string{filepath.Join(e.legacy, "config.migrated.json"): legacyCfg})
			if c.BaseRoot != "" {
				t.Errorf("base_root = %q, want none (the default data root)", c.BaseRoot)
			}
			m := readJSON(t, apppath.ConfigPath())
			if _, ok := m["base_root"]; ok {
				t.Errorf("config.json keeps base_root: %v", m)
			}
			if m["window_width"] != float64(900) {
				t.Errorf("config members lost: %v", m)
			}
			cbrGetters(t, e, local)
			cbrWantInstances(t, "A")
			cbrTranslate(t, map[string]string{
				filepath.Join(e.legacy, "versions", "A"): filepath.Join(local, "Instances", "A"),
			})
			cbrIdempotent(t, 1, "")
		})
	}
}

// B4: base_root names %LOCALAPPDATA%\LeviLauncher, the data root of
// launchers up to 0.0.9, which also kept config.json there (era A).
func TestMigration_B4_BaseRootEqualsNewLocalRoot(t *testing.T) {
	e := setup(t)
	n := cbrLocal(e)
	write(t, filepath.Join(n, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(n, "backups", "worlds", "W", "W_1.mcworld"), "world")
	write(t, filepath.Join(n, "backup", "worlds", "Old", "o.mcworld"), "singular")
	write(t, filepath.Join(n, "installers", "1.21.appx"), "pkg")
	write(t, filepath.Join(n, "updates", "u.zip"), "update")
	write(t, filepath.Join(n, "bin", "launcher_core.dll"), "dll")
	const eraA = `{"window_width":640,"era_a_only":true}`
	write(t, filepath.Join(n, "config.json"), eraA)
	write(t, filepath.Join(e.legacy, "EBWebView", "Default", "x"), "profile")
	write(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device")
	write(t, filepath.Join(e.legacy, "logs", "startup.log"), "old log")
	legacyCfg := cbrLegacyConfig(t, e.legacy, n, map[string]any{"window_width": 900})

	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		cbrTree(t, filepath.Dir(e.roaming))
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, map[string]string{
		filepath.Join(n, "Instances", "A", "version.json"):        `{"name":"A"}`,
		filepath.Join(n, "Backups", "Worlds", "W", "W_1.mcworld"): "world",
		filepath.Join(n, "Backups", "Worlds", "Old", "o.mcworld"): "singular",
		filepath.Join(n, "Cache", "Installers", "1.21.appx"):      "pkg",
		filepath.Join(n, "Cache", "Updates", "u.zip"):             "update",
		filepath.Join(n, "bin", "launcher_core.dll"):              "dll",
		filepath.Join(n, "WebView2", "EBWebView", "Default", "x"): "profile",
		filepath.Join(n, "Accounts", "device.dpapi"):              "device",
		filepath.Join(n, "Logs", "startup.log"):                   "old log",
		filepath.Join(e.legacy, "config.migrated.json"):           legacyCfg,
		// Era A's config.json is legacy data too (README, candidate 4):
		// kept as config.migrated.json, its members merged.
		filepath.Join(n, "config.migrated.json"): eraA,
	}) {
		cbrTree(t, filepath.Dir(e.roaming))
	}
	for p, want := range map[string]string{
		filepath.Join(n, "Backups"):           "Backups",
		filepath.Join(n, "Backups", "Worlds"): "Worlds",
		filepath.Join(n, "backup"):            "backup",
	} {
		if got := actualName(p); got != want {
			t.Errorf("%s is named %q, want %q", p, got, want)
		}
	}
	cbrAbsent(t, filepath.Join(n, "versions"), filepath.Join(n, "installers"), filepath.Join(n, "updates"), filepath.Join(n, "config.json"))
	if c.BaseRoot != "" && !samePath(c.BaseRoot, n) {
		t.Errorf("base_root = %q, want none or %s", c.BaseRoot, n)
	}
	m := readJSON(t, apppath.ConfigPath())
	if m["window_width"] != float64(900) || m["era_a_only"] != true {
		t.Errorf("config members not merged (the newer legacy config winning): %v", m)
	}
	cbrGetters(t, e, n)
	cbrWantInstances(t, "A")
	cbrTranslate(t, map[string]string{
		filepath.Join(n, "versions", "A"):            filepath.Join(n, "Instances", "A"),
		filepath.Join(e.legacy, "microsoft-account"): filepath.Join(n, "Accounts"),
	})
	cbrIdempotent(t, 2, c.BaseRoot)
}

// B5: base_root inside the legacy directory.
func TestMigration_B5_BaseRootNestedInLegacy(t *testing.T) {
	e := setup(t)
	base := filepath.Join(e.legacy, "data")
	seedLegacy(t, e.legacy, base)
	legacyCfg := cbrLegacyConfig(t, e.legacy, base, nil)

	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, cbrMigratedFiles(e, base)) {
		cbrTree(t, filepath.Dir(e.roaming))
	}
	cbrWant(t, map[string]string{filepath.Join(e.legacy, "config.migrated.json"): legacyCfg})
	cbrAbsent(t, filepath.Join(e.legacy, "config.json"), filepath.Join(base, "versions"))
	if c.BaseRoot != base {
		t.Errorf("base_root = %q, want %q", c.BaseRoot, base)
	}
	cbrGetters(t, e, base)
	cbrWantInstances(t, "A")
	cbrTranslate(t, map[string]string{
		filepath.Join(base, "versions", "A"): filepath.Join(base, "Instances", "A"),
	})
	cbrIdempotent(t, 2, base)
}

// B6: base_root inside the local directory, or the current config directory.
func TestMigration_B6_BaseRootNestedInLocalOrConfigDir(t *testing.T) {
	for _, v := range []struct {
		name string
		base func(env) string
	}{
		{"local-subdirectory", func(e env) string { return filepath.Join(cbrLocal(e), "mydata") }},
		{"config-directory", func(e env) string { return filepath.Join(e.roaming, "LeviLauncher") }},
	} {
		t.Run(v.name, func(t *testing.T) {
			e := setup(t)
			base := v.base(e)
			seedLegacy(t, e.legacy, base)
			cbrLegacyConfig(t, e.legacy, base, map[string]any{"window_width": 777})

			outcome, err, c := cbrStart(t)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			if !cbrWant(t, cbrMigratedFiles(e, base)) {
				cbrTree(t, filepath.Dir(e.roaming))
			}
			cbrAbsent(t, filepath.Join(base, "versions"), filepath.Join(base, "installers"))
			if c.BaseRoot != base || c.WindowWidth != 777 {
				t.Errorf("config = base_root %q, window_width %d", c.BaseRoot, c.WindowWidth)
			}
			cbrGetters(t, e, base)
			cbrWantInstances(t, "A")
			cbrIdempotent(t, 2, base)
		})
	}
}

// B7: a data root on another volume: renamed there, nothing copied but the
// small cache files that move from the legacy directory into it.
func TestMigration_B7_BaseRootOtherVolume(t *testing.T) {
	parent := testSecondVolume
	if !isDir(parent) {
		t.Skip("no second volume")
	}
	e := setup(t)
	d, err := os.MkdirTemp(parent, "cbr-b7-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	base := filepath.Join(d, "root")
	if sameVolume(base, e.legacy) {
		t.Skip("D: is the volume of the temporary directory")
	}
	seedLegacy(t, e.legacy, base)
	big := filepath.Join(base, "versions", "A", "big.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(120 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	bigBefore, _ := os.Stat(big)
	accBefore, _ := os.Stat(filepath.Join(e.legacy, "microsoft-account", "device.dpapi"))
	// os.SameFile reads the file IDs lazily, by path: load them now.
	os.SameFile(bigBefore, bigBefore)
	os.SameFile(accBefore, accBefore)
	cbrLegacyConfig(t, e.legacy, base, nil)

	start := time.Now()
	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	t.Logf("migrated in %v", time.Since(start))
	if !cbrWant(t, cbrMigratedFiles(e, base)) {
		cbrTree(t, d, filepath.Dir(e.roaming))
	}
	bigAfter, err := os.Stat(filepath.Join(base, "Instances", "A", "big.bin"))
	if err != nil || !os.SameFile(bigBefore, bigAfter) || bigAfter.Size() != 120<<20 {
		t.Errorf("instance not renamed in place on D: (%v)", err)
	}
	accAfter, err := os.Stat(filepath.Join(cbrLocal(e), "Accounts", "device.dpapi"))
	if err != nil || !os.SameFile(accBefore, accAfter) {
		t.Errorf("accounts not renamed from the legacy directory to the local one (%v)", err)
	}
	cbrAbsent(t, filepath.Join(e.legacy, "microsoft-account"), filepath.Join(e.legacy, "EBWebView"), filepath.Join(base, "versions"))
	// The cache files cross volumes: copied, the originals kept.
	cbrWant(t, map[string]string{
		filepath.Join(e.legacy, "user_gamertag_map.json"):    `{"1":"Steve"}`,
		filepath.Join(e.legacy, "gdk_pending_versions.json"): `[]`,
	})
	if c.BaseRoot != base {
		t.Errorf("base_root = %q, want %q", c.BaseRoot, base)
	}
	cbrGetters(t, e, base)
	cbrWantInstances(t, "A")
	cbrTranslate(t, map[string]string{
		filepath.Join(base, "versions", "A"): filepath.Join(base, "Instances", "A"),
	})
	cbrIdempotent(t, 2, base)
}

// B8: the data root is on a drive that is not connected.
func TestMigration_B8_BaseRootOffline(t *testing.T) {
	e := setup(t)
	base := filepath.Join(filepath.Dir(e.roaming), "missing")
	write(t, filepath.Join(e.legacy, "EBWebView", "Default", "x"), "profile")
	write(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device")
	write(t, filepath.Join(e.legacy, "user_gamertag_map.json"), `{"1":"Steve"}`)
	legacyCfg := cbrLegacyConfig(t, e.legacy, base, map[string]any{"window_width": 640})

	outcome, err, c := cbrStart(t)
	if outcome != Legacy || err == nil {
		t.Fatalf("Prepare = %v, %v; want Legacy with an error", outcome, err)
	}
	if !samePath(apppath.LegacyMode(), e.legacy) {
		t.Errorf("LegacyMode = %q, want %s", apppath.LegacyMode(), e.legacy)
	}
	if !samePath(apppath.ConfigPath(), filepath.Join(e.legacy, "config.json")) {
		t.Errorf("ConfigPath = %s", apppath.ConfigPath())
	}
	if c.BaseRoot != base || c.WindowWidth != 640 {
		t.Errorf("the legacy config.json was not read: %+v", c)
	}
	for _, g := range []struct{ name, got, want string }{
		{"AccountsDir", apppath.AccountsDir(), filepath.Join(e.legacy, "microsoft-account")},
		{"WebView2Dir", apppath.WebView2Dir(), e.legacy},
		{"GamertagsPath", apppath.GamertagsPath(), filepath.Join(e.legacy, "user_gamertag_map.json")},
	} {
		if !samePath(g.got, g.want) {
			t.Errorf("%s = %s, want %s", g.name, g.got, g.want)
		}
	}
	cbrWant(t, map[string]string{
		filepath.Join(e.legacy, "config.json"):                       legacyCfg,
		filepath.Join(e.legacy, "EBWebView", "Default", "x"):         "profile",
		filepath.Join(e.legacy, "microsoft-account", "device.dpapi"): "device",
		filepath.Join(e.legacy, "user_gamertag_map.json"):            `{"1":"Steve"}`,
	})
	cbrAbsent(t,
		apppath.MigrationJournalPath(),
		apppath.MigrationPath(),
		filepath.Join(e.roaming, "LeviLauncher", "config.json"),
		filepath.Join(cbrLocal(e), "Accounts"),
		base,
	)

	// The drive is back.
	write(t, filepath.Join(base, "versions", "A", "version.json"), `{"name":"A"}`)
	outcome, err, c = cbrStart(t)
	if err != nil || outcome != Migrated {
		t.Fatalf("start with the drive back = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, map[string]string{
		filepath.Join(base, "Instances", "A", "version.json"):               `{"name":"A"}`,
		filepath.Join(base, "Cache", "gamertags.json"):                      `{"1":"Steve"}`,
		filepath.Join(cbrLocal(e), "WebView2", "EBWebView", "Default", "x"): "profile",
		filepath.Join(cbrLocal(e), "Accounts", "device.dpapi"):              "device",
		filepath.Join(e.legacy, "config.migrated.json"):                     legacyCfg,
	}) {
		cbrTree(t, filepath.Dir(e.roaming))
	}
	if c.BaseRoot != base || c.WindowWidth != 640 {
		t.Errorf("config = %+v", c)
	}
	cbrGetters(t, e, base)
	cbrWantInstances(t, "A")
	cbrIdempotent(t, 1, base)
}

// cbrExeDirScratch reserves <exe dir>\<name>: the legacy launcher resolved a
// relative base_root against its own directory, its working directory after
// startup.
func cbrExeDirScratch(t *testing.T, name string) (exeDir, dir string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exeDir = filepath.Dir(exe)
	dir = filepath.Join(exeDir, name)
	if exists(dir) {
		t.Fatalf("%s exists already", dir)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return exeDir, dir
}

// B9: a relative base_root, resolved against the launcher's directory
// whatever the working directory at startup.
func TestMigration_B9_RelativeBaseRoot(t *testing.T) {
	for _, cwd := range []string{"exe-dir", "elsewhere", "elsewhere-with-same-name"} {
		t.Run(cwd, func(t *testing.T) {
			e := setup(t)
			rel := fmt.Sprintf("cbr-data-%d", time.Now().UnixNano())
			exeDir, data := cbrExeDirScratch(t, rel)
			write(t, filepath.Join(data, "versions", "A", "version.json"), `{"name":"A"}`)
			write(t, filepath.Join(data, "backups", "worlds", "W", "W_1.mcworld"), "world")
			write(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device")
			cbrLegacyConfig(t, e.legacy, rel, nil)
			other := t.TempDir()
			switch cwd {
			case "exe-dir":
				t.Chdir(exeDir)
			case "elsewhere":
				t.Chdir(other)
			case "elsewhere-with-same-name":
				write(t, filepath.Join(other, rel, "versions", "Unrelated", "f.txt"), "unrelated")
				t.Chdir(other)
			}

			outcome, err, c := cbrStart(t)
			if err != nil || outcome != Migrated {
				cbrTree(t, data, other)
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			if !cbrWant(t, map[string]string{
				filepath.Join(data, "Instances", "A", "version.json"):        `{"name":"A"}`,
				filepath.Join(data, "Backups", "Worlds", "W", "W_1.mcworld"): "world",
				filepath.Join(cbrLocal(e), "Accounts", "device.dpapi"):       "device",
			}) {
				cbrTree(t, data, other)
			}
			if cwd == "elsewhere-with-same-name" {
				cbrWant(t, map[string]string{filepath.Join(other, rel, "versions", "Unrelated", "f.txt"): "unrelated"})
				cbrAbsent(t, filepath.Join(other, rel, "Instances"))
			}
			// base_root names the same directory from any working directory.
			if !filepath.IsAbs(c.BaseRoot) || !samePath(c.BaseRoot, data) {
				t.Errorf("base_root = %q, want a path that resolves to %s", c.BaseRoot, data)
			}
			cbrGetters(t, e, data)
			cbrWantInstances(t, "A")
			cbrIdempotent(t, 1, c.BaseRoot)
		})
	}
}

// B10: base_root with an environment variable, which the legacy launcher
// took literally as a relative directory.
func TestMigration_B10_EnvVarLiteralBaseRoot(t *testing.T) {
	for _, cwdIsExe := range []bool{true, false} {
		name := "cwd-exe-dir"
		if !cwdIsExe {
			name = "cwd-elsewhere"
		}
		t.Run(name, func(t *testing.T) {
			e := setup(t)
			exeDir, top := cbrExeDirScratch(t, "%LOCALAPPDATA%")
			root := filepath.Join(top, "LL")
			write(t, filepath.Join(root, "versions", "A", "version.json"), `{"name":"A"}`)
			cbrLegacyConfig(t, e.legacy, `%LOCALAPPDATA%\LL`, nil)
			if cwdIsExe {
				t.Chdir(exeDir)
			} else {
				t.Chdir(t.TempDir())
			}

			outcome, err, c := cbrStart(t)
			if err != nil || outcome != Migrated {
				cbrTree(t, top)
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			if !cbrWant(t, map[string]string{filepath.Join(root, "Instances", "A", "version.json"): `{"name":"A"}`}) {
				cbrTree(t, top, cbrLocal(e))
			}
			cbrAbsent(t, filepath.Join(e.local, "LL"))
			if !samePath(c.BaseRoot, root) {
				t.Errorf("base_root = %q, want %s", c.BaseRoot, root)
			}
			cbrGetters(t, e, root)
			cbrWantInstances(t, "A")
			cbrIdempotent(t, 1, c.BaseRoot)
		})
	}
}

// B11: a data root on a network share.
func TestMigration_B11_BaseRootUNC(t *testing.T) {
	e := setup(t)
	local := filepath.Join(filepath.Dir(e.roaming), "uncroot")
	if len(local) < 3 || local[1] != ':' {
		t.Skip("the temporary directory has no drive letter")
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	unc := `\\localhost\` + strings.ToLower(local[:1]) + `$` + local[2:]
	if !isDir(unc) {
		t.Skipf("administrative share not reachable: %s", unc)
	}
	seedLegacy(t, e.legacy, unc)
	cbrLegacyConfig(t, e.legacy, unc, nil)

	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, cbrMigratedFiles(e, unc)) {
		cbrTree(t, local)
	}
	cbrAbsent(t, filepath.Join(local, "versions"))
	if c.BaseRoot != unc {
		t.Errorf("base_root = %q, want %q", c.BaseRoot, unc)
	}
	cbrGetters(t, e, unc)
	cbrWantInstances(t, "A")
	cbrIdempotent(t, 1, unc)
}

// B12: the running exe's legacy directory names another exe's legacy
// directory as its data root (reset under one exe name, then renamed), and
// that directory holds its own legacy config.json.
func TestMigration_B12_BaseRootPointsToOtherExeStore(t *testing.T) {
	e := setup(t)
	s1 := e.legacy // %APPDATA%\<running exe>
	s := filepath.Join(e.roaming, "levilauncher.exe")
	const sCfg = `{"base_root":"","window_width":1000,"s_only":true}`
	write(t, filepath.Join(s, "config.json"), sCfg)
	write(t, filepath.Join(s, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(s, "EBWebView", "Default", "x"), "profile S")
	write(t, filepath.Join(s1, "EBWebView", "Default", "x"), "profile S1")
	s1Cfg := cbrLegacyConfig(t, s1, s, map[string]any{"window_width": 1234})

	outcome, err, c := cbrStart(t)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if !cbrWant(t, map[string]string{
		filepath.Join(s, "Instances", "A", "version.json"):                  `{"name":"A"}`,
		filepath.Join(cbrLocal(e), "WebView2", "EBWebView", "Default", "x"): "profile S1",
		filepath.Join(s, "EBWebView", "Default", "x"):                       "profile S",
		filepath.Join(s1, "config.migrated.json"):                           s1Cfg,
		filepath.Join(s, "config.migrated.json"):                            sCfg,
	}) {
		cbrTree(t, filepath.Dir(e.roaming))
	}
	cbrAbsent(t, filepath.Join(s, "config.json"), filepath.Join(s1, "config.json"))
	if !samePath(c.BaseRoot, s) {
		t.Errorf("base_root = %q, want %s, where the instances are", c.BaseRoot, s)
	}
	m := readJSON(t, apppath.ConfigPath())
	if m["window_width"] != float64(1234) || m["s_only"] != true {
		t.Errorf("config members = %v", m)
	}
	cbrGetters(t, e, s)
	cbrWantInstances(t, "A")
	cbrTranslate(t, map[string]string{
		filepath.Join(s, "versions", "A"): filepath.Join(s, "Instances", "A"),
	})
	cbrIdempotent(t, 2, c.BaseRoot)
	cbrWantInstances(t, "A")
}
