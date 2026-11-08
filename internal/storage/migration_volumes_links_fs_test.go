//go:build windows

package storage

// Regression tests for the legacy-storage migration on unusual file systems:
// a second volume, junctions, long paths, Unicode names, locked and read-only
// files and protected DACLs (group "volumes-links-fs"). The expected outcomes
// follow internal/apppath/README.md, "从旧布局迁移".

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

// vlDRoot is a directory on a second NTFS volume; tests create unique
// subdirectories in it only.
var vlDRoot = testSecondVolume

func vlDTemp(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(vlDRoot); err != nil {
		t.Skipf("second volume not available: %v", err)
	}
	dir, err := os.MkdirTemp(vlDRoot, "vlf-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		vlMakeWritable(dir)
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("cleanup %s: %v", dir, err)
		}
	})
	return dir
}

func vlMakeWritable(dir string) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil {
			_ = os.Chmod(p, 0o777)
		}
		return nil
	})
}

func vlResetState() {
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	recordMu.Lock()
	recordLoaded = false
	recordMu.Unlock()
}

// vlSetupEnv is setup() with APPDATA and LOCALAPPDATA at the given places.
func vlSetupEnv(t *testing.T, roaming, local string) env {
	t.Helper()
	e := env{roaming: roaming, local: local}
	for _, d := range []string{e.roaming, e.local} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("APPDATA", e.roaming)
	t.Setenv("LOCALAPPDATA", e.local)
	e.legacy = apppath.LegacyDir()
	vlResetState()
	t.Cleanup(func() {
		releaseLock()
		vlResetState()
	})
	return e
}

// vlSetup is setup() that also clears the data root config.Reload sets.
func vlSetup(t *testing.T) env {
	t.Helper()
	e := setup(t)
	apppath.SetBaseRootOverride("")
	t.Cleanup(func() { apppath.SetBaseRootOverride("") })
	return e
}

// vlSetupCross puts %APPDATA% (the legacy directory) on the second volume
// and %LOCALAPPDATA% on the system volume, as a redirected roaming profile.
func vlSetupCross(t *testing.T) env {
	t.Helper()
	d := vlDTemp(t)
	return vlSetupEnv(t, filepath.Join(d, "Roaming"), filepath.Join(t.TempDir(), "Local"))
}

func vlLocal(e env) string { return filepath.Join(e.local, apppath.AppDirName) }

func vlOutcome(o Outcome) string {
	switch o {
	case Current:
		return "Current"
	case Migrated:
		return "Migrated"
	case Legacy:
		return "Legacy"
	case Deferred:
		return "Deferred"
	}
	return fmt.Sprint(int(o))
}

func vlPrepare(t *testing.T, want Outcome) {
	t.Helper()
	got, err := Prepare()
	if got != want || err != nil {
		t.Fatalf("Prepare = %s, %v; want %s", vlOutcome(got), err, vlOutcome(want))
	}
}

// vlStartup does what main does after Prepare: EnsureDirs, then a fresh
// read of config.json, which sets the data root.
func vlStartup(t *testing.T) config.AppConfig {
	t.Helper()
	if err := apppath.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	c, err := config.Reload()
	if err != nil {
		t.Fatalf("config.Reload: %v", err)
	}
	return c
}

func vlWant(t *testing.T, files map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, p := range keys {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(b) != files[p] {
			t.Errorf("%s = %q, want %q", p, b, files[p])
		}
	}
}

func vlAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if exists(p) {
			t.Errorf("%s exists, want absent", p)
		}
	}
}

// vlLock opens path with share mode 0, as a running game or WebView2 holds
// its files.
func vlLock(t *testing.T, path string) windows.Handle {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("lock %s: %v", path, err)
	}
	return h
}

func vlJunction(t *testing.T, link, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /c mklink /J "` + link + `" "` + target + `"`}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %s %s: %v %s", link, target, err, out)
	}
}

func vlIsReparse(p string) bool {
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	a, err := windows.GetFileAttributes(ptr)
	return err == nil && a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func vlLinkTarget(p string) string {
	tgt, err := os.Readlink(p)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(tgt, `\??\`)
}

// vlSnap describes a tree without following links: directories, links with
// their targets, files with a content hash and permission bits. The lock
// file, which every Prepare opens, and the root itself are left out.
func vlSnap(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if !exists(root) {
		return out
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case p == root, strings.EqualFold(d.Name(), apppath.MigrationLockFileName):
			// The root itself, and the lock file every Prepare opens, are
			// left out.
		case vlIsReparse(p):
			// WalkDir does not follow a link; it is described by its target.
			out[rel] = "L:" + vlLinkTarget(p)
		case d.IsDir():
			out[rel] = "D"
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				out[rel] = "F:unreadable:" + err.Error()
				return nil
			}
			info, _ := d.Info()
			h := sha256.Sum256(b)
			out[rel] = fmt.Sprintf("F:%s:%o", hex.EncodeToString(h[:8]), info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

func vlDiff(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	var keys []string
	for k := range before {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if after[k] != before[k] {
			t.Errorf("%s: %s was %q, now %q", label, k, before[k], after[k])
		}
	}
	keys = keys[:0]
	for k := range after {
		if _, ok := before[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Errorf("%s: %s appeared (%q)", label, k, after[k])
	}
}

func vlTree(t *testing.T, label, root string) {
	t.Helper()
	var lines []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			lines = append(lines, "ERR "+p+": "+err.Error())
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		kind := "f"
		if vlIsReparse(p) {
			kind = "L->" + vlLinkTarget(p)
		} else if d.IsDir() {
			kind = "d"
		}
		lines = append(lines, fmt.Sprintf("  [%s] %s", kind, rel))
		return nil
	})
	t.Logf("tree %s (%s):\n%s", label, root, strings.Join(lines, "\n"))
}

// vlE1Data is where seedLegacy's data-root entries go, under data root tb.
func vlE1Data(tb string) map[string]string {
	return map[string]string{
		filepath.Join(tb, "Instances", "A", "version.json"):          `{"name":"A"}`,
		filepath.Join(tb, "Instances", "A", "Minecraft.Windows.exe"): "exe",
		filepath.Join(tb, "Backups", "Worlds", "W", "W_1.mcworld"):   "world",
		filepath.Join(tb, "Backups", "Instances", "A", "A_1.zip"):    "zip",
		filepath.Join(tb, "Cache", "Installers", "1.21.msixvc"):      "pkg",
		filepath.Join(tb, "Cache", "Updates", "1.0.8.zip"):           "update",
		filepath.Join(tb, "Cache", "gamertags.json"):                 `{"1":"Steve"}`,
		filepath.Join(tb, "Cache", "gdk-pending-versions.json"):      `[]`,
	}
}

// vlE1Local is where seedLegacy's machine-local entries go.
func vlE1Local(n string) map[string]string {
	return map[string]string{
		filepath.Join(n, "Accounts", "device.dpapi"):                                     "device",
		filepath.Join(n, "WebView2", "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
	}
}

// vlGetters checks the paths the app reads, after vlStartup, against data
// root tb and local directory n.
func vlGetters(t *testing.T, tb, n string) {
	t.Helper()
	if apppath.LegacyMode() != "" {
		t.Errorf("legacy mode %q after a migration", apppath.LegacyMode())
	}
	inst, err := apppath.InstancesDir()
	if err != nil || !samePath(inst, filepath.Join(tb, "Instances")) {
		t.Errorf("InstancesDir = %s, %v; want %s", inst, err, filepath.Join(tb, "Instances"))
	}
	installers, err := apppath.InstallersDir()
	if err != nil || !samePath(installers, filepath.Join(tb, "Cache", "Installers")) {
		t.Errorf("InstallersDir = %s, %v", installers, err)
	}
	for _, c := range []struct{ name, got, want string }{
		{"BackupsDir", apppath.BackupsDir(), filepath.Join(tb, "Backups")},
		{"WorldBackupsDir", apppath.WorldBackupsDir(), filepath.Join(tb, "Backups", "Worlds")},
		{"InstanceBackupsDir", apppath.InstanceBackupsDir(), filepath.Join(tb, "Backups", "Instances")},
		{"UpdatesDir", apppath.UpdatesDir(), filepath.Join(tb, "Cache", "Updates")},
		{"GamertagsPath", apppath.GamertagsPath(), filepath.Join(tb, "Cache", "gamertags.json")},
		{"GDKPendingVersionsPath", apppath.GDKPendingVersionsPath(), filepath.Join(tb, "Cache", "gdk-pending-versions.json")},
		{"AccountsDir", apppath.AccountsDir(), filepath.Join(n, "Accounts")},
		{"WebView2Dir", apppath.WebView2Dir(), filepath.Join(n, "WebView2")},
	} {
		if !samePath(c.got, c.want) {
			t.Errorf("%s = %s, want %s", c.name, c.got, c.want)
		}
	}
}

// vlE1Reachable checks seedLegacy's entries through the getters.
func vlE1Reachable(t *testing.T) {
	t.Helper()
	inst, _ := apppath.InstancesDir()
	installers, _ := apppath.InstallersDir()
	vlWant(t, map[string]string{
		filepath.Join(inst, "A", "Minecraft.Windows.exe"):                                        "exe",
		filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"):                             "world",
		filepath.Join(apppath.InstanceBackupsDir(), "A", "A_1.zip"):                              "zip",
		filepath.Join(installers, "1.21.msixvc"):                                                 "pkg",
		filepath.Join(apppath.UpdatesDir(), "1.0.8.zip"):                                         "update",
		apppath.GamertagsPath():                                                                  `{"1":"Steve"}`,
		apppath.GDKPendingVersionsPath():                                                         `[]`,
		filepath.Join(apppath.AccountsDir(), "device.dpapi"):                                     "device",
		filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
	})
}

// vlNames lists the instances the app finds.
func vlNames(t *testing.T) []string {
	t.Helper()
	inst, err := apppath.InstancesDir()
	if err != nil {
		t.Fatal(err)
	}
	ms, err := versions.ScanVersions(inst)
	if err != nil {
		t.Errorf("ScanVersions(%s): %v", inst, err)
	}
	var names []string
	for _, m := range ms {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}

func vlWantNames(t *testing.T, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := vlNames(t); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("instances = %q, want %q", got, want)
	}
}

// vlIdempotent checks that the next start finds nothing to migrate and
// changes nothing in the given trees.
func vlIdempotent(t *testing.T, roots ...string) {
	t.Helper()
	before := make([]map[string]string, len(roots))
	for i, r := range roots {
		before[i] = vlSnap(t, r)
	}
	vlPrepare(t, Current)
	for i, r := range roots {
		vlDiff(t, "next start "+r, before[i], vlSnap(t, r))
	}
}

func vlTranslate(t *testing.T, from, want string) {
	t.Helper()
	if got := TranslateLegacyPath(from); !samePath(got, want) {
		t.Errorf("TranslateLegacyPath(%s) = %s, want %s", from, got, want)
	}
}

// ---------------------------------------------------------------- V1

// V1: %APPDATA% is redirected to another volume. The data root stays where
// it is under the current names, base_root names it, and the machine-local
// entries are copied to %LOCALAPPDATA% with their sources kept.
func TestMigration_V1_RoamingOnOtherVolume(t *testing.T) {
	e := vlSetupCross(t)
	s, n := e.legacy, vlLocal(e)
	seedLegacy(t, s, s)
	const legacyCfg = `{"window_width":900,"future_key":"kept"}`
	write(t, filepath.Join(s, "config.json"), legacyCfg)
	// The current log is open when the migration runs.
	write(t, filepath.Join(n, "Logs", "startup.log"), "new log")

	vlPrepare(t, Migrated)
	vlTree(t, "S", s)
	vlTree(t, "N", n)

	vlWant(t, vlE1Data(s))
	vlWant(t, vlE1Local(n))
	vlWant(t, map[string]string{
		filepath.Join(n, "Logs", "startup.log"):        "new log",
		filepath.Join(n, "Logs", "startup-legacy.log"): "old log",
		// Copied across volumes: the sources are kept.
		filepath.Join(s, "microsoft-account", "device.dpapi"):                "device",
		filepath.Join(s, "EBWebView", "Default", "Local Storage", "leveldb"): "storage",
		filepath.Join(s, "logs", "startup.log"):                              "old log",
		filepath.Join(s, "config.migrated.json"):                             legacyCfg,
	})
	// Renamed on the volume of the data root, not copied.
	vlAbsent(t,
		filepath.Join(s, "versions"), filepath.Join(s, "versions.migrated"),
		filepath.Join(s, "installers"), filepath.Join(s, "updates"),
		filepath.Join(s, "user_gamertag_map.json"), filepath.Join(s, "gdk_pending_versions.json"),
		filepath.Join(s, "config.json"), apppath.MigrationJournalPath())
	if got := actualName(filepath.Join(s, "backups")); got != "Backups" {
		t.Errorf("backups kept its name %q", got)
	}
	cfg := readJSON(t, apppath.ConfigPath())
	if br, _ := cfg["base_root"].(string); !samePath(br, s) {
		t.Errorf("base_root = %v, want %s", cfg["base_root"], s)
	}
	if cfg["future_key"] != "kept" || cfg["window_width"] != float64(900) {
		t.Errorf("config members lost: %v", cfg)
	}
	vlTranslate(t, filepath.Join(s, "versions", "A"), filepath.Join(s, "Instances", "A"))

	c := vlStartup(t)
	if !samePath(c.BaseRoot, s) {
		t.Errorf("config.Load BaseRoot = %q, want %s", c.BaseRoot, s)
	}
	vlGetters(t, s, n)
	vlE1Reachable(t)
	vlWantNames(t, "A")

	// The next start neither migrates nor copies again.
	vlIdempotent(t, s, n, filepath.Join(e.roaming, apppath.AppDirName))
}

// ---------------------------------------------------------------- V2

// V2 (B7): the legacy directory is on the system volume, base_root on
// another one. The data root is renamed in place, base_root is unchanged.
func TestMigration_V2_CustomRootOnOtherVolume(t *testing.T) {
	e := vlSetup(t)
	s, n := e.legacy, vlLocal(e)
	base := filepath.Join(vlDTemp(t), "root")
	seedLegacy(t, s, base)
	big := strings.Repeat("0123456789abcdef", 256*1024) // 4 MiB
	write(t, filepath.Join(base, "versions", "A", "data", "big.bin"), big)
	legacyCfg := `{"base_root":` + quote(base) + `,"window_width":1000}`
	write(t, filepath.Join(s, "config.json"), legacyCfg)

	vlPrepare(t, Migrated)
	vlTree(t, "base", base)
	vlTree(t, "S", s)
	vlTree(t, "N", n)

	vlWant(t, vlE1Data(base))
	vlWant(t, vlE1Local(n))
	vlWant(t, map[string]string{
		filepath.Join(base, "Instances", "A", "data", "big.bin"): big,
		filepath.Join(n, "Logs", "startup.log"):                  "old log",
		filepath.Join(s, "config.migrated.json"):                 legacyCfg,
	})
	// The instances were renamed on their volume, not copied, and the
	// machine-local entries were renamed on the system volume.
	vlAbsent(t,
		filepath.Join(base, "versions"), filepath.Join(base, "versions.migrated"),
		filepath.Join(base, "installers"), filepath.Join(base, "updates"),
		filepath.Join(s, "microsoft-account"), filepath.Join(s, "EBWebView"), filepath.Join(s, "logs"),
		filepath.Join(s, "config.json"))
	cfg := readJSON(t, apppath.ConfigPath())
	if cfg["base_root"] != base || cfg["window_width"] != float64(1000) {
		t.Errorf("config = %v; want base_root %s and window_width 1000", cfg, base)
	}
	vlTranslate(t, filepath.Join(base, "versions", "A"), filepath.Join(base, "Instances", "A"))

	vlStartup(t)
	vlGetters(t, base, n)
	vlE1Reachable(t)
	vlWantNames(t, "A")
	vlIdempotent(t, base, s, n, filepath.Join(e.roaming, apppath.AppDirName))
}

// ---------------------------------------------------------------- V3

// V3: %APPDATA%\<exe name> is a junction to a store on another volume. The
// default data root is then on another volume than %LOCALAPPDATA%: it stays
// where it is (reached through the junction) and the launcher does not run
// on the legacy layout at every start.
func TestMigration_V3_LegacyDirIsJunctionToOtherVolume(t *testing.T) {
	e := vlSetup(t)
	s, n := e.legacy, vlLocal(e)
	store := filepath.Join(vlDTemp(t), "store")
	seedLegacy(t, store, store)
	big := strings.Repeat("x", 1<<20)
	write(t, filepath.Join(store, "versions", "A", "data", "big.bin"), big)
	write(t, filepath.Join(store, "config.json"), `{"window_width":700}`)
	vlJunction(t, s, store)
	t.Logf("volumeOf(S)=%q volumeOf(store)=%q volumeOf(N)=%q", volumeOf(s), volumeOf(store), volumeOf(e.local))

	outcome, err := Prepare()
	vlTree(t, "store", store)
	vlTree(t, "N", n)
	if outcome != Migrated || err != nil {
		t.Fatalf("Prepare = %s, %v; want Migrated", vlOutcome(outcome), err)
	}
	if !vlIsReparse(s) || !samePath(vlLinkTarget(s), store) {
		t.Errorf("the junction %s was replaced", s)
	}
	// The data is reachable where the app reads it.
	c := vlStartup(t)
	inst, _ := apppath.InstancesDir()
	t.Logf("base_root=%q InstancesDir=%s", c.BaseRoot, inst)
	vlE1Reachable(t)
	vlWant(t, map[string]string{filepath.Join(inst, "A", "data", "big.bin"): big})
	vlWantNames(t, "A")
	// The default data root is on another volume than %LOCALAPPDATA%: it is
	// kept in place, as for a redirected roaming profile, rather than every
	// instance being copied to the system volume.
	if got := volumeOf(resolvedPath(inst)); !strings.EqualFold(got, volumeOf(store)) {
		t.Errorf("instances were copied to volume %s (InstancesDir %s); the data root on %s must stay in place",
			got, inst, volumeOf(store))
	}
	if exists(filepath.Join(store, "versions.migrated")) {
		t.Errorf("the instances were copied within %s: versions.migrated duplicates them", store)
	}
	vlIdempotent(t, store, n, filepath.Join(e.roaming, apppath.AppDirName))
}

// ---------------------------------------------------------------- V4

func vlSeedWithoutVersions(t *testing.T, dir string) {
	t.Helper()
	write(t, filepath.Join(dir, "backups", "worlds", "W", "W_1.mcworld"), "world")
	write(t, filepath.Join(dir, "microsoft-account", "device.dpapi"), "device")
	write(t, filepath.Join(dir, "EBWebView", "Default", "x"), "x")
	write(t, filepath.Join(dir, "config.json"), `{"window_width":900}`)
}

// V4: %APPDATA%\<exe name>\versions is a junction to another volume. Every
// instance must be listed after the migration, the data on the other volume
// untouched.
func TestMigration_V4_VersionsIsJunction(t *testing.T) {
	for _, merge := range []bool{false, true} {
		t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
			e := vlSetup(t)
			s, n := e.legacy, vlLocal(e)
			dv := filepath.Join(vlDTemp(t), "versions")
			write(t, filepath.Join(dv, "A", "version.json"), `{"name":"A"}`)
			write(t, filepath.Join(dv, "A", "Minecraft.Windows.exe"), "exe")
			vlSeedWithoutVersions(t, s)
			vlJunction(t, filepath.Join(s, "versions"), dv)
			want := []string{"A"}
			if merge {
				write(t, filepath.Join(n, "Instances", "B", "version.json"), `{"name":"B"}`)
				want = append(want, "B")
			}
			dBefore := vlSnap(t, dv)

			outcome, err := Prepare()
			vlTree(t, "S", s)
			vlTree(t, "N", n)
			if outcome != Migrated || err != nil {
				t.Fatalf("Prepare = %s, %v; want Migrated", vlOutcome(outcome), err)
			}
			// The data on the other volume is not touched.
			vlDiff(t, "junction target", dBefore, vlSnap(t, dv))
			vlAbsent(t, filepath.Join(s, "versions"))

			vlStartup(t)
			inst, _ := apppath.InstancesDir()
			vlWant(t, map[string]string{
				filepath.Join(inst, "A", "version.json"):                     `{"name":"A"}`,
				filepath.Join(inst, "A", "Minecraft.Windows.exe"):            "exe",
				filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"): "world",
				filepath.Join(apppath.AccountsDir(), "device.dpapi"):         "device",
			})
			if merge {
				vlWant(t, map[string]string{filepath.Join(inst, "B", "version.json"): `{"name":"B"}`})
			}
			vlWantNames(t, want...)
			vlIdempotent(t, s, n)
		})
	}
}

// ---------------------------------------------------------------- V5

// V5: an instance is a junction to an external game directory, on the same
// volume or another one. The junction is moved as it is, its target is not
// touched, and the instance is listed.
func TestMigration_V5_InstanceIsJunction(t *testing.T) {
	for _, otherVolume := range []bool{false, true} {
		for _, merge := range []bool{false, true} {
			t.Run(fmt.Sprintf("otherVolume=%v/merge=%v", otherVolume, merge), func(t *testing.T) {
				e := vlSetup(t)
				s, n := e.legacy, vlLocal(e)
				extRoot := t.TempDir()
				if otherVolume {
					extRoot = vlDTemp(t)
				}
				ext := filepath.Join(extRoot, "External Game")
				write(t, filepath.Join(ext, "version.json"), `{"name":"Ext"}`)
				write(t, filepath.Join(ext, "Minecraft.Windows.exe"), "ext exe")
				seedLegacy(t, s, s)
				write(t, filepath.Join(s, "config.json"), `{}`)
				vlJunction(t, filepath.Join(s, "versions", "Ext"), ext)
				want := []string{"A", "Ext"}
				if merge {
					write(t, filepath.Join(n, "Instances", "B", "version.json"), `{"name":"B"}`)
					want = append(want, "B")
				}
				extBefore := vlSnap(t, ext)

				outcome, err := Prepare()
				vlTree(t, "S", s)
				vlTree(t, "N", n)
				if outcome != Migrated || err != nil {
					t.Fatalf("Prepare = %s, %v; want Migrated", vlOutcome(outcome), err)
				}
				link := filepath.Join(n, "Instances", "Ext")
				if !vlIsReparse(link) || !samePath(vlLinkTarget(link), ext) {
					t.Errorf("%s is not the moved junction to %s (target %q)", link, ext, vlLinkTarget(link))
				}
				vlDiff(t, "external directory", extBefore, vlSnap(t, ext))
				vlAbsent(t, filepath.Join(s, "versions"), filepath.Join(s, "versions.migrated"))
				vlTranslate(t, filepath.Join(s, "versions", "Ext"), link)

				vlStartup(t)
				vlGetters(t, n, n)
				vlE1Reachable(t)
				vlWantNames(t, want...)
				vlIdempotent(t, s, n)
			})
		}
	}
}

// ---------------------------------------------------------------- V6

func vlDeep(parts int) string {
	segs := make([]string, parts)
	for i := range segs {
		segs[i] = fmt.Sprintf("segment-%02d-%s", i, strings.Repeat("x", 40))
	}
	return filepath.Join(segs...)
}

// V6: paths longer than MAX_PATH, renamed on one volume and copied across.
func TestMigration_V6_LongPaths(t *testing.T) {
	for _, cross := range []bool{false, true} {
		t.Run(fmt.Sprintf("crossVolume=%v", cross), func(t *testing.T) {
			var e env
			if cross {
				e = vlSetupCross(t)
			} else {
				e = vlSetup(t)
			}
			s, n := e.legacy, vlLocal(e)
			d := vlDeep(7)
			seedLegacy(t, s, s)
			write(t, filepath.Join(s, "config.json"), `{}`)
			files := map[string]string{
				filepath.Join("versions", "A", "mods", d, "f.bin"):         "mod-data",
				filepath.Join("EBWebView", "Default", "Cache", d, "f.bin"): "cache-data",
				filepath.Join("microsoft-account", d, "f.bin"):             "acct-data",
				filepath.Join("logs", d, "f.log"):                          "log-data",
				filepath.Join("backups", "worlds", "W", d, "level.dat"):    "level",
			}
			for rel, c := range files {
				p := filepath.Join(s, rel)
				if len(p) < 300 {
					t.Fatalf("path too short: %d", len(p))
				}
				write(t, p, c)
			}

			vlPrepare(t, Migrated)
			tb := n
			if cross {
				tb = s
			}
			vlWant(t, map[string]string{
				filepath.Join(tb, "Instances", "A", "mods", d, "f.bin"):                   "mod-data",
				filepath.Join(n, "WebView2", "EBWebView", "Default", "Cache", d, "f.bin"): "cache-data",
				filepath.Join(n, "Accounts", d, "f.bin"):                                  "acct-data",
				filepath.Join(n, "Logs", d, "f.log"):                                      "log-data",
				filepath.Join(tb, "Backups", "Worlds", "W", d, "level.dat"):               "level",
			})
			vlWant(t, vlE1Data(tb))
			vlWant(t, vlE1Local(n))
			if cross {
				vlWant(t, map[string]string{
					filepath.Join(s, "EBWebView", "Default", "Cache", d, "f.bin"): "cache-data",
					filepath.Join(s, "microsoft-account", d, "f.bin"):             "acct-data",
					filepath.Join(s, "logs", d, "f.log"):                          "log-data",
				})
			} else {
				vlAbsent(t, filepath.Join(s, "EBWebView"), filepath.Join(s, "microsoft-account"), filepath.Join(s, "logs"))
			}
			vlAbsent(t, filepath.Join(s, "versions"), apppath.MigrationJournalPath())

			vlStartup(t)
			vlGetters(t, tb, n)
			vlE1Reachable(t)
			vlWantNames(t, "A")
			vlIdempotent(t, s, n)
		})
	}
}

// ---------------------------------------------------------------- V7

// V7: Unicode names and inner spaces are moved verbatim.
func TestMigration_V7_UnicodeAndSpaces(t *testing.T) {
	names := []string{"我的世界 1.21", "Ünïcødé  instance", "emoji😀", "ＦＵＬＬ　ｗｉｄｔｈ"}
	for _, merge := range []bool{false, true} {
		t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
			e := vlSetup(t)
			s, n := e.legacy, vlLocal(e)
			for _, nm := range names {
				write(t, filepath.Join(s, "versions", nm, "version.json"), `{"name":`+quote(nm)+`}`)
			}
			write(t, filepath.Join(s, "backups", "worlds", "世界", "世界_1.mcworld"), "w")
			write(t, filepath.Join(s, "backups", "instances", "我的世界 1.21", "我的世界 1.21_1.zip"), "z")
			write(t, filepath.Join(s, "config.json"), `{"window_width":800}`)
			want := append([]string(nil), names...)
			if merge {
				write(t, filepath.Join(n, "Instances", "Other", "version.json"), `{"name":"Other"}`)
				write(t, filepath.Join(n, "Backups", "Worlds", "X", "x.mcworld"), "x")
				want = append(want, "Other")
			}

			vlPrepare(t, Migrated)
			vlTree(t, "N", n)
			ents, err := os.ReadDir(filepath.Join(n, "Instances"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, en := range ents {
				got = append(got, en.Name())
			}
			sort.Strings(got)
			wantSorted := append([]string(nil), want...)
			sort.Strings(wantSorted)
			if strings.Join(got, "|") != strings.Join(wantSorted, "|") {
				t.Errorf("Instances holds %q, want %q", got, wantSorted)
			}
			for _, nm := range names {
				vlWant(t, map[string]string{filepath.Join(n, "Instances", nm, "version.json"): `{"name":` + quote(nm) + `}`})
				vlTranslate(t, filepath.Join(s, "versions", nm), filepath.Join(n, "Instances", nm))
			}
			if got := actualName(filepath.Join(n, "Backups", "Worlds", "世界")); got != "世界" {
				t.Errorf("world backup folder = %q", got)
			}

			vlStartup(t)
			vlWant(t, map[string]string{
				filepath.Join(apppath.WorldBackupsDir(), "世界", "世界_1.mcworld"):              "w",
				filepath.Join(apppath.InstanceBackupsDir(), "我的世界 1.21", "我的世界 1.21_1.zip"): "z",
			})
			if merge {
				vlWant(t, map[string]string{filepath.Join(apppath.WorldBackupsDir(), "X", "x.mcworld"): "x"})
			}
			vlWantNames(t, want...)
			vlIdempotent(t, s, n)
		})
	}
}

// ---------------------------------------------------------------- V8

// V8: the game runs from an instance, its exe open without
// FILE_SHARE_DELETE. Every change is undone and the launcher runs on the
// legacy layout; once the game is closed the next start migrates.
func TestMigration_V8_LockedFileInInstance(t *testing.T) {
	for _, withCurrent := range []bool{false, true} {
		t.Run(fmt.Sprintf("existingCurrentLayout=%v", withCurrent), func(t *testing.T) {
			e := vlSetup(t)
			s, n := e.legacy, vlLocal(e)
			seedLegacy(t, s, s)
			write(t, filepath.Join(s, "config.json"), `{"window_width":900}`)
			cfgDir := filepath.Join(e.roaming, apppath.AppDirName)
			if withCurrent {
				// Instances exists, so versions is merged child by child and
				// the locked instance is not the first entry moved. It is empty:
				// a current layout made without the user's data.
				write(t, filepath.Join(s, "versions", "0first", "version.json"), `{"name":"0first"}`)
				if err := os.MkdirAll(filepath.Join(n, "Instances"), 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, filepath.Join(n, "Logs", "startup.log"), "new log")
				write(t, apppath.ConfigPath(), `{"window_width":1}`)
			}
			sBefore, nBefore, cBefore := vlSnap(t, s), vlSnap(t, n), vlSnap(t, cfgDir)

			h := vlLock(t, filepath.Join(s, "versions", "A", "Minecraft.Windows.exe"))
			outcome, err := Prepare()
			windows.CloseHandle(h)
			vlTree(t, "S", s)
			vlTree(t, "N", n)
			if outcome != Legacy || err == nil {
				t.Fatalf("Prepare = %s, %v; want Legacy with an error", vlOutcome(outcome), err)
			}
			if !samePath(apppath.LegacyMode(), s) {
				t.Errorf("LegacyMode = %q, want %s", apppath.LegacyMode(), s)
			}
			vlDiff(t, "S after undo", sBefore, vlSnap(t, s))
			vlDiff(t, "N after undo", nBefore, vlSnap(t, n))
			vlDiff(t, "config dir after undo", cBefore, vlSnap(t, cfgDir))
			vlAbsent(t, apppath.MigrationJournalPath(), apppath.MigrationPath())
			if !withCurrent {
				vlAbsent(t, filepath.Join(n, "Instances"), filepath.Join(n, "Cache"), filepath.Join(n, "Accounts"),
					filepath.Join(n, "WebView2"), filepath.Join(cfgDir, "config.json"))
			}
			// The legacy layout is where the app reads.
			if inst, _ := apppath.InstancesDir(); !samePath(inst, filepath.Join(s, "versions")) {
				t.Errorf("legacy InstancesDir = %s", inst)
			}
			if !samePath(apppath.AccountsDir(), filepath.Join(s, "microsoft-account")) || !samePath(apppath.ConfigPath(), filepath.Join(s, "config.json")) {
				t.Errorf("legacy getters: accounts %s, config %s", apppath.AccountsDir(), apppath.ConfigPath())
			}

			// The game is closed: the next start migrates.
			apppath.SetLegacyMode("")
			vlPrepare(t, Migrated)
			vlWant(t, vlE1Data(n))
			vlWant(t, vlE1Local(n))
			want := []string{"A"}
			if withCurrent {
				vlWant(t, map[string]string{filepath.Join(n, "Instances", "0first", "version.json"): `{"name":"0first"}`})
				want = append(want, "0first")
			}
			if cfg := readJSON(t, apppath.ConfigPath()); cfg["window_width"] != float64(900) {
				t.Errorf("config = %v; the legacy settings take precedence at a first migration", cfg)
			}
			vlStartup(t)
			vlGetters(t, n, n)
			vlE1Reachable(t)
			vlWantNames(t, want...)
			vlIdempotent(t, s, n, cfgDir)
		})
	}
}

// ---------------------------------------------------------------- V9

// V9: a WebView2 process of the previous launcher still holds a file for
// about a second: the rename is retried and the migration completes.
func TestMigration_V9_TransientLockReleased(t *testing.T) {
	e := vlSetup(t)
	s, n := e.legacy, vlLocal(e)
	seedLegacy(t, s, s)
	write(t, filepath.Join(s, "EBWebView", "x"), "held")
	write(t, filepath.Join(s, "config.json"), `{}`)
	h := vlLock(t, filepath.Join(s, "EBWebView", "x"))
	released := make(chan struct{})
	go func() {
		time.Sleep(time.Second)
		windows.CloseHandle(h)
		close(released)
	}()
	start := time.Now()
	outcome, err := Prepare()
	<-released
	t.Logf("Prepare = %s, %v after %v", vlOutcome(outcome), err, time.Since(start))
	if outcome != Migrated || err != nil {
		t.Fatalf("Prepare = %s, %v; want Migrated", vlOutcome(outcome), err)
	}
	vlWant(t, map[string]string{filepath.Join(n, "WebView2", "EBWebView", "x"): "held"})
	vlWant(t, vlE1Data(n))
	vlWant(t, vlE1Local(n))
	vlAbsent(t, filepath.Join(s, "EBWebView"), apppath.MigrationJournalPath())
	vlStartup(t)
	vlGetters(t, n, n)
	vlE1Reachable(t)
	vlIdempotent(t, s, n)
}

// ---------------------------------------------------------------- V10

// V10: read-only files are migrated, on one volume and across, and an undone
// migration removes its read-only copies and leaves nothing half done.
func TestMigration_V10_ReadOnlyAttributes(t *testing.T) {
	for _, cross := range []bool{false, true} {
		for _, rollback := range []bool{false, true} {
			t.Run(fmt.Sprintf("crossVolume=%v/rollback=%v", cross, rollback), func(t *testing.T) {
				var e env
				if cross {
					e = vlSetupCross(t)
				} else {
					e = vlSetup(t)
				}
				s, n := e.legacy, vlLocal(e)
				t.Cleanup(func() { vlMakeWritable(s); vlMakeWritable(e.local) })
				seedLegacy(t, s, s)
				const legacyCfg = `{"window_width":900}`
				write(t, filepath.Join(s, "config.json"), legacyCfg)
				write(t, filepath.Join(s, "microsoft-account", "wam-account.dpapi"), "wam")
				for _, p := range []string{
					filepath.Join(s, "config.json"),
					filepath.Join(s, "microsoft-account", "device.dpapi"),
					filepath.Join(s, "microsoft-account", "wam-account.dpapi"),
					filepath.Join(s, "EBWebView", "Default", "Local Storage", "leveldb"),
					filepath.Join(s, "user_gamertag_map.json"),
					filepath.Join(s, "versions", "A", "version.json"),
				} {
					if err := os.Chmod(p, 0o444); err != nil {
						t.Fatal(err)
					}
				}
				cfgDir := filepath.Join(e.roaming, apppath.AppDirName)
				sBefore, nBefore := vlSnap(t, s), vlSnap(t, n)

				if rollback {
					// logs is the last item: everything before it, the
					// read-only copies included, must be undone.
					h := vlLock(t, filepath.Join(s, "logs", "startup.log"))
					outcome, err := Prepare()
					windows.CloseHandle(h)
					vlTree(t, "S", s)
					vlTree(t, "N", n)
					if outcome != Legacy || err == nil {
						t.Fatalf("Prepare = %s, %v; want Legacy with an error", vlOutcome(outcome), err)
					}
					vlDiff(t, "S after undo", sBefore, vlSnap(t, s))
					vlDiff(t, "N after undo", nBefore, vlSnap(t, n))
					vlAbsent(t, filepath.Join(cfgDir, "config.json"), apppath.MigrationJournalPath(), apppath.MigrationPath(),
						filepath.Join(n, "Accounts"), filepath.Join(n, "WebView2"), filepath.Join(n, "Instances"))
					apppath.SetLegacyMode("")
				}
				vlPrepare(t, Migrated)
				vlTree(t, "N", n)
				tb := n
				if cross {
					tb = s
				}
				vlWant(t, vlE1Data(tb))
				vlWant(t, vlE1Local(n))
				vlWant(t, map[string]string{
					filepath.Join(n, "Accounts", "wam-account.dpapi"): "wam",
					filepath.Join(s, "config.migrated.json"):          legacyCfg,
				})
				if cfg := readJSON(t, apppath.ConfigPath()); cfg["window_width"] != float64(900) {
					t.Errorf("config = %v", cfg)
				}
				vlStartup(t)
				vlGetters(t, tb, n)
				vlE1Reachable(t)
				vlWantNames(t, "A")
				vlIdempotent(t, s, n, cfgDir)
			})
		}
	}
}

// ---------------------------------------------------------------- V11

func vlSecureCacheDACL(dir string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	// DACL_SECURITY_INFORMATION | PROTECTED_DACL_SECURITY_INFORMATION
	r, _, e := windows.NewLazySystemDLL("advapi32.dll").NewProc("SetFileSecurityW").Call(
		uintptr(unsafe.Pointer(p)), 0x80000004, uintptr(unsafe.Pointer(sd)))
	if r == 0 {
		return e
	}
	return nil
}

func vlDACLProtected(t *testing.T, p string) (bool, string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo %s: %v", p, err)
	}
	ctl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	return ctl&windows.SE_DACL_PROTECTED != 0, sd.String()
}

// vlOnlyUserAndSystem reports whether every ACE of a DACL in SDDL form
// grants the current user or SYSTEM.
func vlOnlyUserAndSystem(t *testing.T, sddl string) bool {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(sddl, "D:")
	if i < 0 {
		return false
	}
	aces := 0
	for _, ace := range strings.Split(sddl[i:], "(")[1:] {
		ace = strings.TrimSuffix(strings.SplitN(ace, ")", 2)[0], ")")
		parts := strings.Split(ace, ";")
		sid := parts[len(parts)-1]
		if sid != "SY" && sid != user.User.Sid.String() {
			return false
		}
		aces++
	}
	return aces > 0
}

// V11: microsoft-account carries the protected DACL of v1.0.x secureCache.
// After the migration the account directory is a real directory, readable,
// with the protected DACL (kept by a rename, set again after a copy).
func TestMigration_V11_AccountsDACL(t *testing.T) {
	for _, cross := range []bool{false, true} {
		t.Run(fmt.Sprintf("crossVolume=%v", cross), func(t *testing.T) {
			var e env
			if cross {
				e = vlSetupCross(t)
			} else {
				e = vlSetup(t)
			}
			s, n := e.legacy, vlLocal(e)
			acct := filepath.Join(s, "microsoft-account")
			if err := os.MkdirAll(acct, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := vlSecureCacheDACL(acct); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(acct, "device.dpapi"), "device")
			write(t, filepath.Join(acct, "wam-account.dpapi"), "wam")
			write(t, filepath.Join(acct, "device.lock"), "")
			write(t, filepath.Join(s, "versions", "A", "version.json"), `{"name":"A"}`)
			write(t, filepath.Join(s, "config.json"), `{}`)
			if prot, sddl := vlDACLProtected(t, acct); !prot {
				t.Fatalf("seeded DACL not protected: %s", sddl)
			}

			vlPrepare(t, Migrated)
			vlStartup(t)
			dir := apppath.AccountsDir()
			if !samePath(dir, filepath.Join(n, "Accounts")) {
				t.Errorf("AccountsDir = %s", dir)
			}
			info, err := os.Lstat(dir)
			if err != nil || !info.IsDir() || vlIsReparse(dir) {
				t.Fatalf("AccountsDir %s is not a real directory: %v %v", dir, info, err)
			}
			vlWant(t, map[string]string{
				filepath.Join(dir, "device.dpapi"):      "device",
				filepath.Join(dir, "wam-account.dpapi"): "wam",
				filepath.Join(dir, "device.lock"):       "",
			})
			if prot, sddl := vlDACLProtected(t, dir); !prot || !vlOnlyUserAndSystem(t, sddl) {
				t.Errorf("%s lost the protected DACL: %s", dir, sddl)
			}
			// The records allow the user and SYSTEM only, inherited or set.
			for _, f := range []string{"device.dpapi", "wam-account.dpapi"} {
				if _, sddl := vlDACLProtected(t, filepath.Join(dir, f)); !vlOnlyUserAndSystem(t, sddl) {
					t.Errorf("%s allows more than the user and SYSTEM: %s", f, sddl)
				}
			}
			// The account code can write there.
			if err := os.WriteFile(filepath.Join(dir, "probe"), []byte("x"), 0o600); err != nil {
				t.Errorf("account directory not writable: %v", err)
			}
			_ = os.Remove(filepath.Join(dir, "probe"))
			if cross {
				vlWant(t, map[string]string{filepath.Join(acct, "device.dpapi"): "device"})
			} else {
				vlAbsent(t, acct)
			}
			vlIdempotent(t, s, n)
		})
	}
}
