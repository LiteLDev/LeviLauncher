package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// otherVolumeDir returns a unique directory on a second NTFS volume, removed
// when the test ends, or skips the test.
func otherVolumeDir(t *testing.T) string {
	t.Helper()
	root := testSecondVolume
	if !isDir(root) {
		t.Skip("no second volume")
	}
	d, err := os.MkdirTemp(root, "hunt-ss-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

func mustPrepare(t *testing.T, want Outcome) {
	t.Helper()
	resetRecordCache()
	outcome, err := Prepare()
	if outcome != want || (want != Legacy && want != Deferred && err != nil) {
		t.Fatalf("Prepare = %v, %v; want %v", outcome, err, want)
	}
}

func mustRead(t *testing.T, files map[string]string) {
	t.Helper()
	for p, want := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(b) != want {
			t.Errorf("%s = %q, want %q", p, b, want)
		}
	}
}

// v0.0.9 kept config.json, versions and installers in %LOCALAPPDATA%\LeviLauncher;
// v0.0.10 copied config.json to %APPDATA%\<exe>, deleted the old one and used
// %APPDATA%\<exe> as the data root, leaving the era-A instances behind.
func TestHunt_state_space_EraAOrphanedByV0010(t *testing.T) {
	e := setup(t)
	local := apppath.LocalDir()
	write(t, filepath.Join(local, "versions", "Old", "version.json"), `{"name":"Old"}`)
	write(t, filepath.Join(local, "installers", "old.appx"), "old pkg")
	era := filepath.Join(e.roaming, "levilauncher.exe")
	write(t, filepath.Join(era, "config.json"), `{"base_root":"","window_width":0,"window_height":0}`)
	write(t, filepath.Join(era, "versions", "New", "version.json"), `{"name":"New"}`)
	write(t, filepath.Join(era, "installers", "new.appx"), "new pkg")

	mustPrepare(t, Migrated)
	mustRead(t, map[string]string{
		filepath.Join(local, "Instances", "Old", "version.json"): `{"name":"Old"}`,
		filepath.Join(local, "Instances", "New", "version.json"): `{"name":"New"}`,
		filepath.Join(local, "Cache", "Installers", "old.appx"):  "old pkg",
		filepath.Join(local, "Cache", "Installers", "new.appx"):  "new pkg",
	})
	mustPrepare(t, Current)
}

// v0.0.9 only, upgraded straight to the current version.
func TestHunt_state_space_EraAOnly(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom root other volume"}[custom], func(t *testing.T) {
			e := setup(t)
			local := apppath.LocalDir()
			base := local
			cfg := `{"base_root":""}`
			if custom {
				base = otherVolumeDir(t)
				cfg = `{"base_root":` + quote(base) + `}`
			}
			write(t, filepath.Join(local, "config.json"), cfg)
			write(t, filepath.Join(base, "versions", "A", "version.json"), `{"name":"A"}`)
			write(t, filepath.Join(base, "installers", "a.appx"), "pkg")

			mustPrepare(t, Migrated)
			mustRead(t, map[string]string{
				filepath.Join(base, "Instances", "A", "version.json"): `{"name":"A"}`,
				filepath.Join(base, "Cache", "Installers", "a.appx"):  "pkg",
			})
			cfgNow := readJSON(t, apppath.ConfigPath())
			if custom && cfgNow["base_root"] != base {
				t.Errorf("base_root = %v, want %s", cfgNow["base_root"], base)
			}
			if !custom && cfgNow["base_root"] != nil && cfgNow["base_root"] != "" {
				t.Errorf("base_root = %v", cfgNow["base_root"])
			}
			if isFile(filepath.Join(local, "config.json")) {
				t.Error("era-A config.json not retired")
			}
			_ = e
			mustPrepare(t, Current)
		})
	}
}

// %APPDATA% redirected to another volume: the first migration keeps the data
// root in the legacy directory. The user then runs an old version again,
// which writes a default config.json and a new instance there.
func TestHunt_state_space_CrossVolumeKeepThenDowngrade(t *testing.T) {
	e := setup(t)
	roaming := filepath.Join(otherVolumeDir(t), "Roaming")
	if err := os.MkdirAll(roaming, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", roaming)
	legacy := apppath.LegacyDir()
	seedLegacy(t, legacy, legacy)
	write(t, filepath.Join(legacy, "config.json"), `{"base_root":"","window_width":900}`)

	mustPrepare(t, Migrated)
	if got := readJSON(t, apppath.ConfigPath())["base_root"]; got != legacy {
		t.Fatalf("base_root = %v, want %s", got, legacy)
	}
	mustPrepare(t, Current)

	// Old version again.
	write(t, filepath.Join(legacy, "config.json"), `{"base_root":"","window_width":800}`)
	write(t, filepath.Join(legacy, "versions", "B", "version.json"), `{"name":"B"}`)
	write(t, filepath.Join(legacy, "installers", "b.appx"), "pkg b")
	write(t, filepath.Join(legacy, "backups", "worlds", "W2", "W2_1.mcworld"), "world2")

	mustPrepare(t, Migrated)
	mustRead(t, map[string]string{
		filepath.Join(legacy, "Instances", "A", "version.json"):          `{"name":"A"}`,
		filepath.Join(legacy, "Instances", "B", "version.json"):          `{"name":"B"}`,
		filepath.Join(legacy, "Cache", "Installers", "b.appx"):           "pkg b",
		filepath.Join(legacy, "Backups", "Worlds", "W", "W_1.mcworld"):   "world",
		filepath.Join(legacy, "Backups", "Worlds", "W2", "W2_1.mcworld"): "world2",
	})
	if got := readJSON(t, apppath.ConfigPath())["base_root"]; got != legacy {
		t.Fatalf("base_root = %v, want %s", got, legacy)
	}
	_ = e
	mustPrepare(t, Current)
}

// After a migration to the default data root, an old version runs again; the
// user points it at a custom data root on a removable drive and installs an
// instance there. The drive is not connected at the next start of the
// current version. README: "its data is migrated once it is back".
func TestHunt_state_space_OfflineLegacyRootAfterMigrationIsNeverMigrated(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":""}`)
	mustPrepare(t, Migrated)

	usb := filepath.Join(otherVolumeDir(t), "USB")
	// Old version again: base_root set to the removable drive, instance B there.
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":`+quote(usb)+`}`)
	write(t, filepath.Join(usb, "versions", "B", "version.json"), `{"name":"B"}`)
	// The drive is unplugged.
	unplugged := usb + ".unplugged"
	if err := os.Rename(usb, unplugged); err != nil {
		t.Fatal(err)
	}
	resetRecordCache()
	outcome, err := Prepare()
	t.Logf("start with the drive unplugged: %v, %v", outcome, err)

	// The drive is back.
	if err := os.Rename(unplugged, usb); err != nil {
		t.Fatal(err)
	}
	resetRecordCache()
	outcome, err = Prepare()
	t.Logf("start with the drive back: %v, %v", outcome, err)

	instances := filepath.Join(apppath.LocalDir(), "Instances")
	found := false
	for _, name := range []string{"B", "B-legacy"} {
		if isFile(filepath.Join(instances, name, "version.json")) {
			found = true
		}
	}
	if !found && isDir(filepath.Join(usb, "versions", "B")) {
		t.Errorf("instance B stays in %s, which no start migrates any more: hidden from the launcher (legacy config.json retired: %v)",
			filepath.Join(usb, "versions"), isFile(filepath.Join(e.legacy, "config.migrated-2.json")))
	}
	// Nothing would ever pick it up again.
	if findLegacyDir() == "" && isDir(filepath.Join(usb, "versions")) {
		t.Errorf("no legacy directory is pending although %s holds an unmigrated instance", filepath.Join(usb, "versions"))
	}
}

// Every legacy directory an old version could have used, in one start: the
// running exe name, levilauncher.exe in both folders, a renamed exe found by
// the scan and era-A, each with an instance named A.
func TestHunt_state_space_AllStoresInOneStart(t *testing.T) {
	e := setup(t)
	local := apppath.LocalDir()
	stores := []string{
		e.legacy,
		filepath.Join(e.roaming, "levilauncher.exe"),
		filepath.Join(e.local, "levilauncher.exe"),
		filepath.Join(e.roaming, "ll-old.exe"),
		filepath.Join(e.local, "zz.exe"),
	}
	for i, s := range stores {
		write(t, filepath.Join(s, "config.json"), `{"base_root":"","window_width":`+string(rune('1'+i))+`00}`)
		write(t, filepath.Join(s, "versions", "A", "version.json"), `{"store":`+quote(s)+`}`)
		write(t, filepath.Join(s, "backups", "worlds", "W", "W_"+string(rune('1'+i))+".mcworld"), s)
	}
	write(t, filepath.Join(local, "config.json"), `{"base_root":""}`)
	write(t, filepath.Join(local, "versions", "A", "version.json"), `{"store":"eraA"}`)

	mustPrepare(t, Migrated)
	instances := filepath.Join(local, "Instances")
	ents, err := os.ReadDir(instances)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != len(stores)+1 {
		var names []string
		for _, en := range ents {
			names = append(names, en.Name())
		}
		t.Errorf("instances = %v, want %d", names, len(stores)+1)
	}
	for i := range stores {
		p := filepath.Join(local, "Backups", "Worlds", "W", "W_"+string(rune('1'+i))+".mcworld")
		if !isFile(p) {
			t.Errorf("world backup %s missing", p)
		}
	}
	// The running exe's settings win.
	if got := readJSON(t, apppath.ConfigPath())["window_width"]; got != float64(100) {
		t.Errorf("window_width = %v", got)
	}
	for _, s := range append(stores, local) {
		if isDir(filepath.Join(s, "versions")) {
			t.Errorf("%s\\versions left", s)
		}
	}
	mustPrepare(t, Current)
}

// A renamed exe whose config.json points base_root at the default legacy
// directory of levilauncher.exe, where its instances are (sharing them).
func TestHunt_state_space_RenamedExePointsAtOtherStore(t *testing.T) {
	e := setup(t)
	shared := filepath.Join(e.roaming, "levilauncher.exe")
	write(t, filepath.Join(shared, "config.json"), `{"base_root":""}`)
	write(t, filepath.Join(shared, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":`+quote(shared)+`,"window_width":777}`)
	write(t, filepath.Join(shared, "versions", "B", "version.json"), `{"name":"B"}`)

	mustPrepare(t, Migrated)
	root := apppath.BaseRoot()
	if got := readJSON(t, apppath.ConfigPath())["base_root"]; got != nil {
		if s, _ := got.(string); s != "" {
			root = s
		}
	}
	mustRead(t, map[string]string{
		filepath.Join(root, "Instances", "A", "version.json"): `{"name":"A"}`,
		filepath.Join(root, "Instances", "B", "version.json"): `{"name":"B"}`,
	})
	mustPrepare(t, Current)
}

// The first migration fails (instance in use) and the launcher runs on the
// legacy layout, where it keeps downloads in <data root>\Cache and creates
// Backups with the current case. The next start migrates everything.
func TestHunt_state_space_LegacyModeRunThenMigrate(t *testing.T) {
	e := setup(t)
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":""}`)
	write(t, filepath.Join(e.legacy, "versions", "A", "Minecraft.Windows.exe"), "exe")
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{}`)
	f, err := os.Open(filepath.Join(e.legacy, "versions", "A", "Minecraft.Windows.exe"))
	if err != nil {
		t.Fatal(err)
	}
	resetRecordCache()
	outcome, _ := Prepare()
	f.Close()
	if outcome != Legacy {
		t.Fatalf("outcome = %v", outcome)
	}
	// What the launcher writes on the legacy layout.
	write(t, filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"), "world")
	write(t, filepath.Join(apppath.DownloadsDir(), "x.mcpack"), "dl")
	write(t, apppath.GamertagsPath(), `{"1":"x"}`)
	inst, _ := apppath.InstancesDir()
	write(t, filepath.Join(inst, "C", "version.json"), `{"name":"C"}`)
	releaseLock()
	apppath.SetLegacyMode("")

	mustPrepare(t, Migrated)
	local := apppath.LocalDir()
	mustRead(t, map[string]string{
		filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"): "world",
		filepath.Join(local, "Cache", "Downloads", "x.mcpack"):        "dl",
		filepath.Join(local, "Cache", "gamertags.json"):               `{"1":"x"}`,
		filepath.Join(local, "Instances", "C", "version.json"):        `{"name":"C"}`,
	})
	mustPrepare(t, Current)
}

// Instances of a downgraded old version on a custom data root on another
// volume while the current data root is the default one.
func TestHunt_state_space_DowngradeCustomRootOtherVolume(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":""}`)
	mustPrepare(t, Migrated)

	d := otherVolumeDir(t)
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":`+quote(d)+`}`)
	write(t, filepath.Join(d, "versions", "B", "version.json"), `{"name":"B"}`)
	write(t, filepath.Join(d, "backups", "worlds", "W", "W_9.mcworld"), "w9")
	mustPrepare(t, Migrated)
	local := apppath.LocalDir()
	mustRead(t, map[string]string{
		filepath.Join(local, "Instances", "B", "version.json"):        `{"name":"B"}`,
		filepath.Join(local, "Backups", "Worlds", "W", "W_9.mcworld"): "w9",
		filepath.Join(local, "Instances", "A", "version.json"):        `{"name":"A"}`,
	})
	if !isDir(filepath.Join(d, "versions.migrated")) {
		t.Error("source not retired")
	}
	mustPrepare(t, Current)
	// Third downgrade cycle with the same custom root: versions recreated.
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":`+quote(d)+`}`)
	write(t, filepath.Join(d, "versions", "B2", "version.json"), `{"name":"B2"}`)
	mustPrepare(t, Migrated)
	mustRead(t, map[string]string{filepath.Join(local, "Instances", "B2", "version.json"): `{"name":"B2"}`})
	mustPrepare(t, Current)
}

var _ = json.Marshal

// %APPDATA% on another volume (first store kept in place) plus stores on the
// local volume: %LOCALAPPDATA%\levilauncher.exe (runs without APPDATA) and
// era-A, all in one start, then idempotent.
func TestHunt_state_space_CrossVolumeFirstStorePlusLocalStores(t *testing.T) {
	e := setup(t)
	roaming := filepath.Join(otherVolumeDir(t), "Roaming")
	if err := os.MkdirAll(roaming, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", roaming)
	legacy := apppath.LegacyDir()
	seedLegacy(t, legacy, legacy)
	write(t, filepath.Join(legacy, "config.json"), `{"base_root":""}`)
	noAppData := filepath.Join(e.local, "levilauncher.exe")
	write(t, filepath.Join(noAppData, "config.json"), `{"base_root":""}`)
	write(t, filepath.Join(noAppData, "versions", "L", "version.json"), `{"name":"L"}`)
	write(t, filepath.Join(noAppData, "microsoft-account", "x"), "acct2")
	local := apppath.LocalDir()
	write(t, filepath.Join(local, "versions", "E", "version.json"), `{"name":"E"}`)
	write(t, filepath.Join(local, "installers", "e.appx"), "e")

	mustPrepare(t, Migrated)
	mustRead(t, map[string]string{
		filepath.Join(legacy, "Instances", "A", "version.json"): `{"name":"A"}`,
		filepath.Join(legacy, "Instances", "L", "version.json"): `{"name":"L"}`,
		filepath.Join(legacy, "Instances", "E", "version.json"): `{"name":"E"}`,
		filepath.Join(legacy, "Cache", "Installers", "e.appx"):  "e",
		filepath.Join(local, "Accounts", "device.dpapi"):        "device",
	})
	mustPrepare(t, Current)
}
