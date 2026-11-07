// Package txn ports lip 0.38's workspace transaction, journal and lock
// (src/Lip.Core/Mcmod/Install/WorkspaceTransaction.cs,
// src/Lip.Core/Mcmod/Lock/WorkspaceMutex.cs and the .lip helpers of
// LockStore.cs; protocol/workspace.md §6, §7).
//
// An operation applies all of its file changes through one Transaction, and
// either commits them or rolls all of them back:
//
//   - A created directory or file is recorded, and deleted again on rollback.
//   - A file that is replaced or deleted is first renamed to a backup next to
//     it (".<name>.bak", or ".<name>.bak1", ... when that name is taken or
//     reserved). Rollback renames it back; commit deletes it. On Windows the
//     backup gets the hidden attribute, and the original's is restored.
//   - A directory that empties is deleted only at commit, if empty then.
//
// Every change is appended to the journal <root>/.lip/journal (one JSON
// object per line, LF-terminated, paths relative to the root and
// '/'-separated) and handed to the operating system before it is made, so
// that a later process can undo the changes of one that was killed
// (Recover). After "Commit" is recorded, recovery finishes the operation
// (deleting the backups) instead of undoing it.
//
// API (names pinned for the installer):
//
//	EnsureLipDir(root) error           create <root>/.lip (hidden on Windows)
//	Lock(root) (*Mutex, error)         take <root>/.lip/lock; *BusyError when held
//	(*Mutex).Close()                   release it (the file is kept)
//	HasJournal(root) bool              an interrupted operation is pending
//	Recover(root) / RecoverWith(...)   finish or undo it; needs the lock
//	Begin(root, Options) (*Transaction, error)
//	(*Transaction).CreateDirectory(rel), WriteFile(rel, r), DeleteFile(rel),
//	  DeleteDirectoryOnCommit(rel), Reserve(rel)
//	(*Transaction).Commit(writeStaged) ([]error, error)
//	                                   stage .lip/workspace.yaml.tmp, record
//	                                   Commit, move it into place, complete;
//	                                   rolls back and returns *RollbackError
//	                                   on failure
//	(*Transaction).Rollback(discardLock) []error
//	(*Transaction).MarkCommitted, Complete
//	                                   the two halves of Commit, as in C#
//	(*Transaction).Close()             leaves the journal unless completed or
//	                                   rolled back (as a killed process)
//
// WriteFile takes no size: Options.Progress receives every chunk written, for
// measured progress. Cancellation is Options.Context; the caller rolls back
// on the error. The FS interface is the seam for fault-injection tests.
package txn
