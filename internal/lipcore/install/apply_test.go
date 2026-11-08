package install

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Ports the tests of lip's McmodInstallerTests.cs and
// McmodInstallerRealFileSystemTests.cs added with deciding preserve and
// purge when a step runs (lip 91f5e3a, 8c9d597), checking the write rules
// at each write (186c6a7, 76be70f) and by long names (01fe78e).

// Preserve is decided when a file is written (protocol/workspace.md §4).

func TestInstallPreservedFileDeletedByPreInstallIsWrittenAndOwned(t *testing.T) {
	h := newHarness(t)
	h.addFile("cfg.ini", "user")
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg.ini", "cfg.ini")), Preserve: []string{"cfg.ini"},
		Hooks: pkgmodel.Hooks{PreInstall: []string{"reset"}}}, src("cfg.ini", "default"))
	h.onRun = func(string) { os.Remove(h.full("cfg.ini")) }

	owned := h.mustInstall(p, h.opts())

	eq(t, "cfg.ini", h.read("cfg.ini"), "default")
	eq(t, "owned", owned, []string{"cfg.ini"})
}

func TestInstallPreservedFileCreatedByPreInstallIsKeptAndNotOwned(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg.ini", "cfg.ini"), place("a.txt", "a.txt")),
		Preserve: []string{"cfg.ini"}, Hooks: pkgmodel.Hooks{PreInstall: []string{"init"}}},
		src("cfg.ini", "default", "a.txt", "a"))
	h.onRun = func(string) { h.addFile("cfg.ini", "hook") }

	owned := h.mustInstall(p, h.opts())

	eq(t, "cfg.ini", h.read("cfg.ini"), "hook")
	eq(t, "owned", owned, []string{"a.txt"})
}

func TestInstallPreservedFileDeletedByPreInstallFailureUndoesTheWrite(t *testing.T) {
	// The file written in place of the deleted one is undone; the deletion
	// by the hook command is not (protocol/workspace.md §6 rule 3).
	h := newHarness(t)
	h.addFile("cfg.ini", "user")
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg.ini", "cfg.ini")), Preserve: []string{"cfg.ini"},
		Hooks: pkgmodel.Hooks{PreInstall: []string{"reset"}, PostInstall: []string{"fail"}}},
		src("cfg.ini", "default"))
	h.onRun = func(command string) {
		if command == "reset" {
			os.Remove(h.full("cfg.ini"))
		}
	}
	h.fail["fail"] = true

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, "fail")
	eq(t, "cfg.ini", h.fileExists("cfg.ini"), false)
}

func TestApplyPurgeMatchingFileAnEarlierPackageWroteKeepsIt(t *testing.T) {
	// a's preserved config/a.json exists when planned, but its pre_install
	// deletes it, so a writes and owns it; b, uninstalled later in the
	// operation, purges config/. Whether another package owns a file is
	// decided when it is deleted (protocol/workspace.md §4, §6).
	for _, pattern := range []string{"config/*", "config/"} {
		t.Run(pattern, func(t *testing.T) {
			h := newHarness(t)
			h.addFile("config/a.json", "user")
			h.addFile("config/b.json", "b")
			a := h.pkgOf("example.com/a", "1.0.0", pkgmodel.Entry{
				Assets:   selfAsset(place("a.json", "config/a.json")),
				Preserve: []string{"config/a.json"},
				Hooks:    pkgmodel.Hooks{PreInstall: []string{"reset"}}}, src("a.json", "default"))
			h.onRun = func(string) { os.Remove(h.full("config/a.json")) }
			b := h.pkgOf("example.com/b", "1.0.0", pkgmodel.Entry{Purge: []string{pattern}}, nil)
			lockedB := locked(b, nil)
			opts := h.opts(lockedB)
			view := NewView(nil)
			install, err := h.inst.PlanInstall(context.Background(), view, opts, a)
			if err != nil {
				t.Fatal(err)
			}
			uninstall, err := h.inst.PlanUninstall(context.Background(), view, opts, &lockedB, b.Entry)
			if err != nil {
				t.Fatal(err)
			}
			h.installed = opts.Installed

			if err := h.apply(context.Background(), nil, install, uninstall); err != nil {
				t.Fatal(err)
			}

			eq(t, "config/a.json", h.read("config/a.json"), "default")
			eq(t, "config/b.json", h.fileExists("config/b.json"), false)
		})
	}
}

func TestUninstallPurgeMatchingWhatPreUninstallCreatedDeletesIt(t *testing.T) {
	// The paths purge matches are those that match when the step is carried
	// out, including paths that hook commands of the operation created
	// (protocol/workspace.md §6 rule 4).
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Purge: []string{"bedrock_server.exe", "logs/"},
		Hooks: pkgmodel.Hooks{PreUninstall: []string{"restore"}}}, nil)
	h.onRun = func(string) {
		h.addFile("bedrock_server.exe", "restored")
		h.addFile("logs/a.log", "log")
	}
	self := locked(p, nil)

	h.mustUninstall(&self, p.Entry, h.opts(self))

	eq(t, "exe", h.fileExists("bedrock_server.exe"), false)
	eq(t, "logs", h.dirExists("logs"), false)
}

func TestUninstallIgnoringScriptsPlansNoDeletionAfterHooks(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Purge: []string{"cache/"}}, nil)
	self := locked(p, nil)

	opts := h.opts(self)
	opts.IgnoreScripts = true
	plan, err := h.inst.PlanUninstall(context.Background(), NewView(nil), opts, &self, p.Entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range plan.Steps {
		if _, ok := s.(*DeleteMatchesStep); ok {
			t.Fatal("planned a DeleteMatchesStep without hook commands")
		}
	}
	if plan.MayChange(false) {
		t.Fatal("MayChange")
	}
}

func TestUninstallPurgeWithTrailingDotsInLiteralDirectoryMatchesNothing(t *testing.T) {
	// No workspace path has a name ending in '.', though Windows opens
	// "logs......" as "logs".
	h := newHarness(t)
	h.addFile("logs/a.log", "a")
	p := h.pkg(pkgmodel.Entry{Purge: []string{"logs....../*"}}, nil)
	self := locked(p, nil)

	h.mustUninstall(&self, p.Entry, h.opts(self))

	eq(t, "logs/a.log", h.read("logs/a.log"), "a")
}

// 8.3 short names (Windows): a path through the short name of a directory
// names what the long name does, so the write rules and purge see it by its
// long name (protocol/workspace.md §4).

func TestInstallDestThroughShortNameOfLipThrowsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	h.addFile(".lip/state.json", "keep")
	short := shortNameOrSkip(t, h.full(".lip"))
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("x.txt", short+"/state.json"))},
		src("x.txt", "x"))

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, ".lip/state.json")
	eq(t, "state", h.read(".lip/state.json"), "keep")
}

func TestInstallDestThroughShortNameOfFileOwnedByAnotherThrowsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	h.addFile("pluginsdirectory/foo.dll", "other")
	short := shortNameOrSkip(t, h.full("pluginsdirectory"))
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("foo.dll", short+"/foo.dll"))},
		src("foo.dll", "mine"))

	_, err := h.install(p, h.opts(locked(other, []string{"pluginsdirectory/foo.dll"})))

	wantInstallError(t, err, "pluginsdirectory/foo.dll (owned by example.com/other)")
	eq(t, "foo.dll", h.read("pluginsdirectory/foo.dll"), "other")
}

func TestInstallDestThroughShortNameOwnsTheLongName(t *testing.T) {
	h := newHarness(t)
	h.addDir("pluginsdirectory")
	short := shortNameOrSkip(t, h.full("pluginsdirectory"))
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("foo.dll", short+"/foo.dll"))},
		src("foo.dll", "mine"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", owned, []string{"pluginsdirectory/foo.dll"})
	eq(t, "foo.dll", h.read("pluginsdirectory/foo.dll"), "mine")
}

func TestUninstallPurgeThroughShortNameOfLipDeletesNothingInLip(t *testing.T) {
	h := newHarness(t)
	h.addFile(".lip/state.json", "keep")
	short := shortNameOrSkip(t, h.full(".lip"))
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Purge: []string{short + "/", short + "/*",
		short + "/state.json"}}, nil)
	self := locked(p, nil)

	h.mustUninstall(&self, p.Entry, h.opts(self))

	eq(t, "state", h.read(".lip/state.json"), "keep")
}

func TestApplyDestThroughShortNameOfDirectoryEarlierPackageCreatedThrows(t *testing.T) {
	// first creates pluginsdirectory in the same operation, after second was
	// planned, so the short name second writes through names the file of
	// first only when it is written.
	h := newHarness(t)
	directory := h.full("pluginsdirectory")
	short := shortNameOfNewDirectoryOrSkip(t, directory)
	first := h.pkgOf("example.com/first", "1.0.0", pkgmodel.Entry{
		Assets: selfAsset(place("foo.dll", "pluginsdirectory/foo.dll"))}, src("foo.dll", "first"))
	second := h.pkgOf("example.com/second", "1.0.0", pkgmodel.Entry{
		Assets: selfAsset(place("foo.dll", short+"/foo.dll"))}, src("foo.dll", "second"))
	view := NewView(nil)
	ctx := context.Background()
	firstPlan, err := h.inst.PlanInstall(ctx, view, h.opts(), first)
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := h.inst.PlanInstall(ctx, view, h.opts(), second)
	if err != nil {
		t.Fatal(err)
	}
	owners := h.inst.Owners(h.root, nil)
	tx, err := txn.Begin(h.root, txn.Options{Context: ctx,
		Reserved: append(firstPlan.WrittenPaths(), secondPlan.WrittenPaths()...)})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	defer tx.Rollback(func() error { return nil })
	if _, err := h.inst.Apply(ctx, tx, firstPlan, owners); err != nil {
		t.Fatal(err)
	}
	if shortName(directory) != short {
		t.Skip("The volume gave the directory another short name.")
	}

	_, err = h.inst.Apply(ctx, tx, secondPlan, owners)

	wantInstallError(t, err, "'pluginsdirectory/foo.dll', which belongs to example.com/first")
	eq(t, "foo.dll", h.read("pluginsdirectory/foo.dll"), "first")
}

func TestInstallDestThroughShortNameOfDirectoryPreInstallCreatedThrowsWhenOwned(t *testing.T) {
	// The file is owned by another package, and pre_install creates its
	// directory.
	h := newHarness(t)
	directory := h.full("pluginsdirectory")
	short := shortNameOfNewDirectoryOrSkip(t, directory)
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("foo.dll", short+"/foo.dll")),
		Hooks: pkgmodel.Hooks{PreInstall: []string{"mkdir pluginsdirectory"}}}, src("foo.dll", "mine"))
	h.onRun = func(string) { os.Mkdir(directory, 0o755) }

	_, err := h.install(p, h.opts(locked(other, []string{"pluginsdirectory/foo.dll"})))

	wantInstallError(t, err, "'pluginsdirectory/foo.dll', which belongs to example.com/other")
	eq(t, "foo.dll", h.fileExists("pluginsdirectory/foo.dll"), false)
}

func TestInstallDestThroughShortNameOfDirectoryPreInstallCreatedOwnsTheLongName(t *testing.T) {
	h := newHarness(t)
	directory := h.full("pluginsdirectory")
	short := shortNameOfNewDirectoryOrSkip(t, directory)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("foo.dll", short+"/foo.dll")),
		Hooks: pkgmodel.Hooks{PreInstall: []string{"mkdir pluginsdirectory"}}}, src("foo.dll", "mine"))
	h.onRun = func(string) { os.Mkdir(directory, 0o755) }

	owned := h.mustInstall(p, h.opts())

	if !h.fileExists("pluginsdirectory/foo.dll") {
		t.Skip("The volume gave the directory another short name.")
	}
	eq(t, "owned", owned, []string{"pluginsdirectory/foo.dll"})
}

func TestInstallDestThroughShortNameInPathLongerThanMaxPathThrowsWhenOwned(t *testing.T) {
	// The full path of the file is longer than MAX_PATH (260 characters).
	h := newHarness(t)
	prefix := strings.Repeat("d", 100) + "/" + strings.Repeat("e", 100) + "/"
	h.addFile(prefix+"pluginsdirectory/foo.dll", "other")
	if len(h.full(prefix+"pluginsdirectory/foo.dll")) <= 260 {
		t.Fatal("path not longer than MAX_PATH")
	}
	short := shortNameOrSkip(t, h.full(prefix+"pluginsdirectory"))
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{
		Assets: selfAsset(place("foo.dll", prefix+short+"/foo.dll"))}, src("foo.dll", "mine"))

	_, err := h.install(p, h.opts(locked(other, []string{prefix + "pluginsdirectory/foo.dll"})))

	wantInstallError(t, err, prefix+"pluginsdirectory/foo.dll (owned by example.com/other)")
	eq(t, "foo.dll", h.read(prefix+"pluginsdirectory/foo.dll"), "other")
}

// A file system that ignores case (Windows, macOS by default): a path
// spelled in another case names what the path does, so the write rules and
// purge see it as the file system spells it (protocol/workspace.md §4).

func TestInstallDestInLipSpelledInAnotherCaseThrowsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	skipUnlessIgnoresCase(t, h)
	h.addFile(".lip/state.json", "keep")
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("x.txt", ".LIP/state.json"))},
		src("x.txt", "x"))

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, "reserved")
	eq(t, "state", h.read(".lip/state.json"), "keep")
}

func TestInstallDestSpelledInAnotherCaseOfFileOwnedByAnotherThrowsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	skipUnlessIgnoresCase(t, h)
	h.addFile("mods/foo/a.dll", "other")
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("a.dll", "Mods/Foo/A.dll"))},
		src("a.dll", "mine"))

	_, err := h.install(p, h.opts(locked(other, []string{"mods/foo/a.dll"})))

	wantInstallError(t, err, "(owned by example.com/other)")
	eq(t, "a.dll", h.read("mods/foo/a.dll"), "other")
}

func TestUninstallPurgeNamingLipOrModsInAnotherCaseDeletesNothingThere(t *testing.T) {
	h := newHarness(t)
	skipUnlessIgnoresCase(t, h)
	h.addFile(".lip/state.json", "keep")
	h.addFile("mods/Other/other.dll", "dll")
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Purge: []string{".LIP/", ".LIP/*",
		".LIP/state.json", "MODS/"}}, nil)
	self := locked(p, nil)

	h.mustUninstall(&self, p.Entry, h.opts(self))

	eq(t, "state", h.read(".lip/state.json"), "keep")
	eq(t, "other.dll", h.read("mods/Other/other.dll"), "dll")
}

func TestInstallDestSpelledInAnotherCaseWhereCaseIsToldIsAnotherFile(t *testing.T) {
	// A file system that tells case apart has both files.
	h := newHarness(t)
	if ignoresCaseFS(h) {
		t.Skip("The file system ignores case.")
	}
	h.addFile("mods/foo/a.dll", "other")
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	p := h.pkgOf("example.com/x", "1.0.0", pkgmodel.Entry{Assets: selfAsset(place("a.dll", "Mods/foo/a.dll"))},
		src("a.dll", "mine"))

	owned := h.mustInstall(p, h.opts(locked(other, []string{"mods/foo/a.dll"})))

	eq(t, "owned", owned, []string{"Mods/foo/a.dll"})
	eq(t, "mods", h.read("mods/foo/a.dll"), "other")
	eq(t, "Mods", h.read("Mods/foo/a.dll"), "mine")
}

// ignoresCaseFS is whether the file system of the workspace ignores case.
func ignoresCaseFS(h *harness) bool {
	return dirExists(filepath.Join(filepath.Dir(h.root), strings.ToUpper(filepath.Base(h.root))))
}

func skipUnlessIgnoresCase(t *testing.T, h *harness) {
	t.Helper()
	if !ignoresCaseFS(h) {
		t.Skip("The file system tells case apart.")
	}
}

// shortNameOrSkip is the 8.3 short name of an existing directory; skips the
// test when it has none, as off Windows or on a volume that does not create
// them.
func shortNameOrSkip(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Only Windows has 8.3 short names.")
	}
	short := shortName(path)
	if short == "" || short == filepath.Base(path) {
		t.Skip("The volume does not create 8.3 short names.")
	}
	return short
}

// shortNameOfNewDirectoryOrSkip is the 8.3 short name a directory created at
// path gets, as found by creating it and deleting it again.
func shortNameOfNewDirectoryOrSkip(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	return shortNameOrSkip(t, path)
}

func TestInstallDamagedZipEntryFailsBeforeScripts(t *testing.T) {
	// The data of a.txt is damaged, which only extracting it reveals (lip
	// cfaa7a1).
	h := newHarness(t)
	z := zipBytes(t, "a.txt", strings.Repeat("content ", 100))
	corruptZipData(t, z, "a.txt")
	self, err := archive.NewZipBytesSource(z, "the ZIP archive")
	if err != nil {
		t.Fatal(err)
	}
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt")),
		Hooks: pkgmodel.Hooks{PreInstall: []string{"pre"}}}, self)

	_, err = h.install(p, h.opts())

	wantInstallError(t, err, "could not get the files of the package")
	if len(h.calls) != 0 || h.fileExists("a.txt") {
		t.Fatalf("calls %v, a.txt %v", h.calls, h.fileExists("a.txt"))
	}
}

// corruptZipData gives the first Deflate block of an entry the reserved
// block type.
func corruptZipData(t *testing.T, z []byte, name string) {
	t.Helper()
	for i := 0; i+30 <= len(z); i++ {
		if binary.LittleEndian.Uint32(z[i:]) == 0x04034b50 &&
			int(binary.LittleEndian.Uint16(z[i+26:])) == len(name) && string(z[i+30:i+30+len(name)]) == name {
			z[i+30+len(name)+int(binary.LittleEndian.Uint16(z[i+28:]))] = 0xFF
			return
		}
	}
	t.Fatalf("no local header for %s", name)
}
