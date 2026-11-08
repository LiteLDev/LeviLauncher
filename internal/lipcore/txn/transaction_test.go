package txn

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Ports tests/Lip.Core.Tests/Mcmod/Install/WorkspaceTransactionTests.cs on
// the real file system.

type fixture struct {
	t    *testing.T
	root string
	fs   *faultyFS
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, root: filepath.Join(t.TempDir(), "ws"), fs: newFaultyFS()}
	f.addFile("keep.txt", "keep")
	f.addFile("replace.txt", "original")
	f.addFile("delete.txt", "deleted")
	f.addFile("dir/inner.txt", "inner")
	makeReadOnly(t, f.full("replace.txt"))
	makeHidden(t, f.full("delete.txt"))
	t.Cleanup(func() {
		// Read-only files cannot be deleted on Windows.
		filepath.WalkDir(f.root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				makeNormal(t, p)
			}
			return nil
		})
	})
	return f
}

func (f *fixture) full(rel string) string {
	return filepath.Join(f.root, filepath.FromSlash(rel))
}

func (f *fixture) journal() string { return f.full(".lip/journal") }

func (f *fixture) addFile(rel, content string) {
	f.t.Helper()
	p := f.full(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(rel string) string {
	f.t.Helper()
	b, err := os.ReadFile(f.full(rel))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}

func (f *fixture) exists(rel string) bool {
	_, err := os.Lstat(f.full(rel))
	return err == nil
}

// snapshot maps every path under the root but .lip to its content and
// attributes, or "<dir>".
func (f *fixture) snapshot() map[string]string {
	f.t.Helper()
	return snapshotOf(f.t, f.root)
}

func snapshotOf(t *testing.T, root string) map[string]string {
	t.Helper()
	s := map[string]string{}
	lip := filepath.Join(root, LipDirectory)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == lip {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			s[rel] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s[rel] = string(b) + " (" + attrString(t, p) + ")"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func assertSnapshot(t *testing.T, want, got map[string]string) {
	t.Helper()
	if !maps.Equal(want, got) {
		t.Fatalf("snapshot differs:\nwant %v\ngot  %v", sorted(want), sorted(got))
	}
}

func sorted(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	slices.Sort(out)
	return out
}

func (f *fixture) begin(opts Options) *Transaction {
	f.t.Helper()
	if opts.FS == nil {
		opts.FS = f.fs
	}
	tx, err := Begin(f.root, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { tx.Close() })
	return tx
}

func (f *fixture) recover(commit, discard func() error) *RecoveryResult {
	f.t.Helper()
	if commit == nil {
		commit = func() error { return nil }
	}
	if discard == nil {
		discard = func() error { return nil }
	}
	r, err := RecoverWith(f.root, RecoverOptions{FS: f.fs, CommitLock: commit, DiscardLock: discard})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// change replaces, creates and deletes files and directories, as an install
// over an uninstall does.
func change(t *testing.T, tx *Transaction) {
	t.Helper()
	must(t, tx.WriteFile("replace.txt", strings.NewReader("new")))
	must(t, tx.WriteFile("new/deep/created.txt", strings.NewReader("created")))
	must(t, tx.DeleteFile("delete.txt"))
	must(t, tx.DeleteFile("dir/inner.txt"))
	must(t, tx.DeleteDirectoryOnCommit("dir"))
	// Deleted, then written again.
	must(t, tx.WriteFile("delete.txt", strings.NewReader("again")))
}

func TestRollback_AfterChanges_RestoresEverythingAndDeletesJournal(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)

	errs := tx.Rollback(func() error { return nil })

	if len(errs) != 0 {
		t.Fatal(errs)
	}
	assertSnapshot(t, before, f.snapshot())
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestRollback_BackupDeletedByOthers_KeepsTheFileCreatedInItsPlace(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	must(t, tx.WriteFile("replace.txt", strings.NewReader("new")))
	makeNormal(t, f.full(".replace.txt.bak"))
	must(t, os.Remove(f.full(".replace.txt.bak")))

	errs := tx.Rollback(func() error { return nil })

	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if got := f.read("replace.txt"); got != "new" {
		t.Fatal(got)
	}
}

func TestWriteFile_ExistingFile_BacksItUpAsHiddenSibling(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})

	must(t, tx.WriteFile("replace.txt", strings.NewReader("new")))

	if got := f.read(".replace.txt.bak"); got != "original" {
		t.Fatal(got)
	}
	if hidden, _ := OS.Hidden(f.full(".replace.txt.bak")); hidden != true {
		t.Fatal("backup not hidden")
	}
	if hiddenBackups && attrString(t, f.full(".replace.txt.bak")) != "RH" {
		t.Fatal(attrString(t, f.full(".replace.txt.bak")))
	}
	if got := f.read("replace.txt"); got != "new" {
		t.Fatal(got)
	}
}

func TestComplete_AfterCommit_DeletesBackupsEmptiedDirectoriesAndJournal(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	change(t, tx)

	must(t, tx.MarkCommitted())
	errs := tx.Complete()

	if len(errs) != 0 {
		t.Fatal(errs)
	}
	assertSnapshot(t, map[string]string{
		".":                    "<dir>",
		"delete.txt":           "again ()",
		"keep.txt":             "keep ()",
		"new":                  "<dir>",
		"new/deep":             "<dir>",
		"new/deep/created.txt": "created ()",
		"replace.txt":          "new ()",
	}, f.snapshot())
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestRecover_KilledBeforeCommit_UndoesChangesAndDiscardsLock(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	tx.Close() // as by a killed process
	discarded := false

	r := f.recover(func() error { t.Fatal("committed"); return nil },
		func() error { discarded = true; return nil })

	if r == nil || r.Committed || r.Started == "" || len(r.Errors) != 0 || !discarded {
		t.Fatalf("%+v discarded=%v", r, discarded)
	}
	if r.Changes != 9 {
		t.Fatalf("changes %d", r.Changes)
	}
	assertSnapshot(t, before, f.snapshot())
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestJournal_Changes_RecordsPathsRelativeToWorkspace(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	change(t, tx)
	tx.Close()

	journal := f.read(".lip/journal")

	if strings.Contains(strings.ToLower(journal), strings.ToLower(jsonString(f.root))) ||
		strings.Contains(strings.ToLower(journal), strings.ToLower(filepath.Base(f.root))) {
		t.Fatal(journal)
	}
	for _, want := range []string{
		`{"op":"CreateFile","path":"new/deep/created.txt"}` + "\n",
		`{"op":"Backup","path":"replace.txt","to":".replace.txt.bak","hidden":false}` + "\n",
		`{"op":"CreateDirectory","path":"new"}` + "\n" + `{"op":"CreateDirectory","path":"new/deep"}` + "\n",
		`{"op":"DeleteDirectoryOnCommit","path":"dir"}` + "\n",
	} {
		if !strings.Contains(journal, want) {
			t.Fatalf("missing %q in\n%s", want, journal)
		}
	}
	wantHidden := `{"op":"Backup","path":"delete.txt","to":".delete.txt.bak","hidden":` +
		map[bool]string{true: "true", false: "false"}[hiddenBackups] + "}\n"
	if !strings.Contains(journal, wantHidden) {
		t.Fatalf("missing %q in\n%s", wantHidden, journal)
	}
}

func TestJournal_Begin_RecordsStartTimeAsDateTimeOffsetRoundTrip(t *testing.T) {
	f := newFixture(t)
	now := time.Date(2026, 11, 7, 12, 34, 56, 123456789, time.FixedZone("x", 3600))
	tx := f.begin(Options{Now: func() time.Time { return now }})
	must(t, tx.WriteFile("keep2.txt", strings.NewReader("k")))
	tx.Close()

	first := strings.SplitN(f.read(".lip/journal"), "\n", 2)[0]

	// '+' is escaped by the default encoder of System.Text.Json.
	if want := `{"op":"Begin","path":"","to":"2026-11-07T11:34:56.1234567` + `\` + `u002B00:00"}`; first != want {
		t.Fatalf("got %s", first)
	}
}

func TestJournal_Commit_IsARecordWithoutMembers(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	change(t, tx)

	must(t, tx.MarkCommitted())
	journal := f.read(".lip/journal")

	if !strings.HasSuffix(journal, `{"op":"Commit"}`+"\n") {
		t.Fatal(journal)
	}
	for _, l := range strings.Split(journal, "\n") {
		if strings.Contains(l, `"hidden"`) && !strings.HasPrefix(l, `{"op":"Backup",`) {
			t.Fatal(l)
		}
	}
}

func TestRecover_LineThatIsNoRecord_IsIgnored(t *testing.T) {
	for _, line := range []string{
		// A last line not ended by LF, as a killed process may leave.
		`{"op":"CreateFile","path":"keep.txt"}`,
		// No path.
		`{"op":"CreateFile"}` + "\n",
		// An op that is not a name of the table.
		`{"op":2,"path":"keep.txt"}` + "\n",
		`{"op":"createfile","path":"keep.txt"}` + "\n",
		// A Backup without a string to.
		`{"op":"Backup","path":"keep.txt","to":1}` + "\n",
		// Not an object.
		`["CreateFile","keep.txt"]` + "\n",
		// A Backup whose hidden is not a boolean.
		`{"op":"Backup","path":"keep.txt","to":".keep.txt.bak","hidden":1}` + "\n",
		// Not UTF-8, and trailing text.
		"{\"op\":\"CreateFile\",\"path\":\"keep\xff.txt\"}\n",
		`{"op":"CreateFile","path":"keep.txt"} x` + "\n",
	} {
		f := newFixture(t)
		before := f.snapshot()
		f.addFile(".lip/journal", `{"op":"Begin","path":""}`+"\n"+line)

		r := f.recover(nil, nil)

		if r.Changes != 0 || len(r.Errors) != 0 || r.Started != "" {
			t.Fatalf("%q: %+v", line, r)
		}
		assertSnapshot(t, before, f.snapshot())
		if fileExists(OS, f.journal()) {
			t.Fatal("journal kept")
		}
	}
}

func TestRecover_WorkspaceMovedAfterKill_UndoesChangesAtNewPath(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	tx.Close()
	moved := filepath.Join(filepath.Dir(f.root), "moved")
	must(t, os.Rename(f.root, moved))

	r, err := RecoverWith(moved, RecoverOptions{
		CommitLock: func() error { return nil }, DiscardLock: func() error { return nil }})

	must(t, err)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	must(t, os.Rename(moved, f.root))
	assertSnapshot(t, before, f.snapshot())
}

func TestRecover_KilledAfterCommit_CommitsLockAndDeletesBackups(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	change(t, tx)
	must(t, tx.MarkCommitted())
	tx.Close()
	committed := false

	r := f.recover(func() error { committed = true; return nil },
		func() error { t.Fatal("discarded"); return nil })

	if !r.Committed || !committed {
		t.Fatalf("%+v", r)
	}
	if got := f.read("delete.txt"); got != "again" {
		t.Fatal(got)
	}
	if f.exists("dir") {
		t.Fatal("dir kept")
	}
	for p := range f.snapshot() {
		if strings.Contains(p, ".bak") {
			t.Fatal(p)
		}
	}
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestRecover_JournalReplayedAfterRollback_ChangesNothing(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	data, err := os.ReadFile(f.journal())
	must(t, err)
	tx.Rollback(func() error { return nil })
	tx.Close()
	must(t, os.WriteFile(f.journal(), data, 0o666))

	r := f.recover(nil, nil)

	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	assertSnapshot(t, before, f.snapshot())
}

func TestRecover_ChangeCannotBeUndone_KeepsItInJournalForNextTime(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	tx.Close()
	created := f.full("new/deep/created.txt")
	f.fs.failOther = func(op, path string) bool { return op == "remove" && path == created }

	first := f.recover(nil, nil)
	kept := f.read(".lip/journal")
	f.fs.failOther = nil
	second := f.recover(nil, nil)

	if len(first.Errors) == 0 || !strings.Contains(first.Errors[0].Error(), "created.txt") {
		t.Fatal(first.Errors)
	}
	// The kept records: the file, and the directories above it.
	lines := strings.Split(strings.TrimSuffix(kept, "\n"), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], `{"op":"Begin","path":"","to":"`) ||
		lines[1] != `{"op":"CreateDirectory","path":"new"}` ||
		lines[2] != `{"op":"CreateDirectory","path":"new/deep"}` ||
		lines[3] != `{"op":"CreateFile","path":"new/deep/created.txt"}` {
		t.Fatalf("kept journal:\n%s", kept)
	}
	if len(second.Errors) != 0 || second.Changes != 3 {
		t.Fatalf("%+v", second)
	}
	assertSnapshot(t, before, f.snapshot())
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestRollback_AfterCommit_UndoesChangesAndUncommitsJournal(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	must(t, tx.MarkCommitted())
	journalWhenDiscarding := ""

	errs := tx.Rollback(func() error {
		journalWhenDiscarding = f.read(".lip/journal")
		return nil
	})

	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if journalWhenDiscarding == "" || strings.Contains(journalWhenDiscarding, `"op":"Commit"`) {
		t.Fatal(journalWhenDiscarding)
	}
	assertSnapshot(t, before, f.snapshot())
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}

func TestRollback_AfterCommitJournalCannotBeReplaced_LeavesItForCompletion(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	change(t, tx)
	must(t, tx.MarkCommitted())
	committed := f.snapshot()
	f.fs.failOther = func(op, path string) bool { return op == "writefile" }
	discarded := false

	errs := tx.Rollback(func() error { discarded = true; return nil })

	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "journal") || discarded {
		t.Fatalf("%v discarded=%v", errs, discarded)
	}
	assertSnapshot(t, committed, f.snapshot())
	if !strings.Contains(f.read(".lip/journal"), `"op":"Commit"`) {
		t.Fatal("uncommitted")
	}
}

func TestWriteFile_JournalOfAnotherOperationExists_FailsAndKeepsThatJournal(t *testing.T) {
	f := newFixture(t)
	f.addFile(".lip/journal", "{\"op\":\"Commit\",\"path\":\"\"}\n")
	before := f.snapshot()
	tx := f.begin(Options{})

	err := tx.WriteFile("created.txt", strings.NewReader("new"))
	tx.Rollback(func() error { return nil })

	var e *Error
	if !errors.As(err, &e) || !strings.HasPrefix(e.Msg, "Cannot create the journal '.lip/journal'") {
		t.Fatal(err)
	}
	assertSnapshot(t, before, f.snapshot())
	if got := f.read(".lip/journal"); got != "{\"op\":\"Commit\",\"path\":\"\"}\n" {
		t.Fatal(got)
	}
}

func TestAnyOpen_WhileATransactionIsOpen_IsTrue(t *testing.T) {
	f := newFixture(t)
	tx := f.begin(Options{})
	if !AnyOpen() {
		t.Fatal("not open")
	}
	tx.Close()
	tx.Close()
	if AnyOpen() {
		t.Fatal("still open")
	}
}

func TestRecover_NoJournal_ReturnsNil(t *testing.T) {
	f := newFixture(t)
	if r := f.recover(nil, nil); r != nil {
		t.Fatal(r)
	}
}

// Beyond the C# tests: the pinned Commit, backups of reserved names,
// progress, cancellation and the journal encoding.

func TestCommit_StagesRecordsAndMovesWorkspaceFileIntoPlace(t *testing.T) {
	f := newFixture(t)
	f.addFile(".lip/state.json", "old")
	tx := f.begin(Options{})
	change(t, tx)

	warnings, err := tx.Commit(func() error {
		return os.WriteFile(f.full(".lip/state.json.tmp"), []byte("new"), 0o666)
	})

	must(t, err)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if got := f.read(".lip/state.json"); got != "new" {
		t.Fatal(got)
	}
	if f.exists(".lip/state.json.tmp") || f.exists(".lip/journal") || f.exists("dir") {
		t.Fatal("left over")
	}
}

func TestCommit_WorkspaceFileCannotBeMoved_UncommitsAndUndoes(t *testing.T) {
	f := newFixture(t)
	f.addFile(".lip/state.json", "old")
	before := f.snapshot()
	tx := f.begin(Options{})
	change(t, tx)
	staged := f.full(".lip/state.json.tmp")
	f.fs.fail = func(op, path string) bool { return op == "move" && path == staged }

	_, err := tx.Commit(func() error { return os.WriteFile(staged, []byte("new"), 0o666) })

	var re *RollbackError
	if !errors.As(err, &re) || len(re.UndoErrors) != 0 ||
		!strings.HasPrefix(re.Error(), "Cannot replace the state file") {
		t.Fatal(err)
	}
	assertSnapshot(t, before, f.snapshot())
	if got := f.read(".lip/state.json"); got != "old" {
		t.Fatal(got)
	}
	if f.exists(".lip/state.json.tmp") || f.exists(".lip/journal") {
		t.Fatal("left over")
	}
}

func TestCommit_Cancelled_RollsBack(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	tx := f.begin(Options{Context: ctx})
	change(t, tx)
	cancel()

	_, err := tx.Commit(nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertSnapshot(t, before, f.snapshot())
}

func TestWriteFile_Cancelled_StopsCopying(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tx := f.begin(Options{Context: ctx})

	err := tx.WriteFile("x.txt", strings.NewReader("x"))

	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if errs := tx.Rollback(nil); len(errs) != 0 || f.exists("x.txt") {
		t.Fatal(errs)
	}
}

func TestWriteFile_Progress_ReportsBytesWritten(t *testing.T) {
	f := newFixture(t)
	var total int64
	calls := 0
	tx := f.begin(Options{Progress: func(n int64) { total += n; calls++ }})

	must(t, tx.WriteFile("big.bin", strings.NewReader(strings.Repeat("x", 200000))))

	if total != 200000 || calls != 3 {
		t.Fatalf("total %d in %d calls", total, calls)
	}
}

func TestBackup_NameTakenOrReserved_UsesNextNumber(t *testing.T) {
	f := newFixture(t)
	f.addFile(".keep.txt.bak", "someone's")
	tx := f.begin(Options{Reserved: []string{".keep.txt.bak1"}})

	must(t, tx.DeleteFile("keep.txt"))

	if got := f.read(".keep.txt.bak2"); got != "keep" {
		t.Fatal(got)
	}
	tx.Reserve(".replace.txt.bak")
	must(t, tx.DeleteFile("replace.txt"))
	if got := f.read(".replace.txt.bak1"); got != "original" {
		t.Fatal(got)
	}
}

func TestJSONString_EncodesLikeSystemTextJson(t *testing.T) {
	u := func(hex ...string) string {
		s := ""
		for _, h := range hex {
			s += `\` + "u" + h
		}
		return s
	}
	for in, want := range map[string]string{
		"a/b c.txt":                         `"a/b c.txt"`,
		`<&>'"+` + "`":                      `"` + u("003C", "0026", "003E", "0027", "0022", "002B", "0060") + `"`,
		string(rune(0xE9)) + "\x7f\x01\t\\": `"` + u("00E9", "007F", "0001") + `\t\\"`,
		string(rune(0x1F600)):               `"` + u("D83D", "DE00") + `"`,
	} {
		if got := jsonString(in); got != want {
			t.Errorf("%q: got %s want %s", in, got, want)
		}
	}
}

func TestParse_RoundTripsSerializedRecords(t *testing.T) {
	for _, e := range []entry{
		{op: opBegin, to: "t", hasTo: true},
		{op: opCreateDirectory, path: "a/ü<"},
		{op: opBackup, path: "a", to: ".a.bak", hasTo: true, hidden: true},
		{op: opDeleteDirectoryOnCommit, path: "d"},
		{op: opCommit},
	} {
		got, ok := parse([]byte(serialize(e)))
		if !ok || got != e {
			t.Errorf("%s: %+v", serialize(e), got)
		}
	}
}

func TestJournal_BackupOfNameStartingWithDot_IsNotHidden(t *testing.T) {
	// "hidden" is the Windows hidden attribute (protocol/workspace.md §7.3),
	// though off Windows .NET reports every name starting with '.' as hidden.
	f := newFixture(t)
	f.addFile(".env", "env")
	tx := f.begin(Options{})

	// Closed without commit or rollback, as by a killed process.
	must(t, tx.DeleteFile(".env"))
	tx.Close()

	want := `{"op":"Backup","path":".env","to":"..env.bak","hidden":false}` + "\n"
	if journal := f.read(".lip/journal"); !strings.Contains(journal, want) {
		t.Fatalf("journal:\n%s", journal)
	}
}

func TestRecover_NameWithBackslashOffWindows_RestoresIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`Windows names cannot contain '\'.`)
	}
	// Off Windows '\' is a character of a name, not a separator
	// (protocol/workspace.md §7.3: the paths of the journal are
	// '/'-separated).
	f := newFixture(t)
	f.addFile(`logs/a\b.txt`, "log")
	tx := f.begin(Options{})
	must(t, tx.DeleteFile(`logs/a\b.txt`))
	tx.Close()

	journal := f.read(".lip/journal")
	f.recover(nil, nil)

	if !strings.Contains(journal, `"path":"logs/a\\b.txt","to":"logs/.a\\b.txt.bak"`) {
		t.Fatalf("journal:\n%s", journal)
	}
	if got := f.read(`logs/a\b.txt`); got != "log" {
		t.Fatal(got)
	}
	if f.exists(`logs/.a\b.txt.bak`) {
		t.Fatal("backup kept")
	}
}

func TestRecover_ChangeCannotBeUndone_KeepsTheStartTime(t *testing.T) {
	// The created file cannot be deleted, so the first recovery rewrites the
	// journal.
	f := newFixture(t)
	clock := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	tx := f.begin(Options{Now: func() time.Time { clock = clock.Add(time.Second); return clock }})
	change(t, tx)
	tx.Close()
	begin := strings.SplitN(f.read(".lip/journal"), "\n", 2)[0]
	created := f.full("new/deep/created.txt")
	f.fs.failOther = func(op, path string) bool { return op == "remove" && path == created }

	first := f.recover(nil, nil)

	// "to" of "Begin" is when the operation started, not when the journal
	// was rewritten.
	if len(first.Errors) == 0 {
		t.Fatal("no errors")
	}
	if got := strings.SplitN(f.read(".lip/journal"), "\n", 2)[0]; got != begin {
		t.Fatalf("%s, want %s", got, begin)
	}
	f.fs.failOther = nil
}

func TestRollback_AfterCommitOfOnlyHookCommands_DeletesTheJournal(t *testing.T) {
	// An operation of only hook commands has a journal with no change
	// (§7.4); then the state file cannot be renamed into place (step 9).
	f := newFixture(t)
	tx := f.begin(Options{})
	must(t, tx.Begin())
	must(t, tx.MarkCommitted())

	errs := tx.Rollback(func() error { return nil })

	// Every record was undone, so the journal is deleted (§7.5).
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if fileExists(OS, f.journal()) {
		t.Fatal("journal kept")
	}
}
