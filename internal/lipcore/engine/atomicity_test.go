package engine

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Ports of McmodServiceAtomicityTests.cs (lip v0.38.0-beta.1): every
// operation is all or nothing (protocol/workspace.md §6, §7).

// addModZip writes <name>.zip, a package example.com/<name> whose asset of
// type "mcmod" is https://example.com/<name>.mcmod; extra holds members of
// the entry, each followed by a comma.
func (h *harness) addModZip(name, extra string) {
	url := "https://example.com/" + name + ".mcmod"
	h.addDownload(url, mcmod(name))
	h.addZip(strings.ToLower(name)+".zip", "lip.yaml", lipYAML("example.com/"+strings.ToLower(name), "1.0.0",
		extra+` "assets": [ { "type": "mcmod", "urls": [ "`+url+`" ] } ]`))
}

func hooks(stage, command string) string {
	return `"hooks": { "` + stage + `": [ "` + command + `" ] },`
}

func TestInstallAssetCannotBeDownloadedChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.addZip("x.zip", "lip.yaml", lipYAML("example.com/x", "1.0.0",
		`"assets": [ { "type": "zip", "urls": [ "https://example.com/missing.zip" ], "place": [ { "src": "a", "dest": "a" } ] } ]`))
	before := h.snapshot()
	err := h.install("x.zip")
	var ie *install.Error
	if !errors.As(err, &ie) || !strings.Contains(err.Error(), "could not obtain asset 0") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(FormatError(err), "https://example.com/missing.zip") {
		t.Fatal(FormatError(err))
	}
	after := h.snapshot()
	delete(after, ".lip")
	equalMaps(t, after, before)
}

func TestInstallSecondPackagePostInstallFailsUndoesFirstPackageAndKeepsWorkspaceFile(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.install("base.zip"))
	h.addModZip("A", hooks("post_install", "a-post"))
	h.addModZip("B", hooks("post_install", "fail"))
	h.failing["fail"] = true
	before := h.snapshot()

	err := h.install("a.zip", "b.zip")
	var cf *install.CommandFailedError
	if !errors.As(err, &cf) {
		t.Fatalf("err = %v", err)
	}
	if h.ran("a-post") != 1 {
		t.Fatal("a-post did not run")
	}
	equalMaps(t, h.snapshot(), before)
	if txn.HasJournal(h.root) {
		t.Fatal("journal left")
	}
	if !h.rec.hasWarning("The operation failed; undoing its changes to the workspace.") {
		t.Fatal(h.rec.warnings)
	}
	if !strings.Contains(FormatError(err), "hint: A hook command of a package failed.") {
		t.Fatal(FormatError(err))
	}
}

func TestInstallPreInstallFailsUndoesLayout(t *testing.T) {
	h := newHarness(t)
	h.addModZip("A", hooks("pre_install", "fail"))
	h.failing["fail"] = true
	before := h.snapshot()
	if err := h.install("a.zip"); err == nil {
		t.Fatal("installed")
	}
	after := h.snapshot()
	delete(after, ".lip")
	equalMaps(t, after, before)
	if h.hasLock() {
		t.Fatal("workspace file written")
	}
}

func TestInstallCancelledWhileApplyingUndoesChanges(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.install("base.zip"))
	h.addModZip("A", hooks("post_install", "a-post"))
	h.addModZip("B", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.e.inst.RunCommand = func(_ context.Context, _, command string, _ io.Writer) error {
		if command == "a-post" {
			cancel()
		}
		return nil
	}
	before := h.snapshot()
	err := h.e.Install(ctx, []string{"a.zip", "b.zip"}, false, false, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	equalMaps(t, h.snapshot(), before)
	if !h.rec.hasWarning("Cancelled; undoing its changes to the workspace.") {
		t.Fatal(h.rec.warnings)
	}
}

func TestOperationAfterInterruptedOperationUndoesItFirst(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.install("base.zip"))
	before := h.snapshot()
	killed, err := txn.Begin(h.root, txn.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h.must(killed.WriteFile("mods/Base/Base.dll", strings.NewReader("half")))
	h.must(killed.WriteFile("mods/New/a/b.txt", strings.NewReader("")))
	h.must(killed.DeleteFile("mods/Base/manifest.json"))
	h.must(pkgfile.StageWorkspace(h.root, &pkgmodel.LockFile{Format: 4, Platform: "server", Runtime: "win-x64"}))
	killed.Close()
	f, err := os.OpenFile(filepath.Join(h.root, ".lip", "journal"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"op":"CreateFi`)
	f.Close()

	if err := h.uninstall("example.com/missing"); err == nil {
		t.Fatal("uninstalled")
	}
	equalMaps(t, h.snapshot(), before)
	equal(t, h.summary(), []string{"example.com/base@1.0.0 explicit"})
	if !h.rec.hasWarning("was interrupted") || !h.rec.hasWarning("undid its") {
		t.Fatal(h.rec.warnings)
	}
}

func TestOperationAfterInterruptedCommitFinishesIt(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.install("base.zip"))
	killed, err := txn.Begin(h.root, txn.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h.must(killed.DeleteFile("mods/Base/Base.dll"))
	h.must(killed.DeleteFile("mods/Base/manifest.json"))
	h.must(killed.DeleteDirectoryOnCommit("mods/Base"))
	lf := h.lock()
	lf.Packages = nil
	h.must(pkgfile.StageWorkspace(h.root, lf))
	h.must(killed.MarkCommitted())
	killed.Close()

	if err := h.uninstall("example.com/base"); err == nil {
		t.Fatal("uninstalled again")
	}
	if len(h.lock().Packages) != 0 {
		t.Fatal("not finished")
	}
	if _, err := os.Stat(h.path("mods/Base")); err == nil {
		t.Fatal("mods/Base left")
	}
	if _, err := os.Stat(h.path("mods")); err != nil {
		t.Fatal("mods deleted")
	}
	if txn.HasJournal(h.root) {
		t.Fatal("journal left")
	}
	if !h.rec.hasInfo("Finished an interrupted lip operation that had completed") {
		t.Fatal(h.rec.infos)
	}
}

func TestDryRunAfterInterruptedOperationWarnsAndLeavesIt(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.install("base.zip"))
	killed, err := txn.Begin(h.root, txn.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h.must(killed.DeleteFile("mods/Base/Base.dll"))
	killed.Close()
	before := h.snapshot()
	h.must(h.e.Uninstall(context.Background(), []string{"example.com/base"}, true, false, false))
	equalMaps(t, h.snapshot(), before)
	if !h.rec.hasWarning("interrupted") {
		t.Fatal(h.rec.warnings)
	}
}

func TestDryRunNewWorkspaceCreatesNoLipDirectory(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Base", "")
	h.must(h.e.Install(context.Background(), []string{"base.zip"}, true, false, false))
	for _, d := range []string{".lip", "mods"} {
		if _, err := os.Stat(h.path(d)); err == nil {
			t.Fatalf("%s created", d)
		}
	}
	if !h.rec.hasInfo("Dry run: would") {
		t.Fatal(h.rec.infos)
	}
}

func TestInstallSucceedsLeavesNoBackupJournalOrStagedFile(t *testing.T) {
	h := newHarness(t)
	h.addModZip("Demo", "")
	h.addFile("mods/Demo/Demo.dll", "unowned")
	h.must(h.install("demo.zip"))
	if h.read("mods/Demo/Demo.dll") != "Demo binary" {
		t.Fatal(h.read("mods/Demo/Demo.dll"))
	}
	entries, err := os.ReadDir(h.path(".lip"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	equal(t, names, []string{"lock", "workspace.yaml"})
	for k := range h.snapshot() {
		if strings.Contains(k, ".bak") {
			t.Fatalf("backup left: %s", k)
		}
	}
}

func TestWorkspaceLockHeldFailsAtOnce(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.must(txn.EnsureLipDir(h.root))
	m, err := txn.Lock(h.root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	var busy *txn.BusyError
	if err := h.install(libID); !errors.As(err, &busy) {
		t.Fatalf("err = %v", err)
	}
}
