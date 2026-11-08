package storage

// Regression tests for group "exe-names-and-appdata": renamed and multiple
// exe names, exe-name case, the APPDATA fallback store, other profiles.
//
// The legacy directory of a pre-PR launcher is %APPDATA%\<exe name>, so the
// name of the running exe matters. The tests start the launcher as a child
// process: a hard link (or copy) of this test binary under the exe name of
// the scenario, which runs TestMigration_ExeChild. The child does what
// main.go does at startup (open the startup log, storage.Prepare,
// apppath.EnsureDirs, config.Load, WebView2 creating its profile) and prints
// the outcome. The parent then reads the result the way the app does.

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

const (
	xaChildEnv    = "LEVILAUNCHER_MIGRATION_TEST_CHILD"
	xaChildMarker = "XA-CHILD-RESULT|"
)

// xaCrossVolume is a directory on a second volume, if any.
var xaCrossVolume = testSecondVolume

// TestMigration_ExeChild is the launcher start run by xaStartAs in a child
// process; it does nothing in a normal test run.
func TestMigration_ExeChild(t *testing.T) {
	if os.Getenv(xaChildEnv) == "" {
		t.Skip("helper process for the exe-name tests")
	}
	// main.go opens the startup log before the migration runs.
	logPath := apppath.StartupLogPath()
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		defer f.Close()
	}
	outcome, err := Prepare()
	msg := ""
	if err != nil {
		msg = strings.ReplaceAll(err.Error(), "\n", " ")
	}
	if !(err != nil && outcome == Current) {
		if derr := apppath.EnsureDirs(); derr != nil {
			msg += " EnsureDirs: " + derr.Error()
		}
		if _, cerr := config.Load(); cerr != nil {
			msg += " config.Load: " + cerr.Error()
		}
		// WebView2 creates its profile in the user data folder.
		_ = os.MkdirAll(filepath.Join(apppath.WebView2Dir(), apppath.LegacyWebViewDirName, "Default"), 0o755)
	}
	fmt.Printf("%s%d|%s|%s|%s\n", xaChildMarker, int(outcome), apppath.LegacyDir(), apppath.LegacyMode(), msg)
	releaseLock()
}

// xaStartAs starts the launcher once as exeName with the APPDATA and
// LOCALAPPDATA that setup set, and returns Prepare's outcome and error text.
func xaStartAs(t *testing.T, exeName string) (Outcome, string) {
	t.Helper()
	binDir, err := os.MkdirTemp(t.TempDir(), "bin")
	if err != nil {
		t.Fatal(err)
	}
	return xaStartIn(t, binDir, exeName)
}

// xaStartIn starts the launcher as binDir\exeName.
func xaStartIn(t *testing.T, binDir, exeName string) (Outcome, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(binDir, exeName)
	if !isFile(exe) {
		if err := os.Link(self, exe); err != nil {
			in, err := os.Open(self)
			if err != nil {
				t.Fatal(err)
			}
			out, err := os.Create(exe)
			if err != nil {
				in.Close()
				t.Fatal(err)
			}
			_, cerr := io.Copy(out, in)
			in.Close()
			out.Close()
			if cerr != nil {
				t.Fatal(cerr)
			}
		}
	}
	cmd := exec.Command(exe, "-test.run=^TestMigration_ExeChild$", "-test.count=1")
	cmd.Dir = binDir // the launcher makes its directory the working directory
	cmd.Env = append(os.Environ(), xaChildEnv+"=1")
	output, runErr := cmd.CombinedOutput()
	sc := bufio.NewScanner(strings.NewReader(string(output)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, xaChildMarker) {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, xaChildMarker), "|", 4)
		if len(parts) != 4 {
			t.Fatalf("malformed child result %q", line)
		}
		n, _ := strconv.Atoi(parts[0])
		// The child really ran under the scenario's exe name.
		if want := filepath.Join(os.Getenv("APPDATA"), strings.ToLower(exeName)); filepath.Clean(parts[1]) != want {
			t.Fatalf("child LegacyDir = %s, want %s", parts[1], want)
		}
		if parts[2] != "" {
			t.Logf("start as %s ran in legacy mode on %s", exeName, parts[2])
		}
		return Outcome(n), strings.TrimSpace(parts[3])
	}
	t.Fatalf("start as %s produced no result (err %v):\n%s", exeName, runErr, output)
	return Current, ""
}

// xaStart is a start that must succeed with the wanted outcome.
func xaStart(t *testing.T, exeName string, want Outcome, label string, roots ...string) {
	t.Helper()
	got, msg := xaStartAs(t, exeName)
	if got != want || msg != "" {
		xaLogTree(t, label, roots...)
		t.Fatalf("%s as %s: outcome %v (%q), want %v without error", label, exeName, got, msg, want)
	}
}

func xaSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, r := range roots {
		_ = filepath.WalkDir(r, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if b, rerr := os.ReadFile(p); rerr == nil {
				m[p] = string(b)
			}
			return nil
		})
	}
	return m
}

func xaLogTree(t *testing.T, label string, roots ...string) {
	t.Helper()
	snap := xaSnapshot(t, roots...)
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := snap[k]
		if len(v) > 60 {
			v = v[:60] + "..."
		}
		fmt.Fprintf(&b, "  %s = %q\n", k, v)
	}
	t.Logf("tree %s:\n%s", label, b.String())
}

// xaNoContentLost checks that every seeded file's content still exists
// somewhere (the migration renames and copies, it never deletes).
func xaNoContentLost(t *testing.T, before, after map[string]string) {
	t.Helper()
	have := map[string]int{}
	for _, v := range after {
		have[v]++
	}
	for p, v := range before {
		if have[v] == 0 {
			t.Errorf("DATA LOST: content %q of %s exists nowhere", v, p)
			continue
		}
		have[v]--
	}
}

// xaUnchanged checks that a start changed no file that existed before it.
func xaUnchanged(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for p, v := range before {
		if got, ok := after[p]; !ok {
			t.Errorf("%s removed %s", label, p)
		} else if got != v {
			t.Errorf("%s changed %s: %q -> %q", label, p, v, got)
		}
	}
}

func xaExpect(t *testing.T, want map[string]string) {
	t.Helper()
	for p, w := range want {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(b) != w {
			t.Errorf("%s = %q, want %q", p, b, w)
		}
	}
}

// xaView is what the app sees after a start, read afresh.
type xaView struct {
	cfg     config.AppConfig
	members map[string]any
}

func xaRead(t *testing.T) xaView {
	t.Helper()
	resetRecordCache()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	if !isFile(apppath.ConfigPath()) {
		t.Fatalf("no config.json at %s", apppath.ConfigPath())
	}
	members := readJSON(t, apppath.ConfigPath())
	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("config.Reload: %v", err)
	}
	return xaView{cfg: cfg, members: members}
}

// xaInstanceNames lists the instances as the app does. It must run last:
// reading the metadata moves version.json into .levilauncher.
func xaInstanceNames(t *testing.T) (string, []string) {
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
	return dir, names
}

func xaWantInstances(t *testing.T, wantDir string, want ...string) {
	t.Helper()
	dir, names := xaInstanceNames(t)
	if !samePath(dir, wantDir) {
		t.Errorf("InstancesDir = %s, want %s", dir, wantDir)
	}
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("ScanVersions(%s) = %v, want %v", dir, names, want)
	}
}

func xaTranslate(t *testing.T, cases map[string]string) {
	t.Helper()
	resetRecordCache()
	for from, want := range cases {
		if got := TranslateLegacyPath(from); !samePath(got, want) {
			t.Errorf("TranslateLegacyPath(%s) = %s, want %s", from, got, want)
		}
	}
}

// xaSeededMoved checks where seedLegacy's items of a legacy directory dir
// (data root dir as well) are after a first migration into the default
// layout, byte for byte, and through the getters the app uses.
func xaSeededMoved(t *testing.T, e env) {
	t.Helper()
	local := filepath.Join(e.local, "LeviLauncher")
	installers, err := apppath.InstallersDir()
	if err != nil {
		t.Fatal(err)
	}
	for got, want := range map[string]string{
		apppath.BackupsDir():      filepath.Join(local, "Backups"),
		apppath.WorldBackupsDir(): filepath.Join(local, "Backups", "Worlds"),
		installers:                filepath.Join(local, "Cache", "Installers"),
		apppath.AccountsDir():     filepath.Join(local, "Accounts"),
		apppath.WebView2Dir():     filepath.Join(local, "WebView2"),
		apppath.GamertagsPath():   filepath.Join(local, "Cache", "gamertags.json"),
	} {
		if !samePath(got, want) {
			t.Errorf("getter = %s, want %s", got, want)
		}
	}
	xaExpect(t, map[string]string{
		filepath.Join(local, "Instances", "A", "version.json"):                                   `{"name":"A"}`,
		filepath.Join(local, "Instances", "A", "Minecraft.Windows.exe"):                          "exe",
		filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"):                             "world",
		filepath.Join(apppath.InstanceBackupsDir(), "A", "A_1.zip"):                              "zip",
		filepath.Join(installers, "1.21.msixvc"):                                                 "pkg",
		filepath.Join(apppath.UpdatesDir(), "1.0.8.zip"):                                         "update",
		filepath.Join(apppath.AccountsDir(), "device.dpapi"):                                     "device",
		filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(apppath.LogsDir(), "startup-legacy.log"):                                   "old log",
		apppath.GamertagsPath():                                                                  `{"1":"Steve"}`,
		apppath.GDKPendingVersionsPath():                                                         `[]`,
	})
	if got := actualName(apppath.WorldBackupsDir()); got != apppath.WorldBackupsDirName {
		t.Errorf("Backups\\Worlds is named %q", got)
	}
}

// xaLegacyConfig is a config.json as every pre-PR launcher wrote it: it
// always has base_root.
func xaLegacyConfig(base string, extra string) string {
	s := `{"base_root":` + quote(base) + `,"window_width":1234,"future_key":"kept"`
	if extra != "" {
		s += "," + extra
	}
	return s + "}"
}

// X1: a portable exe renamed to LL.exe kept everything in %APPDATA%\ll.exe.
// The README searches every "<name>.exe" directory that looks like a
// launcher's, so the data is migrated whether the launcher now runs as
// LL.exe or as LeviLauncher.exe.
func TestMigration_X1_RenamedExe(t *testing.T) {
	for _, runAs := range []string{"LL.exe", "LeviLauncher.exe"} {
		t.Run("running as "+runAs, func(t *testing.T) {
			e := setup(t)
			ll := filepath.Join(e.roaming, "ll.exe")
			seedLegacy(t, ll, ll)
			write(t, filepath.Join(ll, "config.json"), xaLegacyConfig("", ""))
			roots := []string{e.roaming, e.local}
			before := xaSnapshot(t, roots...)

			xaStart(t, runAs, Migrated, "start 1", roots...)
			xaLogTree(t, "after start 1", roots...)
			v := xaRead(t)
			if v.cfg.WindowWidth != 1234 || v.members["future_key"] != "kept" {
				t.Errorf("settings lost: %v", v.members)
			}
			if _, ok := v.members["base_root"]; ok || v.cfg.BaseRoot != "" {
				t.Errorf("default data root got base_root: %v", v.members)
			}
			xaSeededMoved(t, e)
			if exists(filepath.Join(ll, "config.json")) || read(t, filepath.Join(ll, "config.migrated.json")) != xaLegacyConfig("", "") {
				t.Error("legacy config.json not retired as config.migrated.json")
			}
			local := filepath.Join(e.local, "LeviLauncher")
			xaTranslate(t, map[string]string{
				filepath.Join(ll, "versions", "A"):              filepath.Join(local, "Instances", "A"),
				filepath.Join(ll, "versions", "A", "Data", "x"): filepath.Join(local, "Instances", "A", "Data", "x"),
			})
			after := xaSnapshot(t, roots...)
			xaNoContentLost(t, before, after)

			xaStart(t, runAs, Current, "start 2", roots...)
			xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
			xaWantInstances(t, filepath.Join(local, "Instances"), "A")
		})
	}
}

// X2: the user ran "LeviLauncher.exe" (S) and "LeviLauncher (1).exe" (S1).
// Running as "LeviLauncher (1).exe", one start migrates both: S1 first (no
// record yet: its settings win), then S, merged with S1's data preferred.
func TestMigration_X2_TwoStoresBothPopulated(t *testing.T) {
	e := setup(t)
	S1 := filepath.Join(e.roaming, "levilauncher (1).exe")
	S := filepath.Join(e.roaming, apppath.LegacyDefaultExeName)
	N := filepath.Join(e.local, "LeviLauncher")
	inst := filepath.Join(N, "Instances")
	write(t, filepath.Join(S, "config.json"), `{"base_root":"","window_width":1,"only_in_S":"s"}`)
	write(t, filepath.Join(S, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(S, "versions", "Same", "version.json"), `{"name":"Same","from":"S"}`)
	write(t, filepath.Join(S, "EBWebView", "Default", "Preferences"), "web-S")
	write(t, filepath.Join(S, "microsoft-account", "device.dpapi"), "acct-S")
	write(t, filepath.Join(S, "user_gamertag_map.json"), `{"S":"s"}`)
	write(t, filepath.Join(S1, "config.json"), `{"base_root":"","window_width":2,"only_in_S1":"s1"}`)
	write(t, filepath.Join(S1, "versions", "B", "version.json"), `{"name":"B"}`)
	write(t, filepath.Join(S1, "versions", "Same", "version.json"), `{"name":"Same","from":"S1"}`)
	write(t, filepath.Join(S1, "EBWebView", "Default", "Preferences"), "web-S1")
	roots := []string{e.roaming, e.local}
	before := xaSnapshot(t, roots...)

	xaStart(t, "LeviLauncher (1).exe", Migrated, "start 1", roots...)
	xaLogTree(t, "after start 1", roots...)
	v := xaRead(t)
	if v.cfg.WindowWidth != 2 || v.members["only_in_S"] != "s" || v.members["only_in_S1"] != "s1" {
		t.Errorf("config = %v, want window_width 2 and both only_in_ keys", v.members)
	}
	if v.cfg.BaseRoot != "" {
		t.Errorf("base_root = %q", v.cfg.BaseRoot)
	}
	xaExpect(t, map[string]string{
		filepath.Join(inst, "A", "version.json"):                                    `{"name":"A"}`,
		filepath.Join(inst, "B", "version.json"):                                    `{"name":"B"}`,
		filepath.Join(inst, "Same", "version.json"):                                 `{"name":"Same","from":"S1"}`,
		filepath.Join(inst, "Same-legacy", "version.json"):                          `{"name":"Same","from":"S"}`,
		filepath.Join(apppath.AccountsDir(), "device.dpapi"):                        "acct-S",
		apppath.GamertagsPath():                                                     `{"S":"s"}`,
		filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Preferences"): "web-S1",
		// The current profile wins after the first migration; S's stays put.
		filepath.Join(S, "EBWebView", "Default", "Preferences"): "web-S",
		filepath.Join(S, "config.migrated.json"):                `{"base_root":"","window_width":1,"only_in_S":"s"}`,
		filepath.Join(S1, "config.migrated.json"):               `{"base_root":"","window_width":2,"only_in_S1":"s1"}`,
	})
	xaTranslate(t, map[string]string{
		filepath.Join(S1, "versions", "Same"): filepath.Join(inst, "Same"),
		filepath.Join(S1, "versions", "B"):    filepath.Join(inst, "B"),
		filepath.Join(S, "versions", "A"):     filepath.Join(inst, "A"),
		filepath.Join(S, "versions", "Same"):  filepath.Join(inst, "Same-legacy"),
	})
	after := xaSnapshot(t, roots...)
	xaNoContentLost(t, before, after)

	xaStart(t, "LeviLauncher (1).exe", Current, "start 2", roots...)
	xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
	// Back to the regular exe name: nothing left either.
	xaStart(t, "LeviLauncher.exe", Current, "start 3", roots...)
	xaUnchanged(t, "start 3", after, xaSnapshot(t, roots...))
	xaWantInstances(t, inst, "A", "B", "Same", "Same-legacy")
}

// X3: as X2, but one of the two stores had a custom base_root. Every
// instance stays reachable and base_root names the data root that holds them.
func TestMigration_X3_TwoStoresOneCustomRoot(t *testing.T) {
	type tc struct {
		name string
		// custom returns the custom data root; customInS1 tells which store
		// has it.
		custom     func(t *testing.T, e env) string
		customInS1 bool
	}
	tmpRoot := func(name string) func(*testing.T, env) string {
		return func(_ *testing.T, e env) string { return filepath.Join(filepath.Dir(e.roaming), name) }
	}
	crossVolume := func(t *testing.T, e env) string {
		if !isDir(xaCrossVolume) {
			t.Skipf("%s not available", xaCrossVolume)
		}
		d, err := os.MkdirTemp(xaCrossVolume, "xa-x3-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(d) })
		return filepath.Join(d, "R0")
	}
	for _, c := range []tc{
		{"running exe store has the custom root", tmpRoot("R1"), true},
		{"levilauncher.exe store has the custom root", tmpRoot("R0"), false},
		{"levilauncher.exe store has a custom root on another volume", crossVolume, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := setup(t)
			custom := c.custom(t, e)
			S1 := filepath.Join(e.roaming, "levilauncher (1).exe")
			S := filepath.Join(e.roaming, apppath.LegacyDefaultExeName)
			N := filepath.Join(e.local, "LeviLauncher")
			customStore, defaultStore := S, S1
			if c.customInS1 {
				customStore, defaultStore = S1, S
			}
			write(t, filepath.Join(customStore, "config.json"), `{"base_root":`+quote(custom)+`,"window_width":11}`)
			write(t, filepath.Join(custom, "versions", "C", "version.json"), `{"name":"C"}`)
			write(t, filepath.Join(custom, "backups", "worlds", "W", "W_1.mcworld"), "world-C")
			write(t, filepath.Join(defaultStore, "config.json"), `{"base_root":"","window_width":22}`)
			write(t, filepath.Join(defaultStore, "versions", "D", "version.json"), `{"name":"D"}`)
			roots := []string{e.roaming, e.local, custom}
			before := xaSnapshot(t, roots...)

			xaStart(t, "LeviLauncher (1).exe", Migrated, "start 1", roots...)
			xaLogTree(t, "after start 1", roots...)
			v := xaRead(t)
			// S1 is migrated first and decides the data root; S merges into it.
			wantRoot, wantWidth := N, 22
			if c.customInS1 {
				wantRoot, wantWidth = custom, 11
				if !samePath(v.cfg.BaseRoot, custom) {
					t.Errorf("base_root = %q, want %s", v.cfg.BaseRoot, custom)
				}
			} else if v.cfg.BaseRoot != "" {
				t.Errorf("base_root = %q, want the default", v.cfg.BaseRoot)
			}
			if v.cfg.WindowWidth != wantWidth {
				t.Errorf("window_width = %d, want %d", v.cfg.WindowWidth, wantWidth)
			}
			xaExpect(t, map[string]string{
				filepath.Join(wantRoot, "Instances", "C", "version.json"):    `{"name":"C"}`,
				filepath.Join(wantRoot, "Instances", "D", "version.json"):    `{"name":"D"}`,
				filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"): "world-C",
			})
			if !samePath(apppath.WorldBackupsDir(), filepath.Join(wantRoot, "Backups", "Worlds")) {
				t.Errorf("WorldBackupsDir = %s", apppath.WorldBackupsDir())
			}
			xaTranslate(t, map[string]string{
				filepath.Join(custom, "versions", "C"):       filepath.Join(wantRoot, "Instances", "C"),
				filepath.Join(defaultStore, "versions", "D"): filepath.Join(wantRoot, "Instances", "D"),
			})
			after := xaSnapshot(t, roots...)
			xaNoContentLost(t, before, after)

			xaStart(t, "LeviLauncher (1).exe", Current, "start 2", roots...)
			xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
			xaWantInstances(t, filepath.Join(wantRoot, "Instances"), "C", "D")
		})
	}
}

// X4: the legacy directory's name differs in case from the exe name, because
// WebView2 created it first or the exe was spelled in capitals. NTFS ignores
// case, so it is found and migrated.
func TestMigration_X4_ExeNameCase(t *testing.T) {
	for _, c := range []struct{ dir, runAs string }{
		{"LeviLauncher.exe", "LeviLauncher.exe"},
		{"LEVILAUNCHER.EXE", "LeviLauncher.exe"},
		{"levilauncher.exe", "LEVILAUNCHER.EXE"},
		{"LL.EXE", "ll.exe"},
	} {
		t.Run(c.dir+" as "+c.runAs, func(t *testing.T) {
			e := setup(t)
			S := filepath.Join(e.roaming, c.dir)
			seedLegacy(t, S, S)
			write(t, filepath.Join(S, "config.json"), xaLegacyConfig("", ""))
			roots := []string{e.roaming, e.local}
			before := xaSnapshot(t, roots...)

			xaStart(t, c.runAs, Migrated, "start 1", roots...)
			xaLogTree(t, "after start 1", roots...)
			v := xaRead(t)
			if v.cfg.WindowWidth != 1234 || v.members["future_key"] != "kept" || v.cfg.BaseRoot != "" {
				t.Errorf("config = %v", v.members)
			}
			xaSeededMoved(t, e)
			if !isFile(filepath.Join(S, "config.migrated.json")) {
				t.Error("legacy config.json not retired")
			}
			local := filepath.Join(e.local, "LeviLauncher")
			xaTranslate(t, map[string]string{
				filepath.Join(S, "versions", "A"):                                 filepath.Join(local, "Instances", "A"),
				filepath.Join(e.roaming, strings.ToLower(c.dir), "versions", "A"): filepath.Join(local, "Instances", "A"),
			})
			after := xaSnapshot(t, roots...)
			xaNoContentLost(t, before, after)

			xaStart(t, c.runAs, Current, "start 2", roots...)
			xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
			xaWantInstances(t, filepath.Join(local, "Instances"), "A")
		})
	}
}

// X5: a pre-PR launcher started without APPDATA kept its store in
// %LOCALAPPDATA%\levilauncher.exe. The README searches it (step 2), so it is
// migrated even when APPDATA is set now.
func TestMigration_X5_AppDataEmptyFallbackStore(t *testing.T) {
	t.Run("only the fallback store", func(t *testing.T) {
		e := setup(t)
		L := filepath.Join(e.local, apppath.LegacyDefaultExeName)
		seedLegacy(t, L, L)
		write(t, filepath.Join(L, "config.json"), xaLegacyConfig("", ""))
		roots := []string{e.roaming, e.local}
		before := xaSnapshot(t, roots...)

		xaStart(t, "LeviLauncher.exe", Migrated, "start 1", roots...)
		xaLogTree(t, "after start 1", roots...)
		v := xaRead(t)
		if v.cfg.WindowWidth != 1234 || v.members["future_key"] != "kept" || v.cfg.BaseRoot != "" {
			t.Errorf("config = %v", v.members)
		}
		xaSeededMoved(t, e)
		local := filepath.Join(e.local, "LeviLauncher")
		xaTranslate(t, map[string]string{
			filepath.Join(L, "versions", "A"): filepath.Join(local, "Instances", "A"),
		})
		after := xaSnapshot(t, roots...)
		xaNoContentLost(t, before, after)

		xaStart(t, "LeviLauncher.exe", Current, "start 2", roots...)
		xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
		xaWantInstances(t, filepath.Join(local, "Instances"), "A")
	})
	// The user ran the launcher both with and without APPDATA: both stores.
	t.Run("fallback store and roaming store", func(t *testing.T) {
		e := setup(t)
		L := filepath.Join(e.local, apppath.LegacyDefaultExeName)
		R := filepath.Join(e.roaming, apppath.LegacyDefaultExeName)
		write(t, filepath.Join(R, "config.json"), `{"base_root":"","window_width":1,"only_roaming":"r"}`)
		write(t, filepath.Join(R, "versions", "A", "version.json"), `{"name":"A"}`)
		write(t, filepath.Join(L, "config.json"), `{"base_root":"","window_width":2,"only_local":"l"}`)
		write(t, filepath.Join(L, "versions", "B", "version.json"), `{"name":"B"}`)
		write(t, filepath.Join(L, "versions", "A", "version.json"), `{"name":"A","from":"L"}`)
		roots := []string{e.roaming, e.local}
		before := xaSnapshot(t, roots...)

		xaStart(t, "LeviLauncher.exe", Migrated, "start 1", roots...)
		xaLogTree(t, "after start 1", roots...)
		v := xaRead(t)
		// %APPDATA% is searched first: its settings win.
		if v.cfg.WindowWidth != 1 || v.members["only_roaming"] != "r" || v.members["only_local"] != "l" {
			t.Errorf("config = %v", v.members)
		}
		inst := filepath.Join(e.local, "LeviLauncher", "Instances")
		xaExpect(t, map[string]string{
			filepath.Join(inst, "A", "version.json"):        `{"name":"A"}`,
			filepath.Join(inst, "A-legacy", "version.json"): `{"name":"A","from":"L"}`,
			filepath.Join(inst, "B", "version.json"):        `{"name":"B"}`,
		})
		xaTranslate(t, map[string]string{
			filepath.Join(R, "versions", "A"): filepath.Join(inst, "A"),
			filepath.Join(L, "versions", "A"): filepath.Join(inst, "A-legacy"),
			filepath.Join(L, "versions", "B"): filepath.Join(inst, "B"),
		})
		after := xaSnapshot(t, roots...)
		xaNoContentLost(t, before, after)

		xaStart(t, "LeviLauncher.exe", Current, "start 2", roots...)
		xaUnchanged(t, "start 2", after, xaSnapshot(t, roots...))
		xaWantInstances(t, inst, "A", "A-legacy", "B")
	})
}

// X6: an elevated self-update under another administrator kept that
// account's data in its own profile. The current user's migration searches
// only the current user's known folders and leaves other profiles alone.
func TestMigration_X6_OtherProfileIgnored(t *testing.T) {
	e := setup(t)
	other := filepath.Join(filepath.Dir(e.roaming), "OtherUser", "AppData")
	otherStore := filepath.Join(other, "Roaming", apppath.LegacyDefaultExeName)
	seedLegacy(t, otherStore, otherStore)
	write(t, filepath.Join(otherStore, "config.json"), xaLegacyConfig("", ""))
	otherLocal := filepath.Join(other, "Local", apppath.LegacyDefaultExeName)
	write(t, filepath.Join(otherLocal, "config.json"), xaLegacyConfig("", ""))
	before := xaSnapshot(t, other)

	for _, c := range legacyCandidates() {
		if !strings.HasPrefix(xaNorm(c.dir), xaNorm(e.roaming)+`\`) && !strings.HasPrefix(xaNorm(c.dir), xaNorm(e.local)+`\`) {
			t.Errorf("candidate %s is outside the current user's folders", c.dir)
		}
	}
	xaStart(t, "LeviLauncher.exe", Current, "start 1", e.roaming, e.local, other)
	xaUnchanged(t, "start 1", before, xaSnapshot(t, other))
	if r := readRecord(); r != nil && len(r.Moves) > 0 {
		t.Error("a migration record with moves was written")
	}
	v := xaRead(t)
	if v.cfg.WindowWidth == 1234 {
		t.Error("settings of another profile were taken over")
	}
	xaWantInstances(t, filepath.Join(e.local, "LeviLauncher", "Instances"))
}

func xaNorm(p string) string { return strings.ToLower(filepath.Clean(p)) }
