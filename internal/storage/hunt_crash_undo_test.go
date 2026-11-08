//go:build windows

package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Crash-and-undo hunt: a reference migration is run while its journal is
// captured; then, for every journal record, a fresh environment is seeded the
// same way and brought to the state a crash right before or right after that
// record leaves (optionally with an old launcher running in between, or a
// recovery that itself was cut off), and the next Prepare must restore and
// migrate to the reference result.

var huntDRoot = testSecondVolume

type huntEnv struct {
	root, droot, roaming, local, legacy string
}

func huntSetup(t *testing.T, appdataOnD bool) huntEnv {
	t.Helper()
	e := huntEnv{root: t.TempDir()}
	// Without a second volume the "D" side is another folder on the same
	// volume: the reference run is made the same way, so the scenarios
	// still compare like with like.
	parent := huntDRoot
	if !isDir(parent) {
		parent = t.TempDir()
	}
	d, err := os.MkdirTemp(parent, "hunt-cu-")
	if err != nil {
		t.Fatalf("D volume: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	e.droot = d
	e.roaming = filepath.Join(e.root, "Roaming")
	if appdataOnD {
		e.roaming = filepath.Join(e.droot, "Roaming")
	}
	e.local = filepath.Join(e.root, "Local")
	for _, dir := range []string{e.roaming, e.local} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	releaseLock()
	t.Setenv("APPDATA", e.roaming)
	t.Setenv("LOCALAPPDATA", e.local)
	e.legacy = apppath.LegacyDir()
	apppath.SetLegacyMode("")
	apppath.SetBaseRootOverride("")
	resetRecordCache()
	t.Cleanup(func() {
		releaseLock()
		apppath.SetLegacyMode("")
		resetRecordCache()
	})
	return e
}

// tr translates a path or text of env a into env b.
func (a huntEnv) tr(b huntEnv, s string) string {
	repl := func(s, from, to string) string {
		s = replaceFold(s, from, to)
		fj, _ := json.Marshal(from)
		tj, _ := json.Marshal(to)
		return replaceFold(s, strings.Trim(string(fj), `"`), strings.Trim(string(tj), `"`))
	}
	s = repl(s, a.root, b.root)
	s = repl(s, a.droot, b.droot)
	return s
}

func replaceFold(s, old, new string) string {
	if old == "" {
		return s
	}
	var out strings.Builder
	ls, lo := strings.ToLower(s), strings.ToLower(old)
	for {
		i := strings.Index(ls, lo)
		if i < 0 {
			out.WriteString(s)
			return out.String()
		}
		out.WriteString(s[:i])
		out.WriteString(new)
		s, ls = s[i+len(old):], ls[i+len(old):]
	}
}

// captureJournals holds a handle on every journal the migration writes, so
// its records can be read after the commit removed it.
func captureJournals(path string) func() [][]record {
	var mu sync.Mutex
	var out [][]record
	stop := make(chan struct{})
	done := make(chan struct{})
	readAll := func(f *os.File) {
		_, _ = f.Seek(0, io.SeekStart)
		b, _ := io.ReadAll(f)
		_ = f.Close()
		mu.Lock()
		out = append(out, parseRecords(b))
		mu.Unlock()
	}
	go func() {
		defer close(done)
		p, _ := windows.UTF16PtrFromString(path)
		var cur *os.File
		for {
			select {
			case <-stop:
				if cur != nil {
					readAll(cur)
				}
				return
			default:
			}
			if cur == nil {
				h, err := windows.CreateFile(p, windows.GENERIC_READ,
					windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
					nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
				if err == nil {
					cur = os.NewFile(uintptr(h), path)
				}
				continue
			}
			if _, err := os.Lstat(path); err != nil {
				readAll(cur)
				cur = nil
			}
		}
	}()
	return func() [][]record {
		close(stop)
		<-done
		return out
	}
}

func parseRecords(b []byte) []record {
	var out []record
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var r record
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Op != "" {
			out = append(out, r)
		}
	}
	return out
}

type huntScenario struct {
	name       string
	appdataOnD bool
	seed       func(t *testing.T, e huntEnv)
}

func junction(t *testing.T, link, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
}

func huntScenarios() []huntScenario {
	return []huntScenario{
		{name: "default", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":900,"future_key":"kept"}`)
			write(t, filepath.Join(e.local, "LeviLauncher", "Logs", "startup.log"), "new log")
		}},
		{name: "customRootInPlace", seed: func(t *testing.T, e huntEnv) {
			base := filepath.Join(e.root, "Data")
			seedLegacy(t, e.legacy, base)
			write(t, filepath.Join(base, "Cache", "Downloads", "dl.zip"), "dl")
			cfg, _ := json.Marshal(map[string]any{"base_root": base, "window_width": 901})
			write(t, filepath.Join(e.legacy, "config.json"), string(cfg))
		}},
		{name: "crossVolumeKeepOnD", appdataOnD: true, seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":902}`)
		}},
		{name: "crossVolumeCopyToD", seed: func(t *testing.T, e huntEnv) {
			// No legacy config.json; a current config.json points to D.
			seedLegacy(t, e.legacy, e.legacy)
			data := filepath.Join(e.droot, "Data")
			write(t, filepath.Join(data, "Instances", "A", "version.json"), `{"name":"A on D"}`)
			write(t, filepath.Join(data, "Backups", "Worlds", "W", "W_0.mcworld"), "world on D")
			cfg, _ := json.Marshal(map[string]any{"base_root": data, "window_width": 903})
			write(t, filepath.Join(e.roaming, "LeviLauncher", "config.json"), string(cfg))
		}},
		{name: "followUpLegacyNames", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":904}`)
			if o, err := Prepare(); err != nil || o != Migrated {
				t.Fatalf("first migration: %v %v", o, err)
			}
			releaseLock()
			resetRecordCache()
			write(t, filepath.Join(e.roaming, "LeviLauncher", "config.json"), `{"window_width":1500}`)
			// The old version runs again.
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":800,"old_only":true}`)
			write(t, filepath.Join(e.legacy, "versions", "A", "version.json"), `{"name":"A again"}`)
			write(t, filepath.Join(e.legacy, "versions", "B", "version.json"), `{"name":"B"}`)
			write(t, filepath.Join(e.legacy, "backups", "worlds", "W", "W_1.mcworld"), "world again")
			write(t, filepath.Join(e.legacy, "backups", "worlds", "W", "W_2.mcworld"), "world 2")
			write(t, filepath.Join(e.legacy, "EBWebView", "Default", "old"), "old profile")
			write(t, filepath.Join(e.legacy, "logs", "startup.log"), "old run log")
		}},
		{name: "deepBackupsMerge", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "backups", "worlds", "V2", "F_X", "X_1.mcworld"), "x1")
			write(t, filepath.Join(e.legacy, "backups", "worlds", "W", "W_9.mcworld"), "w9")
			write(t, filepath.Join(e.legacy, "config.json"), `{}`)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(local, "Backups", "Worlds", "W", "W_1.mcworld"), "current world")
			write(t, filepath.Join(local, "Backups", "Worlds", "V3", "Y.mcworld"), "y")
			write(t, filepath.Join(local, "Backups", "Instances", "Z", "Z_1.zip"), "z")
			write(t, filepath.Join(local, "Cache", "Installers", "1.21.msixvc"), "current pkg")
		}},
		{name: "junctionReplacesEmptyDir", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			target := filepath.Join(e.root, "Elsewhere", "versions")
			write(t, filepath.Join(target, "J", "version.json"), `{"name":"J"}`)
			if err := os.RemoveAll(filepath.Join(e.legacy, "versions")); err != nil {
				t.Fatal(err)
			}
			junction(t, filepath.Join(e.legacy, "versions"), target)
			if err := os.MkdirAll(filepath.Join(e.local, "LeviLauncher", "Instances"), 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(e.legacy, "config.json"), `{}`)
		}},
		{name: "junctionToOtherVolume", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			target := filepath.Join(e.droot, "versions")
			write(t, filepath.Join(target, "J", "version.json"), `{"name":"J on D"}`)
			if err := os.RemoveAll(filepath.Join(e.legacy, "versions")); err != nil {
				t.Fatal(err)
			}
			junction(t, filepath.Join(e.legacy, "versions"), target)
			write(t, filepath.Join(e.legacy, "config.json"), `{}`)
		}},
		{name: "fileMovedAside", seed: func(t *testing.T, e huntEnv) {
			seedLegacy(t, e.legacy, e.legacy)
			write(t, filepath.Join(e.legacy, "config.json"), `{"window_width":905}`)
			local := filepath.Join(e.local, "LeviLauncher")
			write(t, filepath.Join(local, "Instances"), "a file named Instances")
			write(t, filepath.Join(local, "Cache"), "a file named Cache")
			write(t, filepath.Join(e.roaming, "LeviLauncher", "config.json"), `{"window_wid`)
			write(t, filepath.Join(local, "Accounts", "device.dpapi"), "fresh device")
		}},
	}
}

// snapshot maps every entry below the env's roots to its content.
func snapshot(t *testing.T, e huntEnv) map[string]string {
	t.Helper()
	out := map[string]string{}
	for tok, root := range map[string]string{"<C>": e.root, "<D>": e.droot} {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || p == root {
				return nil
			}
			rel := tok + p[len(root):]
			base := strings.ToLower(filepath.Base(p))
			if base == "storage-migration.lock" {
				return nil
			}
			li, lerr := os.Lstat(p)
			if lerr == nil && li.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				tgt, _ := filepath.EvalSymlinks(p)
				out[rel] = "<link>" + e.token(tgt)
				return nil
			}
			if info.IsDir() {
				out[rel] = "<dir>"
				return nil
			}
			b, _ := os.ReadFile(p)
			s := e.token(string(b))
			if base == "storage-migration.json" {
				var r Record
				_ = json.Unmarshal(b, &r)
				r.MigratedAt = Record{}.MigratedAt
				bb, _ := json.Marshal(r)
				s = e.token(string(bb))
			}
			out[rel] = s
			return nil
		})
	}
	return out
}

func (e huntEnv) token(s string) string {
	tok := huntEnv{root: "<C>", droot: "<D>"}
	return e.tr(tok, s)
}

func diffSnap(want, got map[string]string) []string {
	var d []string
	for k, v := range want {
		g, ok := got[k]
		if !ok {
			d = append(d, "missing "+k)
		} else if g != v {
			d = append(d, fmt.Sprintf("differs %s: want %q got %q", k, short(v), short(g)))
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

func short(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func lowerKeys(m map[string]string) map[string]string {
	o := map[string]string{}
	for k, v := range m {
		o[strings.ToLower(k)] = v
	}
	return o
}

type huntRef struct {
	env     huntEnv
	records []record
	final   map[string]string
	rec     *Record
}

func runReference(t *testing.T, sc huntScenario) huntRef {
	t.Helper()
	e := huntSetup(t, sc.appdataOnD)
	sc.seed(t, e)
	stop := captureJournals(apppath.MigrationJournalPath())
	o, err := Prepare()
	journals := stop()
	if err != nil || o != Migrated {
		t.Fatalf("reference Prepare = %v, %v", o, err)
	}
	if len(journals) != 1 {
		t.Fatalf("captured %d journals", len(journals))
	}
	recs := journals[0]
	if len(recs) < 2 || recs[0].Op != opBegin || recs[len(recs)-1].Op != opCommit {
		t.Fatalf("incomplete journal captured: %+v", recs)
	}
	ref := huntRef{env: e, records: recs, final: snapshot(t, e), rec: readRecord()}
	releaseLock()
	if o, err := Prepare(); err != nil || o != Current {
		t.Fatalf("reference second Prepare = %v, %v", o, err)
	}
	releaseLock()
	return ref
}

// copySource finds where the reference migration copied path from.
func (r huntRef) copySource(path string) string {
	best := ""
	bestTo := ""
	for _, m := range r.rec.Moves {
		if samePath(m.To, path) || strings.HasPrefix(strings.ToLower(path), strings.ToLower(m.To)+`\`) {
			if len(m.To) > len(bestTo) {
				bestTo, best = m.To, m.From+path[len(m.To):]
			}
		}
	}
	return best
}

// execRecord performs what the migration did for r in env e.
func (r huntRef) execRecord(t *testing.T, e huntEnv, rc record, partial bool) {
	t.Helper()
	path := r.env.tr(e, rc.Path)
	from := r.env.tr(e, rc.From)
	switch rc.Op {
	case opMove:
		if partial {
			return
		}
		if err := os.Rename(from, path); err != nil && !isCrossDevice(err) {
			t.Fatalf("replay move %s -> %s: %v", from, path, err)
		}
	case opMkdir:
		if partial {
			return
		}
		if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
			t.Fatalf("replay mkdir %s: %v", path, err)
		}
	case opRmdir:
		if partial {
			return
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("replay rmdir %s: %v", path, err)
		}
	case opCreate:
		if partial {
			// WriteFileAtomic was cut off: only its temporary file exists.
			write(t, path+".cut.tmp", "partial")
			return
		}
		b, err := os.ReadFile(rc.Path)
		if err != nil {
			t.Fatalf("reference created file %s: %v", rc.Path, err)
		}
		write(t, path, r.env.tr(e, string(b)))
	case opCopy:
		src := r.env.tr(e, r.copySource(rc.Path))
		if src == "" {
			t.Fatalf("no source for copy %s", rc.Path)
		}
		if isDir(src) {
			if partial {
				write(t, filepath.Join(path, "partial.bin"), "partial")
				return
			}
			if err := utils.CopyDir(src, path); err != nil {
				t.Fatalf("replay copy %s -> %s: %v", src, path, err)
			}
			return
		}
		if partial {
			write(t, path, "par")
			return
		}
		if err := copyFile(src, path); err != nil {
			t.Fatalf("replay copy %s: %v", src, err)
		}
	}
}

func writeJournal(t *testing.T, recs []record, cutLine bool) {
	t.Helper()
	var b bytes.Buffer
	for _, r := range recs {
		j, _ := json.Marshal(r)
		b.Write(j)
		b.WriteByte('\n')
	}
	if cutLine {
		b.WriteString(`{"op":"Mo`)
	}
	write(t, apppath.MigrationJournalPath(), b.String())
}

// oldLauncherRuns simulates a pre-PR launcher started between the crash and
// the next start: it uses %APPDATA%\<exe>, writes a default config.json when
// there is none, its log and WebView2 profile, creates versions and
// installers under its data root, and, with userActs, the user installs an
// instance and backs up a world.
func oldLauncherRuns(t *testing.T, e huntEnv, userActs bool) {
	t.Helper()
	cfgPath := filepath.Join(e.legacy, "config.json")
	if !exists(cfgPath) {
		write(t, cfgPath, `{"base_root":"","old_default":true}`)
	}
	base := e.legacy
	if m, err := readConfigMembers(cfgPath); err == nil {
		if b := baseRootOf(m); b != "" {
			base = b
		}
	}
	write(t, filepath.Join(e.legacy, "logs", "startup.log"), "old launcher run")
	prof := filepath.Join(e.legacy, "EBWebView", "Default", "Local Storage", "leveldb")
	if !exists(prof) {
		write(t, prof, "old launcher fresh profile")
	}
	for _, d := range []string{"versions", "installers"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if userActs {
		write(t, filepath.Join(base, "versions", "NewB", "version.json"), `{"name":"NewB"}`)
		write(t, filepath.Join(base, "backups", "worlds", "V9", "F_X", "X_1.mcworld"), "old launcher backup")
	}
}

type crashVariant int

const (
	crashBefore    crashVariant = iota // record written, change not made
	crashPartial                       // record written, change half made
	crashAfter                         // record written, change made
	crashAfterOld                      // after, then the old launcher runs
	crashAfterUser                     // after, then the old launcher runs and the user acts
	crashUndoHalf                      // after, then a recovery cut off half way
	crashUndoAll                       // after, then a recovery cut off before removing the journal
	crashUndoOld                       // after, half recovery, then the old launcher runs and the user acts
)

func (v crashVariant) String() string {
	return [...]string{"before", "partial", "after", "afterOld", "afterUser", "undoHalf", "undoAll", "undoOld"}[v]
}

// userFiles are the reference result's files that hold user data: they must
// all be where the reference put them.
func userFiles(final map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range final {
		lk := strings.ToLower(k)
		if v == "<dir>" || strings.HasSuffix(lk, `\storage-migration.json`) || strings.Contains(lk, `\logs\`) ||
			strings.HasSuffix(lk, `\config.json`) || strings.Contains(lk, "config.migrated") || strings.Contains(lk, "config-legacy") {
			continue
		}
		out[k] = v
	}
	return out
}

func runCrashCase(t *testing.T, sc huntScenario, ref huntRef, i int, v crashVariant) {
	e := huntSetup(t, sc.appdataOnD)
	sc.seed(t, e)
	// The crashed process had taken the lock in the local directory.
	if err := os.MkdirAll(apppath.LocalDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	recs := ref.records
	commitAt := len(recs) - 1
	upto := i // records[0..i] are in the journal
	for k := 1; k < upto; k++ {
		ref.execRecord(t, e, recs[k], false)
	}
	switch v {
	case crashBefore:
	case crashPartial:
		ref.execRecord(t, e, recs[upto], true)
	default:
		ref.execRecord(t, e, recs[upto], false)
	}
	committed := upto == commitAt
	// keep is how many records the journal holds at the next start: undo
	// shortens it after every step, so a recovery cut off after undoing
	// record k leaves records[0..k] (k undone again at most once).
	keep := upto + 1

	switch v {
	case crashAfterOld:
		oldLauncherRuns(t, e, false)
	case crashAfterUser:
		oldLauncherRuns(t, e, true)
	case crashUndoHalf, crashUndoAll, crashUndoOld:
		if !committed {
			// The recovery undoes from the last record backwards.
			stopAt := 1
			if v != crashUndoAll {
				stopAt = (upto + 1) / 2
			}
			for k := upto; k >= stopAt && k >= 1; k-- {
				rc := recs[k]
				rc.Path = ref.env.tr(e, rc.Path)
				rc.From = ref.env.tr(e, rc.From)
				if err := undoRecord(rc); err != nil {
					t.Fatalf("partial undo of %+v: %v", rc, err)
				}
			}
			keep = stopAt + 1
		}
		if v == crashUndoOld {
			oldLauncherRuns(t, e, true)
		}
	}
	// The journal names paths of this env.
	tr := make([]record, keep)
	for k := range tr {
		tr[k] = recs[k]
		tr[k].Path = ref.env.tr(e, tr[k].Path)
		tr[k].From = ref.env.tr(e, tr[k].From)
	}
	writeJournal(t, tr, v == crashBefore)

	o, err := Prepare()
	if err != nil {
		t.Fatalf("Prepare after crash = %v, %v", o, err)
	}
	if exists(apppath.MigrationJournalPath()) {
		t.Fatal("journal left")
	}
	releaseLock()
	if o2, err := Prepare(); err != nil || o2 != Current {
		t.Fatalf("second Prepare = %v, %v (first %v)", o2, err, o)
	}
	releaseLock()
	got := snapshot(t, e)

	switch v {
	case crashBefore, crashPartial, crashAfter, crashUndoHalf, crashUndoAll:
		if committed && o != Current {
			t.Errorf("committed migration ran again: %v", o)
		}
		if !committed && o != Migrated {
			t.Errorf("outcome = %v, want Migrated", o)
		}
		var d []string
		for _, line := range diffSnap(ref.final, got) {
			// A cut-off WriteFileAtomic leaves its temporary file.
			if v == crashPartial && strings.HasSuffix(line, ".cut.tmp") {
				continue
			}
			d = append(d, line)
		}
		if len(d) > 0 {
			t.Errorf("crash after record %d %+v (%v) ends differently from the reference:\n  %s", i, recs[i], v, strings.Join(d, "\n  "))
		}
	default:
		var lost []string
		for k, want := range userFiles(ref.final) {
			if got[k] != want {
				lost = append(lost, fmt.Sprintf("%s want %q got %q", k, short(want), short(got[k])))
			}
		}
		sort.Strings(lost)
		if len(lost) > 0 {
			t.Errorf("crash after record %d %+v (%v): user data not where the new layout reads it:\n  %s", i, recs[i], v, strings.Join(lost, "\n  "))
		}
		if v != crashAfterOld {
			// Looked up through the file system: a junction in the data root
			// (Instances) is how the new layout reads what is behind it.
			base := readRecord().BaseRoot
			for _, p := range []string{
				filepath.Join(base, "Instances", "NewB", "version.json"),
				filepath.Join(base, "Backups", "Worlds", "V9", "F_X", "X_1.mcworld"),
			} {
				if !isFile(p) {
					var where []string
					for k := range got {
						if strings.Contains(strings.ToLower(k), "newb") || strings.Contains(strings.ToLower(k), "x_1") {
							where = append(where, k)
						}
					}
					sort.Strings(where)
					t.Errorf("crash after record %d %+v (%v): the old launcher's %s is not reachable; found at %v", i, recs[i], v, p, where)
				}
			}
		}
	}
}

// fullCrashMatrix runs every crash variant at every journal record; by
// default each record is crashed with the variants that cover the most (a
// change half made, an old launcher run with user changes after the crash,
// and that after a recovery cut off half way), to keep go test ./... within
// its default timeout.
var fullCrashMatrix = os.Getenv("LEVILAUNCHER_STORAGE_CRASH_MATRIX") == "1"

func huntCrashScenario(t *testing.T, name string) {
	for _, sc := range huntScenarios() {
		if sc.name != name {
			continue
		}
		ref := runReference(t, sc)
		t.Logf("%s: %d records", sc.name, len(ref.records))
		for i := 1; i < len(ref.records); i++ {
			for v := crashBefore; v <= crashUndoOld; v++ {
				if (v == crashBefore || v == crashPartial) && ref.records[i].Op == opCommit {
					continue
				}
				if !fullCrashMatrix && v != crashPartial && v != crashAfterUser && v != crashUndoOld {
					continue
				}
				i, v := i, v
				t.Run(fmt.Sprintf("r%02d_%s_%s", i, ref.records[i].Op, v), func(t *testing.T) {
					runCrashCase(t, sc, ref, i, v)
				})
			}
		}
		return
	}
	t.Fatalf("no scenario %s", name)
}

func TestHunt_crash_and_undo_default(t *testing.T) { huntCrashScenario(t, "default") }
func TestHunt_crash_and_undo_customRootInPlace(t *testing.T) {
	huntCrashScenario(t, "customRootInPlace")
}
func TestHunt_crash_and_undo_crossVolumeKeepOnD(t *testing.T) {
	huntCrashScenario(t, "crossVolumeKeepOnD")
}
func TestHunt_crash_and_undo_crossVolumeCopyToD(t *testing.T) {
	huntCrashScenario(t, "crossVolumeCopyToD")
}
func TestHunt_crash_and_undo_followUpLegacyNames(t *testing.T) {
	huntCrashScenario(t, "followUpLegacyNames")
}
func TestHunt_crash_and_undo_deepBackupsMerge(t *testing.T) { huntCrashScenario(t, "deepBackupsMerge") }
func TestHunt_crash_and_undo_junctionReplacesEmptyDir(t *testing.T) {
	huntCrashScenario(t, "junctionReplacesEmptyDir")
}
func TestHunt_crash_and_undo_junctionToOtherVolume(t *testing.T) {
	huntCrashScenario(t, "junctionToOtherVolume")
}
func TestHunt_crash_and_undo_fileMovedAside(t *testing.T) { huntCrashScenario(t, "fileMovedAside") }
