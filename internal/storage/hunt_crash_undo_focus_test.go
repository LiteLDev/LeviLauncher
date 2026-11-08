//go:build windows

package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// Focused reproductions of what the crash matrix in hunt_crash_undo_test.go
// found.

func huntJournal(t *testing.T, recs ...record) {
	t.Helper()
	writeJournal(t, append([]record{{Op: opBegin}}, recs...), false)
}

func listTree(root string) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && p != root {
			out = append(out, strings.TrimPrefix(p, root))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// 1. undo() keeps going after a step fails and leaves the whole journal for
// the next start, which then undoes every record again, also those already
// undone. The second undo of the Create of config.json removes the current
// config.json that the first undo of the Discard move had put back; the same
// happens to storage-migration.json. The follow-up migration then believes
// it is the first one: the old launcher's default config.json wins, base_root
// is lost and instances in the data root are no longer listed. A recovery
// cut off after its last step, before it removed the journal, does the same.
func TestHunt_crash_and_undo_rerunUndoDeletesRestoredConfig(t *testing.T) {
	e := huntSetup(t, false)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900}`)
	if o, err := Prepare(); err != nil || o != Migrated {
		t.Fatalf("first migration: %v %v", o, err)
	}
	releaseLock()
	// The user then chose a data root of their own.
	data := filepath.Join(e.root, "Data")
	write(t, filepath.Join(data, "Instances", "Mine", "version.json"), `{"name":"Mine"}`)
	current, _ := json.Marshal(map[string]any{"base_root": data, "window_width": 1500})
	cfg, rec := apppath.ConfigPath(), apppath.MigrationPath()
	write(t, cfg, string(current))
	firstRecord := read(t, rec)

	// The old version runs again: default config.json, a new instance.
	oldCfg := filepath.Join(e.legacy, "config.json")
	write(t, oldCfg, `{"base_root":"","window_width":0}`)
	write(t, filepath.Join(e.legacy, "versions", "B", "version.json"), `{"name":"B"}`)

	// The follow-up migration runs up to the record, then the process dies.
	versions := filepath.Join(e.legacy, "versions")
	b := filepath.Join(data, "Instances", "B")
	cfgBackup, recBackup := uniqueName(cfg, "-before-migration"), uniqueName(rec, "-before-migration")
	retired := uniqueName(filepath.Join(e.legacy, "config.migrated.json"), "")
	for _, step := range [][2]string{{filepath.Join(versions, "B"), b}, {oldCfg, retired}, {cfg, cfgBackup}, {rec, recBackup}} {
		if err := os.Rename(step[0], step[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(versions); err != nil {
		t.Fatal(err)
	}
	write(t, cfg, string(current))
	write(t, rec, firstRecord)
	huntJournal(t,
		record{Op: opMove, From: filepath.Join(versions, "B"), Path: b},
		record{Op: opRmdir, Path: versions},
		record{Op: opMove, From: oldCfg, Path: retired},
		record{Op: opMove, From: cfg, Path: cfgBackup, Discard: true},
		record{Op: opCreate, Path: cfg},
		record{Op: opMove, From: rec, Path: recBackup, Discard: true},
		record{Op: opCreate, Path: rec},
	)

	// Next start: a scanner holds a file of B for a few seconds, so the undo
	// of its move fails after every later step was undone.
	f, err := os.Open(filepath.Join(b, "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := Prepare()
	f.Close()
	if err == nil {
		t.Fatalf("Prepare with B held = %v, nil; want the undo to fail", o)
	}
	t.Logf("start with B held: %v, %v", o, err)
	if got := read(t, cfg); got != string(current) {
		t.Fatalf("the first undo did not restore config.json: %q", got)
	}
	releaseLock()

	// The start after that.
	if o, err := Prepare(); err != nil {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	if !exists(cfg) {
		t.Fatal("config.json is gone")
	}
	got := readJSON(t, cfg)
	if got["base_root"] != data || got["window_width"] != float64(1500) {
		t.Errorf("current config.json lost: %v (want base_root %s, window_width 1500)", got, data)
	}
	if !exists(filepath.Join(data, "Instances", "B", "version.json")) {
		t.Errorf("legacy instance B not merged into the current data root %s", data)
	}
	if r := readRecord(); r == nil || !strings.Contains(read(t, rec), `versions`) {
		t.Errorf("the first migration's record was lost; was %s", short(firstRecord))
	}
}

// 2. Same re-run, for a junction that replaced an empty directory: the second
// undo of the Move finds the empty Instances directory that the undo of the
// Rmdir recreated, and renames the junction away to versions-legacy, which is
// never migrated.
func TestHunt_crash_and_undo_rerunUndoRenamesJunctionAside(t *testing.T) {
	e := huntSetup(t, false)
	target := filepath.Join(e.root, "Elsewhere", "versions")
	write(t, filepath.Join(target, "J", "version.json"), `{"name":"J"}`)
	versions := filepath.Join(e.legacy, "versions")
	junction(t, versions, target)
	instances := filepath.Join(e.local, "LeviLauncher", "Instances")
	if err := os.MkdirAll(instances, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	// Undone already, but the process ended before the undo shortened the
	// journal past its last step: only that step is undone again (undo
	// shortens the journal after every step, so no earlier one can be).
	huntJournal(t,
		record{Op: opRmdir, Path: instances},
	)
	if o, err := Prepare(); err != nil || o != Migrated {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	if !exists(filepath.Join(instances, "J", "version.json")) {
		t.Errorf("instance J not in Instances; legacy dir: %v", listTree(e.legacy))
	}
}

// 3. Same re-run, for a file moved aside: the second undo of the Move of
// versions finds the restored file "Instances" and swaps it with versions,
// which is renamed versions-legacy and never migrated.
func TestHunt_crash_and_undo_rerunUndoSwapsMovedAsideFile(t *testing.T) {
	e := huntSetup(t, false)
	write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A"}`)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	instances := filepath.Join(e.local, "LeviLauncher", "Instances")
	write(t, instances, "a file named Instances")
	// Undone already; the journal still holds the last step undone.
	huntJournal(t,
		record{Op: opMove, From: instances, Path: instances + "-before-migration"},
	)
	if o, err := Prepare(); err != nil || o != Migrated {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	if !exists(filepath.Join(instances, "A", "version.json")) {
		t.Errorf("instance A not in Instances; legacy dir: %v", listTree(e.legacy))
	}
}

// 4. Crash after a whole directory was renamed, then the old launcher runs
// before the next start and recreates that directory (WebView2 always makes
// its profile; a world backup recreates backups\worlds). The undo merges the
// migrated directory back one level only and gives the original child the
// "-legacy" name, so the user's WebView2 profile and world backups end up in
// Default-legacy and worlds-legacy, where the new layout does not read them.
func TestHunt_crash_and_undo_oldLauncherShadowsMergedBackChildren(t *testing.T) {
	t.Run("WebView2 profile", func(t *testing.T) {
		e := huntSetup(t, false)
		seedLegacy(t, e.legacy, e.legacy)
		write(t, filepath.Join(e.legacy, "config.json"), `{}`)
		local := filepath.Join(e.local, "LeviLauncher")
		if err := os.MkdirAll(local, 0o755); err != nil {
			t.Fatal(err)
		}
		wv := filepath.Join(local, "WebView2")
		from := filepath.Join(e.legacy, "EBWebView")
		to := filepath.Join(wv, "EBWebView")
		if err := os.Mkdir(wv, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		huntJournal(t, record{Op: opMkdir, Path: wv}, record{Op: opMove, From: from, Path: to})
		// The old launcher starts: WebView2 creates a fresh profile.
		write(t, filepath.Join(from, "Default", "Local Storage", "leveldb"), "fresh profile")

		if o, err := Prepare(); err != nil || o != Migrated {
			t.Fatalf("Prepare = %v, %v", o, err)
		}
		if got := read(t, filepath.Join(to, "Default", "Local Storage", "leveldb")); got != "storage" {
			t.Errorf("the user's WebView2 profile is not in use: Default holds %q; WebView2: %v", got, listTree(wv))
		}
	})
	t.Run("world backups", func(t *testing.T) {
		e := huntSetup(t, false)
		seedLegacy(t, e.legacy, e.legacy)
		write(t, filepath.Join(e.legacy, "config.json"), `{}`)
		local := filepath.Join(e.local, "LeviLauncher")
		if err := os.MkdirAll(local, 0o755); err != nil {
			t.Fatal(err)
		}
		from := filepath.Join(e.legacy, "backups")
		to := filepath.Join(local, "Backups")
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		// The migration merges backups deeply, and journals it so.
		huntJournal(t, record{Op: opMove, From: from, Path: to, Deep: true})
		// The old launcher runs and the user backs up a world.
		write(t, filepath.Join(from, "worlds", "V9", "F_X", "X_1.mcworld"), "new backup")

		if o, err := Prepare(); err != nil || o != Migrated {
			t.Fatalf("Prepare = %v, %v", o, err)
		}
		for p, want := range map[string]string{
			filepath.Join(to, "Worlds", "W", "W_1.mcworld"):         "world",
			filepath.Join(to, "Worlds", "V9", "F_X", "X_1.mcworld"): "new backup",
		} {
			b, _ := os.ReadFile(p)
			if string(b) != want {
				t.Errorf("%s = %q, want %q; Backups: %v", p, b, want, listTree(to))
			}
		}
	})
}

// 5. Not crash-specific, found by the matrix's reference run: a legacy
// versions folder that is a junction to another volume is "copied" with
// utils.CopyDir, whose filepath.Walk does not enter a junction given as its
// root. Instances stays empty, the junction is retired as versions.migrated,
// and every instance disappears from the launcher.
func TestHunt_crash_and_undo_junctionToOtherVolumeCopiesNothing(t *testing.T) {
	e := huntSetup(t, false)
	target := filepath.Join(e.droot, "versions")
	write(t, filepath.Join(target, "J", "version.json"), `{"name":"J on D"}`)
	junction(t, filepath.Join(e.legacy, "versions"), target)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)

	if o, err := Prepare(); err != nil || o != Migrated {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	instances := filepath.Join(e.local, "LeviLauncher", "Instances")
	if got, _ := os.ReadFile(filepath.Join(instances, "J", "version.json")); string(got) != `{"name":"J on D"}` {
		t.Errorf("instance J not migrated; Instances: %v", listTree(instances))
	}
}

// 6. Not crash-specific, found by the matrix's old-launcher variants: a user
// who started with the new version (no legacy data, so no migration record),
// chose a data root, then ran an old version once (which writes a default
// config.json with "base_root": "" and creates versions\) and upgraded again.
// The migration counts as the first one and lets the old launcher's default
// config.json win: base_root is dropped and every setting reset.
func TestHunt_crash_and_undo_freshInstallDowngradeUpgradeLosesBaseRoot(t *testing.T) {
	e := huntSetup(t, false)
	if o, err := Prepare(); err != nil || o != Current {
		t.Fatalf("fresh start: %v %v", o, err)
	}
	releaseLock()
	data := filepath.Join(e.droot, "Data")
	write(t, filepath.Join(data, "Instances", "X", "version.json"), `{"name":"X"}`)
	current, _ := json.Marshal(map[string]any{"base_root": data, "window_width": 1500})
	write(t, apppath.ConfigPath(), string(current))

	// The old version runs once.
	write(t, filepath.Join(e.legacy, "config.json"), `{"base_root":"","window_width":0,"window_height":0}`)
	write(t, filepath.Join(e.legacy, "versions", "Y", "version.json"), `{"name":"Y"}`)

	if o, err := Prepare(); err != nil || o != Migrated {
		t.Fatalf("Prepare = %v, %v", o, err)
	}
	got := readJSON(t, apppath.ConfigPath())
	if got["base_root"] != data {
		t.Errorf("base_root = %v, want %s: instance X in %s is no longer listed", got["base_root"], data, data)
	}
	if got["window_width"] != float64(1500) {
		t.Errorf("window_width = %v, want 1500 (old defaults overwrote the settings)", got["window_width"])
	}
}
