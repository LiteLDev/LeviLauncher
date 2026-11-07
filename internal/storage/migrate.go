// Package storage moves the launcher's files from the legacy layout
// (%APPDATA%\<exe name>) to the one internal/apppath/README.md specifies,
// without losing or overwriting anything.
//
// The migration runs once, at startup, before config.json is read. It only
// renames on one volume, or copies and keeps the source; it never deletes a
// file of the legacy layout. When it cannot complete, for example because the
// game still runs from an instance, every change is undone and the launcher
// runs on the legacy layout until the next start (apppath.SetLegacyMode).
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
	if exists(apppath.ConfigPath()) {
		return Current, nil
	}
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

// findLegacyDir returns the legacy directory when it holds anything the
// launcher wrote.
func findLegacyDir() string {
	candidates := []string{apppath.LegacyDir(), filepath.Join(apppath.AppData(), apppath.LegacyDefaultExeName)}
	for _, dir := range candidates {
		for _, name := range []string{
			apppath.ConfigFileName,
			apppath.LegacyVersionsDirName,
			apppath.LegacyBackupsDirName,
			apppath.LegacyInstallersDirName,
			apppath.LegacyAccountsDirName,
			apppath.LegacyWebViewDirName,
		} {
			if exists(filepath.Join(dir, name)) {
				return dir
			}
		}
	}
	return ""
}

// item is one entry of the legacy layout and where it goes.
type item struct {
	from, to string
	// renameOnly items are too large to copy: they move only by a rename on
	// one volume. Other items are copied, keeping the source, when a rename
	// is not possible.
	renameOnly bool
}

type plan struct {
	legacy string
	// config is legacy config.json as members, so that keys this version
	// does not know survive the migration.
	config     map[string]json.RawMessage
	baseRoot   string // the new base_root ("" for the default)
	legacyBase string
	targetBase string
	items      []item
}

func newPlan(legacy string) (*plan, error) {
	p := &plan{legacy: legacy, config: map[string]json.RawMessage{}}
	if b, err := os.ReadFile(filepath.Join(legacy, apppath.ConfigFileName)); err == nil {
		if err := json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &p.config); err != nil {
			return nil, fmt.Errorf("read legacy config: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
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
		{from: filepath.Join(legacy, apppath.LegacyGamertagsFileName), to: filepath.Join(cache, apppath.GamertagsFileName)},
		{from: filepath.Join(legacy, apppath.LegacyGDKPendingFileName), to: filepath.Join(cache, apppath.GDKPendingVersionsFileName)},
		{from: filepath.Join(legacy, apppath.LegacyAccountsDirName), to: filepath.Join(local, apppath.AccountsDirName)},
		{from: filepath.Join(legacy, apppath.LegacyWebViewDirName), to: filepath.Join(local, apppath.WebView2DirName, apppath.LegacyWebViewDirName)},
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
	defer func() {
		if err == nil {
			return
		}
		if rerr := j.rollback(); rerr != nil {
			err = fmt.Errorf("%w: %v (after: %v)", errUndoFailed, rerr, err)
		}
	}()

	rec := Record{Format: recordFormat, MigratedAt: time.Now().UTC(), LegacyDir: p.legacy, BaseRoot: p.targetBase}
	for _, it := range p.items {
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

	if err := m.writeConfig(p); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := m.createFile(apppath.MigrationPath(), data); err != nil {
		return err
	}
	return j.commit()
}

type migrator struct {
	j *journal
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
		target := uniqueName(to)
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
		return true, m.place(from, uniqueName(to), true, renameOnly)
	}
	ents, err := os.ReadDir(from)
	if err != nil {
		return false, err
	}
	for _, e := range ents {
		src := filepath.Join(from, e.Name())
		dst := filepath.Join(to, e.Name())
		if exists(dst) {
			dst = uniqueName(dst)
		}
		if err := m.place(src, dst, e.IsDir(), renameOnly); err != nil {
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

// writeConfig writes the current config.json last: its existence marks the
// layout as migrated.
func (m *migrator) writeConfig(p *plan) error {
	cfg := p.config
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
	return m.createFile(apppath.ConfigPath(), data)
}

// uniqueName returns a free name next to p: "<name>-legacy", then
// "<name>-legacy-2" and so on, before the extension of a file.
func uniqueName(p string) string {
	dir, base := filepath.Split(p)
	ext := ""
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		ext = filepath.Ext(base)
	}
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		suffix := "-legacy"
		if i > 1 {
			suffix = fmt.Sprintf("-legacy-%d", i)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
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
