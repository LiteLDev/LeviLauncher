package storage

// Regression tests for the migration of legacy config.json files whose
// content is unusual but legal for some pre-PR version or a hand edit
// (internal/apppath/README.md, "从旧布局迁移"), and of a current config.json
// that exists before the first migration.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	gjson "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// ---------- helpers ----------

func cfgEdgeRoot(e env) string { return filepath.Dir(e.roaming) }

func cfgEdgeCurrentCfg(e env) string {
	return filepath.Join(e.roaming, "LeviLauncher", "config.json")
}

func cfgEdgeLegacyCfg(e env) string { return filepath.Join(e.legacy, "config.json") }

// cfgEdgeSnapshot maps every entry below root to its content ("<dir>" for a
// directory).
func cfgEdgeSnapshot(root string) map[string]string {
	m := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			m[rel] = "<dir>"
			return nil
		}
		b, _ := os.ReadFile(p)
		m[rel] = string(b)
		return nil
	})
	return m
}

func cfgEdgeLogTree(t *testing.T, root string) {
	t.Helper()
	s := cfgEdgeSnapshot(root)
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := s[k]
		if len(v) > 60 {
			v = v[:60] + "..."
		}
		b.WriteString("  " + k + " = " + strings.ReplaceAll(v, "\n", " ") + "\n")
	}
	t.Logf("tree of %s:\n%s", root, b.String())
}

// cfgEdgeContentsKept checks that no file content of before is gone: every
// file still exists somewhere with the same bytes (a migration renames, it
// never deletes). Paths in drop may disappear (the spec deletes them).
func cfgEdgeContentsKept(t *testing.T, before, after map[string]string, drop ...string) {
	t.Helper()
	dropped := map[string]bool{}
	for _, d := range drop {
		dropped[strings.ToLower(d)] = true
	}
	have := map[string]int{}
	for _, v := range after {
		if v != "<dir>" {
			have[v]++
		}
	}
	for k, v := range before {
		if v == "<dir>" || dropped[strings.ToLower(k)] {
			continue
		}
		if have[v] == 0 {
			t.Errorf("content of %s (%q) is gone", k, v)
			continue
		}
		have[v]--
	}
}

// cfgEdgeFreshStart emulates a new launcher process.
func cfgEdgeFreshStart() {
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
}

// cfgEdgeSetup is setup plus resetting what config.Load leaves behind.
func cfgEdgeSetup(t *testing.T) env {
	t.Helper()
	e := setup(t)
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	return e
}

// cfgEdgePrepare runs Prepare and wants Migrated; it returns the snapshot
// of the test root taken before.
func cfgEdgePrepare(t *testing.T, e env) map[string]string {
	t.Helper()
	before := cfgEdgeSnapshot(cfgEdgeRoot(e))
	var out Outcome
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Prepare panicked: %v", r)
			}
		}()
		out, err = Prepare()
	}()
	if err != nil || out != Migrated {
		cfgEdgeLogTree(t, cfgEdgeRoot(e))
		t.Fatalf("Prepare = %v, %v; want Migrated", out, err)
	}
	if apppath.LegacyMode() != "" {
		t.Errorf("legacy mode %q after a migration", apppath.LegacyMode())
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Error("journal left after a committed migration")
	}
	return before
}

// cfgEdgeLoad does what main does after Prepare: EnsureDirs, then
// config.Load (a fresh read of apppath.ConfigPath()).
func cfgEdgeLoad(t *testing.T) config.AppConfig {
	t.Helper()
	if err := apppath.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	c, err := config.Reload()
	if err != nil {
		t.Fatalf("config.Load after the migration: %v (the launcher would not start)", err)
	}
	return c
}

// cfgEdgeMembers reads the current config.json, which must be a JSON object
// without a byte order mark.
func cfgEdgeMembers(t *testing.T, e env) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(cfgEdgeCurrentCfg(e))
	if err != nil {
		t.Fatalf("read current config.json: %v", err)
	}
	if bytes.HasPrefix(b, []byte("\xef\xbb\xbf")) {
		t.Errorf("current config.json starts with a BOM")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("current config.json is not a JSON object: %v (%q)", err, b)
	}
	if m == nil {
		t.Fatalf("current config.json is %q, want an object", b)
	}
	return m
}

// cfgEdgeAny decodes the members for comparison, numbers as json.Number.
func cfgEdgeAny(t *testing.T, m map[string]json.RawMessage) map[string]any {
	t.Helper()
	out := map[string]any{}
	for k, v := range m {
		d := json.NewDecoder(bytes.NewReader(v))
		d.UseNumber()
		var x any
		if err := d.Decode(&x); err != nil {
			t.Fatalf("member %s: %v", k, err)
		}
		out[k] = x
	}
	return out
}

// cfgEdgeWantMembers decodes want the same way.
func cfgEdgeWantMembers(t *testing.T, want string) map[string]any {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(want), &m); err != nil {
		t.Fatal(err)
	}
	return cfgEdgeAny(t, m)
}

// cfgEdgeCheckReachable checks that the data seedLegacy(t, e.legacy, base)
// wrote is where the launcher looks for it after the migration, through
// the getters it uses, byte-identical, with dataRoot as the data root.
func cfgEdgeCheckReachable(t *testing.T, e env, base, dataRoot string) {
	t.Helper()
	cfgEdgeLoad(t)
	if got := apppath.BaseRoot(); !sameLocation(got, dataRoot) {
		cfgEdgeLogTree(t, cfgEdgeRoot(e))
		t.Fatalf("BaseRoot = %s, want %s", got, dataRoot)
	}
	inst, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	installers, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(inst, "A", "version.json"):                                                 `{"name":"A"}`,
		filepath.Join(inst, "A", "Minecraft.Windows.exe"):                                        "exe",
		filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"):                             "world",
		filepath.Join(apppath.InstanceBackupsDir(), "A", "A_1.zip"):                              "zip",
		filepath.Join(installers, "1.21.msixvc"):                                                 "pkg",
		filepath.Join(apppath.UpdatesDir(), "1.0.8.zip"):                                         "update",
		filepath.Join(apppath.AccountsDir(), "device.dpapi"):                                     "device",
		filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(apppath.LogsDir(), "startup.log"):                                          "old log",
		apppath.GamertagsPath():                                                                  `{"1":"Steve"}`,
		apppath.GDKPendingVersionsPath():                                                         `[]`,
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if string(b) != want {
			t.Errorf("%s = %q, want %q", path, b, want)
		}
	}
	for _, d := range []string{inst, apppath.BackupsDir(), apppath.WorldBackupsDir(), apppath.InstanceBackupsDir()} {
		if got, want := actualName(d), filepath.Base(d); got != want {
			t.Errorf("%s is named %q on disk, want %q", d, got, want)
		}
	}
	// Legacy names are gone from the data root (renamed, not copied).
	for _, n := range []string{"versions", "backups", "installers", "updates"} {
		p := filepath.Join(base, n)
		if isDir(p) && actualName(p) == n {
			t.Errorf("legacy %s still in the data root", p)
		}
	}
	if got := TranslateLegacyPath(filepath.Join(base, "versions", "A")); !samePath(got, filepath.Join(inst, "A")) {
		t.Errorf("TranslateLegacyPath(%s) = %s, want %s", filepath.Join(base, "versions", "A"), got, filepath.Join(inst, "A"))
	}
	metas, err := versions.ScanVersions(inst)
	if err != nil {
		t.Fatalf("ScanVersions: %v", err)
	}
	var names []string
	for _, m := range metas {
		names = append(names, m.Name)
	}
	if !reflect.DeepEqual(names, []string{"A"}) {
		t.Errorf("ScanVersions names = %v, want [A]", names)
	}
}

// cfgEdgeSecondStart checks that the next start finds the current layout
// and changes nothing.
func cfgEdgeSecondStart(t *testing.T, e env) {
	t.Helper()
	cfgEdgeFreshStart()
	before := cfgEdgeSnapshot(cfgEdgeRoot(e))
	out, err := Prepare()
	if err != nil || out != Current {
		t.Errorf("second Prepare = %v, %v; want Current", out, err)
	}
	after := cfgEdgeSnapshot(cfgEdgeRoot(e))
	if !reflect.DeepEqual(before, after) {
		for k, v := range before {
			if a, ok := after[k]; !ok || a != v {
				t.Errorf("second start changed %s", k)
			}
		}
		for k := range after {
			if _, ok := before[k]; !ok {
				t.Errorf("second start created %s", k)
			}
		}
	}
}

// cfgEdgeCheckRetired checks that the legacy config.json was kept verbatim
// as config.migrated.json.
func cfgEdgeCheckRetired(t *testing.T, e env, content string) {
	t.Helper()
	if exists(cfgEdgeLegacyCfg(e)) {
		t.Error("legacy config.json still marks the legacy layout as not migrated")
	}
	if got := read(t, filepath.Join(e.legacy, "config.migrated.json")); got != content {
		t.Errorf("config.migrated.json = %q, want the legacy config.json %q", got, content)
	}
}

// cfgEdgeDefaultCase seeds the default-root legacy layout with the given
// legacy config.json, migrates it and checks everything; it returns the
// members of the current config.json.
func cfgEdgeDefaultCase(t *testing.T, content string) (env, map[string]json.RawMessage) {
	t.Helper()
	e := cfgEdgeSetup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, cfgEdgeLegacyCfg(e), content)
	before := cfgEdgePrepare(t, e)
	cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)))
	cfgEdgeCheckRetired(t, e, content)
	m := cfgEdgeMembers(t, e)
	for k := range m {
		if strings.EqualFold(k, "base_root") {
			t.Errorf("default data root kept %s in the current config.json", k)
		}
	}
	cfgEdgeCheckReachable(t, e, e.legacy, filepath.Join(e.local, "LeviLauncher"))
	return e, m
}

// ---------- C1 ----------

// Notepad saves a UTF-8 byte order mark; the migration reads past it and
// writes a current config.json without one.
func TestMigration_C1_LegacyConfigWithBOM(t *testing.T) {
	content := "\xef\xbb\xbf" + `{"base_root":"","window_width":900}`
	e, m := cfgEdgeDefaultCase(t, content)
	if got := cfgEdgeAny(t, m); !reflect.DeepEqual(got, map[string]any{"window_width": json.Number("900")}) {
		t.Errorf("config = %v", got)
	}
	if c := cfgEdgeLoad(t); c.WindowWidth != 900 {
		t.Errorf("config.Load window_width = %d", c.WindowWidth)
	}
	cfgEdgeSecondStart(t, e)
}

// ---------- C2 ----------

// A legacy launcher wrote config.json in place: a crash leaves it empty or
// cut off. It is migrated as an empty config, keeping base_root when the
// text still holds it, and kept verbatim as config.migrated.json.
func TestMigration_C2_EmptyOrTruncatedLegacyConfig(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"empty", ""},
		{"truncated in base_root", `{"base_root":"`},
		{"truncated after a member", `{"window_width":900,"base_r`},
		{"whitespace only", "  \r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, m := cfgEdgeDefaultCase(t, tc.content)
			if len(m) != 0 {
				t.Errorf("config = %v, want an empty object", cfgEdgeAny(t, m))
			}
			cfgEdgeSecondStart(t, e)
		})
	}

	// base_root survives in the text of a cut-off file: the custom data root
	// is migrated in place and kept.
	t.Run("truncated after a custom base_root", func(t *testing.T) {
		e := cfgEdgeSetup(t)
		ll := filepath.Join(cfgEdgeRoot(e), "LLroot")
		seedLegacy(t, e.legacy, ll)
		content := `{"base_root":` + quote(ll) + `,"window_wid`
		write(t, cfgEdgeLegacyCfg(e), content)
		before := cfgEdgePrepare(t, e)
		cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)))
		cfgEdgeCheckRetired(t, e, content)
		got := cfgEdgeAny(t, cfgEdgeMembers(t, e))
		if !reflect.DeepEqual(got, map[string]any{"base_root": ll}) {
			t.Errorf("config = %v, want base_root %s only", got, ll)
		}
		cfgEdgeCheckReachable(t, e, ll, ll)
		cfgEdgeSecondStart(t, e)
	})
}

// ---------- C3 ----------

// goccy accepted a null document and a null base_root: both mean the default
// data root, and the current config.json is an object.
func TestMigration_C3_NullDocumentAndNullBaseRoot(t *testing.T) {
	for _, tc := range []struct {
		name, content, want string
	}{
		{"null document", `null`, `{}`},
		{"null base_root", `{"base_root":null,"window_width":1}`, `{"window_width":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, m := cfgEdgeDefaultCase(t, tc.content)
			if got, want := cfgEdgeAny(t, m), cfgEdgeWantMembers(t, tc.want); !reflect.DeepEqual(got, want) {
				t.Errorf("config = %v, want %v", got, want)
			}
			cfgEdgeSecondStart(t, e)
		})
	}
}

// ---------- C4 ----------

// A base_root that is not a string, or a document that is not an object:
// no panic, the default data root, nothing lost, and a current config.json
// that config.Load accepts.
func TestMigration_C4_NonStringBaseRootAndNonObjectDocument(t *testing.T) {
	for _, tc := range []struct {
		name, content, want string
	}{
		{"number", `{"base_root":123,"window_width":5}`, `{"window_width":5}`},
		{"object", `{"base_root":{},"window_width":5}`, `{"window_width":5}`},
		{"bool", `{"base_root":true,"window_width":5}`, `{"window_width":5}`},
		// Not an object: read as a damaged config.json, migrated as empty.
		{"array document", `["x"]`, `{}`},
		{"string document", `"x"`, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, m := cfgEdgeDefaultCase(t, tc.content)
			if got, want := cfgEdgeAny(t, m), cfgEdgeWantMembers(t, tc.want); !reflect.DeepEqual(got, want) {
				t.Errorf("config = %v, want %v", got, want)
			}
			cfgEdgeSecondStart(t, e)
		})
	}
}

// ---------- C5 ----------

// config.Load (goccy, as every pre-PR version since 0.0.29) matches keys
// without regard to case, and the last of duplicate keys wins. The data root
// the legacy launcher used is the one config.Load reads; the migration must
// treat the same one as the data root (README: "base_root 键名大小写不论，
// 与 config.Load 一致").
func TestMigration_C5_CaseVariantAndDuplicateBaseRootKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  func(ll string) string
	}{
		{"Base_Root", func(ll string) string { return `{"Base_Root":` + quote(ll) + `}` }},
		{"BASE_ROOT", func(ll string) string { return `{"BASE_ROOT":` + quote(ll) + `}` }},
		{"duplicate base_root, custom last", func(ll string) string { return `{"base_root":"","base_root":` + quote(ll) + `}` }},
		{"duplicate base_root, custom first", func(ll string) string { return `{"base_root":` + quote(ll) + `,"base_root":""}` }},
		{"exact empty then case variant custom", func(ll string) string { return `{"base_root":"","Base_Root":` + quote(ll) + `}` }},
		{"case variant custom then exact empty", func(ll string) string { return `{"Base_Root":` + quote(ll) + `,"base_root":""}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := cfgEdgeSetup(t)
			ll := filepath.Join(cfgEdgeRoot(e), "LLroot")
			content := tc.cfg(ll)
			// The data root of the legacy launcher, as config.Load reads it.
			var pre config.AppConfig
			if err := gjson.Unmarshal([]byte(content), &pre); err != nil {
				t.Fatal(err)
			}
			effective := strings.TrimSpace(pre.BaseRoot)
			dataRoot := filepath.Join(e.local, "LeviLauncher")
			base := e.legacy
			if effective != "" {
				base, dataRoot = effective, effective
			}
			t.Logf("legacy launcher data root = %q", base)
			seedLegacy(t, e.legacy, base)
			write(t, cfgEdgeLegacyCfg(e), content)

			before := cfgEdgePrepare(t, e)
			cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)))
			cfgEdgeCheckRetired(t, e, content)
			m := cfgEdgeAny(t, cfgEdgeMembers(t, e))
			want := map[string]any{}
			if effective != "" {
				want["base_root"] = effective
			}
			if !reflect.DeepEqual(m, want) {
				t.Errorf("config = %v, want %v", m, want)
			}
			cfgEdgeCheckReachable(t, e, base, dataRoot)
			cfgEdgeSecondStart(t, e)
			if isDir(filepath.Join(ll, "versions")) {
				t.Errorf("%s still holds un-migrated instances that no later start migrates", filepath.Join(ll, "versions"))
				cfgEdgeLogTree(t, cfgEdgeRoot(e))
			}
		})
	}
}

// ---------- C6 ----------

// Members this version does not know, and newer ones, are kept as they are.
func TestMigration_C6_UnknownAndFutureKeysPreserved(t *testing.T) {
	content := `{"base_root":"","future":{"a":[1,2]},"on_game_launch":"minimize","launcher_launch_count":7,` +
		`"weird \"key\"":"\u00e9\u4e2d","big":12345678901234567890,"f":1.50,"n":null}`
	e, m := cfgEdgeDefaultCase(t, content)
	want := cfgEdgeWantMembers(t, content)
	delete(want, "base_root")
	if got := cfgEdgeAny(t, m); !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
	// Numbers keep their digits.
	if string(m["big"]) != "12345678901234567890" {
		t.Errorf("big = %s", m["big"])
	}
	c := cfgEdgeLoad(t)
	if c.OnGameLaunch != "minimize" || c.LauncherLaunchCount != 7 {
		t.Errorf("config.Load = %+v", c)
	}
	cfgEdgeSecondStart(t, e)
}

// ---------- C7 ----------

// base_root is trimmed: blank means the default data root, a padded path the
// custom one, stored trimmed.
func TestMigration_C7_WhitespaceBaseRoot(t *testing.T) {
	t.Run("blank", func(t *testing.T) {
		e, m := cfgEdgeDefaultCase(t, `{"base_root":"   ","window_width":3}`)
		if got, want := cfgEdgeAny(t, m), cfgEdgeWantMembers(t, `{"window_width":3}`); !reflect.DeepEqual(got, want) {
			t.Errorf("config = %v, want %v", got, want)
		}
		cfgEdgeSecondStart(t, e)
	})
	t.Run("padded custom", func(t *testing.T) {
		e := cfgEdgeSetup(t)
		ll := filepath.Join(cfgEdgeRoot(e), "LLroot")
		seedLegacy(t, e.legacy, ll)
		content := `{"base_root":` + quote("  "+ll+"  ") + `,"window_width":3}`
		write(t, cfgEdgeLegacyCfg(e), content)
		before := cfgEdgePrepare(t, e)
		cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)))
		cfgEdgeCheckRetired(t, e, content)
		got := cfgEdgeAny(t, cfgEdgeMembers(t, e))
		if want := map[string]any{"base_root": ll, "window_width": json.Number("3")}; !reflect.DeepEqual(got, want) {
			t.Errorf("config = %v, want %v", got, want)
		}
		cfgEdgeCheckReachable(t, e, ll, ll)
		cfgEdgeSecondStart(t, e)
	})
}

// ---------- C8 ----------

// A current config.json made before the first migration (by a tool, or a
// start that did not migrate) is merged with the legacy one, whose members
// win; the replaced file is deleted after the commit.
func TestMigration_C8_CurrentConfigBeforeFirstMigration(t *testing.T) {
	e := cfgEdgeSetup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, cfgEdgeCurrentCfg(e), `{"window_width":111,"only_current":1}`)
	content := `{"window_width":900,"only_legacy":2,"base_root":""}`
	write(t, cfgEdgeLegacyCfg(e), content)

	before := cfgEdgePrepare(t, e)
	// The replaced current config.json is the one file the spec deletes.
	cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)), filepath.Join("Roaming", "LeviLauncher", "config.json"))
	cfgEdgeCheckRetired(t, e, content)
	got := cfgEdgeAny(t, cfgEdgeMembers(t, e))
	if want := cfgEdgeWantMembers(t, `{"window_width":900,"only_current":1,"only_legacy":2}`); !reflect.DeepEqual(got, want) {
		t.Errorf("config = %v, want %v", got, want)
	}
	ents, _ := os.ReadDir(filepath.Join(e.roaming, "LeviLauncher"))
	for _, en := range ents {
		if en.Name() != "config.json" {
			t.Errorf("left in the config directory: %s", en.Name())
		}
	}
	cfgEdgeCheckReachable(t, e, e.legacy, filepath.Join(e.local, "LeviLauncher"))
	if c := cfgEdgeLoad(t); c.WindowWidth != 900 {
		t.Errorf("config.Load window_width = %d", c.WindowWidth)
	}
	cfgEdgeSecondStart(t, e)
}

// ---------- C9 ----------

// A current config.json that cannot be parsed is kept as
// config-before-migration.json; the legacy one takes its place
// (README: "无法解析的新 config.json 改名为 config-before-migration.json 保留").
func TestMigration_C9_CurrentConfigCorrupt(t *testing.T) {
	for _, corrupt := range []string{`{`, ""} {
		name := "truncated"
		if corrupt == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			e := cfgEdgeSetup(t)
			seedLegacy(t, e.legacy, e.legacy)
			write(t, cfgEdgeCurrentCfg(e), corrupt)
			content := `{"base_root":"","window_width":900}`
			write(t, cfgEdgeLegacyCfg(e), content)

			before := cfgEdgePrepare(t, e)
			cfgEdgeContentsKept(t, before, cfgEdgeSnapshot(cfgEdgeRoot(e)))
			cfgEdgeCheckRetired(t, e, content)
			if got := read(t, filepath.Join(e.roaming, "LeviLauncher", "config-before-migration.json")); got != corrupt {
				t.Errorf("config-before-migration.json = %q, want %q", got, corrupt)
			}
			got := cfgEdgeAny(t, cfgEdgeMembers(t, e))
			if want := cfgEdgeWantMembers(t, `{"window_width":900}`); !reflect.DeepEqual(got, want) {
				t.Errorf("config = %v, want %v", got, want)
			}
			cfgEdgeCheckReachable(t, e, e.legacy, filepath.Join(e.local, "LeviLauncher"))
			if c := cfgEdgeLoad(t); c.WindowWidth != 900 {
				t.Errorf("config.Load window_width = %d", c.WindowWidth)
			}
			cfgEdgeSecondStart(t, e)
			if !exists(filepath.Join(e.roaming, "LeviLauncher", "config-before-migration.json")) {
				t.Error("the kept corrupt config.json was removed")
			}
		})
	}
}
