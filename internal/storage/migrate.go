// Package storage moves the launcher's files from the legacy layout
// (%APPDATA%\<exe name>) to the one internal/apppath/README.md specifies,
// without losing or overwriting anything.
//
// The migration runs at startup, before config.json is read, whenever a
// legacy directory holds data that has not been migrated, whether or not the
// current layout exists already. It renames on one volume, or copies and
// keeps the source; it never deletes a file of the legacy layout. When it
// cannot complete, for example because the game still runs from an instance,
// every change is undone and the launcher runs on the layout that holds the
// user's data until the next start.
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Outcome of Prepare.
type Outcome int

const (
	// Current: the current layout is in use; nothing was migrated.
	Current Outcome = iota
	// Migrated: legacy data was migrated during this start.
	Migrated
	// Legacy: the migration was undone or not attempted; the launcher runs
	// on the legacy layout, which holds the user's data, and tries again at
	// the next start.
	Legacy
	// Deferred: the current layout is in use, and legacy data that appeared
	// after an earlier migration could not be migrated this time; the next
	// start tries again.
	Deferred
)

// maxStores bounds the legacy directories migrated in one start.
const maxStores = 16

// Prepare decides the layout for this run, migrating every legacy directory
// that holds data. An error with Outcome Current means a migration that an
// earlier start left behind could not be undone; the launcher must not
// start, as it would find its files in neither layout.
func Prepare() (Outcome, error) {
	resetRecordCache()
	exclusive, err := acquireLock(apppath.MigrationLockPath())
	if err != nil {
		return Current, err
	}
	if !exclusive {
		// Another launcher process runs on this layout (or migrates it right
		// now and holds the lock until it is done): leave the files to it.
		return busy()
	}
	defer releaseToShared()

	if err := recoverJournal(apppath.MigrationJournalPath()); err != nil {
		return Current, err
	}
	outcome := Current
	done := map[string]bool{}
	for i := 0; i < maxStores; i++ {
		legacy := nextLegacyDir(done)
		if legacy == "" {
			break
		}
		done[utils.NormalizeWindowsPath(legacy)] = true
		current := currentHoldsData(legacy)
		plan, err := newPlan(legacy)
		if err == nil {
			err = plan.run()
		}
		resetRecordCache()
		if err != nil {
			if errors.Is(err, errUndoFailed) {
				return Current, err
			}
			if current {
				return Deferred, err
			}
			apppath.SetLegacyMode(legacy)
			return Legacy, err
		}
		outcome = Migrated
	}
	if outcome == Current && readRecord() == nil {
		markCurrentLayoutInUse()
	}
	return outcome, nil
}

// PrepareShared is Prepare for the command line, which may run next to the
// window. When windowRunning reports true it neither migrates nor recovers a
// journal and decides the layout as a process that finds the migration lock
// held does; the window's shared hold of that lock normally says the same,
// and this also covers a lock file that could not be opened.
func PrepareShared(windowRunning func() bool) (Outcome, error) {
	if windowRunning != nil && windowRunning() {
		resetRecordCache()
		return busy()
	}
	return Prepare()
}

// markCurrentLayoutInUse writes a migration record without moves when this
// launcher starts on a machine without legacy data: from then on the current
// layout holds the user's data, and legacy data that appears later (an
// older launcher run afterwards) does not take precedence over it.
func markCurrentLayoutInUse() {
	data, err := json.MarshalIndent(Record{Format: recordFormat, MigratedAt: time.Now().UTC(), ShortcutsRepaired: true}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(apppath.MigrationPath()), 0o755); err != nil {
		return
	}
	_ = fsx.WriteFileAtomic(apppath.MigrationPath(), data, 0o644)
	resetRecordCache()
}

// busy decides the layout while another launcher process holds the
// migration lock.
func busy() (Outcome, error) {
	if exists(apppath.MigrationJournalPath()) {
		return Current, errors.New("another LeviLauncher process left a migration unfinished; close every LeviLauncher window and start again")
	}
	legacy := nextLegacyDir(nil)
	if legacy == "" {
		return Current, nil
	}
	err := errors.New("another LeviLauncher process is running; the migration is deferred")
	if currentHoldsData(legacy) {
		return Deferred, err
	}
	// The running process found the same legacy data and runs on it.
	apppath.SetLegacyMode(legacy)
	return Legacy, err
}

// currentHoldsData reports whether the user's data is in the current layout
// rather than in the legacy directory legacy:
//   - after an earlier migration, or a start of this launcher on a machine
//     without legacy data (storage-migration.json exists);
//   - when a current config.json exists and the legacy directory has none to
//     say where its data is;
//   - when the current config.json names a data root, or the current data
//     root holds instances: the user worked with this version, and an older
//     one ran afterwards.
//
// Otherwise the current layout was made without the user's data (by a start
// that skipped the migration, or by a tool) and the legacy data takes
// precedence. A migration of legacy that cannot complete leaves the launcher
// on the layout that holds the data: Deferred, or Legacy.
func currentHoldsData(legacy string) bool {
	if readRecord() != nil {
		return true
	}
	m, err := readConfigMembers(apppath.ConfigPath())
	if err != nil {
		return false
	}
	if !isFile(filepath.Join(legacy, apppath.ConfigFileName)) {
		return true
	}
	root := resolveBaseRoot(baseRootOf(m))
	if root != "" {
		return true
	}
	return isNotEmpty(filepath.Join(apppath.LocalDir(), apppath.InstancesDirName))
}

// findLegacyDir returns the first legacy directory that holds data that has
// not been migrated, or "".
func findLegacyDir() string { return nextLegacyDir(nil) }

func nextLegacyDir(skip map[string]bool) string {
	ctx := newDetectContext()
	for _, c := range legacyCandidates() {
		if skip[utils.NormalizeWindowsPath(c.dir)] {
			continue
		}
		if ctx.hasPendingLegacyData(c) {
			return c.dir
		}
	}
	return ""
}

// candidate is a directory that may hold a legacy layout.
type candidate struct {
	dir string
	// strict candidates are found by a directory scan: they must look like
	// a LeviLauncher directory, not merely hold a "versions" folder.
	strict bool
	// eraA is %LOCALAPPDATA%\LeviLauncher, where launchers up to 0.0.9 kept
	// config.json, versions and installers. It is the current local
	// directory, so only the legacy data directories and config.json count.
	eraA bool
}

// legacyCandidates lists the directories of the legacy layout, in order:
// %APPDATA%\<running exe name>, %APPDATA%\levilauncher.exe, the same two in
// %LOCALAPPDATA% (where launchers kept them when APPDATA was not set), every
// other "<name>.exe" directory in both folders (a launcher run under another
// exe name), and %LOCALAPPDATA%\LeviLauncher of launchers up to 0.0.9.
func legacyCandidates() []candidate {
	exeName := strings.ToLower(filepath.Base(apppath.LegacyDir()))
	roots := []string{apppath.AppData(), apppath.LocalAppData()}
	var out []candidate
	seen := map[string]bool{}
	add := func(c candidate) {
		key := utils.NormalizeWindowsPath(c.dir)
		if c.dir == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, c)
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		add(candidate{dir: filepath.Join(root, exeName)})
		add(candidate{dir: filepath.Join(root, apppath.LegacyDefaultExeName)})
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range ents {
			if strings.EqualFold(filepath.Ext(e.Name()), ".exe") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			add(candidate{dir: filepath.Join(root, n), strict: true})
		}
	}
	add(candidate{dir: apppath.LocalDir(), eraA: true})
	return out
}

// detectContext is what the current layout says about where data lives.
type detectContext struct {
	record *Record
	// currentRoot is the data root of the current config.json, "" when there
	// is none.
	currentRoot string
}

func newDetectContext() detectContext {
	ctx := detectContext{record: readRecord()}
	if m, err := readConfigMembers(apppath.ConfigPath()); err == nil {
		base := resolveBaseRoot(baseRootOf(m))
		if base == "" {
			base = apppath.LocalDir()
		}
		ctx.currentRoot = base
	}
	return ctx
}

// isCurrentRoot reports whether dir is a data root of the current layout,
// where a migration that kept the data in place renamed backups to Backups.
func (c detectContext) isCurrentRoot(dir string) bool {
	if c.currentRoot != "" && sameLocation(dir, c.currentRoot) {
		return true
	}
	return c.record != nil && c.record.BaseRoot != "" && sameLocation(dir, c.record.BaseRoot)
}

// legacyDataDirNames are the data directories of the legacy layout; one that
// exists marks a legacy directory as not migrated.
var legacyDataDirNames = []string{
	apppath.LegacyVersionsDirName,
	apppath.LegacyBackupsDirName,
	apppath.LegacyEraABackupDirName,
	apppath.LegacyInstallersDirName,
	apppath.LegacyUpdatesDirName,
}

// hasPendingLegacyData reports whether c holds legacy data that has not been
// migrated:
//   - config.json, which every legacy launcher wrote and a migration renames
//     to config.migrated.json;
//   - a legacy data directory (versions, backups, installers, updates). In a
//     data root of the current layout only the exact lowercase name counts,
//     because a migration that keeps the data root in place renames backups
//     to Backups, which Windows finds under either name;
//   - before the first migration on this machine, account records, the
//     WebView2 profile or the cache files alone. After it they do not count:
//     a migration across volumes copies them and keeps the source.
func (ctx detectContext) hasPendingLegacyData(c candidate) bool {
	if c.strict && !looksLikeLauncherDir(c.dir) {
		return false
	}
	if isFile(filepath.Join(c.dir, apppath.ConfigFileName)) {
		return true
	}
	exact := c.eraA || ctx.isCurrentRoot(c.dir)
	for _, name := range legacyDataDirNames {
		p := filepath.Join(c.dir, name)
		if isDir(p) && (!exact || actualName(p) == name) {
			return true
		}
		if len(undoneDataDirs(c.dir, name)) > 0 {
			return true
		}
	}
	if c.eraA || ctx.record != nil {
		return false
	}
	for _, name := range []string{apppath.LegacyAccountsDirName, apppath.LegacyWebViewDirName} {
		if isDir(filepath.Join(c.dir, name)) {
			return true
		}
	}
	for _, name := range []string{apppath.LegacyGamertagsFileName, apppath.LegacyGDKPendingFileName} {
		if isFile(filepath.Join(c.dir, name)) {
			return true
		}
	}
	return false
}

// undoneDataDirs lists the "<name>-legacy", "<name>-legacy-2", ...
// directories in dir: an undo that found <name> recreated by an older
// launcher (a junction, which cannot be merged into) keeps that one under
// such a name. Nothing else names a legacy data directory so.
func undoneDataDirs(dir, name string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	prefix := name + "-legacy"
	for _, e := range ents {
		n := e.Name()
		if !strings.EqualFold(n, prefix) && !(len(n) > len(prefix)+1 && strings.EqualFold(n[:len(prefix)+1], prefix+"-")) {
			continue
		}
		if p := filepath.Join(dir, n); isDir(p) {
			out = append(out, p)
		}
	}
	return out
}

// looksLikeLauncherDir tells a LeviLauncher directory found by a scan from an
// unrelated "<name>.exe" folder: its config.json has base_root, which every
// launcher wrote, or its versions folder holds an instance.
func looksLikeLauncherDir(dir string) bool {
	if b, err := os.ReadFile(filepath.Join(dir, apppath.ConfigFileName)); err == nil {
		if bytes.Contains(bytes.ToLower(b), []byte(`"base_root"`)) {
			return true
		}
	}
	ents, err := os.ReadDir(filepath.Join(dir, apppath.LegacyVersionsDirName))
	if err != nil {
		return false
	}
	for _, e := range ents {
		inst := filepath.Join(dir, apppath.LegacyVersionsDirName, e.Name())
		if isFile(filepath.Join(inst, apppath.LegacyInstanceMetaFileName)) ||
			isFile(filepath.Join(inst, apppath.InstanceStateDirName, apppath.InstanceMetaFileName)) {
			return true
		}
	}
	return false
}

// item is one entry of the legacy layout and where it goes.
type item struct {
	from, to string
	dir      bool // a directory; a file of that name is not legacy data
	// pending items are the legacy data directories: when a copy keeps the
	// source, it is retired as "<name>.migrated" so that it no longer marks
	// the legacy layout as not migrated.
	pending bool
	// deep items merge into an existing target recursively, directory by
	// directory (backups, caches); others merge their children only.
	deep bool
	// replace items are taken whole from the legacy layout when the plan
	// prefers it (plan.legacyWins): an existing entry at to is moved aside
	// rather than merged with, since a merge would leave the legacy entry
	// unused (the WebView2 profile, the account records). When the current
	// layout is preferred, an existing entry at to is kept and the legacy
	// one stays where it is.
	replace bool
	// secure items get the account directory's protected DACL when copied.
	secure bool
}

type plan struct {
	legacy string
	// config is legacy config.json as members, so that keys this version
	// does not know survive the migration.
	config map[string]json.RawMessage
	// hasConfig is set when the legacy directory has config.json.
	hasConfig bool
	// current is the current config.json, when one exists already; nil
	// otherwise. currentCorrupt is set when it exists but cannot be read.
	current        map[string]json.RawMessage
	currentCorrupt bool
	// keepCurrentRoot is set when the current config.json decides the data
	// root: after an earlier migration, or when the legacy directory has no
	// config.json to say where its data is.
	keepCurrentRoot bool
	baseRoot        string // the new base_root ("" for the default)
	legacyBase      string
	targetBase      string
	items           []item
	// record is the record of an earlier migration, or nil.
	record *Record
	// legacyWins is set when the current layout does not hold the user's
	// data (see currentHoldsData): a current layout that exists then was made
	// without it (by a start that skipped the migration, or by a tool), so the
	// legacy settings, WebView2 profile and accounts take precedence.
	// Otherwise legacy data comes from running an old version again, and the
	// current data takes precedence.
	legacyWins bool
}

func readConfigMembers(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	// config.Load matches base_root without regard to case and takes the last
	// of duplicate keys: keep that value only, under the exact key.
	var eff struct {
		BaseRoot json.RawMessage `json:"base_root"`
	}
	if json.Unmarshal(b, &eff) == nil && eff.BaseRoot != nil {
		dropBaseRoot(m)
		m["base_root"] = eff.BaseRoot
	}
	return m, nil
}

var baseRootPattern = regexp.MustCompile(`(?i)"base_root"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// readLegacyConfig reads the legacy config.json. One that cannot be parsed,
// for example cut off by a crash (the legacy launcher wrote it in place), is
// read as empty apart from base_root, which is recovered from the text when
// it is there, so that the data is still found.
func readLegacyConfig(path string) (map[string]json.RawMessage, bool, error) {
	m, err := readConfigMembers(path)
	if err == nil {
		return m, true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]json.RawMessage{}, false, nil
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, true, fmt.Errorf("read legacy config: %w", rerr)
	}
	m = map[string]json.RawMessage{}
	if sm := baseRootPattern.FindSubmatch(b); sm != nil {
		var s string
		if json.Unmarshal(append(append([]byte{'"'}, sm[1]...), '"'), &s) == nil {
			raw, _ := json.Marshal(s)
			m["base_root"] = raw
		}
	}
	return m, true, nil
}

// baseRootOf returns the base_root member, matching the key without regard
// to case as config.Load does, and "" when it is not a string.
func baseRootOf(m map[string]json.RawMessage) string {
	raw, ok := m["base_root"]
	if !ok {
		for k, v := range m {
			if strings.EqualFold(k, "base_root") {
				raw, ok = v, true
				break
			}
		}
	}
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// dropBaseRoot removes base_root under every spelling of the key.
func dropBaseRoot(m map[string]json.RawMessage) {
	for k := range m {
		if strings.EqualFold(k, "base_root") {
			delete(m, k)
		}
	}
}

// resolveBaseRoot makes a relative base_root absolute against the launcher
// directory, which the launcher makes its working directory at startup: that
// is where the legacy launcher found it.
func resolveBaseRoot(base string) string {
	base = strings.TrimSpace(base)
	if base == "" || filepath.IsAbs(base) {
		return base
	}
	dir := apppath.LauncherDir()
	if filepath.VolumeName(base) == "" && (strings.HasPrefix(base, `\`) || strings.HasPrefix(base, "/")) {
		return filepath.Clean(filepath.VolumeName(dir) + base)
	}
	return filepath.Join(dir, base)
}

func newPlan(legacy string) (*plan, error) {
	p := &plan{legacy: legacy}
	var err error
	p.config, p.hasConfig, err = readLegacyConfig(filepath.Join(legacy, apppath.ConfigFileName))
	if err != nil {
		return nil, err
	}
	if m, err := readConfigMembers(apppath.ConfigPath()); err == nil {
		p.current = m
	} else if !errors.Is(err, fs.ErrNotExist) {
		// A current config.json that cannot be read is kept aside; the
		// legacy one, which holds the user's settings, takes its place.
		p.currentCorrupt = true
	}
	p.record = readRecord()
	p.legacyWins = !currentHoldsData(legacy)
	p.keepCurrentRoot = p.current != nil && !p.legacyWins

	local := apppath.LocalDir()
	base := resolveBaseRoot(baseRootOf(p.config))
	isDefault := base == "" || sameLocation(base, legacy)
	p.legacyBase = base
	if isDefault {
		p.legacyBase = legacy
	}

	if p.keepCurrentRoot {
		cur := resolveBaseRoot(baseRootOf(p.current))
		p.baseRoot = cur
		p.targetBase = cur
		if cur == "" {
			p.targetBase = local
		}
		if !exists(p.targetBase) {
			return nil, fmt.Errorf("data root %s is not available", p.targetBase)
		}
		if !isDefault && !exists(base) {
			// The legacy data root is offline. The legacy config.json is the
			// only thing that points at it: keep everything until the root
			// is back; the current layout stays in use (Deferred).
			return nil, fmt.Errorf("data root %s is not available", base)
		}
	} else {
		switch {
		case !isDefault && !exists(base):
			// A data root on a drive that is not connected would be skipped
			// and keep its legacy names: migrate once it is back.
			return nil, fmt.Errorf("data root %s is not available", base)
		case !isDefault:
			// A data root the user chose stays where it is.
			p.baseRoot = base
			p.targetBase = base
		case sameVolume(legacy, local):
			p.baseRoot = ""
			p.targetBase = local
		default:
			// Moving the data to another volume would mean copying every
			// instance: keep it, under the current names.
			p.baseRoot = legacy
			p.targetBase = legacy
		}
	}

	tb := p.targetBase
	cache := filepath.Join(tb, apppath.CacheDirName)
	// Data directories of the legacy data root and, when the data root was
	// elsewhere, of the legacy directory itself, where launchers created
	// them before reading config.json.
	var bases []string
	if p.legacyBase != "" {
		bases = append(bases, p.legacyBase)
	}
	if !sameLocation(legacy, p.legacyBase) {
		bases = append(bases, legacy)
	}
	for _, lb := range bases {
		p.items = append(p.items,
			item{from: filepath.Join(lb, apppath.LegacyVersionsDirName), to: filepath.Join(tb, apppath.InstancesDirName), dir: true, pending: true},
			item{from: filepath.Join(lb, apppath.LegacyBackupsDirName), to: filepath.Join(tb, apppath.BackupsDirName), dir: true, pending: true, deep: true},
			// Launchers 0.0.8 and 0.0.9 kept world backups in backup\worlds.
			item{from: filepath.Join(lb, apppath.LegacyEraABackupDirName), to: filepath.Join(tb, apppath.BackupsDirName), dir: true, pending: true, deep: true},
			item{from: filepath.Join(lb, apppath.LegacyInstallersDirName), to: filepath.Join(cache, apppath.InstallersDirName), dir: true, pending: true, deep: true},
			item{from: filepath.Join(lb, apppath.LegacyUpdatesDirName), to: filepath.Join(cache, apppath.UpdatesDirName), dir: true, pending: true, deep: true},
			// A launcher that ran on the legacy layout (apppath.SetLegacyMode)
			// kept its downloads and imports in <legacy data root>\Cache.
			item{from: filepath.Join(lb, apppath.CacheDirName), to: cache, dir: true, deep: true},
		)
		// What an undo kept aside because an older launcher had recreated
		// the directory in the meantime goes where the directory goes, after
		// it: a junction there then keeps its place.
		for _, d := range []struct {
			name, to string
			deep     bool
		}{
			{apppath.LegacyVersionsDirName, filepath.Join(tb, apppath.InstancesDirName), false},
			{apppath.LegacyBackupsDirName, filepath.Join(tb, apppath.BackupsDirName), true},
			{apppath.LegacyInstallersDirName, filepath.Join(cache, apppath.InstallersDirName), true},
			{apppath.LegacyUpdatesDirName, filepath.Join(cache, apppath.UpdatesDirName), true},
		} {
			for _, from := range undoneDataDirs(lb, d.name) {
				p.items = append(p.items, item{from: from, to: d.to, dir: true, pending: true, deep: d.deep})
			}
		}
	}
	p.items = append(p.items,
		item{from: filepath.Join(legacy, apppath.LegacyGamertagsFileName), to: filepath.Join(cache, apppath.GamertagsFileName), replace: true},
		item{from: filepath.Join(legacy, apppath.LegacyGDKPendingFileName), to: filepath.Join(cache, apppath.GDKPendingVersionsFileName), replace: true},
		item{from: filepath.Join(legacy, apppath.LegacyAccountsDirName), to: filepath.Join(local, apppath.AccountsDirName), dir: true, replace: true, secure: true},
		item{from: filepath.Join(legacy, apppath.LegacyWebViewDirName), to: filepath.Join(local, apppath.WebView2DirName, apppath.LegacyWebViewDirName), dir: true, replace: true},
		// The current startup.log is open while the migration runs, so the
		// legacy logs are always merged.
		item{from: filepath.Join(legacy, apppath.LegacyLogsDirName), to: filepath.Join(local, apppath.LogsDirName), dir: true},
	)
	return p, nil
}

var errUndoFailed = errors.New("the migration could not be undone")

// Record is storage-migration.json, kept after a migration so that paths the
// legacy layout recorded elsewhere (package registrations, shortcuts) can be
// translated.
type Record struct {
	Format     int       `json:"format"`
	MigratedAt time.Time `json:"migrated_at"`
	LegacyDir  string    `json:"legacy_dir"`
	BaseRoot   string    `json:"base_root"`
	Moves      []Move    `json:"moves"`
	// ShortcutsRepaired is set once desktop shortcuts point to the moved
	// instances.
	ShortcutsRepaired bool `json:"shortcuts_repaired,omitempty"`
}

// Move is one entry that moved: what was at From is now at To.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

const recordFormat = 1

func (p *plan) run() (err error) {
	j, err := beginJournal(apppath.MigrationJournalPath())
	if err != nil {
		return err
	}
	m := &migrator{j: j}
	committed := false
	defer func() {
		if committed {
			// The files that the migration replaced, kept aside until now so
			// that an undo could restore them.
			for _, b := range m.backups {
				_ = os.Remove(b)
			}
			return
		}
		if err == nil {
			return
		}
		if rerr := j.rollback(); rerr != nil {
			err = fmt.Errorf("%w: %v (after: %v)", errUndoFailed, rerr, err)
		}
	}()

	rec := Record{Format: recordFormat, MigratedAt: time.Now().UTC(), LegacyDir: p.legacy, BaseRoot: p.targetBase}
	if p.record != nil {
		// Moves of the earlier migration still translate legacy paths.
		rec.Moves = append(rec.Moves, p.record.Moves...)
	}
	for _, it := range p.items {
		if samePath(it.from, it.to) {
			// The legacy name is the current one up to case (Backups in a
			// data root migrated in place, logs in the local directory).
			if it.dir && it.pending {
				if err := m.fixCase(it.from, it.to); err != nil {
					return err
				}
			}
			continue
		}
		info, statErr := os.Stat(it.from)
		if statErr != nil || info.IsDir() != it.dir {
			// Missing, or a file where the layout has a directory (or the
			// other way round): not legacy data.
			continue
		}
		if it.replace && exists(it.to) {
			if !p.legacyWins {
				// The current entry wins; the legacy one stays in place.
				continue
			}
			if err := m.moveAside(it.to); err != nil {
				return err
			}
		}
		moves, copied, err := m.moveEntry(it.from, it.to, it.deep)
		if err != nil {
			return err
		}
		rec.Moves = append(rec.Moves, moves...)
		if copied && it.secure {
			secureDir(it.to)
		}
		if it.pending && exists(it.from) {
			// Copied across volumes: keep the source under a name that no
			// longer marks the legacy layout as not migrated.
			retired := uniqueName(it.from+".migrated", "")
			if err := m.j.append(record{Op: opMove, From: it.from, Path: retired, Deep: it.deep}); err != nil {
				return err
			}
			if err := renameRetry(it.from, retired); err != nil {
				return err
			}
		}
	}
	// Inside Backups, the legacy names differ only in case.
	backups := filepath.Join(p.targetBase, apppath.BackupsDirName)
	if err := m.fixCase(backups, backups); err != nil {
		return err
	}
	for _, pair := range [][2]string{
		{apppath.LegacyWorldBackupsDirName, apppath.WorldBackupsDirName},
		{apppath.LegacyInstanceBackupsDirName, apppath.InstanceBackupsDirName},
	} {
		if err := m.fixCase(filepath.Join(backups, pair[0]), filepath.Join(backups, pair[1])); err != nil {
			return err
		}
	}

	if err := m.retireLegacyConfig(p.legacy); err != nil {
		return err
	}
	if p.currentCorrupt {
		if err := m.moveAside(apppath.ConfigPath()); err != nil {
			return err
		}
	}
	if err := m.writeConfig(p); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := m.replaceFile(apppath.MigrationPath(), data); err != nil {
		return err
	}
	if err := j.commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

type migrator struct {
	j *journal
	// backups are files replaced by the migration, removed after the commit.
	backups []string
}

// move renames from to to, journaled.
func (m *migrator) move(from, to string) error {
	if err := m.j.append(record{Op: opMove, From: from, Path: to}); err != nil {
		return err
	}
	return renameRetry(from, to)
}

// moveAside renames an existing entry to a free "<name>-before-migration"
// name next to it, so that the legacy entry can take its place.
func (m *migrator) moveAside(p string) error {
	return m.move(p, uniqueName(p, "-before-migration"))
}

// retireLegacyConfig renames the legacy config.json in place to
// config.migrated.json, so that the legacy directory no longer holds data to
// migrate. Its members are in the current config.json.
func (m *migrator) retireLegacyConfig(legacy string) error {
	from := filepath.Join(legacy, apppath.ConfigFileName)
	if !exists(from) {
		return nil
	}
	to := filepath.Join(legacy, apppath.LegacyMigratedConfigFileName)
	if exists(to) {
		// Retired by an earlier migration: keep both.
		to = uniqueName(to, "")
	}
	return m.move(from, to)
}

// replaceFile writes path, first moving an existing file aside, so that an
// undo restores it: the undo of a created file only removes it. The file
// moved aside is removed once the migration is committed.
func (m *migrator) replaceFile(path string, data []byte) error {
	if exists(path) {
		backup := uniqueName(path, "-before-migration")
		if err := m.j.append(record{Op: opMove, From: path, Path: backup, Discard: true}); err != nil {
			return err
		}
		if err := renameRetry(path, backup); err != nil {
			return err
		}
		m.backups = append(m.backups, backup)
	}
	return m.createFile(path, data)
}

// mkdirAll creates dir and the parents it lacks, journaling each. A file
// where a directory of the layout belongs is moved aside.
func (m *migrator) mkdirAll(dir string) error {
	if isDir(dir) {
		return nil
	}
	if exists(dir) {
		if err := m.moveAside(dir); err != nil {
			return err
		}
	} else if err := m.mkdirAll(filepath.Dir(dir)); err != nil {
		return err
	}
	if err := m.j.append(record{Op: opMkdir, Path: dir}); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

// moveEntry moves from to to. When to exists, the children of a directory
// are merged into it (recursively when deep), and a child whose name is
// taken keeps a "-legacy" name. It returns the moves for the migration
// record, the most specific ones last, and whether anything was copied
// rather than renamed.
func (m *migrator) moveEntry(from, to string, deep bool) ([]Move, bool, error) {
	info, err := os.Stat(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !exists(to) {
		if err := m.mkdirAll(filepath.Dir(to)); err != nil {
			return nil, false, err
		}
		copied, err := m.place(from, to, info.IsDir(), deep)
		return []Move{{From: from, To: to}}, copied, err
	}
	toInfo, err := os.Stat(to)
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() && !toInfo.IsDir() {
		// A file stands where the layout has a directory: keep it aside.
		if err := m.moveAside(to); err != nil {
			return nil, false, err
		}
		copied, err := m.place(from, to, true, deep)
		return []Move{{From: from, To: to}}, copied, err
	}
	if !info.IsDir() || !toInfo.IsDir() {
		target := uniqueName(to, "-legacy")
		copied, err := m.place(from, target, info.IsDir(), deep)
		return []Move{{From: from, To: target}}, copied, err
	}
	if isLink(from) && !isNotEmpty(to) {
		// A junction or symbolic link (a data folder kept on another drive)
		// takes the place of an empty directory as it is.
		if err := m.j.append(record{Op: opRmdir, Path: to}); err != nil {
			return nil, false, err
		}
		if err := os.Remove(to); err != nil {
			return nil, false, err
		}
		copied, err := m.place(from, to, true, deep)
		return []Move{{From: from, To: to}}, copied, err
	}
	ents, err := os.ReadDir(from)
	if err != nil {
		return nil, false, err
	}
	moves := []Move{{From: from, To: to}}
	anyCopied := false
	for _, e := range ents {
		src := filepath.Join(from, e.Name())
		dst := filepath.Join(to, e.Name())
		if exists(dst) {
			if deep && isDir(src) && isDir(dst) {
				sub, copied, err := m.moveEntry(src, dst, true)
				if err != nil {
					return nil, false, err
				}
				// The merge removed src once it was empty.
				moves = append(moves, sub...)
				anyCopied = anyCopied || copied
				continue
			}
			dst = uniqueName(dst, "-legacy")
		}
		// Every child gets a move of its own: it must outrank an older, more
		// specific move of the same legacy path (TranslateLegacyPath).
		moves = append(moves, Move{From: src, To: dst})
		copied, err := m.place(src, dst, isDir(src), deep)
		if err != nil {
			return nil, false, err
		}
		anyCopied = anyCopied || copied
	}
	// The children were renamed away: remove the empty legacy directory, so
	// that it no longer marks the legacy layout as not migrated. Copied
	// children keep the directory, and a link is retired by its caller.
	if !isLink(from) && !isNotEmpty(from) {
		if err := m.rmdir(from); err != nil {
			return nil, false, err
		}
	}
	return moves, anyCopied, nil
}

func (m *migrator) rmdir(dir string) error {
	if err := m.j.append(record{Op: opRmdir, Path: dir}); err != nil {
		return err
	}
	return os.Remove(dir)
}

// place renames from to to, or copies it, keeping the source, when the
// rename would cross volumes. It reports whether it copied.
func (m *migrator) place(from, to string, isDir, deep bool) (bool, error) {
	// A rename moves a junction or symbolic link itself, which is stored in
	// its parent directory: judge it by that directory's volume, not by the
	// volume it points to.
	entry := resolvedPath(from)
	if isLink(from) {
		entry = filepath.Dir(from)
	}
	if sameVolume(entry, filepath.Dir(to)) {
		if err := m.j.append(record{Op: opMove, From: from, Path: to, Deep: deep}); err != nil {
			return false, err
		}
		if err := renameRetry(from, to); err == nil || !isCrossDevice(err) {
			return false, err
		}
	}
	if err := m.j.append(record{Op: opCopy, Path: to}); err != nil {
		return true, err
	}
	if isDir {
		return true, utils.CopyDir(from, to)
	}
	return true, copyFile(from, to)
}

// fixCase renames a directory whose name differs only in case.
func (m *migrator) fixCase(from, to string) error {
	if !isDir(from) || isLink(from) {
		return nil
	}
	if actualName(from) == filepath.Base(to) {
		return nil
	}
	return m.move(from, filepath.Join(filepath.Dir(from), filepath.Base(to)))
}

func (m *migrator) createFile(path string, data []byte) error {
	if err := m.mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if err := m.j.append(record{Op: opCreate, Path: path}); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, data, 0o644)
}

// writeConfig writes the current config.json from the legacy one, merged with
// a current config.json that exists already: members of the preferred one
// (plan.legacyWins) win, members only the other has are kept. base_root is
// the plan's, as the data now is there; when the current config.json decides
// the data root, its base_root is kept as it is.
func (m *migrator) writeConfig(p *plan) error {
	cfg := map[string]json.RawMessage{}
	first, second := p.current, p.config
	if !p.legacyWins {
		first, second = p.config, p.current
	}
	for k, v := range first {
		cfg[k] = v
	}
	for k, v := range second {
		cfg[k] = v
	}
	dropBaseRoot(cfg)
	if p.keepCurrentRoot {
		for k, v := range p.current {
			if strings.EqualFold(k, "base_root") {
				cfg["base_root"] = v
			}
		}
	} else if p.baseRoot != "" {
		raw, err := json.Marshal(p.baseRoot)
		if err != nil {
			return err
		}
		cfg["base_root"] = raw
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return m.replaceFile(apppath.ConfigPath(), data)
}

// uniqueName returns a free name next to p: "<name><suffix>", then
// "<name><suffix>-2" and so on, before the extension of a file.
func uniqueName(p, suffix string) string {
	dir, base := filepath.Split(p)
	ext := ""
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		ext = filepath.Ext(base)
	}
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		s := suffix
		if i > 1 {
			s = fmt.Sprintf("%s-%d", suffix, i)
		}
		candidate := filepath.Join(dir, stem+s+ext)
		if !exists(candidate) {
			return candidate
		}
	}
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, info.Mode().Perm())
}

// renameRetry renames, retrying briefly while another process (a WebView2
// process of the launcher that just exited, a virus scanner) still holds a
// file.
func renameRetry(from, to string) error {
	var err error
	for attempt := 0; attempt < 15; attempt++ {
		if err = os.Rename(from, to); err == nil || !isTransient(err) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

func samePath(a, b string) bool {
	return utils.NormalizeWindowsPath(a) == utils.NormalizeWindowsPath(b)
}

// sameLocation is samePath after resolving junctions, symbolic links and 8.3
// short names of paths that exist.
func sameLocation(a, b string) bool {
	if samePath(a, b) {
		return true
	}
	return utils.CanonicalWindowsPath(a) == utils.CanonicalWindowsPath(b)
}

// resolvedPath is p with junctions and symbolic links resolved, or p.
func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// isLink reports a symbolic link or a junction.
func isLink(p string) bool {
	info, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0
}

// actualName is the name of p's last element as the file system stores it.
func actualName(p string) string {
	base := filepath.Base(p)
	ents, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		return base
	}
	for _, e := range ents {
		if strings.EqualFold(e.Name(), base) {
			return e.Name()
		}
	}
	return base
}
