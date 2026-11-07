// Package storage moves the launcher's files from the legacy layout
// (%APPDATA%\<exe name>) to the one internal/apppath/README.md specifies,
// without losing or overwriting anything.
//
// The migration runs at startup, before config.json is read, whenever the
// legacy layout holds data that has not been migrated, whether or not the
// current layout exists already. It only renames on one volume, or copies and
// keeps the source; it never deletes a file of the legacy layout. When it
// cannot complete, for example because the game still runs from an instance,
// every change is undone and the launcher runs on the legacy layout until the
// next start (apppath.SetLegacyMode).
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	// Migrated: the legacy layout was migrated during this start.
	Migrated
	// Legacy: the migration was undone; the launcher runs on the legacy
	// layout and tries again at the next start.
	Legacy
)

// Prepare decides the layout for this run, migrating the legacy layout when
// it exists. An error with Outcome Current means a migration that an earlier
// start left behind could not be undone; the launcher must not start, as it
// would find its files in neither layout.
func Prepare() (Outcome, error) {
	if err := recoverJournal(apppath.MigrationJournalPath()); err != nil {
		return Current, err
	}
	// Whether to migrate depends only on the legacy data: a current
	// config.json may exist without it ever having been migrated, for example
	// when something created the current layout before the first start of
	// this version.
	legacy := findLegacyDir()
	if legacy == "" {
		return Current, nil
	}
	plan, err := newPlan(legacy)
	if err != nil {
		apppath.SetLegacyMode(legacy)
		return Legacy, err
	}
	if err := plan.run(); err != nil {
		if errors.Is(err, errUndoFailed) {
			return Current, err
		}
		apppath.SetLegacyMode(legacy)
		return Legacy, err
	}
	return Migrated, nil
}

// findLegacyDir returns the legacy directory when it holds data that has not
// been migrated.
func findLegacyDir() string {
	candidates := []string{apppath.LegacyDir(), filepath.Join(apppath.AppData(), apppath.LegacyDefaultExeName)}
	for _, dir := range candidates {
		if hasPendingLegacyData(dir) {
			return dir
		}
	}
	return ""
}

// hasPendingLegacyData reports whether dir holds the legacy config.json,
// which every legacy launcher wrote at its first start and which a migration
// renames to config.migrated.json, or a legacy data directory under the
// default data root. microsoft-account, EBWebView and logs alone do not
// count: a migration across volumes copies them and keeps the source, which
// would then start the migration again at every start.
func hasPendingLegacyData(dir string) bool {
	if exists(filepath.Join(dir, apppath.ConfigFileName)) {
		return true
	}
	for _, name := range []string{
		apppath.LegacyVersionsDirName,
		apppath.LegacyBackupsDirName,
		apppath.LegacyInstallersDirName,
		apppath.LegacyUpdatesDirName,
	} {
		// The exact name: a migration that keeps the data root in place
		// renames backups to Backups, which Windows finds under either name.
		p := filepath.Join(dir, name)
		if exists(p) && actualName(p) == name {
			return true
		}
	}
	return false
}

// item is one entry of the legacy layout and where it goes.
type item struct {
	from, to string
	// renameOnly items are too large to copy: they move only by a rename on
	// one volume. Other items are copied, keeping the source, when a rename
	// is not possible.
	renameOnly bool
	// replace items are taken whole from the legacy layout when the plan
	// prefers it (plan.legacyWins): an existing entry at to is moved aside
	// rather than merged with, since a merge would leave the legacy entry
	// unused (the WebView2 profile, the account records).
	replace bool
}

type plan struct {
	legacy string
	// config is legacy config.json as members, so that keys this version
	// does not know survive the migration.
	config map[string]json.RawMessage
	// current is the current config.json, when one exists already; nil
	// otherwise.
	current    map[string]json.RawMessage
	baseRoot   string // the new base_root ("" for the default)
	legacyBase string
	targetBase string
	items      []item
	// record is the record of an earlier migration, or nil.
	record *Record
	// legacyWins is set when no migration happened before: a current layout
	// that exists then was made without the user's data (by a start that
	// skipped the migration, or by a tool), so the legacy settings, WebView2
	// profile and accounts take precedence. After an earlier migration, legacy
	// data comes from running an old version again, and the current data
	// takes precedence.
	legacyWins bool
}

func readConfigMembers(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func newPlan(legacy string) (*plan, error) {
	p := &plan{legacy: legacy, config: map[string]json.RawMessage{}}
	if m, err := readConfigMembers(filepath.Join(legacy, apppath.ConfigFileName)); err == nil {
		p.config = m
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
	if m, err := readConfigMembers(apppath.ConfigPath()); err == nil {
		p.current = m
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read current config: %w", err)
	}
	p.record = readRecord()
	p.legacyWins = p.record == nil
	var base string
	if raw, ok := p.config["base_root"]; ok {
		_ = json.Unmarshal(raw, &base)
	}
	base = strings.TrimSpace(base)

	local := apppath.LocalDir()
	p.legacyBase = base
	isDefault := base == "" || samePath(base, legacy)
	if isDefault {
		p.legacyBase = legacy
	}
	switch {
	case !isDefault && !exists(base):
		// A data root on a drive that is not connected would be skipped and
		// keep its legacy names: migrate once it is back.
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

	lb, tb := p.legacyBase, p.targetBase
	cache := filepath.Join(tb, apppath.CacheDirName)
	p.items = []item{
		{from: filepath.Join(lb, apppath.LegacyVersionsDirName), to: filepath.Join(tb, apppath.InstancesDirName), renameOnly: true},
		{from: filepath.Join(lb, apppath.LegacyBackupsDirName), to: filepath.Join(tb, apppath.BackupsDirName), renameOnly: true},
		{from: filepath.Join(lb, apppath.LegacyInstallersDirName), to: filepath.Join(cache, apppath.InstallersDirName), renameOnly: true},
		{from: filepath.Join(lb, apppath.LegacyUpdatesDirName), to: filepath.Join(cache, apppath.UpdatesDirName), renameOnly: true},
		{from: filepath.Join(legacy, apppath.LegacyGamertagsFileName), to: filepath.Join(cache, apppath.GamertagsFileName), replace: true},
		{from: filepath.Join(legacy, apppath.LegacyGDKPendingFileName), to: filepath.Join(cache, apppath.GDKPendingVersionsFileName), replace: true},
		{from: filepath.Join(legacy, apppath.LegacyAccountsDirName), to: filepath.Join(local, apppath.AccountsDirName), replace: true},
		{from: filepath.Join(legacy, apppath.LegacyWebViewDirName), to: filepath.Join(local, apppath.WebView2DirName, apppath.LegacyWebViewDirName), replace: true},
		// The current startup.log is open while the migration runs, so the
		// legacy logs are always merged.
		{from: filepath.Join(legacy, apppath.LegacyLogsDirName), to: filepath.Join(local, apppath.LogsDirName)},
	}
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
		if it.replace && p.legacyWins && exists(it.from) && exists(it.to) && !samePath(it.from, it.to) {
			if err := m.moveAside(it.to); err != nil {
				return err
			}
		}
		moved, err := m.moveEntry(it.from, it.to, it.renameOnly)
		if err != nil {
			return err
		}
		if moved {
			rec.Moves = append(rec.Moves, Move{From: it.from, To: it.to})
		}
	}
	// Inside Backups, the legacy names differ only in case.
	backups := filepath.Join(p.targetBase, apppath.BackupsDirName)
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

// moveAside renames an existing entry to a free "<name>-before-migration"
// name next to it, so that the legacy entry can take its place.
func (m *migrator) moveAside(p string) error {
	aside := uniqueName(p, "-before-migration")
	if err := m.j.append(record{Op: opMove, From: p, Path: aside}); err != nil {
		return err
	}
	return renameRetry(p, aside)
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
	if err := m.j.append(record{Op: opMove, From: from, Path: to}); err != nil {
		return err
	}
	return renameRetry(from, to)
}

// replaceFile writes path, first moving an existing file aside, so that an
// undo restores it: the undo of a created file only removes it.
func (m *migrator) replaceFile(path string, data []byte) error {
	if exists(path) {
		backup := uniqueName(path, "-before-migration")
		if err := m.j.append(record{Op: opMove, From: path, Path: backup}); err != nil {
			return err
		}
		if err := renameRetry(path, backup); err != nil {
			return err
		}
		m.backups = append(m.backups, backup)
	}
	return m.createFile(path, data)
}

// mkdirAll creates dir and the parents it lacks, journaling each.
func (m *migrator) mkdirAll(dir string) error {
	if exists(dir) {
		return nil
	}
	if err := m.mkdirAll(filepath.Dir(dir)); err != nil {
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
// are merged into it, and a child whose name is taken keeps a "-legacy"
// name. It reports whether anything moved.
func (m *migrator) moveEntry(from, to string, renameOnly bool) (bool, error) {
	info, err := os.Lstat(from)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !exists(to) || samePath(from, to) {
		if err := m.mkdirAll(filepath.Dir(to)); err != nil {
			return false, err
		}
		return true, m.place(from, to, info.IsDir(), renameOnly)
	}
	if !info.IsDir() {
		target := uniqueName(to, "-legacy")
		if err := m.place(from, target, false, renameOnly); err != nil {
			return false, err
		}
		return true, nil
	}
	toInfo, err := os.Stat(to)
	if err != nil {
		return false, err
	}
	if !toInfo.IsDir() {
		return true, m.place(from, uniqueName(to, "-legacy"), true, renameOnly)
	}
	ents, err := os.ReadDir(from)
	if err != nil {
		return false, err
	}
	for _, e := range ents {
		src := filepath.Join(from, e.Name())
		dst := filepath.Join(to, e.Name())
		if exists(dst) {
			dst = uniqueName(dst, "-legacy")
		}
		if err := m.place(src, dst, e.IsDir(), renameOnly); err != nil {
			return false, err
		}
	}
	// The children were renamed away: remove the empty legacy directory, so
	// that it no longer marks the legacy layout as not migrated. Copied
	// children keep the directory.
	if !isNotEmpty(from) {
		if err := m.j.append(record{Op: opRmdir, Path: from}); err != nil {
			return false, err
		}
		if err := os.Remove(from); err != nil {
			return false, err
		}
	}
	return true, nil
}

// place renames from to to, or copies it when the rename crosses volumes and
// the entry may be copied.
func (m *migrator) place(from, to string, isDir, renameOnly bool) error {
	if renameOnly || sameVolume(from, filepath.Dir(to)) {
		if err := m.j.append(record{Op: opMove, From: from, Path: to}); err != nil {
			return err
		}
		return renameRetry(from, to)
	}
	if err := m.j.append(record{Op: opCopy, Path: to}); err != nil {
		return err
	}
	if isDir {
		return utils.CopyDir(from, to)
	}
	return copyFile(from, to)
}

// fixCase renames a directory whose name differs only in case.
func (m *migrator) fixCase(from, to string) error {
	info, err := os.Stat(from)
	if err != nil || !info.IsDir() {
		return nil
	}
	if actual := actualName(from); actual == filepath.Base(to) {
		return nil
	}
	if err := m.j.append(record{Op: opMove, From: from, Path: to}); err != nil {
		return err
	}
	return renameRetry(from, to)
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
// always the plan's, as the data now is there.
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
	if p.baseRoot == "" {
		delete(cfg, "base_root")
	} else {
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
