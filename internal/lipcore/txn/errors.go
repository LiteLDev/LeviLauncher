package txn

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// Error is a file system failure with a message saying what lip was doing
// (WorkspaceErrors.Wrap); the failure is kept as its cause.
type Error struct {
	Msg string
	Err error
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Err }

func wrap(err error, msg string) error { return &Error{Msg: msg, Err: err} }

// ErrBug is the cause of the errors returned when lip misuses a transaction
// (InvalidOperationException in WorkspaceTransaction.cs).
var ErrBug = errors.New("txn: invalid use of a transaction")

func bug(msg string) error { return &Error{Msg: msg, Err: ErrBug} }

// RollbackError is returned by Transaction.Commit when the operation failed
// and was rolled back: Err is why it failed, UndoErrors what could not be
// undone (each one a warning for the user; the journal keeps those changes
// for the next operation).
type RollbackError struct {
	Err        error
	UndoErrors []error
}

func (e *RollbackError) Error() string { return e.Err.Error() }
func (e *RollbackError) Unwrap() error { return e.Err }

// reason is WorkspaceErrors.Reason: the message of an error as a sentence.
func reason(err error) string {
	msg := strings.TrimSpace(err.Error())
	if msg == "" || strings.HasSuffix(msg, ".") || strings.HasSuffix(msg, "!") ||
		strings.HasSuffix(msg, "?") {
		return msg
	}
	return msg + "."
}

// show is WorkspaceErrors.Show: a path relative to the workspace root and
// '/'-separated when it is in the workspace, else as it is.
func show(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
		rooted(rel) {
		return path
	}
	return strings.ReplaceAll(rel, `\`, "/")
}

// pathKey makes paths compare as WorkspaceLayout.PathComparer does:
// ignoring case on Windows, ordinal elsewhere.
func pathKey(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(p)
	}
	return p
}

// describe is JournalEntry.Describe.
func describe(root string, e entry) string {
	switch e.op {
	case opCreateFile:
		return fmt.Sprintf("writing '%s'", show(root, e.path))
	case opBackup:
		return fmt.Sprintf("replacing or deleting '%s' (the original file is kept as '%s'; rename it"+
			" back to restore it)", show(root, e.path), show(root, e.to))
	case opCreateDirectory:
		return fmt.Sprintf("creating the directory '%s'", show(root, e.path))
	default:
		return fmt.Sprintf("%s '%s'", e.op, show(root, e.path))
	}
}

// setHiddenWrapped is FileSystemExtensions.SetHidden with its message.
func setHiddenWrapped(fsys FS, path string, hidden bool) error {
	if err := fsys.SetHidden(path, hidden); err != nil {
		verb := "unhide"
		if hidden {
			verb = "hide"
		}
		return wrap(err, fmt.Sprintf("Could not %s %s (set its hidden attribute).", verb, path))
	}
	return nil
}
