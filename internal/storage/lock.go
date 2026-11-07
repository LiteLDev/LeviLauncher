package storage

import (
	"os"
	"path/filepath"
	"sync"
)

// The migration lock keeps two launcher processes (the window and a
// command-line run, or two windows racing for the single-instance mutex)
// from migrating, or undoing a migration, at the same time. A process that
// decides the layout holds the lock exclusively while it migrates, then
// shared for as long as it runs: a later process that cannot take it
// exclusively knows that another one uses the layout, and leaves the files
// alone.
var (
	lockMu   sync.Mutex
	lockFile *os.File
	lockPath string
)

// acquireLock opens the lock file and takes it exclusively when no other
// process holds it. It reports whether it did; otherwise it waits until it
// holds the lock shared, that is until a migration in another process is
// done. A lock file that cannot be opened is not an error: the migration
// then runs unguarded, as the single-instance mutex still guards the
// window.
func acquireLock(path string) (bool, error) {
	lockMu.Lock()
	defer lockMu.Unlock()
	if lockFile != nil {
		// Prepare ran before in this process (tests): start over.
		unlockFile(lockFile)
		_ = lockFile.Close()
		lockFile = nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return true, nil
	}
	f, err := openLockFile(path)
	if err != nil {
		return true, nil
	}
	if lockFileNow(f, true) {
		lockFile, lockPath = f, path
		return true, nil
	}
	if !lockFileShared(f) {
		_ = f.Close()
		return false, nil
	}
	lockFile, lockPath = f, path
	return false, nil
}

// releaseToShared turns the exclusive lock into a shared one, held until the
// process ends.
func releaseToShared() {
	lockMu.Lock()
	defer lockMu.Unlock()
	if lockFile == nil {
		return
	}
	unlockFile(lockFile)
	if !lockFileShared(lockFile) {
		_ = lockFile.Close()
		lockFile = nil
	}
}

// releaseLock gives up the lock; Prepare takes it again. Tests use it to
// let another simulated process in, and to remove their directories.
func releaseLock() {
	lockMu.Lock()
	defer lockMu.Unlock()
	if lockFile != nil {
		unlockFile(lockFile)
		_ = lockFile.Close()
		lockFile = nil
	}
}
