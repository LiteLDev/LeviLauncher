package storage

// Regression tests for the group "journal-crash-downgrade": interrupted
// migrations and their recovery, repeated starts, and cycles of running an
// old version (v1.0.x) and the new one again. The expected outcomes are the
// ones internal/apppath/README.md ("从旧布局迁移") specifies.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// ------------------------------------------------------------------ helpers

// jcdOldDefaultConfig is what v1.0.x config.Load writes when config.json is
// missing: the zero AppConfig, indented.
const jcdOldDefaultConfig = `{
  "base_root": "",
  "window_width": 0,
  "window_height": 0,
  "disable_discord_rpc": false,
  "enable_beta_updates": false,
  "loader_migrated_v1": false
}`

// jcdEnvAt is setup() with fixed roots, so that journal records captured in
// one run stay valid for a tree seeded again.
func jcdEnvAt(t *testing.T, roaming, local string) env {
	t.Helper()
	e := env{roaming: roaming, local: local}
	t.Setenv("APPDATA", e.roaming)
	t.Setenv("LOCALAPPDATA", e.local)
	e.legacy = apppath.LegacyDir()
	jcdWipe(t, e)
	t.Cleanup(func() {
		releaseLock()
		apppath.SetLegacyMode("")
		apppath.SetBaseRootOverride("")
		resetRecordCache()
	})
	return e
}

// jcdWipe empties both roots and any extra directory.
func jcdWipe(t *testing.T, e env, extra ...string) {
	t.Helper()
	releaseLock()
	for _, d := range extra {
		if err := os.RemoveAll(d); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{e.roaming, e.local} {
		if err := os.RemoveAll(d); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
}

// jcdPrepare is a fresh start of the launcher: no legacy mode and no base
// root from an earlier config.Load in this process.
func jcdPrepare() (Outcome, error) {
	releaseLock()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
	return Prepare()
}

// jcdSeedE1 is a v1.0.x layout: seedLegacy below the data root base (the
// legacy directory for the default), its config.json, and the startup.log the
// new version opens before Prepare.
func jcdSeedE1(t *testing.T, e env, base string) {
	t.Helper()
	seedLegacy(t, e.legacy, base)
	cfg := map[string]any{"base_root": "", "window_width": 900, "future_key": "kept"}
	if base != e.legacy {
		cfg["base_root"] = base
	}
	b, _ := json.Marshal(cfg)
	write(t, filepath.Join(e.legacy, "config.json"), string(b))
	write(t, filepath.Join(e.local, "LeviLauncher", "Logs", "startup.log"), "new log")
}

// jcdSnapshot maps every entry below roots to its content ("<dir>" for a
// directory), with the name case as stored. The migration record's
// migrated_at and the lock file are left out.
func jcdSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				out[p] = "<err " + err.Error() + ">"
				return nil
			}
			if p == root || strings.EqualFold(d.Name(), apppath.MigrationLockFileName) {
				return nil
			}
			if d.IsDir() {
				out[p] = "<dir>"
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				out[p] = "<unreadable>"
				return nil
			}
			if strings.EqualFold(d.Name(), apppath.MigrationFileName) {
				var m map[string]any
				if json.Unmarshal(b, &m) == nil {
					delete(m, "migrated_at")
					b, _ = json.Marshal(m)
				}
			}
			out[p] = string(b)
			return nil
		})
	}
	return out
}

func jcdDiff(want, got map[string]string) []string {
	var d []string
	for k, v := range want {
		g, ok := got[k]
		switch {
		case !ok:
			d = append(d, "missing "+k)
		case g != v:
			d = append(d, fmt.Sprintf("changed %s: %q -> %q", k, v, g))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			d = append(d, "extra "+k)
		}
	}
	sort.Strings(d)
	return d
}

func jcdLogTree(t *testing.T, roots ...string) {
	t.Helper()
	s := jcdSnapshot(t, roots...)
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
		fmt.Fprintf(&b, "  %s = %q\n", k, v)
	}
	t.Logf("tree:\n%s", b.String())
}

func jcdMustRead(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return
	}
	if string(b) != want {
		t.Errorf("%s = %q, want %q", path, string(b), want)
	}
}

// jcdStartApp does what main.go does after Prepare: EnsureDirs, then
// config.Load (Reload, as its cache is process-wide), which sets the data
// root the getters use.
func jcdStartApp(t *testing.T) config.AppConfig {
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

// jcdInstanceNames lists the instances the launcher shows.
func jcdInstanceNames(t *testing.T) []string {
	t.Helper()
	dir, err := apppath.InstancesDir()
	if err != nil {
		t.Fatalf("InstancesDir: %v", err)
	}
	ms, err := versions.ScanVersions(dir)
	if err != nil {
		t.Fatalf("ScanVersions(%s): %v", dir, err)
	}
	var names []string
	for _, m := range ms {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}

func jcdContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// jcdInstanceMeta is the stored metadata of an instance, wherever it is.
func jcdInstanceMeta(t *testing.T, dir string) string {
	t.Helper()
	for _, p := range []string{
		filepath.Join(dir, apppath.InstanceStateDirName, apppath.InstanceMetaFileName),
		filepath.Join(dir, apppath.LegacyInstanceMetaFileName),
	} {
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
	}
	t.Errorf("no metadata in %s", dir)
	return ""
}

// jcdAssertSeedReachable starts the app and checks that every item
// seedLegacy wrote is where the getters of the running launcher look,
// byte-identical.
func jcdAssertSeedReachable(t *testing.T) config.AppConfig {
	t.Helper()
	c := jcdStartApp(t)
	names := jcdInstanceNames(t)
	if !jcdContains(names, "A") {
		t.Errorf("instances = %v, want A", names)
	}
	inst, _ := apppath.InstancesDir()
	jcdMustRead(t, filepath.Join(inst, "A", "Minecraft.Windows.exe"), "exe")
	if got := jcdInstanceMeta(t, filepath.Join(inst, "A")); got != `{"name":"A"}` {
		t.Errorf("metadata of A = %q", got)
	}
	jcdMustRead(t, filepath.Join(apppath.WorldBackupsDir(), "W", "W_1.mcworld"), "world")
	jcdMustRead(t, filepath.Join(apppath.InstanceBackupsDir(), "A", "A_1.zip"), "zip")
	if got := actualName(apppath.WorldBackupsDir()); got != apppath.WorldBackupsDirName {
		t.Errorf("world backups directory is named %q", got)
	}
	installers, _ := apppath.InstallersDir()
	jcdMustRead(t, filepath.Join(installers, "1.21.msixvc"), "pkg")
	jcdMustRead(t, filepath.Join(apppath.UpdatesDir(), "1.0.8.zip"), "update")
	jcdMustRead(t, filepath.Join(apppath.AccountsDir(), "device.dpapi"), "device")
	jcdMustRead(t, filepath.Join(apppath.WebView2Dir(), "EBWebView", "Default", "Local Storage", "leveldb"), "storage")
	jcdMustRead(t, apppath.GamertagsPath(), `{"1":"Steve"}`)
	jcdMustRead(t, apppath.GDKPendingVersionsPath(), `[]`)
	jcdMustRead(t, filepath.Join(apppath.LogsDir(), "startup-legacy.log"), "old log")
	return c
}

func jcdWriteJournal(t *testing.T, recs []record, torn *record) {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		j, _ := json.Marshal(r)
		b.Write(j)
		b.WriteByte('\n')
	}
	if torn != nil {
		j, _ := json.Marshal(*torn)
		b.Write(j[:len(j)/2])
	}
	write(t, apppath.MigrationJournalPath(), b.String())
}

// jcdApplyForward makes the change a journal record announces, as the
// migration does right after appending it.
func jcdApplyForward(t *testing.T, r record, created map[string]string) {
	t.Helper()
	var err error
	switch r.Op {
	case opBegin, opCommit:
	case opMkdir:
		err = os.Mkdir(r.Path, 0o755)
	case opMove:
		err = os.Rename(r.From, r.Path)
	case opRmdir:
		err = os.Remove(r.Path)
	case opCreate:
		c, ok := created[strings.ToLower(r.Path)]
		if !ok {
			t.Fatalf("no content for created file %s", r.Path)
		}
		err = os.WriteFile(r.Path, []byte(c), 0o644)
	default:
		t.Fatalf("cannot replay %+v", r)
	}
	if err != nil {
		t.Fatalf("replay %+v: %v", r, err)
	}
}

// jcdCaptureJournal runs Prepare while a handle without FILE_SHARE_DELETE
// keeps the journal from being removed, so that every record the migration
// appended, Commit included, is left to read. A file of the legacy WebView2
// profile is held open until that handle is taken, so that the migration
// waits for it (renameRetry).
func jcdCaptureJournal(t *testing.T, e env) ([]record, Outcome, error) {
	t.Helper()
	hold, err := os.Open(filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb"))
	if err != nil {
		t.Fatal(err)
	}
	type res struct {
		o   Outcome
		err error
	}
	done := make(chan res, 1)
	go func() {
		o, err := jcdPrepare()
		done <- res{o, err}
	}()
	jp := apppath.MigrationJournalPath()
	p16, _ := windows.UTF16PtrFromString(jp)
	var h windows.Handle
	deadline := time.Now().Add(2 * time.Second)
	for {
		h, err = windows.CreateFile(p16, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			hold.Close()
			<-done
			t.Fatalf("journal never appeared: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	hold.Close()
	r := <-done
	windows.CloseHandle(h)
	recs, rerr := readJournal(jp)
	if rerr != nil {
		t.Fatalf("read journal: %v", rerr)
	}
	return recs, r.o, r.err
}

// jcdCrossVolumeEnv puts %APPDATA% (and so the legacy directory) on the
// second NTFS volume and %LOCALAPPDATA% on the system one.
func jcdCrossVolumeEnv(t *testing.T) env {
	t.Helper()
	parent := testSecondVolume
	if _, err := os.Stat(parent); err != nil {
		t.Skipf("no second volume: %v", err)
	}
	var rb [6]byte
	_, _ = rand.Read(rb[:])
	dRoot := filepath.Join(parent, "mig-jcd-"+hex.EncodeToString(rb[:]))
	if err := os.Mkdir(dRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		releaseLock()
		_ = os.RemoveAll(dRoot)
	})
	e := jcdEnvAt(t, filepath.Join(dRoot, "Roaming"), filepath.Join(t.TempDir(), "Local"))
	if sameVolume(e.legacy, e.local) {
		t.Skip("not two volumes")
	}
	return e
}

func jcdFindFileWithContent(t *testing.T, dir, content string) string {
	t.Helper()
	found := ""
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil && string(b) == content {
			found = p
		}
		return nil
	})
	return found
}

// ------------------------------------------------------------------ J1

// J1: the process is killed after any journal append, before or after the
// change it announces, or while it writes a journal line. The next start
// undoes an uncommitted migration and migrates again, ending exactly where an
// uninterrupted migration ends; a committed one only loses its journal.
func TestMigration_J1_CrashAtEachJournalStep(t *testing.T) {
	type variant struct {
		name string
		seed func(t *testing.T, e env)
	}
	for _, v := range []variant{
		{"E1", func(t *testing.T, e env) { jcdSeedE1(t, e, e.legacy) }},
		// A current layout made without the user's data: moveAside and the
		// config.json kept for an undo (Discard) are journaled too.
		{"E1_into_existing_current_layout", func(t *testing.T, e env) {
			jcdSeedE1(t, e, e.legacy)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(e.roaming, "LeviLauncher", "config.json"), `{"window_width":1024,"current_only":"kept"}`)
			write(t, filepath.Join(local, "Accounts", "device.dpapi"), "fresh device")
			write(t, filepath.Join(local, "WebView2", "EBWebView", "Default", "fresh"), "fresh profile")
		}},
	} {
		t.Run(v.name, func(t *testing.T) {
			root := t.TempDir()
			e := jcdEnvAt(t, filepath.Join(root, "Roaming"), filepath.Join(root, "Local"))
			roots := []string{e.roaming, e.local}

			// Golden: an uninterrupted migration.
			v.seed(t, e)
			if o, err := jcdPrepare(); err != nil || o != Migrated {
				t.Fatalf("golden Prepare = %v, %v", o, err)
			}
			if exists(apppath.MigrationJournalPath()) {
				t.Fatal("journal left after a committed migration")
			}
			golden := jcdSnapshot(t, roots...)
			created := map[string]string{
				strings.ToLower(apppath.ConfigPath()):    read(t, apppath.ConfigPath()),
				strings.ToLower(apppath.MigrationPath()): read(t, apppath.MigrationPath()),
			}
			if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "A")); !samePath(got, filepath.Join(e.local, "LeviLauncher", "Instances", "A")) {
				t.Errorf("golden TranslateLegacyPath = %s", got)
			}
			c := jcdAssertSeedReachable(t)
			cfg := readJSON(t, apppath.ConfigPath())
			if c.WindowWidth != 900 || cfg["future_key"] != "kept" {
				t.Errorf("golden config = %v", cfg)
			}
			if _, ok := cfg["base_root"]; ok {
				t.Errorf("golden config has base_root: %v", cfg)
			}

			// Capture the journal of the same migration.
			jcdWipe(t, e)
			v.seed(t, e)
			recs, capO, capErr := jcdCaptureJournal(t, e)
			t.Logf("journal (%d records):", len(recs))
			for i, r := range recs {
				t.Logf("  %2d %s %s <- %s (discard=%v)", i, r.Op, r.Path, r.From, r.Discard)
			}
			if len(recs) < 3 || recs[0].Op != opBegin || recs[len(recs)-1].Op != opCommit {
				t.Fatalf("unexpected journal")
			}

			// The Commit record is durable but the journal cannot be removed
			// (a scanner holds it). The migration is complete; this start
			// runs on it, and the next one removes the journal.
			t.Run("commit_written_journal_not_removable", func(t *testing.T) {
				if capErr != nil && capO == Current {
					t.Errorf("Prepare = %v, %v: the launcher refuses to start after a committed migration", capO, capErr)
				}
				o, err := jcdPrepare()
				if err != nil {
					jcdLogTree(t, roots...)
					t.Fatalf("next Prepare = %v, %v", o, err)
				}
				if exists(apppath.MigrationJournalPath()) {
					t.Error("journal left behind")
				}
				if d := jcdDiff(golden, jcdSnapshot(t, roots...)); len(d) > 0 {
					jcdLogTree(t, roots...)
					t.Errorf("state differs from an uninterrupted migration:\n%s", strings.Join(d, "\n"))
				}
			})

			// A torn line is the state after k-1 plus garbage: once per kind
			// of record is enough to keep the run fast.
			tornSeen := map[string]bool{}
			for k := 1; k < len(recs); k++ {
				for _, mode := range []string{"after", "before", "torn"} {
					if recs[k].Op == opCommit && mode == "before" {
						continue
					}
					if mode == "torn" {
						kind := fmt.Sprintf("%s/%v", recs[k].Op, recs[k].Discard)
						if tornSeen[kind] {
							continue
						}
						tornSeen[kind] = true
					}
					k, mode := k, mode
					t.Run(fmt.Sprintf("k%02d_%s_%s", k, recs[k].Op, mode), func(t *testing.T) {
						jcdWipe(t, e)
						v.seed(t, e)
						applied := k
						if mode != "after" {
							applied = k - 1
						}
						for i := 1; i <= applied; i++ {
							jcdApplyForward(t, recs[i], created)
						}
						if mode == "torn" {
							jcdWriteJournal(t, recs[:k], &recs[k])
						} else {
							jcdWriteJournal(t, recs[:k+1], nil)
						}
						committed := recs[k].Op == opCommit && mode == "after"
						want := Migrated
						if committed {
							want = Current
						}
						o, err := jcdPrepare()
						if err != nil || o != want {
							jcdLogTree(t, roots...)
							t.Fatalf("Prepare = %v, %v; want %v", o, err, want)
						}
						if exists(apppath.MigrationJournalPath()) {
							t.Error("journal left behind")
						}
						if d := jcdDiff(golden, jcdSnapshot(t, roots...)); len(d) > 0 {
							jcdLogTree(t, roots...)
							t.Errorf("state differs from an uninterrupted migration:\n%s", strings.Join(d, "\n"))
						}
						before := jcdSnapshot(t, roots...)
						if o, err := jcdPrepare(); err != nil || o != Current {
							t.Errorf("second Prepare = %v, %v", o, err)
						}
						if d := jcdDiff(before, jcdSnapshot(t, roots...)); len(d) > 0 {
							t.Errorf("second Prepare changed the tree:\n%s", strings.Join(d, "\n"))
						}
					})
				}
			}
		})
	}
}

// ------------------------------------------------------------------ J2

// J2: a migration was interrupted, then the user ran an old version, which
// recreated what had been moved (versions, config.json, the WebView2
// profile). The undo does not overwrite: it merges back and keeps a name
// that is taken as "-legacy". The launcher starts, and no data is lost.
func TestMigration_J2_UndoBlockedByRecreatedSource(t *testing.T) {
	for _, reuseName := range []bool{false, true} {
		name := "versions_recreated_with_New"
		if reuseName {
			name = "versions_recreated_with_A_again"
		}
		t.Run(name, func(t *testing.T) {
			e := setup(t)
			jcdSeedE1(t, e, e.legacy)
			local := filepath.Join(e.local, "LeviLauncher")
			from := filepath.Join(e.legacy, "versions")
			to := filepath.Join(local, "Instances")
			if err := os.Rename(from, to); err != nil {
				t.Fatal(err)
			}
			jcdWriteJournal(t, []record{{Op: opBegin}, {Op: opMove, From: from, Path: to}}, nil)
			// v1.0.x runs: its config.json is still there, so it uses
			// S\versions, which it recreates, and installs an instance.
			newName, newMeta := "New", `{"name":"New"}`
			if reuseName {
				newName, newMeta = "A", `{"name":"A","old":true}`
			}
			write(t, filepath.Join(from, newName, "version.json"), newMeta)

			o, err := jcdPrepare()
			if err != nil || o != Migrated {
				jcdLogTree(t, e.roaming, e.local)
				t.Fatalf("Prepare = %v, %v; want Migrated", o, err)
			}
			if exists(apppath.MigrationJournalPath()) {
				t.Error("journal left behind")
			}
			c := jcdStartApp(t)
			if c.WindowWidth != 900 {
				t.Errorf("window_width = %d", c.WindowWidth)
			}
			inst, _ := apppath.InstancesDir()
			if !samePath(inst, to) {
				t.Errorf("InstancesDir = %s", inst)
			}
			names := jcdInstanceNames(t)
			if reuseName {
				// The undo merged the original A back; the old version's A,
				// made since, was in its way and keeps the name A-legacy.
				if !jcdContains(names, "A") || !jcdContains(names, "A-legacy") {
					jcdLogTree(t, e.roaming, e.local)
					t.Fatalf("instances = %v, want A and A-legacy", names)
				}
				if got := jcdInstanceMeta(t, filepath.Join(inst, "A-legacy")); !strings.Contains(got, `"A-legacy"`) || !strings.Contains(got, `"old"`) {
					t.Errorf("A-legacy metadata = %q", got)
				}
				jcdMustRead(t, filepath.Join(inst, "A", "Minecraft.Windows.exe"), "exe")
				if got := jcdInstanceMeta(t, filepath.Join(inst, "A")); strings.Contains(got, `"old"`) {
					t.Errorf("A metadata = %q", got)
				}
				if got := TranslateLegacyPath(filepath.Join(from, "A")); !samePath(got, filepath.Join(inst, "A")) {
					t.Errorf("TranslateLegacyPath(S\\versions\\A) = %s", got)
				}
			} else {
				if !jcdContains(names, "A") || !jcdContains(names, "New") {
					jcdLogTree(t, e.roaming, e.local)
					t.Fatalf("instances = %v, want A and New", names)
				}
				jcdMustRead(t, filepath.Join(inst, "A", "Minecraft.Windows.exe"), "exe")
				if got := jcdInstanceMeta(t, filepath.Join(inst, "New")); got != `{"name":"New"}` {
					t.Errorf("New metadata = %q", got)
				}
				if got := TranslateLegacyPath(filepath.Join(from, "New")); !samePath(got, filepath.Join(inst, "New")) {
					t.Errorf("TranslateLegacyPath(S\\versions\\New) = %s", got)
				}
			}
			if exists(from) {
				t.Error("S\\versions still marks the legacy layout as not migrated")
			}
			if o, err := jcdPrepare(); err != nil || o != Current {
				t.Errorf("next Prepare = %v, %v", o, err)
			}
		})
	}

	// The crash came after the legacy config.json was retired, that is
	// after every item had moved. The old version then found no config.json
	// and wrote its default, recreated S\versions with a new instance and its
	// WebView2 profile.
	t.Run("crash_after_config_retired_then_old_version", func(t *testing.T) {
		root := t.TempDir()
		e := jcdEnvAt(t, filepath.Join(root, "Roaming"), filepath.Join(root, "Local"))
		jcdSeedE1(t, e, e.legacy)
		recs, _, _ := jcdCaptureJournal(t, e)
		jcdWipe(t, e)
		jcdSeedE1(t, e, e.legacy)
		legacyCfg := filepath.Join(e.legacy, "config.json")
		userCfg := read(t, legacyCfg)
		cut := -1
		for i, r := range recs {
			if r.Op == opMove && samePath(r.From, legacyCfg) {
				cut = i
				break
			}
		}
		if cut < 0 {
			t.Fatal("no retire of config.json in the journal")
		}
		for i := 1; i <= cut; i++ {
			jcdApplyForward(t, recs[i], nil)
		}
		jcdWriteJournal(t, recs[:cut+1], nil)
		write(t, legacyCfg, jcdOldDefaultConfig)
		write(t, filepath.Join(e.legacy, "versions", "New", "version.json"), `{"name":"New"}`)
		write(t, filepath.Join(e.legacy, "EBWebView", "Default", "fresh"), "old version profile")

		o, err := jcdPrepare()
		if err != nil || o != Migrated {
			jcdLogTree(t, e.roaming, e.local)
			t.Fatalf("Prepare = %v, %v; want Migrated", o, err)
		}
		c := jcdStartApp(t)
		cfg := readJSON(t, apppath.ConfigPath())
		if c.WindowWidth != 900 || cfg["future_key"] != "kept" {
			t.Errorf("the user's settings were not migrated: %v", cfg)
		}
		if _, ok := cfg["base_root"]; ok {
			t.Errorf("base_root present: %v", cfg)
		}
		names := jcdInstanceNames(t)
		if !jcdContains(names, "A") || !jcdContains(names, "New") {
			jcdLogTree(t, e.roaming, e.local)
			t.Errorf("instances = %v, want A and New", names)
		}
		// Nothing is lost: the user's config.json, the old version's
		// default, and both WebView2 profiles are kept.
		if p := jcdFindFileWithContent(t, e.legacy, userCfg); p == "" {
			t.Error("the user's legacy config.json is gone")
		}
		if p := jcdFindFileWithContent(t, e.legacy, jcdOldDefaultConfig); p == "" {
			t.Error("the old version's config.json is gone")
		}
		wv := filepath.Join(apppath.WebView2Dir(), "EBWebView")
		if p := jcdFindFileWithContent(t, wv, "storage"); p == "" {
			t.Error("the user's WebView2 profile is gone")
		} else {
			t.Logf("user's WebView2 leveldb is at %s", p)
		}
		if p := jcdFindFileWithContent(t, wv, "old version profile"); p == "" {
			t.Error("the old version's WebView2 profile is gone")
		}
		if o, err := jcdPrepare(); err != nil || o != Current {
			t.Errorf("next Prepare = %v, %v", o, err)
		}
	})
}

// ------------------------------------------------------------------ J3

// J3: after a migration of the default layout the user runs v1.0.x again
// (it writes a fresh config.json, installs an instance, recreates its
// WebView2 profile, logs and account records), then the new version. The new
// data wins (README "之后的迁移"): the instance is merged, config.json keeps
// the current members, the WebView2 profile and accounts stay where they are,
// and the record accumulates.
func TestMigration_J3_DowngradeCycleDefault(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "old_version_installs_Z"
		if conflict {
			name = "old_version_installs_Z_and_A_again"
		}
		t.Run(name, func(t *testing.T) {
			e := setup(t)
			jcdSeedE1(t, e, e.legacy)
			if o, err := jcdPrepare(); err != nil || o != Migrated {
				t.Fatalf("first Prepare = %v, %v", o, err)
			}
			local := filepath.Join(e.local, "LeviLauncher")
			rec1 := readRecord()
			cfg1 := readJSON(t, apppath.ConfigPath())
			migrated1 := read(t, filepath.Join(e.legacy, "config.migrated.json"))

			// v1.0.x runs again.
			write(t, filepath.Join(e.legacy, "config.json"), jcdOldDefaultConfig)
			write(t, filepath.Join(e.legacy, "versions", "Z", "version.json"), `{"name":"Z"}`)
			if conflict {
				write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A","old":true}`)
			}
			write(t, filepath.Join(e.legacy, "EBWebView", "Default", "fresh"), "old version profile")
			write(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device2")
			write(t, filepath.Join(e.legacy, "logs", "startup.log"), "old log 2")

			o, err := jcdPrepare()
			if err != nil || o != Migrated {
				jcdLogTree(t, e.roaming, e.local)
				t.Fatalf("second Prepare = %v, %v", o, err)
			}
			jcdMustRead(t, filepath.Join(e.legacy, "config.migrated.json"), migrated1)
			jcdMustRead(t, filepath.Join(e.legacy, "config.migrated-2.json"), jcdOldDefaultConfig)
			if exists(filepath.Join(e.legacy, "config.json")) || exists(filepath.Join(e.legacy, "versions")) {
				t.Error("legacy data still pending")
			}
			// The current WebView2 profile and accounts win; the old
			// version's stay where they are.
			jcdMustRead(t, filepath.Join(e.legacy, "EBWebView", "Default", "fresh"), "old version profile")
			jcdMustRead(t, filepath.Join(e.legacy, "microsoft-account", "device.dpapi"), "device2")
			if exists(filepath.Join(local, "WebView2", "EBWebView-before-migration")) || exists(filepath.Join(local, "Accounts-before-migration")) {
				t.Error("the current WebView2 profile or accounts were replaced")
			}
			if p := jcdFindFileWithContent(t, filepath.Join(local, "Logs"), "old log 2"); p == "" {
				t.Error("the old version's log was not merged")
			}

			cfg := readJSON(t, apppath.ConfigPath())
			for k, v := range cfg1 {
				if fmt.Sprint(cfg[k]) != fmt.Sprint(v) {
					t.Errorf("config %s = %v, want the current %v", k, cfg[k], v)
				}
			}
			if _, ok := cfg["base_root"]; ok {
				t.Errorf("base_root present: %v", cfg)
			}
			rec2 := readRecord()
			if rec2 == nil || len(rec2.Moves) <= len(rec1.Moves) {
				t.Fatalf("record moves did not accumulate: %+v", rec2)
			}
			for i, m := range rec1.Moves {
				if rec2.Moves[i] != m {
					t.Errorf("earlier move %d changed: %+v -> %+v", i, m, rec2.Moves[i])
				}
			}
			resetRecordCache()
			if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "Z")); !samePath(got, filepath.Join(local, "Instances", "Z")) {
				t.Errorf("TranslateLegacyPath(Z) = %s", got)
			}

			c := jcdAssertSeedReachable(t)
			if c.WindowWidth != 900 || c.BaseRoot != "" {
				t.Errorf("config = %+v", c)
			}
			names := jcdInstanceNames(t)
			if !jcdContains(names, "A") || !jcdContains(names, "Z") {
				t.Errorf("instances = %v", names)
			}
			inst, _ := apppath.InstancesDir()
			if got := jcdInstanceMeta(t, filepath.Join(inst, "Z")); got != `{"name":"Z"}` {
				t.Errorf("Z metadata = %q", got)
			}
			if conflict {
				if !jcdContains(names, "A-legacy") {
					t.Errorf("instances = %v, want A-legacy", names)
				}
				if got := jcdInstanceMeta(t, filepath.Join(inst, "A-legacy")); !strings.Contains(got, `"old": true`) && !strings.Contains(got, `"old":true`) {
					t.Errorf("A-legacy metadata = %q", got)
				}
				// The old version's S\versions\A (registered with Windows
				// from there) is now Instances\A-legacy.
				if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "A")); !samePath(got, filepath.Join(inst, "A-legacy")) {
					t.Errorf("TranslateLegacyPath(S\\versions\\A) = %s, want Instances\\A-legacy", got)
				}
			}

			// Idempotent.
			before := jcdSnapshot(t, e.roaming, e.local)
			if o, err := jcdPrepare(); err != nil || o != Current {
				t.Errorf("third Prepare = %v, %v", o, err)
			}
			if d := jcdDiff(before, jcdSnapshot(t, e.roaming, e.local)); len(d) > 0 {
				t.Errorf("third Prepare changed the tree:\n%s", strings.Join(d, "\n"))
			}
		})
	}
}

// ------------------------------------------------------------------ J4

// J4: as J3 after a migration that kept a custom data root in place. v1.0.x
// finds no config.json (it was retired), writes its default (base_root "")
// and creates S\versions. The current config.json decides the data root:
// base_root stays, and the old version's data is merged into it.
func TestMigration_J4_DowngradeCycleCustomRoot(t *testing.T) {
	for _, variant := range []string{"versions_empty", "new_instance_Z", "old_version_reselects_custom_root"} {
		t.Run(variant, func(t *testing.T) {
			e := setup(t)
			base := filepath.Join(filepath.Dir(e.roaming), "LLroot")
			jcdSeedE1(t, e, base)
			if o, err := jcdPrepare(); err != nil || o != Migrated {
				t.Fatalf("first Prepare = %v, %v", o, err)
			}
			if got := readJSON(t, apppath.ConfigPath())["base_root"]; got != base {
				t.Fatalf("first migration base_root = %v", got)
			}
			oldCfg := jcdOldDefaultConfig
			zAt := filepath.Join(e.legacy, "versions", "Z")
			switch variant {
			case "versions_empty":
				if err := os.MkdirAll(filepath.Join(e.legacy, "versions"), 0o755); err != nil {
					t.Fatal(err)
				}
				zAt = ""
			case "new_instance_Z":
				write(t, filepath.Join(zAt, "version.json"), `{"name":"Z"}`)
			case "old_version_reselects_custom_root":
				// The user points the old version at the data root again;
				// it creates versions there.
				b, _ := json.Marshal(map[string]any{"base_root": base, "window_width": 0})
				oldCfg = string(b)
				zAt = filepath.Join(base, "versions", "Z")
				write(t, filepath.Join(zAt, "version.json"), `{"name":"Z"}`)
			}
			write(t, filepath.Join(e.legacy, "config.json"), oldCfg)

			o, err := jcdPrepare()
			if err != nil || o != Migrated {
				jcdLogTree(t, e.roaming, e.local, base)
				t.Fatalf("second Prepare = %v, %v", o, err)
			}
			cfg := readJSON(t, apppath.ConfigPath())
			if cfg["base_root"] != base {
				t.Errorf("base_root = %v, want %s", cfg["base_root"], base)
			}
			if cfg["window_width"] != float64(900) || cfg["future_key"] != "kept" {
				t.Errorf("current members did not win: %v", cfg)
			}
			jcdMustRead(t, filepath.Join(e.legacy, "config.migrated-2.json"), oldCfg)
			c := jcdAssertSeedReachable(t)
			if c.BaseRoot != base {
				t.Errorf("config.BaseRoot = %q", c.BaseRoot)
			}
			inst, _ := apppath.InstancesDir()
			if !samePath(inst, filepath.Join(base, "Instances")) {
				t.Errorf("InstancesDir = %s", inst)
			}
			names := jcdInstanceNames(t)
			if zAt != "" {
				if !jcdContains(names, "Z") {
					jcdLogTree(t, e.roaming, e.local, base)
					t.Errorf("instances = %v, want Z", names)
				}
				if got := jcdInstanceMeta(t, filepath.Join(inst, "Z")); got != `{"name":"Z"}` {
					t.Errorf("Z metadata = %q", got)
				}
				resetRecordCache()
				if got := TranslateLegacyPath(zAt); !samePath(got, filepath.Join(inst, "Z")) {
					t.Errorf("TranslateLegacyPath(%s) = %s", zAt, got)
				}
			}
			lowerVersions := filepath.Join(base, "versions")
			if exists(filepath.Join(e.legacy, "versions")) || (isDir(lowerVersions) && actualName(lowerVersions) == "versions") {
				t.Error("a versions directory still marks the legacy layout as not migrated")
			}
			before := jcdSnapshot(t, e.roaming, e.local, base)
			if o, err := jcdPrepare(); err != nil || o != Current {
				t.Errorf("third Prepare = %v, %v", o, err)
			}
			if d := jcdDiff(before, jcdSnapshot(t, e.roaming, e.local, base)); len(d) > 0 {
				t.Errorf("third Prepare changed the tree:\n%s", strings.Join(d, "\n"))
			}
		})
	}
}

// ------------------------------------------------------------------ J5

// J5: the first migration kept the data root in the legacy directory on
// another volume (base_root = S). v1.0.x runs again and installs Z into
// S\versions; the next migration merges it into S\Instances and keeps
// base_root.
func TestMigration_J5_DowngradeCrossVolume(t *testing.T) {
	e := jcdCrossVolumeEnv(t)
	jcdSeedE1(t, e, e.legacy)
	if o, err := jcdPrepare(); err != nil || o != Migrated {
		t.Fatalf("first Prepare = %v, %v", o, err)
	}
	if got := readJSON(t, apppath.ConfigPath())["base_root"]; got != e.legacy {
		t.Fatalf("first migration base_root = %v, want %s", got, e.legacy)
	}
	// v1.0.x runs again.
	write(t, filepath.Join(e.legacy, "config.json"), jcdOldDefaultConfig)
	write(t, filepath.Join(e.legacy, "versions", "Z", "version.json"), `{"name":"Z"}`)

	o, err := jcdPrepare()
	if err != nil || o != Migrated {
		jcdLogTree(t, e.roaming, e.local)
		t.Fatalf("second Prepare = %v, %v", o, err)
	}
	cfg := readJSON(t, apppath.ConfigPath())
	if cfg["base_root"] != e.legacy || cfg["window_width"] != float64(900) {
		t.Errorf("config = %v, want base_root %s and the current members", cfg, e.legacy)
	}
	jcdMustRead(t, filepath.Join(e.legacy, "config.migrated-2.json"), jcdOldDefaultConfig)
	c := jcdAssertSeedReachable(t)
	if c.BaseRoot != e.legacy {
		t.Errorf("config.BaseRoot = %q", c.BaseRoot)
	}
	inst, _ := apppath.InstancesDir()
	if !samePath(inst, filepath.Join(e.legacy, "Instances")) {
		t.Errorf("InstancesDir = %s", inst)
	}
	names := jcdInstanceNames(t)
	if !jcdContains(names, "A") || !jcdContains(names, "Z") {
		t.Errorf("instances = %v", names)
	}
	if got := jcdInstanceMeta(t, filepath.Join(inst, "Z")); got != `{"name":"Z"}` {
		t.Errorf("Z metadata = %q", got)
	}
	resetRecordCache()
	if got := TranslateLegacyPath(filepath.Join(e.legacy, "versions", "Z")); !samePath(got, filepath.Join(inst, "Z")) {
		t.Errorf("TranslateLegacyPath(Z) = %s", got)
	}
	if exists(filepath.Join(e.legacy, "versions")) {
		t.Error("S\\versions still pending")
	}

	before := jcdSnapshot(t, e.roaming, e.local)
	if o, err := jcdPrepare(); err != nil || o != Current {
		t.Errorf("third Prepare = %v, %v", o, err)
	}
	if d := jcdDiff(before, jcdSnapshot(t, e.roaming, e.local)); len(d) > 0 {
		t.Errorf("third Prepare changed the tree:\n%s", strings.Join(d, "\n"))
	}
}

// ------------------------------------------------------------------ J6

// J6: the migration cannot complete (the game runs from an instance), so the
// launcher runs on the legacy layout. Reading the instances there must leave
// version.json in place, as v1.0.x reads nothing else; the next start then
// migrates the metadata unchanged.
func TestMigration_J6_LegacyModeRunKeepsVersionJSON(t *testing.T) {
	e := setup(t)
	jcdSeedE1(t, e, e.legacy)
	meta := filepath.Join(e.legacy, "versions", "A", "version.json")
	f, err := os.Open(filepath.Join(e.legacy, "versions", "A", "Minecraft.Windows.exe"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := jcdPrepare()
	f.Close()
	if o != Legacy || err == nil {
		t.Fatalf("Prepare = %v, %v; want Legacy with an error", o, err)
	}
	if !samePath(apppath.LegacyMode(), e.legacy) {
		t.Fatalf("LegacyMode = %q", apppath.LegacyMode())
	}
	c := jcdStartApp(t)
	if c.WindowWidth != 900 {
		t.Errorf("legacy config not read: %+v", c)
	}
	inst, _ := apppath.InstancesDir()
	if !samePath(inst, filepath.Join(e.legacy, "versions")) {
		t.Fatalf("legacy InstancesDir = %s", inst)
	}
	if names := jcdInstanceNames(t); !jcdContains(names, "A") {
		t.Fatalf("instances = %v", names)
	}
	if _, err := versions.ReadMeta(filepath.Join(inst, "A")); err != nil {
		t.Fatal(err)
	}
	jcdMustRead(t, meta, `{"name":"A"}`)
	if exists(filepath.Join(e.legacy, "versions", "A", ".levilauncher")) {
		t.Error("a legacy-mode read created .levilauncher")
	}

	// The game is closed: the next start migrates.
	if o, err := jcdPrepare(); err != nil || o != Migrated {
		t.Fatalf("next Prepare = %v, %v", o, err)
	}
	jcdAssertSeedReachable(t)
}

// ------------------------------------------------------------------ J7

// J7: starts after a successful migration change nothing.
func TestMigration_J7_IdempotenceAfterSuccess(t *testing.T) {
	cases := []struct {
		name string
		env  func(t *testing.T) env
		base func(e env) string
	}{
		{"E1", func(t *testing.T) env { return setup(t) }, func(e env) string { return e.legacy }},
		{"B1", func(t *testing.T) env { return setup(t) }, func(e env) string { return filepath.Join(filepath.Dir(e.roaming), "LLroot") }},
		{"V1", jcdCrossVolumeEnv, func(e env) string { return e.legacy }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.env(t)
			base := tc.base(e)
			jcdSeedE1(t, e, base)
			roots := []string{e.roaming, e.local}
			if base != e.legacy {
				roots = append(roots, base)
			}
			if o, err := jcdPrepare(); err != nil || o != Migrated {
				t.Fatalf("first Prepare = %v, %v", o, err)
			}
			c := jcdAssertSeedReachable(t)
			wantBase := ""
			if base != e.legacy || tc.name == "V1" {
				wantBase = base
			}
			if c.BaseRoot != wantBase || c.WindowWidth != 900 {
				t.Errorf("config = %+v, want base_root %q", c, wantBase)
			}
			first := jcdSnapshot(t, roots...)
			rec := read(t, apppath.MigrationPath())
			for i := 2; i <= 3; i++ {
				if o, err := jcdPrepare(); err != nil || o != Current {
					t.Errorf("Prepare #%d = %v, %v", i, o, err)
				}
				if d := jcdDiff(first, jcdSnapshot(t, roots...)); len(d) > 0 {
					jcdLogTree(t, roots...)
					t.Errorf("Prepare #%d changed the tree:\n%s", i, strings.Join(d, "\n"))
				}
				if got := read(t, apppath.MigrationPath()); got != rec {
					t.Errorf("record changed by Prepare #%d", i)
				}
			}
			jcdAssertSeedReachable(t)
		})
	}
}
