package txn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The journal and recovery parts of McmodServiceAtomicityTests.cs, at the
// level of the transaction: every failure point of an operation leaves the
// workspace and its workspace file as they were, whether the operation rolls
// back itself or is killed and recovered by the next one.

// operation replaces, creates and deletes files, then commits a new
// workspace file. Returns the error of the first step that failed.
func operation(f *fixture, tx *Transaction) error {
	steps := []func() error{
		func() error { return tx.WriteFile("replace.txt", strings.NewReader("new")) },
		func() error { return tx.WriteFile("new/deep/created.txt", strings.NewReader("created")) },
		func() error { return tx.DeleteFile("delete.txt") },
		func() error { return tx.DeleteFile("dir/inner.txt") },
		func() error { return tx.DeleteDirectoryOnCommit("dir") },
		func() error { return tx.WriteFile("delete.txt", strings.NewReader("again")) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	_, err := tx.Commit(func() error {
		return os.WriteFile(f.full(".lip/workspace.yaml.tmp"), []byte("new"), 0o666)
	})
	return err
}

func newOperationFixture(t *testing.T) (*fixture, map[string]string) {
	f := newFixture(t)
	f.addFile(".lip/workspace.yaml", "old")
	return f, f.snapshot()
}

func TestOperation_EveryFileOperationFailing_LeavesWorkspaceAndWorkspaceFileUnchanged(t *testing.T) {
	n := 1
	for ; ; n++ {
		f, before := newOperationFixture(t)
		tx := f.begin(Options{})
		f.fs.failNthCall(n)

		err := operation(f, tx)

		if err == nil {
			if got := f.read(".lip/workspace.yaml"); got != "new" {
				t.Fatal(got)
			}
			break
		}
		injected := false
		for e := err; e != nil; e = errors.Unwrap(e) {
			injected = injected || strings.HasPrefix(e.Error(), "injected failure")
		}
		if !injected {
			t.Fatalf("call %d: %v", n, err)
		}
		if _, ok := err.(*RollbackError); !ok {
			if errs := tx.Rollback(nil); len(errs) != 0 {
				t.Fatalf("call %d: %v", n, errs)
			}
		}
		assertSnapshot(t, before, f.snapshot())
		if got := f.read(".lip/workspace.yaml"); got != "old" {
			t.Fatalf("call %d: workspace file %q", n, got)
		}
		if f.exists(".lip/journal") || f.exists(".lip/workspace.yaml.tmp") {
			t.Fatalf("call %d: journal or staged file left", n)
		}
	}
	// Three files created, three backups and the workspace file moved.
	if n-1 != 7 {
		t.Fatalf("%d failure points", n-1)
	}
}

func TestOperation_KilledAtEveryFailurePoint_RecoveryRestoresOrFinishes(t *testing.T) {
	for n := 1; ; n++ {
		f, before := newOperationFixture(t)
		tx := f.begin(Options{})
		f.fs.failNthCall(n)
		// A process killed instead of failing: the step stops, nothing is rolled back.
		killed := false
		f.fs.fail = func(fail func(string, string) bool) func(string, string) bool {
			return func(op, path string) bool {
				if fail(op, path) {
					killed = true
					panic("killed")
				}
				return false
			}
		}(f.fs.fail)
		func() {
			defer func() {
				if r := recover(); r != nil && r != "killed" {
					panic(r)
				}
			}()
			operation(f, tx)
		}()
		tx.Close()
		f.fs.fail = nil
		if !killed {
			break
		}

		r, err := RecoverWith(f.root, RecoverOptions{FS: f.fs})

		must(t, err)
		if r != nil && len(r.Errors) != 0 {
			t.Fatalf("call %d: %v", n, r.Errors)
		}
		if r != nil && r.Committed {
			if got := f.read(".lip/workspace.yaml"); got != "new" {
				t.Fatalf("call %d: workspace file %q", n, got)
			}
			if f.exists("dir") || f.exists(".replace.txt.bak") {
				t.Fatalf("call %d: not finished", n)
			}
		} else {
			assertSnapshot(t, before, f.snapshot())
			if got := f.read(".lip/workspace.yaml"); got != "old" {
				t.Fatalf("call %d: workspace file %q", n, got)
			}
		}
		if f.exists(".lip/journal") || f.exists(".lip/workspace.yaml.tmp") {
			t.Fatalf("call %d: journal or staged file left", n)
		}
	}
}

func TestOperation_KilledAfterCommitBeforeMove_RecoveryMovesWorkspaceFile(t *testing.T) {
	f, _ := newOperationFixture(t)
	tx := f.begin(Options{})
	change(t, tx)
	must(t, os.WriteFile(f.full(".lip/workspace.yaml.tmp"), []byte("new"), 0o666))
	must(t, tx.MarkCommitted())
	tx.Close()

	r, err := Recover(f.root)

	must(t, err)
	if !r.Committed || len(r.Errors) != 0 {
		t.Fatalf("%+v", r)
	}
	if got := f.read(".lip/workspace.yaml"); got != "new" {
		t.Fatal(got)
	}
	if f.exists(".lip/workspace.yaml.tmp") || f.exists(".lip/journal") || f.exists("dir") {
		t.Fatal("left over")
	}
}

func TestOperation_KilledBeforeCommit_RecoveryDiscardsStagedFile(t *testing.T) {
	f, before := newOperationFixture(t)
	tx := f.begin(Options{})
	change(t, tx)
	must(t, os.WriteFile(f.full(".lip/workspace.yaml.tmp"), []byte("new"), 0o666))
	tx.Close()
	// A cut-off last line, as when lip is killed while writing it.
	jf, err := os.OpenFile(f.journal(), os.O_APPEND|os.O_WRONLY, 0)
	must(t, err)
	jf.WriteString(`{"op":"CreateFi`)
	jf.Close()

	r, err := Recover(f.root)

	must(t, err)
	if r.Committed || r.Changes != 9 || len(r.Errors) != 0 {
		t.Fatalf("%+v", r)
	}
	assertSnapshot(t, before, f.snapshot())
	if got := f.read(".lip/workspace.yaml"); got != "old" {
		t.Fatal(got)
	}
	if f.exists(".lip/workspace.yaml.tmp") || f.exists(".lip/journal") {
		t.Fatal("left over")
	}
	if HasJournal(f.root) {
		t.Fatal("journal")
	}
}

func TestEnsureLipDir_CreatesHiddenDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	must(t, os.MkdirAll(root, 0o777))

	must(t, EnsureLipDir(root))

	if !dirExists(OS, filepath.Join(root, LipDirectory)) {
		t.Fatal("no .lip")
	}
	if hidden, _ := OS.Hidden(filepath.Join(root, LipDirectory)); !hidden {
		t.Fatal("not hidden")
	}
	must(t, EnsureLipDir(root))
}
