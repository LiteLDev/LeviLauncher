package storage

// Regression tests for group "era-default-layouts": the default on-disk layout
// of each pre-PR era, migrated as it is (same volume, no custom base_root),
// with the outcome internal/apppath/README.md ("从旧布局迁移") specifies.
//
// Notation: S = %APPDATA%\levilauncher.exe (the second legacy candidate; the
// test binary's own LegacyDir is %APPDATA%\storage.test.exe),
// N = %LOCALAPPDATA%\LeviLauncher (the default data root and, for launchers up
// to 0.0.9, the era-A legacy directory), CFG = %APPDATA%\LeviLauncher\config.json.

import (
	"bytes"
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

type edlEnv struct {
	env
	root string // parent of Roaming and Local
	S    string
	N    string
	CFG  string
}

func edlSetup(t *testing.T) edlEnv {
	t.Helper()
	e := setup(t)
	a := edlEnv{
		env:  e,
		root: filepath.Dir(e.roaming),
		S:    filepath.Join(e.roaming, "levilauncher.exe"),
		N:    filepath.Join(e.local, "LeviLauncher"),
		CFG:  filepath.Join(e.roaming, "LeviLauncher", "config.json"),
	}
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	return a
}

// edlSeed writes files below base; keys use backslashes. A key ending in "\"
// creates an empty directory.
func edlSeed(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(base, rel)
		if strings.HasSuffix(rel, `\`) {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		write(t, p, content)
	}
}

// edlSnapshot lists every entry below root: files with their content,
// directories as "<dir>", keyed by the relative path with on-disk case.
func edlSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
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
		b, err := os.ReadFile(p)
		if err != nil {
			out[rel] = "<unreadable>"
			return nil
		}
		out[rel] = string(b)
		return nil
	})
	return out
}

func edlLogTree(t *testing.T, label, root string) {
	t.Helper()
	snap := edlSnapshot(t, root)
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := snap[k]
		if v == "<dir>" {
			v = "/"
		} else {
			if len(v) > 40 {
				v = v[:40] + "..."
			}
			v = "= " + strings.ReplaceAll(v, "\n", `\n`)
		}
		b.WriteString("  " + k + " " + v + "\n")
	}
	t.Logf("tree %s (%s):\n%s", label, root, b.String())
}

// edlDiff requires after to equal before exactly.
func edlDiff(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if av, ok := after[k]; !ok {
			t.Errorf("%s: %s disappeared", label, k)
		} else if av != v {
			t.Errorf("%s: %s changed", label, k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s appeared", label, k)
		}
	}
}

// edlTarget maps a file of S (relative path, any case) to where a default
// layout migration puts it; moved=false means it stays in S.
func edlTarget(a edlEnv, rel string) (target string, moved bool) {
	parts := strings.Split(rel, `\`)
	rest := filepath.Join(parts[1:]...)
	in := func(elems ...string) string { return filepath.Join(append(elems, rest)...) }
	switch strings.ToLower(parts[0]) {
	case "versions":
		return in(a.N, "Instances"), true
	case "backups":
		if len(parts) > 2 {
			rest2 := filepath.Join(parts[2:]...)
			switch strings.ToLower(parts[1]) {
			case "worlds":
				return filepath.Join(a.N, "Backups", "Worlds", rest2), true
			case "instances":
				return filepath.Join(a.N, "Backups", "Instances", rest2), true
			}
		}
		return in(a.N, "Backups"), true
	case "installers":
		return in(a.N, "Cache", "Installers"), true
	case "updates":
		return in(a.N, "Cache", "Updates"), true
	case "user_gamertag_map.json":
		return filepath.Join(a.N, "Cache", "gamertags.json"), true
	case "gdk_pending_versions.json":
		return filepath.Join(a.N, "Cache", "gdk-pending-versions.json"), true
	case "microsoft-account":
		return in(a.N, "Accounts"), true
	case "ebwebview":
		return in(a.N, "WebView2", "EBWebView"), true
	case "logs":
		return in(a.N, "Logs"), true
	case "config.json":
		return filepath.Join(a.S, "config.migrated.json"), true
	}
	return filepath.Join(a.S, rel), false
}

// edlAssertPlacement checks that every seeded file of S is at its target,
// byte-identical, and that a renamed source no longer exists. override maps
// an S-relative path to a different expected absolute target.
func edlAssertPlacement(t *testing.T, a edlEnv, seeded, override map[string]string) {
	t.Helper()
	for rel, content := range seeded {
		if strings.HasSuffix(rel, `\`) {
			target, _ := edlTarget(a, strings.TrimSuffix(rel, `\`))
			if !isDir(target) {
				t.Errorf("S\\%s: directory expected at %s", rel, target)
			}
			continue
		}
		target, moved := edlTarget(a, rel)
		if o, ok := override[rel]; ok {
			target = o
		}
		b, err := os.ReadFile(target)
		if err != nil {
			t.Errorf("S\\%s: expected at %s: %v", rel, target, err)
			continue
		}
		if !bytes.Equal(b, []byte(content)) {
			t.Errorf("S\\%s: content at %s = %q, want %q", rel, target, b, content)
		}
		src := filepath.Join(a.S, rel)
		if moved && !samePath(src, target) && exists(src) {
			t.Errorf("S\\%s still exists after a same-volume rename", rel)
		}
	}
}

// edlStart mirrors main.go: Prepare, EnsureDirs, config.Load (a fresh read).
func edlStart(t *testing.T) (Outcome, error, config.AppConfig) {
	t.Helper()
	resetRecordCache()
	outcome, err := Prepare()
	if err != nil {
		return outcome, err, config.AppConfig{}
	}
	if err := apppath.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	c, cerr := config.Reload()
	if cerr != nil {
		t.Fatalf("config.Reload: %v", cerr)
	}
	return outcome, nil, c
}

// edlAssertNextStartIsCurrent runs another start and asserts that it finds
// the current layout and changes nothing in either known folder.
func edlAssertNextStartIsCurrent(t *testing.T, a edlEnv) {
	t.Helper()
	before := edlSnapshot(t, a.root)
	resetRecordCache()
	if outcome, err := Prepare(); err != nil || outcome != Current {
		t.Errorf("next Prepare = %v, %v; want Current", outcome, err)
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Error("next Prepare left a journal")
	}
	edlDiff(t, "next Prepare", before, edlSnapshot(t, a.root))
}

// edlAssertConfig reads CFG afresh: it must hold every member of the legacy
// config (compacted JSON equal) except base_root, which must be absent, and
// no other member.
func edlAssertConfig(t *testing.T, a edlEnv, legacy string) {
	t.Helper()
	want := map[string]json.RawMessage{}
	if legacy != "" {
		if err := json.Unmarshal([]byte(legacy), &want); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readConfigMembers(apppath.ConfigPath())
	if err != nil {
		t.Fatalf("read %s: %v", apppath.ConfigPath(), err)
	}
	if !samePath(apppath.ConfigPath(), a.CFG) {
		t.Errorf("ConfigPath = %s, want %s", apppath.ConfigPath(), a.CFG)
	}
	for k := range got {
		if strings.EqualFold(k, "base_root") {
			t.Errorf("CFG has %s = %s; the default data root must not be written", k, got[k])
		}
	}
	for k, v := range want {
		if k == "base_root" {
			continue
		}
		var c1, c2 bytes.Buffer
		_ = json.Compact(&c1, v)
		_ = json.Compact(&c2, got[k])
		if c1.String() != c2.String() {
			t.Errorf("CFG[%s] = %s, want %s", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("CFG has unexpected member %s", k)
		}
	}
}

func edlRecord(t *testing.T, a edlEnv) Record {
	t.Helper()
	var r Record
	if err := json.Unmarshal([]byte(read(t, filepath.Join(a.N, "storage-migration.json"))), &r); err != nil {
		t.Fatal(err)
	}
	if r.Format != 1 {
		t.Errorf("record format = %d", r.Format)
	}
	return r
}

// edlAssertMoves requires every move of want in the record.
func edlAssertMoves(t *testing.T, r Record, want map[string]string) {
	t.Helper()
	for from, to := range want {
		found := false
		for _, m := range r.Moves {
			if samePath(m.From, from) && samePath(m.To, to) {
				found = true
			}
		}
		if !found {
			t.Errorf("record lacks move %s -> %s; moves=%v", from, to, r.Moves)
		}
	}
}

// edlAssertTranslate checks TranslateLegacyPath for each legacy path.
func edlAssertTranslate(t *testing.T, want map[string]string) {
	t.Helper()
	resetRecordCache()
	for from, to := range want {
		if got := TranslateLegacyPath(from); !samePath(got, to) {
			t.Errorf("TranslateLegacyPath(%s) = %s, want %s", from, got, to)
		}
	}
}

func edlScanNames(t *testing.T) []string {
	t.Helper()
	dir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
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

func edlAssertName(t *testing.T, p, want string) {
	t.Helper()
	if !exists(p) {
		t.Errorf("%s does not exist", p)
		return
	}
	if got := actualName(p); got != want {
		t.Errorf("on-disk name of %s = %q, want %q", p, got, want)
	}
}

// edlAssertGetters checks that the getters the app uses point into N.
func edlAssertGetters(t *testing.T, a edlEnv) {
	t.Helper()
	inst, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	installers, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	for got, want := range map[string]string{
		apppath.BaseRoot():               a.N,
		inst:                             filepath.Join(a.N, "Instances"),
		apppath.BackupsDir():             filepath.Join(a.N, "Backups"),
		apppath.WorldBackupsDir():        filepath.Join(a.N, "Backups", "Worlds"),
		apppath.InstanceBackupsDir():     filepath.Join(a.N, "Backups", "Instances"),
		installers:                       filepath.Join(a.N, "Cache", "Installers"),
		apppath.UpdatesDir():             filepath.Join(a.N, "Cache", "Updates"),
		apppath.AccountsDir():            filepath.Join(a.N, "Accounts"),
		apppath.WebView2Dir():            filepath.Join(a.N, "WebView2"),
		apppath.GamertagsPath():          filepath.Join(a.N, "Cache", "gamertags.json"),
		apppath.GDKPendingVersionsPath(): filepath.Join(a.N, "Cache", "gdk-pending-versions.json"),
	} {
		if !samePath(got, want) {
			t.Errorf("getter = %s, want %s", got, want)
		}
	}
	if apppath.LegacyMode() != "" {
		t.Errorf("legacy mode %s after a migration", apppath.LegacyMode())
	}
}

// ---------------------------------------------------------------- E1

func TestMigration_E1_V1xFullDefault(t *testing.T) {
	a := edlSetup(t)
	legacyCfg := `{"base_root":"","window_width":1280,"window_height":800,"disable_discord_rpc":false,"enable_beta_updates":false,"loader_migrated_v1":true,"lip_github_proxy":"x","mirror_source_enabled":true,"future_unknown":{"k":[1,2]}}`
	aMeta := `{"name":"A","gameVersion":"1.21.120.4","type":"release","packageType":"gdk","enableIsolation":true}`
	seeded := map[string]string{
		`config.json`:                           legacyCfg,
		`versions\A\version.json`:               aMeta,
		`versions\A\Minecraft.Windows.exe`:      "MZ\x90\x00\x03\x00\xff\xfe exe",
		`versions\A\MicrosoftGame.config`:       "<Game/>",
		`versions\A\LeviLauncher.dll`:           "MZ dll",
		`versions\A\LargeLogo.png`:              "\x89PNG\r\n\x1a\n",
		`versions\A\LargeLogo.ico`:              "\x00\x00\x01\x00ico",
		`versions\A\mods\M\manifest.json.close`: `{"name":"M"}`,
		`versions\A\Minecraft Bedrock\Users\Shared\games\com.mojang\minecraftWorlds\w1\level.dat`: "\x0a\x00level",
		`versions\A\.lip\state.json`:                     "ws: 1\n",
		`versions\A\lip.toml`:                                "lip: 1\n",
		`versions\U\AppxManifest.xml`:                        "<Package/>",
		`versions\U\.levilauncher-xbox-auth-key-v1`:          "patched\n",
		`versions\U\version.json`:                            `{"name":"U","gameVersion":"1.21.0.3","type":"release","packageType":"uwp"}`,
		`backups\worlds\W\W_20260101-000000.mcworld`:         "world W",
		`backups\worlds\A\f_W\W_ts.mcworld`:                  "world A",
		`backups\instances\A\A_ts.zip`:                       "PK zip",
		`installers\1.21.120.4.msixvc`:                       "pkg",
		`installers\x.msixvc.download`:                       "partial",
		`installers\vc_redist.x64.exe`:                       "vc",
		`updates\1.0.8.zip.part`:                             "upd",
		`microsoft-account\device.dpapi`:                     "dev",
		`microsoft-account\wam-account.dpapi`:                "wam",
		`EBWebView\Default\Local Storage\leveldb\000003.log`: "ls",
		`logs\startup.log`:                                   "old log",
		`user_gamertag_map.json`:                             `{"1":"Steve"}`,
		`gdk_pending_versions.json`:                          `["1.21.120.4"]`,
		`bin\lip\lipd.exe`:                                   "lipd",
	}
	edlSeed(t, a.S, seeded)
	// The current startup.log is open while the migration runs.
	write(t, filepath.Join(a.N, "Logs", "startup.log"), "new log")
	logf, err := os.OpenFile(filepath.Join(a.N, "Logs", "startup.log"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { logf.Close() })

	outcome, err, cfg := edlStart(t)
	edlLogTree(t, "after migration", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	edlAssertPlacement(t, a, seeded, map[string]string{
		`logs\startup.log`: filepath.Join(a.N, "Logs", "startup-legacy.log"),
	})
	if got := read(t, filepath.Join(a.N, "Logs", "startup.log")); got != "new log" {
		t.Errorf("current startup.log = %q", got)
	}
	edlAssertName(t, filepath.Join(a.N, "Instances"), "Instances")
	edlAssertName(t, filepath.Join(a.N, "Backups"), "Backups")
	edlAssertName(t, filepath.Join(a.N, "Backups", "Worlds"), "Worlds")
	edlAssertName(t, filepath.Join(a.N, "Backups", "Instances"), "Instances")
	edlAssertName(t, filepath.Join(a.N, "Cache", "Installers"), "Installers")
	edlAssertName(t, filepath.Join(a.N, "Cache", "Updates"), "Updates")
	edlAssertConfig(t, a, legacyCfg)
	if got := read(t, filepath.Join(a.S, "config.migrated.json")); got != legacyCfg {
		t.Errorf("config.migrated.json = %q", got)
	}
	for _, gone := range []string{"versions", "backups", "installers", "updates", "microsoft-account", "EBWebView", "logs", "config.json", "user_gamertag_map.json", "gdk_pending_versions.json"} {
		if exists(filepath.Join(a.S, gone)) {
			t.Errorf("S\\%s still present after a same-volume migration", gone)
		}
	}
	if !isDir(filepath.Join(a.S, "bin")) {
		t.Error("S\\bin removed")
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Error("journal left")
	}
	r := edlRecord(t, a)
	if !samePath(r.LegacyDir, a.S) || !samePath(r.BaseRoot, a.N) {
		t.Errorf("record legacy_dir = %s, base_root = %s", r.LegacyDir, r.BaseRoot)
	}
	edlAssertMoves(t, r, map[string]string{
		filepath.Join(a.S, "versions"):                  filepath.Join(a.N, "Instances"),
		filepath.Join(a.S, "backups"):                   filepath.Join(a.N, "Backups"),
		filepath.Join(a.S, "installers"):                filepath.Join(a.N, "Cache", "Installers"),
		filepath.Join(a.S, "updates"):                   filepath.Join(a.N, "Cache", "Updates"),
		filepath.Join(a.S, "user_gamertag_map.json"):    filepath.Join(a.N, "Cache", "gamertags.json"),
		filepath.Join(a.S, "gdk_pending_versions.json"): filepath.Join(a.N, "Cache", "gdk-pending-versions.json"),
		filepath.Join(a.S, "microsoft-account"):         filepath.Join(a.N, "Accounts"),
		filepath.Join(a.S, "EBWebView"):                 filepath.Join(a.N, "WebView2", "EBWebView"),
		filepath.Join(a.S, "logs"):                      filepath.Join(a.N, "Logs"),
		filepath.Join(a.S, "logs", "startup.log"):       filepath.Join(a.N, "Logs", "startup-legacy.log"),
	})
	edlAssertTranslate(t, map[string]string{
		filepath.Join(a.S, "versions", "A"):                     filepath.Join(a.N, "Instances", "A"),
		filepath.Join(a.S, "versions", "U", "AppxManifest.xml"): filepath.Join(a.N, "Instances", "U", "AppxManifest.xml"),
		filepath.Join(a.S, "versions", "A", "LargeLogo.ico"):    filepath.Join(a.N, "Instances", "A", "LargeLogo.ico"),
		filepath.Join(a.S, "backups", "worlds", "W"):            filepath.Join(a.N, "Backups", "worlds", "W"),
	})

	// What the app reads next.
	edlAssertGetters(t, a)
	if cfg.BaseRoot != "" {
		t.Errorf("loaded base_root = %q", cfg.BaseRoot)
	}
	if cfg.LipGithubProxy != "x" || cfg.MirrorSourceEnabled == nil || !*cfg.MirrorSourceEnabled || !cfg.LoaderMigratedV1 || cfg.WindowWidth != 1280 || cfg.WindowHeight != 800 {
		t.Errorf("loaded config lost values: %+v", cfg)
	}
	if read(t, filepath.Join(apppath.AccountsDir(), "wam-account.dpapi")) != "wam" || read(t, filepath.Join(apppath.AccountsDir(), "device.dpapi")) != "dev" {
		t.Error("account records not at AccountsDir")
	}
	if read(t, filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb", "000003.log")) != "ls" {
		t.Error("WebView2 profile not at WebView2Dir")
	}
	if read(t, apppath.GamertagsPath()) != `{"1":"Steve"}` || read(t, apppath.GDKPendingVersionsPath()) != `["1.21.120.4"]` {
		t.Error("gamertags / gdk pending not at the new paths")
	}
	installers, _ := apppath.InstallersDir()
	for _, n := range []string{"1.21.120.4.msixvc", "x.msixvc.download", "vc_redist.x64.exe"} {
		if !isFile(filepath.Join(installers, n)) {
			t.Errorf("InstallersDir lacks %s", n)
		}
	}
	if !isFile(filepath.Join(apppath.UpdatesDir(), "1.0.8.zip.part")) {
		t.Error("UpdatesDir lacks 1.0.8.zip.part")
	}
	if read(t, filepath.Join(apppath.WorldBackupsDir(), "A", "f_W", "W_ts.mcworld")) != "world A" ||
		read(t, filepath.Join(apppath.WorldBackupsDir(), "W", "W_20260101-000000.mcworld")) != "world W" ||
		read(t, filepath.Join(apppath.InstanceBackupsDir(), "A", "A_ts.zip")) != "PK zip" {
		t.Error("backups not at the new getters")
	}

	edlAssertNextStartIsCurrent(t, a)

	if got := edlScanNames(t); strings.Join(got, ",") != "A,U" {
		t.Errorf("ScanVersions = %v, want [A U]", got)
	}
	// ReadMeta moves version.json into .levilauncher\instance.json unchanged;
	// LargeLogo.ico stays for unrepaired shortcuts.
	if got := read(t, filepath.Join(a.N, "Instances", "A", ".levilauncher", "instance.json")); got != aMeta {
		t.Errorf("instance.json = %q", got)
	}
	if !isFile(filepath.Join(a.N, "Instances", "A", "LargeLogo.ico")) {
		t.Error("LargeLogo.ico removed")
	}
	if m, err := versions.ReadMeta(filepath.Join(a.N, "Instances", "U")); err != nil || m.PackageType != versions.PackageTypeUWP {
		t.Errorf("ReadMeta(U) = %+v, %v", m, err)
	}
	// A start after the instance metadata moved still finds nothing to do.
	edlAssertNextStartIsCurrent(t, a)
}

// ---------------------------------------------------------------- E2

func TestMigration_E2_V03xDefault(t *testing.T) {
	a := edlSetup(t)
	legacyCfg := `{"base_root":"","window_width":1280,"window_height":800,"disable_discord_rpc":false,"enable_beta_updates":false}`
	bMeta := `{"name":"B","type":"release","gameVersion":"1.21.80.3","enableIsolation":true,"enableRenderDragon":true,"launchArgs":"-x","envVars":"A=1","createdAt":"2025-12-01T10:00:00Z","registered":true}`
	seeded := map[string]string{
		`config.json`:                                    legacyCfg,
		`versions\B\version.json`:                        bMeta,
		`versions\B\PeEditor.exe`:                        "pe",
		`versions\B\PreLoader.dll`:                       "pre",
		`versions\B\Minecraft.Windows.exe.bak`:           "bak",
		`versions\B\vcruntime140_1.dll`:                  "vc",
		`versions\B\AppXManifest.xml`:                    "<Package/>",
		`backups\worlds\B\f_W\W_20251201-000000.mcworld`: "w",
		`backups\instances\B\B_ts.zip`:                   "zip",
		`installers\dotnet-install.ps1`:                  "ps1",
		`logs\startup.log`:                               "log",
		`lip\lip.exe`:                                    "lip",
		`bin\launcher_core.dll`:                          "core",
		`bin\lip\lipd.exe`:                               "lipd",
		`bin\resource_pack_rules.bin`:                    "rules",
		`user_gamertag_map.json`:                         `{}`,
		`EBWebView\Default\Preferences`:                  "{}",
	}
	edlSeed(t, a.S, seeded)
	write(t, filepath.Join(a.roaming, "lip", "liprc.json"), `{"proxy":""}`)
	lipBefore := edlSnapshot(t, filepath.Join(a.roaming, "lip"))

	outcome, err, _ := edlStart(t)
	edlLogTree(t, "after migration", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	edlAssertPlacement(t, a, seeded, nil)
	edlAssertConfig(t, a, legacyCfg)
	edlDiff(t, "R\\lip", lipBefore, edlSnapshot(t, filepath.Join(a.roaming, "lip")))
	edlAssertName(t, filepath.Join(a.N, "Backups", "Worlds"), "Worlds")
	edlAssertName(t, filepath.Join(a.N, "Backups", "Instances"), "Instances")
	edlAssertGetters(t, a)
	edlAssertTranslate(t, map[string]string{
		filepath.Join(a.S, "versions", "B"): filepath.Join(a.N, "Instances", "B"),
	})
	edlAssertNextStartIsCurrent(t, a)

	m, err := versions.ReadMeta(filepath.Join(a.N, "Instances", "B"))
	if err != nil || m.Name != "B" || m.LaunchArgs != "-x" || m.EnvVars != "A=1" || !m.Registered {
		t.Errorf("ReadMeta(B) = %+v, %v", m, err)
	}
	if got := read(t, filepath.Join(a.N, "Instances", "B", ".levilauncher", "instance.json")); got != bMeta {
		t.Errorf("instance.json lost members: %q", got)
	}
	if got := edlScanNames(t); strings.Join(got, ",") != "B" {
		t.Errorf("ScanVersions = %v", got)
	}
	edlAssertNextStartIsCurrent(t, a)
}

// ---------------------------------------------------------------- E3

func TestMigration_E3_V01V02Default(t *testing.T) {
	a := edlSetup(t)
	legacyCfg := `{"base_root":"","window_width":1024,"window_height":640,"disable_discord_rpc":true,"enable_beta_updates":true}`
	seeded := map[string]string{
		`config.json`:                              legacyCfg,
		`versions\C\version.json`:                  `{"name":"C","gameVersion":"1.21.50.7","type":"release"}`,
		`versions\C\Minecraft.Windows.Console.exe`: "console",
		`backups\worlds\C\f_W\W_ts.mcworld`:        "w",
		`installers\1.21.50.msixvc`:                "trunc",
		`installers\GameInputRedist.msi.part`:      "part",
		`updates\v0.2.1.zip`:                       "zip",
		`lip\lip.exe`:                              "lip",
		`bin\launcher_core.dll`:                    "core",
		`bin\launcher_api.dll`:                     "api",
		`bin\libHttpClient.dll`:                    "http",
		`bin\vcruntime140_1.dll`:                   "vc",
		`user_gamertag_map.json`:                   `{"2":"Alex"}`,
		`EBWebView\Default\Preferences`:            "{}",
	}
	edlSeed(t, a.S, seeded)

	outcome, err, _ := edlStart(t)
	edlLogTree(t, "after migration", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	edlAssertPlacement(t, a, seeded, nil)
	edlAssertConfig(t, a, legacyCfg)
	for _, keep := range []string{"lip", "bin", "config.migrated.json"} {
		if !exists(filepath.Join(a.S, keep)) {
			t.Errorf("S\\%s missing", keep)
		}
	}
	if exists(filepath.Join(a.N, "Backups", "Instances")) {
		t.Error("Backups\\Instances created though S had no instance backups")
	}
	edlAssertGetters(t, a)
	edlAssertNextStartIsCurrent(t, a)
	if got := edlScanNames(t); strings.Join(got, ",") != "C" {
		t.Errorf("ScanVersions = %v", got)
	}
}

// ---------------------------------------------------------------- E4

func TestMigration_E4_V0015To0029Default(t *testing.T) {
	a := edlSetup(t)
	legacyCfg := `{"base_root":"","window_width":1024,"window_height":640,"disable_discord_rpc":false}`
	seeded := map[string]string{
		`config.json`:                       legacyCfg,
		`versions\D\version.json`:           `{"name":"D","gameVersion":"1.21.2.2","type":"release","enableIsolation":false,"enableConsole":true,"enableEditorMode":false,"createdAt":"2025-11-20T00:00:00Z"}`,
		`versions\D\PreLoader.dll`:          "pre",
		`versions\D\PeEditor.exe`:           "pe",
		`versions\D\LargeLogo.png`:          "png",
		`versions\D\mods\`:                  "",
		`versions\Orphan\mods\`:             "",
		`backups\worlds\D\f_W\W_ts.mcworld`: "w",
		`installers\GDK.zip`:                "gdk",
		`bin\launcher_core.dll`:             "core",
		`EBWebView\Default\Preferences`:     "{}",
	}
	edlSeed(t, a.S, seeded)

	outcome, err, _ := edlStart(t)
	edlLogTree(t, "after migration", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	edlAssertPlacement(t, a, seeded, nil)
	edlAssertConfig(t, a, legacyCfg)
	if !isDir(filepath.Join(a.N, "Instances", "Orphan", "mods")) {
		t.Error("Orphan instance folder lost")
	}
	edlAssertGetters(t, a)
	edlAssertNextStartIsCurrent(t, a)
	if got := edlScanNames(t); strings.Join(got, ",") != "D" {
		t.Errorf("ScanVersions = %v", got)
	}
	if !isDir(filepath.Join(a.N, "Instances", "Orphan")) {
		t.Error("ScanVersions removed the Orphan folder")
	}
}

// ---------------------------------------------------------------- E5

// v0.0.6-0.0.9 left data in N (era A); v0.0.10+ moved only config.json to S
// and the user re-installed there. Both legacy directories are migrated in
// one start: S first (it holds the data the user used last, so its instance
// keeps the name), then N's lowercase era-A folders, whose same-named
// instance is kept as "Old-legacy".
func TestMigration_E5_V0010WithEraALeftovers(t *testing.T) {
	for _, withEraAConfig := range []bool{false, true} {
		name := "eraA-config-deleted"
		if withEraAConfig {
			name = "eraA-config-kept"
		}
		t.Run(name, func(t *testing.T) {
			a := edlSetup(t)
			// WebView2 / v0.0.10 created S with the exe's original case.
			a.S = filepath.Join(a.roaming, "LeviLauncher.exe")
			eraACfg := `{"base_root":"","window_width":777,"eraA_only":1}`
			eraA := map[string]string{
				`versions\Old\version.json`:          `{"name":"Old","gameVersion":"1.21.0.1"}`,
				`versions\Old\Minecraft.Windows.exe`: "old era A exe",
				`installers\old.msixvc`:              "old pkg",
				`bin\launcher_core.dll`:              "old core",
			}
			if withEraAConfig {
				eraA[`config.json`] = eraACfg
			}
			edlSeed(t, a.N, eraA)
			legacyCfg := `{"base_root":"","window_width":1024,"window_height":640}`
			seeded := map[string]string{
				`config.json`:                         legacyCfg,
				`versions\Old\version.json`:           `{"name":"Old","gameVersion":"1.21.30.3"}`,
				`versions\Old\Minecraft.Windows.exe`:  "reinstalled exe",
				`versions\New\version.json`:           `{"name":"New","gameVersion":"1.21.40.1"}`,
				`backups\worlds\Old\f_W\W_ts.mcworld`: "w",
				`EBWebView\Default\Preferences`:       "{}",
			}
			edlSeed(t, a.S, seeded)

			outcome, err, cfg := edlStart(t)
			edlLogTree(t, "after migration", a.root)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			edlAssertPlacement(t, a, seeded, nil)
			// Era-A data: nothing lost, every item reachable.
			for rel, want := range map[string]string{
				filepath.Join("Instances", "Old-legacy", "version.json"):          eraA[`versions\Old\version.json`],
				filepath.Join("Instances", "Old-legacy", "Minecraft.Windows.exe"): "old era A exe",
				filepath.Join("Cache", "Installers", "old.msixvc"):                "old pkg",
				filepath.Join("bin", "launcher_core.dll"):                         "old core",
			} {
				if got := read(t, filepath.Join(a.N, rel)); got != want {
					t.Errorf("N\\%s = %q, want %q", rel, got, want)
				}
			}
			if withEraAConfig {
				if got := read(t, filepath.Join(a.N, "config.migrated.json")); got != eraACfg {
					t.Errorf("era-A config.migrated.json = %q", got)
				}
				if exists(filepath.Join(a.N, "config.json")) {
					t.Error("era-A config.json not retired")
				}
			}
			for _, gone := range []string{"versions", "installers"} {
				if exists(filepath.Join(a.N, gone)) {
					t.Errorf("era-A N\\%s still present after a same-volume migration", gone)
				}
			}
			// S's settings win (migrated first); era-A members only it has
			// are kept.
			want := legacyCfg
			if withEraAConfig {
				want = `{"base_root":"","window_width":1024,"window_height":640,"eraA_only":1}`
			}
			edlAssertConfig(t, a, want)
			if cfg.WindowWidth != 1024 {
				t.Errorf("window_width = %d", cfg.WindowWidth)
			}
			edlAssertGetters(t, a)
			edlAssertTranslate(t, map[string]string{
				filepath.Join(a.S, "versions", "Old"): filepath.Join(a.N, "Instances", "Old"),
				filepath.Join(a.S, "versions", "New"): filepath.Join(a.N, "Instances", "New"),
				filepath.Join(a.N, "versions", "Old"): filepath.Join(a.N, "Instances", "Old-legacy"),
			})
			edlAssertNextStartIsCurrent(t, a)
			if got := edlScanNames(t); strings.Join(got, ",") != "New,Old,Old-legacy" {
				t.Errorf("ScanVersions = %v", got)
			}
			if got := read(t, filepath.Join(a.N, "Instances", "Old", "Minecraft.Windows.exe")); got != "reinstalled exe" {
				t.Errorf("Instances\\Old is not S's instance: %q", got)
			}
			edlAssertNextStartIsCurrent(t, a)
		})
	}
}

// ---------------------------------------------------------------- E6

// Only v0.0.6..v0.0.9 ever ran: the data is in N with lowercase names and
// WebView2 kept its profile in %APPDATA%\LeviLauncher.exe. README candidate 4
// migrates N's config.json and lowercase legacy data directories; the
// WebView2 profile alone marks S as pending before the first migration.
func TestMigration_E6_EraAOnlyLocal(t *testing.T) {
	for _, tc := range []struct {
		name            string
		withCfg, withWV bool
	}{
		{"with-config", true, true},
		{"no-config", false, true},
		{"no-config-no-webview", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := edlSetup(t)
			eraACfg := `{"base_root":"","window_width":900}`
			eraA := map[string]string{
				`versions\X\version.json`:          `{"name":"X","gameVersion":"1.21.0.1"}`,
				`versions\X\Minecraft.Windows.exe`: "exe",
				`installers\x.msixvc`:              "pkg",
				`backup\worlds\X\f_W\W_ts.mcworld`: "singular",
				`backups\worlds\L\L_ts.mcworld`:    "intermediate",
				`bin\launcher_core.dll`:            "core",
			}
			if tc.withCfg {
				eraA[`config.json`] = eraACfg
			}
			edlSeed(t, a.N, eraA)
			web := filepath.Join(a.roaming, "LeviLauncher.exe")
			if tc.withWV {
				edlSeed(t, web, map[string]string{`EBWebView\Default\Preferences`: `{"ui":1}`})
			}

			outcome, err, _ := edlStart(t)
			edlLogTree(t, "after start", a.root)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			for rel, want := range map[string]string{
				`Instances\X\version.json`:          eraA[`versions\X\version.json`],
				`Instances\X\Minecraft.Windows.exe`: "exe",
				`Cache\Installers\x.msixvc`:         "pkg",
				`Backups\Worlds\L\L_ts.mcworld`:     "intermediate",
				`Backups\Worlds\X\f_W\W_ts.mcworld`: "singular", // backup\worlds of 0.0.8 and 0.0.9
				`bin\launcher_core.dll`:             "core",
			} {
				if got := read(t, filepath.Join(a.N, rel)); got != want {
					t.Errorf("N\\%s = %q, want %q", rel, got, want)
				}
			}
			edlAssertName(t, filepath.Join(a.N, "Backups"), "Backups")
			edlAssertName(t, filepath.Join(a.N, "Backups", "Worlds"), "Worlds")
			edlAssertName(t, filepath.Join(a.N, "Instances"), "Instances")
			for _, gone := range []string{"versions", "installers", "config.json"} {
				if exists(filepath.Join(a.N, gone)) {
					t.Errorf("era-A N\\%s still present", gone)
				}
			}
			if tc.withCfg {
				if got := read(t, filepath.Join(a.N, "config.migrated.json")); got != eraACfg {
					t.Errorf("N\\config.migrated.json = %q", got)
				}
				edlAssertConfig(t, a, eraACfg)
			} else {
				edlAssertConfig(t, a, "")
			}
			if tc.withWV {
				if got := read(t, filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Preferences")); got != `{"ui":1}` {
					t.Errorf("WebView2 profile = %q", got)
				}
				if exists(filepath.Join(web, "EBWebView")) {
					t.Error("S\\EBWebView still present after a same-volume rename")
				}
			}
			edlAssertGetters(t, a)
			if got := read(t, filepath.Join(apppath.WorldBackupsDir(), "L", "L_ts.mcworld")); got != "intermediate" {
				t.Errorf("WorldBackupsDir L = %q", got)
			}
			edlAssertTranslate(t, map[string]string{
				filepath.Join(a.N, "versions", "X"):          filepath.Join(a.N, "Instances", "X"),
				filepath.Join(a.N, "installers", "x.msixvc"): filepath.Join(a.N, "Cache", "Installers", "x.msixvc"),
			})
			edlAssertNextStartIsCurrent(t, a)
			if got := edlScanNames(t); strings.Join(got, ",") != "X" {
				t.Errorf("ScanVersions = %v", got)
			}
			edlAssertNextStartIsCurrent(t, a)
		})
	}
}

// v0.0.8 and v0.0.9 wrote world backups to <data root>\backup\worlds
// (singular; 0ec708d renamed it to backups). They must stay reachable where
// the app lists world backups after the migration.
func TestMigration_E6_EraASingularBackupReachable(t *testing.T) {
	a := edlSetup(t)
	edlSeed(t, a.N, map[string]string{
		`config.json`:                      `{"base_root":""}`,
		`versions\X\version.json`:          `{"name":"X"}`,
		`backup\worlds\X\f_W\W_ts.mcworld`: "singular",
	})
	outcome, err, _ := edlStart(t)
	edlLogTree(t, "after start", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	if got := read(t, filepath.Join(a.N, "Backups", "Worlds", "X", "f_W", "W_ts.mcworld")); got != "singular" {
		// Not deleted (else read fails above); checked first so a loss is
		// told apart from unreachability.
		t.Errorf("backup = %q", got)
	}
	p := filepath.Join(apppath.WorldBackupsDir(), "X", "f_W", "W_ts.mcworld")
	if !isFile(p) {
		t.Errorf("v0.0.8-0.0.9 world backup N\\backup\\worlds\\X\\f_W\\W_ts.mcworld is not reachable under WorldBackupsDir() %s", apppath.WorldBackupsDir())
	}
	edlAssertNextStartIsCurrent(t, a)
}

// ---------------------------------------------------------------- E7

func TestMigration_E7_UninstalledLeftoversNoConfig(t *testing.T) {
	a := edlSetup(t)
	seeded := map[string]string{
		`versions\A\version.json`:           `{"name":"A","gameVersion":"1.21.120.4"}`,
		`versions\A\Minecraft.Windows.exe`:  "exe",
		`backups\worlds\A\f_W\W_ts.mcworld`: "w",
		`installers\p.msixvc`:               "pkg",
		`updates\1.0.7.zip`:                 "upd",
		`logs\startup.log`:                  "log",
		`microsoft-account\device.dpapi`:    "dev",
		`gdk_pending_versions.json`:         `[]`,
	}
	edlSeed(t, a.S, seeded)
	write(t, filepath.Join(a.N, "Logs", "startup.log"), "new log")

	outcome, err, cfg := edlStart(t)
	edlLogTree(t, "after migration", a.root)
	if err != nil || outcome != Migrated {
		t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
	}
	edlAssertPlacement(t, a, seeded, map[string]string{
		`logs\startup.log`: filepath.Join(a.N, "Logs", "startup-legacy.log"),
	})
	edlAssertConfig(t, a, "")
	if exists(filepath.Join(a.S, "config.migrated.json")) {
		t.Error("config.migrated.json created without a legacy config")
	}
	if cfg.BaseRoot != "" {
		t.Errorf("base_root = %q", cfg.BaseRoot)
	}
	edlAssertGetters(t, a)
	if read(t, filepath.Join(apppath.AccountsDir(), "device.dpapi")) != "dev" || read(t, apppath.GDKPendingVersionsPath()) != `[]` {
		t.Error("accounts / gdk pending not at the new getters")
	}
	edlAssertTranslate(t, map[string]string{
		filepath.Join(a.S, "versions", "A"): filepath.Join(a.N, "Instances", "A"),
	})
	edlAssertNextStartIsCurrent(t, a)
	if got := edlScanNames(t); strings.Join(got, ",") != "A" {
		t.Errorf("ScanVersions = %v", got)
	}
}

// ---------------------------------------------------------------- E8

func TestMigration_E8_OnlyAuxiliaryData(t *testing.T) {
	type variant struct {
		name  string
		files map[string]string
		check func(t *testing.T, a edlEnv)
	}
	gCfg := `{"base_root":"","window_width":1280,"window_height":800,"disable_discord_rpc":false,"enable_beta_updates":false,"loader_migrated_v1":true}`
	vs := []variant{
		{"a-logs-accounts-gdkpending", map[string]string{
			`logs\startup.log`: "log", `microsoft-account\device.dpapi`: "dev", `gdk_pending_versions.json`: `["x"]`,
		}, func(t *testing.T, a edlEnv) {
			if read(t, filepath.Join(apppath.AccountsDir(), "device.dpapi")) != "dev" || read(t, apppath.GDKPendingVersionsPath()) != `["x"]` {
				t.Error("accounts / gdk pending not at the new getters")
			}
		}},
		{"b-ebwebview", map[string]string{`EBWebView\Default\Preferences`: `{"p":1}`}, func(t *testing.T, a edlEnv) {
			if read(t, filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Preferences")) != `{"p":1}` {
				t.Error("WebView2 profile not at WebView2Dir")
			}
		}},
		{"c-world-backups", map[string]string{`backups\worlds\W\W_ts.mcworld`: "w"}, func(t *testing.T, a edlEnv) {
			edlAssertName(t, filepath.Join(a.N, "Backups"), "Backups")
			edlAssertName(t, filepath.Join(a.N, "Backups", "Worlds"), "Worlds")
			if exists(filepath.Join(a.N, "Backups", "Instances")) {
				t.Error("Backups\\Instances created")
			}
			if read(t, filepath.Join(apppath.WorldBackupsDir(), "W", "W_ts.mcworld")) != "w" {
				t.Error("world backup not under WorldBackupsDir")
			}
		}},
		{"d-instance-backups", map[string]string{`backups\instances\A\A_ts.zip`: "z"}, func(t *testing.T, a edlEnv) {
			edlAssertName(t, filepath.Join(a.N, "Backups", "Instances"), "Instances")
			if exists(filepath.Join(a.N, "Backups", "Worlds")) {
				t.Error("Backups\\Worlds created")
			}
			if read(t, filepath.Join(apppath.InstanceBackupsDir(), "A", "A_ts.zip")) != "z" {
				t.Error("instance backup not under InstanceBackupsDir")
			}
		}},
		{"e-installers", map[string]string{`installers\p.msixvc`: "p"}, func(t *testing.T, a edlEnv) {
			d, _ := apppath.InstallersDir()
			if read(t, filepath.Join(d, "p.msixvc")) != "p" {
				t.Error("installer not under InstallersDir")
			}
		}},
		{"f-updates", map[string]string{`updates\1.0.8.zip`: "u"}, func(t *testing.T, a edlEnv) {
			if read(t, filepath.Join(apppath.UpdatesDir(), "1.0.8.zip")) != "u" {
				t.Error("update not under UpdatesDir")
			}
		}},
		{"g-config-only", map[string]string{`config.json`: gCfg}, nil},
		{"h-gamertags-only", map[string]string{`user_gamertag_map.json`: `{"3":"Zed"}`}, func(t *testing.T, a edlEnv) {
			if read(t, apppath.GamertagsPath()) != `{"3":"Zed"}` {
				t.Error("gamertags not at GamertagsPath")
			}
		}},
	}
	for _, v := range vs {
		t.Run(v.name, func(t *testing.T) {
			a := edlSetup(t)
			edlSeed(t, a.S, v.files)
			write(t, filepath.Join(a.N, "Logs", "startup.log"), "new log")
			outcome, err, _ := edlStart(t)
			edlLogTree(t, "after start", a.root)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			edlAssertPlacement(t, a, v.files, map[string]string{
				`logs\startup.log`: filepath.Join(a.N, "Logs", "startup-legacy.log"),
			})
			if read(t, filepath.Join(a.N, "Logs", "startup.log")) != "new log" {
				t.Error("current startup.log changed")
			}
			edlAssertConfig(t, a, v.files[`config.json`])
			if _, ok := v.files[`config.json`]; !ok && exists(filepath.Join(a.S, "config.migrated.json")) {
				t.Error("config.migrated.json created without a legacy config")
			}
			edlAssertGetters(t, a)
			if v.check != nil {
				v.check(t, a)
			}
			edlAssertNextStartIsCurrent(t, a)
		})
	}
}

// ---------------------------------------------------------------- E9

func TestMigration_E9_CapitalisedLegacyNames(t *testing.T) {
	for _, withCfg := range []bool{true, false} {
		name := "no-config"
		if withCfg {
			name = "with-config"
		}
		t.Run(name, func(t *testing.T) {
			a := edlSetup(t)
			seeded := map[string]string{
				`Versions\A\version.json`:       `{"name":"A"}`,
				`Backups\Worlds\W\W_ts.mcworld`: "w",
				`INSTALLERS\p.msixvc`:           "p",
			}
			if withCfg {
				seeded[`config.json`] = `{"base_root":""}`
			}
			edlSeed(t, a.S, seeded)
			outcome, err, _ := edlStart(t)
			edlLogTree(t, "after start", a.root)
			if err != nil || outcome != Migrated {
				t.Fatalf("Prepare = %v, %v; want Migrated", outcome, err)
			}
			edlAssertPlacement(t, a, seeded, nil)
			edlAssertName(t, filepath.Join(a.N, "Instances"), "Instances")
			edlAssertName(t, filepath.Join(a.N, "Backups"), "Backups")
			edlAssertName(t, filepath.Join(a.N, "Backups", "Worlds"), "Worlds")
			edlAssertName(t, filepath.Join(a.N, "Cache", "Installers"), "Installers")
			edlAssertConfig(t, a, seeded[`config.json`])
			edlAssertGetters(t, a)
			edlAssertTranslate(t, map[string]string{
				filepath.Join(a.S, "Versions", "A"): filepath.Join(a.N, "Instances", "A"),
				filepath.Join(a.S, "versions", "A"): filepath.Join(a.N, "Instances", "A"),
			})
			for _, gone := range []string{"Versions", "Backups", "INSTALLERS"} {
				if exists(filepath.Join(a.S, gone)) {
					t.Errorf("S\\%s still present", gone)
				}
			}
			edlAssertNextStartIsCurrent(t, a)
			if got := edlScanNames(t); strings.Join(got, ",") != "A" {
				t.Errorf("ScanVersions = %v", got)
			}
		})
	}
}
