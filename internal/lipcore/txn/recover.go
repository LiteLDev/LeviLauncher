package txn

import (
	"fmt"
	"path/filepath"
)

// RecoveryResult is what the journal of an interrupted operation held, and
// what was done about it (WorkspaceTransaction.RecoveryResult). Committed:
// the operation had committed and was finished; otherwise its changes were
// undone. Started: when the operation started, as recorded ("" when
// unknown). Changes: the number of changes recorded. Errors: what could not
// be done; a change that could not be undone stays in the journal for the
// next attempt, a backup that could not be deleted is left.
type RecoveryResult struct {
	Committed bool
	Started   string
	Changes   int
	Errors    []error
}

// RecoverOptions configure RecoverWith.
type RecoverOptions struct {
	// FS is the file system (OS when nil).
	FS FS
	// CommitLock makes the result of a committed operation durable; it must
	// do nothing when that was already done. Default: moves
	// .lip/state.json.tmp onto .lip/state.json when present.
	CommitLock func() error
	// DiscardLock throws the result of an uncommitted operation away.
	// Default: deletes .lip/state.json.tmp when present.
	DiscardLock func() error
}

// Recover finishes or undoes the operation the journal of root left behind
// (protocol/workspace.md §7.5), then deletes the journal; it returns nil
// when there is no journal. The caller must hold the workspace lock.
func Recover(root string) (*RecoveryResult, error) {
	return RecoverWith(root, RecoverOptions{})
}

// RecoverWith is Recover with explicit options.
func RecoverWith(root string, opts RecoverOptions) (*RecoveryResult, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	fsys := opts.FS
	if fsys == nil {
		fsys = OS
	}
	if opts.CommitLock == nil {
		opts.CommitLock = func() error { return CommitStaged(fsys, root) }
	}
	if opts.DiscardLock == nil {
		opts.DiscardLock = func() error { return DiscardStaged(fsys, root) }
	}
	journalPath := filepath.Join(root, LipDirectory, JournalFile)
	if !fileExists(fsys, journalPath) {
		return nil, nil
	}
	shown := show(root, journalPath)
	data, err := fsys.ReadFile(journalPath)
	if err != nil {
		return nil, wrap(err, fmt.Sprintf("Cannot read the journal '%s' of an interrupted lip"+
			" operation, so lip cannot finish or undo that operation. If a program is using the"+
			" file, close it; check that you may read and write .lip; then try again.", shown))
	}
	records, _ := parseJournal(data)
	var entries []entry
	result := &RecoveryResult{}
	startedKnown := false
	for _, e := range records {
		switch e.op {
		case opBegin:
			if !startedKnown && e.hasTo {
				result.Started = e.to
				startedKnown = true
			}
		case opCommit:
			result.Committed = true
		default:
			entries = append(entries, fromJournal(root, e))
		}
	}
	var left []entry
	verb := "undo"
	if result.Committed {
		verb = "finish"
		err = opts.CommitLock()
		if err == nil {
			result.Errors = finish(fsys, root, entries)
		}
	} else {
		err = opts.DiscardLock()
		if err == nil {
			left, result.Errors = undo(fsys, root, entries)
		}
	}
	if err != nil {
		return nil, wrap(err, fmt.Sprintf("Cannot %s an interrupted lip operation on this"+
			" workspace. The journal '%s' is kept, so the next lip command that changes the"+
			" workspace tries again.", verb, shown))
	}
	if len(left) == 0 {
		err = fsys.Remove(journalPath)
	} else {
		var started *string
		if startedKnown {
			started = &result.Started
		}
		err = writeJournal(fsys, journalPath, root, left, result.Committed, started)
	}
	if err != nil {
		done, what := "undone", "updated"
		if result.Committed {
			done = "finished"
		}
		if len(left) == 0 {
			what = "deleted"
		}
		return nil, wrap(err, fmt.Sprintf("An interrupted lip operation on this workspace was %s,"+
			" but its journal '%s' cannot be %s. If a program is using the file, close it; check"+
			" that you may write to .lip; then try again.", done, shown, what))
	}
	result.Changes = len(entries)
	return result, nil
}

// CommitStaged is LockStore.CommitStaged without its tooth_lock.json
// migration: moves the staged state file <root>/.lip/state.json.tmp
// onto <root>/.lip/state.json in one rename, if it exists. root is a
// full path (LockStore.NormalizeRoot). The error is a *Error.
func CommitStaged(fsys FS, root string) error {
	path := filepath.Join(root, LipDirectory, WorkspaceFile)
	temp := filepath.Join(root, LipDirectory, StagedFile)
	if !fileExists(fsys, temp) {
		return nil
	}
	if err := fsys.Rename(temp, path); err != nil {
		return wrap(err, fmt.Sprintf("Cannot replace the state file '%s' with the new one '%s'.",
			path, temp))
	}
	return nil
}

// DiscardStaged is LockStore.DiscardStaged: deletes the staged workspace
// file <root>/.lip/state.json.tmp, if it exists. root is a full path.
// The error is a *Error.
func DiscardStaged(fsys FS, root string) error {
	temp := filepath.Join(root, LipDirectory, StagedFile)
	if !fileExists(fsys, temp) {
		return nil
	}
	// File.Delete does nothing for a missing file.
	if err := fsys.Remove(temp); err != nil && !notFound(err) {
		return wrap(err, fmt.Sprintf("Cannot delete the unused new state file '%s'.", temp))
	}
	return nil
}
