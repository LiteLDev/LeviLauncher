package txn

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
)

// Options configure a Transaction.
type Options struct {
	// Reserved are paths that backups must never take, such as the files
	// the operation writes: relative to the root ('/' or the OS separator)
	// or absolute.
	Reserved []string
	// Context cancels WriteFile and Commit; the caller then rolls back.
	Context context.Context
	// Progress, when set, is called with the number of bytes of every chunk
	// WriteFile writes, so that callers can report measured progress.
	Progress func(written int64)
	// FS is the file system (OS when nil).
	FS FS
	// Now is the clock of the Begin records (time.Now when nil).
	Now func() time.Time
	// CommitStaged makes the operation's result durable at the commit point
	// (by default: moves .lip/state.json.tmp onto .lip/state.json;
	// nothing when no file is staged). Used by Commit.
	CommitStaged func() error
	// DiscardStaged throws the operation's result away on rollback (by
	// default: deletes .lip/state.json.tmp). Used by Commit and by
	// Rollback(nil), which ignore its failure: a staged state file is
	// never read, and the next save replaces it.
	DiscardStaged func() error
}

var open atomic.Int64

// AnyOpen reports whether a transaction of this process is open, i.e. an
// operation is changing a workspace: ending the process now leaves changes
// for the next operation to undo.
func AnyOpen() bool { return open.Load() > 0 }

// Transaction is WorkspaceTransaction.cs: the changes of one operation to a
// workspace, which are either all committed or all rolled back
// (protocol/workspace.md §6, §7). See the package documentation.
type Transaction struct {
	fs          FS
	root        string
	journalPath string
	reserved    map[string]struct{}
	opts        Options
	entries     []entry
	journal     io.WriteCloser
	// started is the start time of the "Begin" record, once the journal
	// exists (WorkspaceTransaction._started).
	started   *string
	committed bool
	closed    bool
}

// Begin starts a transaction on the workspace root. Nothing is written until
// the first change, which creates the journal <root>/.lip/journal; it fails
// when the journal of another operation exists (Recover it first). Close the
// transaction when done with it.
func Begin(root string, opts Options) (*Transaction, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	if opts.FS == nil {
		opts.FS = OS
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	t := &Transaction{
		fs:          opts.FS,
		root:        root,
		journalPath: filepath.Join(root, LipDirectory, JournalFile),
		reserved:    map[string]struct{}{},
		opts:        opts,
	}
	for _, r := range opts.Reserved {
		t.reserved[pathKey(t.abs(r))] = struct{}{}
	}
	open.Add(1)
	return t, nil
}

// Root is the workspace root, as a full path.
func (t *Transaction) Root() string { return t.root }

// abs is a path of the workspace as a full path.
func (t *Transaction) abs(p string) string { return absolutePath(t.root, p) }

// CreateDirectory creates a directory and its missing ancestors, recording
// each of them, top-down.
func (t *Transaction) CreateDirectory(rel string) error {
	return t.createDirectory(t.abs(rel))
}

func (t *Transaction) createDirectory(path string) error {
	var missing []string
	for d := path; d != "" && !dirExists(t.fs, d); {
		missing = append(missing, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := t.record(entry{op: opCreateDirectory, path: missing[i]}); err != nil {
			return err
		}
		if err := t.fs.MkdirAll(missing[i]); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile writes a file, creating its directory; an existing file is
// backed up first. The content is copied in chunks, each reported to
// Options.Progress; the copy stops when Options.Context is cancelled.
func (t *Transaction) WriteFile(rel string, content io.Reader) error {
	path := t.abs(rel)
	if err := t.createDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if fileExists(t.fs, path) {
		if err := t.backup(path); err != nil {
			return err
		}
	}
	if err := t.record(entry{op: opCreateFile, path: path}); err != nil {
		return err
	}
	out, err := t.fs.Create(path)
	if err != nil {
		return err
	}
	if err := t.copy(out, content); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// copy is Stream.CopyToAsync with a cancellation token: chunks of 81920
// bytes.
func (t *Transaction) copy(w io.Writer, r io.Reader) error {
	buf := make([]byte, 81920)
	for {
		if err := t.opts.Context.Err(); err != nil {
			return err
		}
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if t.opts.Progress != nil {
				t.opts.Progress(int64(n))
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// DeleteFile deletes a file: renames it to a backup, deleted at commit.
func (t *Transaction) DeleteFile(rel string) error { return t.backup(t.abs(rel)) }

// DeleteDirectoryOnCommit deletes the directory at commit, if it is empty
// then.
func (t *Transaction) DeleteDirectoryOnCommit(rel string) error {
	return t.record(entry{op: opDeleteDirectoryOnCommit, path: t.abs(rel)})
}

// Reserve adds a path that backups must never take (Options.Reserved).
func (t *Transaction) Reserve(rel string) {
	t.reserved[pathKey(t.abs(rel))] = struct{}{}
}

func (t *Transaction) backup(path string) error {
	backup := t.backupPath(path)
	// "hidden" is the Windows hidden attribute (protocol/workspace.md §7.3);
	// elsewhere .NET reports every name starting with '.' as hidden.
	wasHidden := false
	if runtime.GOOS == "windows" {
		var err error
		if wasHidden, err = t.fs.Hidden(path); err != nil {
			return err
		}
	}
	if err := t.record(entry{op: opBackup, path: path, to: backup, hasTo: true,
		hidden: wasHidden}); err != nil {
		return err
	}
	// File.Move moves files only; a directory is not found.
	if !fileExists(t.fs, path) {
		return &os.PathError{Op: "move", Path: path, Err: os.ErrNotExist}
	}
	if err := t.fs.Rename(path, backup); err != nil {
		return err
	}
	return setHiddenWrapped(t.fs, backup, true)
}

// backupPath is ".{name}.bak" in the directory of the file, or
// ".{name}.bak1", ".{name}.bak2", ... when that name is taken or reserved.
func (t *Transaction) backupPath(path string) string {
	dir := filepath.Dir(path)
	name := "." + filepath.Base(path) + ".bak"
	backup := filepath.Join(dir, name)
	for i := 1; ; i++ {
		// File.Exists and Directory.Exists also see a dangling link.
		_, statErr := t.fs.Stat(backup)
		if statErr != nil {
			_, statErr = t.fs.Lstat(backup)
		}
		_, reserved := t.reserved[pathKey(backup)]
		if statErr != nil && !reserved {
			return backup
		}
		backup = filepath.Join(dir, name+strconv.Itoa(i))
	}
}

// MarkCommitted is WorkspaceTransaction.Commit: appends "Commit" to the
// journal. From then on recovery finishes the operation instead of undoing
// it; the caller makes the operation's result durable right after this,
// then calls Complete (or Rollback when that fails).
func (t *Transaction) MarkCommitted() error {
	if t.journal != nil {
		if err := t.append(entry{op: opCommit}); err != nil {
			return err
		}
	}
	t.committed = true
	return nil
}

// Complete, after MarkCommitted, deletes the backups and the directories to
// delete at commit, then the journal. Best effort: it returns what could not
// be done (warnings); a backup that cannot be deleted is left.
func (t *Transaction) Complete() []error {
	if !t.committed {
		return []error{bug("lip tried to complete an operation on the workspace that is not" +
			" committed. This is a bug in lip.")}
	}
	errs := finish(t.fs, t.root, t.entries)
	t.closeJournal(nil, true, &errs)
	t.entries = nil
	return errs
}

// Commit is the commit point of McmodService.Apply: it checks for
// cancellation, writes the staged state file (writeStaged, may be nil),
// records the commit in the journal, moves the staged file into place
// (Options.CommitStaged) and completes the transaction, returning the
// warnings of Complete. When any step fails, the transaction is rolled back
// and a *RollbackError is returned.
func (t *Transaction) Commit(writeStaged func() error) ([]error, error) {
	fail := func(err error) ([]error, error) {
		return nil, &RollbackError{Err: err, UndoErrors: t.Rollback(nil)}
	}
	if err := t.opts.Context.Err(); err != nil {
		return fail(err)
	}
	if writeStaged != nil {
		if err := writeStaged(); err != nil {
			return fail(err)
		}
	}
	if err := t.MarkCommitted(); err != nil {
		return fail(err)
	}
	commit := t.opts.CommitStaged
	if commit == nil {
		commit = func() error { return CommitStaged(t.fs, t.root) }
	}
	if err := commit(); err != nil {
		return fail(err)
	}
	return t.Complete(), nil
}

// Rollback undoes the operation (protocol/workspace.md §7.5): discardLock
// throws its result away (nil: Options.DiscardStaged, else deleting
// .lip/state.json.tmp, ignoring failures), then every change is undone
// in reverse order. Best effort: it returns what could not be undone, which
// the journal keeps for the next operation. A committed transaction loses
// its "Commit" record first (§7.4); when it cannot, nothing is undone and
// the journal is left for the next operation to complete. A discardLock
// failure is returned without undoing anything, leaving the journal.
func (t *Transaction) Rollback(discardLock func() error) []error {
	if discardLock == nil {
		discard := t.opts.DiscardStaged
		if discard == nil {
			discard = func() error { return DiscardStaged(t.fs, t.root) }
		}
		discardLock = func() error { _ = discard(); return nil }
	}
	var errs []error
	if t.committed && t.journal != nil {
		t.journal.Close()
		t.journal = nil
		if err := writeJournal(t.fs, t.journalPath, t.root, t.entries, false, t.started); err != nil {
			// The journal still commits the operation, and the staged state file is kept,
			// so that the next operation completes it rather than finding it partly undone.
			return append(errs, wrap(err, fmt.Sprintf("Could not undo the operation, because the"+
				" journal '%s' cannot be updated: %s The changes are kept, and the next install,"+
				" uninstall or update of this workspace completes the operation.",
				show(t.root, t.journalPath), reason(err))))
		}
	}
	if err := discardLock(); err != nil {
		return append(errs, err)
	}
	left, undoErrs := undo(t.fs, t.root, t.entries)
	errs = append(errs, undoErrs...)
	t.closeJournal(left, false, &errs)
	t.entries = nil
	t.committed = false
	return errs
}

// Close releases the transaction. One closed without Complete or Rollback
// leaves its journal, as a killed process would.
func (t *Transaction) Close() error {
	if t.journal != nil {
		t.journal.Close()
		t.journal = nil
	}
	if !t.closed {
		t.closed = true
		open.Add(-1)
	}
	return nil
}

func (t *Transaction) record(e entry) error {
	if err := t.Begin(); err != nil {
		return err
	}
	if err := t.append(e); err != nil {
		return err
	}
	t.entries = append(t.entries, e)
	return nil
}

// Begin is WorkspaceTransaction.Begin: creates the journal with its "Begin"
// record unless it exists, before the first change or hook command of the
// operation (protocol/workspace.md §7.4 step 5).
func (t *Transaction) Begin() error {
	if t.committed {
		return bug("lip tried to change the workspace after committing the operation. This is a" +
			" bug in lip.")
	}
	if t.journal == nil {
		if len(t.entries) > 0 || t.started != nil {
			return bug("lip tried to change the workspace after the operation ended. This is a bug" +
				" in lip.")
		}
		// Create new: the journal of an interrupted operation is recovered before a new
		// operation starts, and never overwritten.
		err := t.fs.MkdirAll(filepath.Dir(t.journalPath))
		if err == nil {
			t.journal, err = t.fs.CreateNew(t.journalPath)
		}
		if err != nil {
			t.journal = nil
			return wrap(err, fmt.Sprintf("Cannot create the journal '%s', where lip records its"+
				" changes so that they can be undone; lip changed no files.",
				show(t.root, t.journalPath)))
		}
		started := startTime(t.opts.Now())
		t.started = &started
		if err := t.append(entry{op: opBegin, to: started, hasTo: true}); err != nil {
			return err
		}
	}
	return nil
}

// append writes a record and hands it to the operating system, which keeps
// it when lip is killed.
func (t *Transaction) append(e entry) error {
	if _, err := io.WriteString(t.journal, serialize(toJournal(t.root, e))+"\n"); err != nil {
		return wrap(err, fmt.Sprintf("Cannot write to the journal '%s', where lip records its"+
			" changes so that they can be undone.", show(t.root, t.journalPath)))
	}
	return nil
}

// closeJournal deletes the journal this transaction wrote when nothing is
// left, else replaces it with what is left. A failure is added to errs: the
// journal only matters after a crash.
func (t *Transaction) closeJournal(left []entry, committed bool, errs *[]error) {
	// Begin sets started when it creates the journal, which an operation of
	// only hook commands has too, though it records no change.
	written := t.started != nil || t.journal != nil || len(t.entries) > 0
	if t.journal != nil {
		t.journal.Close()
		t.journal = nil
	}
	if !written {
		return
	}
	var err error
	if len(left) == 0 {
		if fileExists(t.fs, t.journalPath) {
			err = t.fs.Remove(t.journalPath)
		}
	} else {
		err = writeJournal(t.fs, t.journalPath, t.root, left, committed, t.started)
	}
	if err != nil {
		*errs = append(*errs, wrap(err, fmt.Sprintf("Could not update the journal '%s': %s The next"+
			" install, uninstall or update of this workspace checks it again.",
			show(t.root, t.journalPath), reason(err))))
	}
}

// writeJournal replaces the journal, through journal.tmp, with a Begin, the
// entries and, when committed, Commit. started is the time the operation
// started, as its "Begin" record gave it, or nil when unknown.
func writeJournal(fsys FS, path, root string, entries []entry, committed bool, started *string) error {
	var b strings.Builder
	begin := entry{op: opBegin}
	if started != nil {
		begin.to, begin.hasTo = *started, true
	}
	all := append([]entry{begin}, entries...)
	if committed {
		all = append(all, entry{op: opCommit})
	}
	for _, e := range all {
		b.WriteString(serialize(toJournal(root, e)))
		b.WriteByte('\n')
	}
	temp := path + ".tmp"
	if err := fsys.WriteFile(temp, []byte(b.String())); err != nil {
		return err
	}
	return fsys.Rename(temp, path)
}

// undo undoes the entries in reverse order; returns those that could not be
// undone, in order. Every undo checks the state first, so undoing a change
// that was never made, or was undone before, does nothing.
func undo(fsys FS, root string, entries []entry) ([]entry, []error) {
	var errs []error
	var left []entry
	firstBackup := map[string]int{}
	for i, e := range entries {
		if e.op == opBackup {
			if _, ok := firstBackup[pathKey(e.path)]; !ok {
				firstBackup[pathKey(e.path)] = i
			}
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		// A file created where an earlier change backed one up: once that backup is gone
		// (undone before, or deleted by someone else), the file at the path is not deleted.
		if e.op == opCreateFile {
			if b, ok := firstBackup[pathKey(e.path)]; ok && b < i && !fileExists(fsys, entries[b].to) {
				continue
			}
		}
		// A created directory still holding a change that could not be undone is kept for
		// the next attempt.
		if e.op == opCreateDirectory {
			prefix := pathKey(strings.TrimRight(e.path, `/\`) + string(filepath.Separator))
			holds := false
			for _, l := range left {
				if strings.HasPrefix(pathKey(l.path), prefix) {
					holds = true
					break
				}
			}
			if holds {
				left = append([]entry{e}, left...)
				continue
			}
		}
		if err := undoEntry(fsys, e); err != nil {
			errs = append(errs, wrap(err, fmt.Sprintf("Could not undo %s: %s",
				describe(root, e), reason(err))))
			left = append([]entry{e}, left...)
		}
	}
	return left, errs
}

func undoEntry(fsys FS, e entry) error {
	switch e.op {
	case opCreateFile:
		if fileExists(fsys, e.path) {
			if err := fsys.ClearReadOnly(e.path); err != nil {
				return err
			}
			return fsys.Remove(e.path)
		}
	case opBackup:
		if fileExists(fsys, e.to) {
			if fileExists(fsys, e.path) {
				if err := fsys.ClearReadOnly(e.path); err != nil {
					return err
				}
				if err := fsys.Remove(e.path); err != nil {
					return err
				}
			}
			if err := fsys.Rename(e.to, e.path); err != nil {
				return err
			}
			return setHiddenWrapped(fsys, e.path, e.hidden)
		}
	case opCreateDirectory:
		if dirExists(fsys, e.path) {
			empty, err := dirEmpty(fsys, e.path)
			if err != nil {
				return err
			}
			if empty {
				return fsys.Remove(e.path)
			}
		}
	}
	return nil
}

// finish deletes the backups, then the directories to delete at commit that
// are empty, deepest (longest) first. Best effort.
func finish(fsys FS, root string, entries []entry) []error {
	var errs []error
	for _, e := range entries {
		if e.op != opBackup || !fileExists(fsys, e.to) {
			continue
		}
		err := fsys.ClearReadOnly(e.to)
		if err == nil {
			err = fsys.Remove(e.to)
		}
		if err != nil {
			errs = append(errs, wrap(err, fmt.Sprintf("Could not delete '%s', the backup of the old"+
				" '%s': %s It is no longer needed; delete it yourself.",
				show(root, e.to), show(root, e.path), reason(err))))
		}
	}
	var dirs []entry
	for _, e := range entries {
		if e.op == opDeleteDirectoryOnCommit {
			dirs = append(dirs, e)
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool {
		return utf16Len(dirs[i].path) > utf16Len(dirs[j].path)
	})
	for _, e := range dirs {
		if err := deleteEmptyDirectory(fsys, e.path); err != nil {
			errs = append(errs, wrap(err, fmt.Sprintf("Could not delete the empty directory '%s': %s"+
				" You may delete it yourself.", show(root, e.path), reason(err))))
		}
	}
	return errs
}

func deleteEmptyDirectory(fsys FS, path string) error {
	if !dirExists(fsys, path) {
		return nil
	}
	link, err := isLink(fsys, path)
	if err != nil || link {
		return err
	}
	empty, err := dirEmpty(fsys, path)
	if err != nil || !empty {
		return err
	}
	return fsys.Remove(path)
}

// utf16Len is string.Length.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
