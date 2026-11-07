package txn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ports tests/Lip.Core.Tests/Mcmod/Lock/WorkspaceMutexTests.cs on the real
// file system, where the lock is enforced between handles as between
// processes.

func newLockRoot(t *testing.T) (string, string) {
	root := filepath.Join(t.TempDir(), "ws")
	must(t, os.MkdirAll(filepath.Join(root, LipDirectory), 0o777))
	return root, filepath.Join(root, LipDirectory, MutexFile)
}

func TestLock_WhileHeld_FailsBusy(t *testing.T) {
	root, _ := newLockRoot(t)
	held, err := Lock(root)
	must(t, err)
	defer held.Close()

	_, err = Lock(root)

	var busy *BusyError
	if !errors.As(err, &busy) ||
		!strings.Contains(err.Error(), "another lip command is changing the workspace '"+root+"'") {
		t.Fatal(err)
	}
}

func TestClose_Held_ReleasesAndKeepsTheFile(t *testing.T) {
	root, path := newLockRoot(t)
	held, err := Lock(root)
	must(t, err)

	must(t, held.Close())

	if !fileExists(OS, path) {
		t.Fatal("lock file deleted")
	}
	again, err := Lock(root)
	must(t, err)
	again.Close()
	if !fileExists(OS, path) {
		t.Fatal("lock file deleted")
	}
}

func TestLock_LeftoverFileOfKilledProcess_TakesItOver(t *testing.T) {
	root, path := newLockRoot(t)
	must(t, os.WriteFile(path, nil, 0o666))

	m, err := Lock(root)

	must(t, err)
	m.Close()
	if !fileExists(OS, path) {
		t.Fatal("lock file deleted")
	}
}

func TestLock_NoLipDirectory_FailsNotBusy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")

	_, err := Lock(root)

	var busy *BusyError
	if err == nil || errors.As(err, &busy) ||
		!strings.HasPrefix(err.Error(), "Cannot open the workspace lock file") {
		t.Fatal(err)
	}
}
