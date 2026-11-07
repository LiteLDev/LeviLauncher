//go:build windows

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// privateMutex points the migration lock at a name only this test uses, so
// a launcher running on the machine is never blocked by the tests.
func privateMutex(t *testing.T) {
	t.Helper()
	old := migrationMutexName
	migrationMutexName = fmt.Sprintf(`Local\LeviLauncher_StorageMigration_test_%d_%s`, os.Getpid(), t.Name())
	t.Cleanup(func() { migrationMutexName = old })
}

// holdLock takes the migration lock on another thread, as another process
// would, until release is closed; done closes once it is released.
func holdLock(t *testing.T) (release chan struct{}, done chan struct{}) {
	t.Helper()
	held := make(chan bool)
	release, done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		unlock, ok := lockMigration(-1)
		held <- ok
		<-release
		if ok {
			unlock()
		}
	}()
	if !<-held {
		t.Fatal("lockMigration(-1) failed")
	}
	return release, done
}

func TestPrepareSharedNextToWindowUsesLegacyLayout(t *testing.T) {
	privateMutex(t)
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)

	outcome, err := PrepareShared(func() bool { return true })
	if outcome != Legacy || !errors.Is(err, errMigrationBusy) {
		t.Fatalf("PrepareShared = %v, %v; want Legacy, errMigrationBusy", outcome, err)
	}
	if apppath.LegacyMode() != e.legacy {
		t.Fatalf("legacy mode = %q, want %q", apppath.LegacyMode(), e.legacy)
	}
	if !exists(filepath.Join(e.legacy, "config.json")) || exists(apppath.MigrationJournalPath()) {
		t.Fatal("PrepareShared must not migrate while a window runs")
	}
}

func TestPrepareSharedWithoutLegacyDataNextToWindow(t *testing.T) {
	privateMutex(t)
	setup(t)
	if outcome, err := PrepareShared(func() bool { return true }); outcome != Current || err != nil {
		t.Fatalf("PrepareShared = %v, %v; want Current, nil", outcome, err)
	}
}

func TestPrepareSharedMigratesAlone(t *testing.T) {
	privateMutex(t)
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	if outcome, err := PrepareShared(func() bool { return false }); outcome != Migrated || err != nil {
		t.Fatalf("PrepareShared = %v, %v; want Migrated, nil", outcome, err)
	}
}

func TestPrepareSharedGivesUpWhileAnotherProcessMigrates(t *testing.T) {
	privateMutex(t)
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	old := sharedLockWait
	sharedLockWait = 50 * time.Millisecond
	t.Cleanup(func() { sharedLockWait = old })

	release, done := holdLock(t)
	outcome, err := PrepareShared(func() bool { return false })
	close(release)
	<-done
	if outcome != Legacy || !errors.Is(err, errMigrationBusy) {
		t.Fatalf("PrepareShared = %v, %v; want Legacy, errMigrationBusy", outcome, err)
	}
	if !exists(filepath.Join(e.legacy, "config.json")) {
		t.Fatal("PrepareShared must not migrate while another process holds the lock")
	}
}

func TestPrepareSharedWaitsForAnotherProcessToFinish(t *testing.T) {
	privateMutex(t)
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)

	release, done := holdLock(t)
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()
	outcome, err := PrepareShared(func() bool { return false })
	<-done
	if outcome != Migrated || err != nil {
		t.Fatalf("PrepareShared = %v, %v; want Migrated, nil", outcome, err)
	}
}

func TestLockFailureWithJournalRefusesToStart(t *testing.T) {
	privateMutex(t)
	setup(t)
	old := sharedLockWait
	sharedLockWait = 0
	t.Cleanup(func() { sharedLockWait = old })
	write(t, apppath.MigrationJournalPath(), `{}`)

	release, done := holdLock(t)
	outcome, err := PrepareShared(nil)
	close(release)
	<-done
	if outcome != Current || !errors.Is(err, errMigrationLock) {
		t.Fatalf("PrepareShared = %v, %v; want Current, errMigrationLock", outcome, err)
	}
}
