package txn

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Names of lip's own files in <root>/.lip (LockStore.cs).
const (
	LipDirectory  = ".lip"
	WorkspaceFile = "state.json"
	StagedFile    = "state.json.tmp"
	MutexFile     = "lock"
	JournalFile   = "journal"
)

// BusyError is WorkspaceBusyException: another process holds the workspace
// lock.
type BusyError struct {
	Path, Root string
	Err        error
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("lip cannot take the workspace lock '%s': most likely another lip command is"+
		" changing the workspace '%s'. Wait for the other lip command to finish,"+
		" then try again. If no other lip command is running, a program may be holding the"+
		" lock file open: close it, then try again.", e.Path, e.Root)
}

func (e *BusyError) Unwrap() error { return e.Err }

// Mutex is the workspace lock (WorkspaceMutex.cs, protocol/workspace.md
// §7.2): the file <root>/.lip/lock held open without sharing on Windows, and
// under an exclusive flock elsewhere. The file is never deleted. A killed
// process releases it with its handles.
type Mutex struct {
	f    *os.File
	path string
}

// Lock takes the workspace lock of root, or fails at once with *BusyError
// when another process holds it. The directory .lip must exist (EnsureLipDir).
func Lock(root string) (*Mutex, error) {
	root, err := normalizeRoot(root)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, LipDirectory, MutexFile)
	f, busy, err := acquire(path)
	if err != nil {
		if busy {
			return nil, &BusyError{Path: path, Root: root, Err: err}
		}
		return nil, wrap(err, fmt.Sprintf("Cannot open the workspace lock file '%s', which lip holds"+
			" while it changes the workspace.", path))
	}
	return &Mutex{f: f, path: path}, nil
}

// Path is the path of the lock file.
func (m *Mutex) Path() string { return m.path }

// Close releases the lock and keeps the file.
func (m *Mutex) Close() error {
	if m == nil || m.f == nil {
		return nil
	}
	err := m.f.Close()
	m.f = nil
	return err
}

// EnsureLipDir creates <root>/.lip when it is missing, with the hidden
// attribute on Windows (LockStore.CreateLipDirectory; failing to hide it is
// ignored).
func EnsureLipDir(root string) error { return EnsureLipDirFS(OS, root) }

// EnsureLipDirFS is EnsureLipDir on fsys. The error is a *Error, or the
// error of making root a full path.
func EnsureLipDirFS(fsys FS, root string) error {
	root, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, LipDirectory)
	if dirExists(fsys, dir) {
		return nil
	}
	if err := fsys.MkdirAll(dir); err != nil {
		return wrap(err, fmt.Sprintf("Cannot create the directory '%s', where lip keeps the workspace"+
			" file. Check that you may write to the workspace.", dir))
	}
	_ = fsys.SetHidden(dir, true) // A visible .lip works the same.
	return nil
}

// HasJournal reports whether root has the journal of an interrupted
// operation.
func HasJournal(root string) bool {
	root, err := normalizeRoot(root)
	if err != nil {
		return false
	}
	return fileExists(OS, filepath.Join(root, LipDirectory, JournalFile))
}

// normalizeRoot is LockStore.NormalizeRoot: the full path without a
// trailing separator.
func normalizeRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// notFound is FileNotFoundException or DirectoryNotFoundException.
func notFound(err error) bool { return errors.Is(err, os.ErrNotExist) }
