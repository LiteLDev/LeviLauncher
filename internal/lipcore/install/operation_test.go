package install

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Ports of the installer parts of WorkspaceLayoutTests.cs,
// McmodServiceAtomicityTests.cs, McmodInstallerRealFileSystemTests.cs and
// CommandRunnerTests.cs, with operations driven as McmodService.Execute
// drives them: plan everything against one view, then apply every plan
// through one transaction.

// operation plans the uninstalls, then the installs, against one view with
// the layout, and applies them in one transaction.
func (h *harness) operation(ctx context.Context, uninstall []pkgmodel.LockedPackage,
	uninstallEntries []*pkgmodel.Entry, install []*Package, installed []pkgmodel.LockedPackage) error {
	h.t.Helper()
	layout, err := PlanLayout(h.root)
	if err != nil {
		return err
	}
	view := NewView(layout)
	remaining := append([]pkgmodel.LockedPackage(nil), installed...)
	var plans []*Plan
	for i := range uninstall {
		plan, err := h.inst.PlanUninstall(ctx, view, h.opts(remaining...), &uninstall[i], uninstallEntries[i])
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		for j := range remaining {
			if LockedIdentity(&remaining[j]) == LockedIdentity(&uninstall[i]) {
				remaining = append(remaining[:j], remaining[j+1:]...)
				break
			}
		}
	}
	for _, p := range install {
		plan, err := h.inst.PlanInstall(ctx, view, h.opts(remaining...), p)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		remaining = append(remaining, locked(p, plan.OwnedFiles))
	}
	return h.apply(ctx, layout, plans...)
}

func TestLayoutEmptyWorkspaceCreatesMods(t *testing.T) {
	h := newHarness(t)
	layout, err := PlanLayout(h.root)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "layout", *layout, LayoutPlan{Mods: filepath.Join(h.root, "mods"), CreateMods: true})
	eq(t, "dry run", layout.DryRunMessages(), []string{"Dry run: would create the directory mods/"})
	eq(t, "mods exists", h.dirExists("mods"), false)

	if err := h.apply(context.Background(), layout); err != nil {
		t.Fatal(err)
	}
	eq(t, "mods exists", h.dirExists("mods"), true)

	layout, err = PlanLayout(h.root)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "create", layout.CreateMods, false)
	eq(t, "dry run", len(layout.DryRunMessages()), 0)
}

func TestLayoutModsIsALinkThrowsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addDir("shared")
	if err := makeDirLink(h.full("shared"), h.full("mods")); err != nil {
		t.Skipf("cannot create a link: %v", err)
	}
	before := h.snapshot()

	_, err := PlanLayout(h.root)

	var le *LayoutError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(le.Msg, "The mods directory '"+h.full("mods")+"' is a link to '") ||
		!strings.HasSuffix(le.Msg, "', but lip only installs into a real directory, so that it never writes"+
			" outside the workspace. Replace the link with a directory (move the mods into it), then try again.") {
		t.Errorf("message %q", le.Msg)
	}
	h.assertUnchanged(before)
}

func TestLayoutFileNamedModsInTheWay(t *testing.T) {
	h := newHarness(t)
	h.addFile("mods", "x")
	layout, err := PlanLayout(h.root)
	if err != nil {
		t.Fatal(err)
	}

	err = h.apply(context.Background(), layout)

	var le *LayoutError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v", err)
	}
	eq(t, "message", le.Msg, "Cannot create the mods directory '"+h.full("mods")+"'. A file named mods is in"+
		" the way: move or delete it, then try again.")
}

func TestOperationInstallCreatesModsAndPluginsIsOrdinary(t *testing.T) {
	h := newHarness(t)
	h.addFile("plugins/Old/Old.dll", "old")
	mod := h.legacyMoney(pkgmodel.Entry{}, nil, "")
	lib := h.pkgOf("example.com/lib", "1.0.0",
		pkgmodel.Entry{Assets: selfAsset(place("file.dll", "plugins/lib/lib.dll"))}, src("file.dll", "lib"))

	if err := h.operation(context.Background(), nil, nil, []*Package{mod, lib}, nil); err != nil {
		t.Fatal(err)
	}

	eq(t, "dll", h.read("mods/LegacyMoney/LegacyMoney.dll"), "dll")
	eq(t, "old", h.read("plugins/Old/Old.dll"), "old")
	eq(t, "lib", h.read("plugins/lib/lib.dll"), "lib")
}

func TestOperationUninstallLastPackageKeepsMods(t *testing.T) {
	h := newHarness(t)
	mod := h.legacyMoney(pkgmodel.Entry{}, nil, "")
	l := locked(mod, h.mustInstall(mod, h.opts()))

	if err := h.operation(context.Background(), []pkgmodel.LockedPackage{l}, []*pkgmodel.Entry{mod.Entry},
		nil, []pkgmodel.LockedPackage{l}); err != nil {
		t.Fatal(err)
	}

	eq(t, "mods/LegacyMoney", h.dirExists("mods/LegacyMoney"), false)
	eq(t, "mods", h.dirExists("mods"), true)
}

func TestOperationSecondPackagePostInstallFailsUndoesFirstPackage(t *testing.T) {
	h := newHarness(t)
	base := h.pkgOf("example.com/base", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/Base/"))},
		src("Base.dll", "base"))
	baseLocked := locked(base, h.mustInstall(base, h.opts()))
	a := h.pkgOf("example.com/a", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/A/")),
		Hooks: pkgmodel.Hooks{PostInstall: []string{"a-post"}}}, src("A.dll", "a"))
	b := h.pkgOf("example.com/b", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/B/")),
		Hooks: pkgmodel.Hooks{PostInstall: []string{"fail"}}}, src("B.dll", "b"))
	h.fail["fail"] = true
	before := h.snapshot()

	err := h.operation(context.Background(), nil, nil, []*Package{a, b}, []pkgmodel.LockedPackage{baseLocked})

	wantInstallError(t, err, "command 'fail' failed")
	eq(t, "calls", h.calls, []hookCall{{"a-post", h.root}, {"fail", h.root}})
	h.assertUnchanged(before)
	eq(t, "journal", txn.HasJournal(h.root), false)
}

func TestOperationPreInstallFailsUndoesLayout(t *testing.T) {
	h := newHarness(t)
	h.addFile("plugins/Old/Old.dll", "old")
	a := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"fail"}}}, nil, "")
	h.fail["fail"] = true
	before := h.snapshot()

	err := h.operation(context.Background(), nil, nil, []*Package{a}, nil)

	wantInstallError(t, err, "In the pre_install hook")
	h.assertUnchanged(before)
}

// updateFailingAt installs Demo 1.0.0, then makes the update to Demo 2.0.0
// fail at the n-th file creation or move; returns whether the update failed.
func updateFailingAt(t *testing.T, n int) (*harness, bool) {
	h := newHarness(t)
	h.addFile("plugins/Old/Old.dll", "old")
	h.addFile("plugins/loose.txt", "loose")
	h.assets["https://example.com/demo-1.0.0.mcmod"] = src("manifest.json", `{"name":"Demo"}`,
		"Demo.dll", "dll 1", "data/a.txt", "a 1", "data/b.txt", "b 1")
	v1 := h.pkgOf("example.com/demo", "1.0.0", pkgmodel.Entry{
		Assets: []pkgmodel.Asset{{Type: pkgmodel.AssetMcmod, URLs: []string{"https://example.com/demo-1.0.0.mcmod"}}},
		Hooks:  pkgmodel.Hooks{PreUninstall: []string{"pre"}},
	}, nil)
	if err := h.operation(context.Background(), nil, nil, []*Package{v1}, nil); err != nil {
		t.Fatal(err)
	}
	lockedV1 := locked(v1, []string{"mods/Demo/manifest.json", "mods/Demo/Demo.dll", "mods/Demo/data/a.txt",
		"mods/Demo/data/b.txt"})
	h.addFile("mods/Demo/config.json", "user")
	h.assets["https://example.com/demo-2.0.0.mcmod"] = src("manifest.json", `{"name":"Demo"}`,
		"Demo.dll", "dll 2", "data/a.txt", "a 2", "new/c.txt", "c 2", "config.json", "default")
	v2 := h.pkgOf("example.com/demo", "2.0.0", pkgmodel.Entry{
		Assets:   []pkgmodel.Asset{{Type: pkgmodel.AssetMcmod, URLs: []string{"https://example.com/demo-2.0.0.mcmod"}}},
		Preserve: []string{"mods/Demo/config.json"},
	}, nil)
	before := h.snapshot()
	h.faults = newFaultyFS()
	h.faults.failNthCall(n)

	err := h.operation(context.Background(), []pkgmodel.LockedPackage{lockedV1}, []*pkgmodel.Entry{v1.Entry},
		[]*Package{v2}, []pkgmodel.LockedPackage{lockedV1})
	if err != nil {
		var ie *Error
		if !errors.As(err, &ie) {
			t.Fatalf("failure %d: err = %v (%T)", n, err, err)
		}
		h.assertUnchanged(before)
		eq(t, "journal", txn.HasJournal(h.root), false)
		return h, true
	}
	return h, false
}

func TestOperationUpdateEveryFileOperationFailingLeavesWorkspaceUnchanged(t *testing.T) {
	n := 1
	var h *harness
	for {
		var failed bool
		h, failed = updateFailingAt(t, n)
		if !failed {
			break
		}
		n++
	}

	if n <= 8 {
		t.Errorf("only %d failure points", n-1)
	}
	eq(t, "dll", h.read("mods/Demo/Demo.dll"), "dll 2")
	eq(t, "config", h.read("mods/Demo/config.json"), "user")
	eq(t, "b", h.fileExists("mods/Demo/data/b.txt"), false)
	_ = filepath.WalkDir(h.root, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".bak") {
			t.Errorf("backup left: %s", p)
		}
		return nil
	})
}

func TestOperationUninstallDeleteFailsRestoresOtherPackage(t *testing.T) {
	h := newHarness(t)
	a := h.pkgOf("example.com/a", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/A/"))}, src("A.dll", "a"))
	b := h.pkgOf("example.com/b", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/B/"))}, src("B.dll", "b"))
	if err := h.operation(context.Background(), nil, nil, []*Package{a, b}, nil); err != nil {
		t.Fatal(err)
	}
	la, lb := locked(a, []string{"mods/A/A.dll"}), locked(b, []string{"mods/B/B.dll"})
	before := h.snapshot()
	h.faults = newFaultyFS()
	h.faults.fail = func(op, path string) bool { return op == "move" && strings.HasSuffix(path, "A.dll") }

	// B is uninstalled first (reverse installation order), then A fails.
	err := h.operation(context.Background(), []pkgmodel.LockedPackage{lb, la},
		[]*pkgmodel.Entry{b.Entry, a.Entry}, nil, []pkgmodel.LockedPackage{la, lb})

	wantInstallError(t, err, "Cannot uninstall example.com/a: lip cannot delete 'mods/A/A.dll'.")
	h.assertUnchanged(before)
}

func TestOperationUninstallPostUninstallFailsRestoresDeletedFilesAndDirectories(t *testing.T) {
	h := newHarness(t)
	a := h.pkgOf("example.com/a", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/A/")),
		Hooks: pkgmodel.Hooks{PostUninstall: []string{"fail"}}}, src("A.dll", "a", "deep/dir/file.txt", "f"))
	la := locked(a, h.mustInstall(a, h.opts()))
	h.fail["fail"] = true
	before := h.snapshot()

	err := h.operation(context.Background(), []pkgmodel.LockedPackage{la}, []*pkgmodel.Entry{a.Entry}, nil,
		[]pkgmodel.LockedPackage{la})

	wantInstallError(t, err, "In the post_uninstall hook of example.com/a")
	h.assertUnchanged(before)
}

func TestOperationCancelledWhileApplyingUndoesChanges(t *testing.T) {
	h := newHarness(t)
	a := h.pkgOf("example.com/a", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/A/")),
		Hooks: pkgmodel.Hooks{PostInstall: []string{"a-post"}}}, src("A.dll", "a"))
	b := h.pkgOf("example.com/b", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("", "mods/B/"))}, src("B.dll", "b"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.onRun = func(command string) {
		if command == "a-post" {
			cancel()
		}
	}
	before := h.snapshot()

	err := h.operation(ctx, nil, nil, []*Package{a, b}, nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	var ie *Error
	if errors.As(err, &ie) {
		t.Errorf("cancellation wrapped: %v", err)
	}
	h.assertUnchanged(before)
}

func TestApplyCancelledHookCommandReturnsContextError(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.inst.RunCommand = func(ctx context.Context, dir, command string, _ io.Writer) error {
		cancel()
		return ctx.Err()
	}
	a := h.pkg(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"x"}}}, nil)
	plan, err := h.inst.PlanInstall(ctx, NewView(nil), h.opts(), a)
	if err != nil {
		t.Fatal(err)
	}

	err = h.apply(ctx, nil, plan)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanInstallAddsWritesToTheView(t *testing.T) {
	h := newHarness(t)
	view := NewView(nil)
	a := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "x/a.txt"))}, src("a.txt", "a"))
	plan, err := h.inst.PlanInstall(context.Background(), view, h.opts(), a)
	if err != nil {
		t.Fatal(err)
	}

	eq(t, "file", view.FileExists(h.full("x/a.txt")), true)
	eq(t, "dir", view.DirectoryExists(h.full("x")), true)
	entries, err := view.Enumerate(h.root)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "entries", entries, []Entry{{FullName: h.full("x"), IsDirectory: true}})
	eq(t, "written", plan.WrittenPaths(), []string{h.full("x/a.txt")})

	// A second package writing a file below x/a.txt finds a file in the way.
	b := h.pkgOf("example.com/b", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("b.txt", "x/a.txt/b.txt"))},
		src("b.txt", "b"))
	_, err = h.inst.PlanInstall(context.Background(), view, h.opts(), b)
	wantInstallError(t, err, "but 'x/a.txt' is a file, not a directory.")
}

func TestViewIsCaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("paths are case-sensitive here")
	}
	h := newHarness(t)
	h.addFile("Dir/File.txt", "x")
	view := NewView(nil)
	view.DeleteFile(h.full("DIR/FILE.TXT"))

	eq(t, "file", view.FileExists(h.full("dir/file.txt")), false)
	empty, err := view.IsEmpty(h.full("dir"))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "empty", empty, true)
}

// Progress.

func TestApplyReportsMeasuredProgress(t *testing.T) {
	h := newHarness(t)
	a := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("", "")),
		Hooks: pkgmodel.Hooks{PreInstall: []string{"one", "two"}}}, src("a.txt", "aaaa", "b.txt", "bb"))
	l := locked(a, h.mustInstall(a, h.opts()))

	var place, scripts []report.Progress
	for _, p := range h.rep.progress {
		switch p.Stage {
		case report.StagePlace:
			place = append(place, p)
		case report.StageScripts:
			scripts = append(scripts, p)
		}
	}
	last := place[len(place)-1]
	if !last.Done || last.Unit != report.UnitBytes || last.Current != 6 || last.Total != 6 ||
		last.Package != "example.com/pack@1.0.0" || last.Message != "Placing files" {
		t.Errorf("last place update %+v", last)
	}
	if place[0].Current != 0 || place[0].Done {
		t.Errorf("first place update %+v", place[0])
	}
	lastScript := scripts[len(scripts)-1]
	if !lastScript.Done || lastScript.Current != 2 || lastScript.Total != 2 ||
		lastScript.Unit != report.UnitCommands || lastScript.Message != "Running pre_install scripts" {
		t.Errorf("last scripts update %+v", lastScript)
	}

	h.rep.progress = nil
	h.mustUninstall(&l, a.Entry, h.opts())
	var remove []report.Progress
	for _, p := range h.rep.progress {
		if p.Stage == report.StageRemove {
			remove = append(remove, p)
		}
	}
	lastRemove := remove[len(remove)-1]
	if !lastRemove.Done || lastRemove.Current != 2 || lastRemove.Total != 2 || lastRemove.Unit != report.UnitFiles {
		t.Errorf("last remove update %+v", lastRemove)
	}
}

func TestApplyProgressCountsFilesWhenASizeIsUnknown(t *testing.T) {
	h := newHarness(t)
	s := src("a.txt", "aaaa", "b.txt", "bb")
	s.noSize = true
	a := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("", ""))}, s)
	h.mustInstall(a, h.opts())

	last := h.rep.progress[len(h.rep.progress)-1]
	if last.Stage != report.StagePlace || !last.Done || last.Unit != report.UnitFiles || last.Current != 2 ||
		last.Total != 2 {
		t.Errorf("last update %+v", last)
	}
}

// The real file system and the real command runner
// (McmodInstallerRealFileSystemTests.cs).
func TestInstallAndUninstallRealCommandRunnerRunsHooksInWorkspaceRoot(t *testing.T) {
	h := newHarness(t)
	h.inst.RunCommand = nil
	z, err := archive.NewZipBytesSource(zipBytes(t, "manifest.json", `{"name":"LegacyMoney"}`,
		"LegacyMoney.dll", "dll", "lang/en_US.json", "{}"), "LegacyMoney.mcmod")
	if err != nil {
		t.Fatal(err)
	}
	h.assets[legacyMoneyURL] = z
	self, err := archive.NewZipBytesSource(zipBytes(t, "lip.yaml", "{}"), "self.zip")
	if err != nil {
		t.Fatal(err)
	}
	pre, post, done := "echo pre> pre_install.txt", "echo post> post_install.txt", "echo done> post_uninstall.txt"
	if runtime.GOOS != "windows" {
		pre, post, done = "echo pre > pre_install.txt", "echo post > post_install.txt", "echo done > post_uninstall.txt"
	}
	p := h.pkgOf("example.com/legacymoney", "0.22.0", pkgmodel.Entry{
		Assets: []pkgmodel.Asset{{Type: pkgmodel.AssetMcmod, URLs: []string{legacyMoneyURL}}},
		Purge:  []string{"mods/LegacyMoney/data/"},
		Hooks: pkgmodel.Hooks{PreInstall: []string{pre}, PostInstall: []string{post},
			PostUninstall: []string{done}},
	}, self)

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned count", len(owned), 3)
	for _, f := range owned {
		if !strings.HasPrefix(f, "mods/LegacyMoney/") {
			t.Errorf("owned %s", f)
		}
	}
	eq(t, "dll", h.read("mods/LegacyMoney/LegacyMoney.dll"), "dll")
	eq(t, "pre", h.fileExists("pre_install.txt"), true)
	eq(t, "post", h.fileExists("post_install.txt"), true)

	h.addFile("mods/LegacyMoney/data/x/a.db", "a")
	l := locked(p, owned)
	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "mods/LegacyMoney", h.dirExists("mods/LegacyMoney"), false)
	eq(t, "mods", h.dirExists("mods"), true)
	eq(t, "done", h.fileExists("post_uninstall.txt"), true)
}

func TestCommandRunnerRunsInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	command := "echo x > marker"
	if runtime.GOOS == "windows" {
		command = "echo x> marker"
	}
	var out strings.Builder
	if err := DefaultCommandRunner(context.Background(), dir, "echo hello", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("output %q", out.String())
	}
	if err := DefaultCommandRunner(context.Background(), dir, command, nil); err != nil {
		t.Fatal(err)
	}
	eq(t, "marker", exists(filepath.Join(dir, "marker")), true)
}

func TestCommandRunnerMergesStderrAndReportsExitCode(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	err := DefaultCommandRunner(context.Background(), dir, "echo out&& echo err 1>&2&& exit 3", &out)
	if runtime.GOOS != "windows" {
		err = DefaultCommandRunner(context.Background(), dir, "echo out; echo err 1>&2; exit 3", &out)
	}

	var cf *CommandFailedError
	if !errors.As(err, &cf) {
		t.Fatalf("err = %v", err)
	}
	eq(t, "message", cf.Msg, "The command exited with code 3. Its output is shown above and in the trace of"+
		" the full report.")
	if cf.ExitCode == nil || *cf.ExitCode != 3 || cf.Dir != dir {
		t.Errorf("error %+v", cf)
	}
	if !strings.Contains(out.String(), "out") || !strings.Contains(out.String(), "err") {
		t.Errorf("output %q", out.String())
	}
}

func TestCommandRunnerCannotStart(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	err := DefaultCommandRunner(context.Background(), missing, "echo x", nil)

	var cf *CommandFailedError
	if !errors.As(err, &cf) {
		t.Fatalf("err = %v", err)
	}
	shell := "sh"
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
	}
	eq(t, "message", cf.Msg, "Could not start "+shell+" in '"+missing+"' to run the command.")
	if cf.ExitCode != nil {
		t.Errorf("exit code %v", *cf.ExitCode)
	}
}
