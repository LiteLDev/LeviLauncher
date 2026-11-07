package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// Ports of tests/Lip.Core.Tests/Mcmod/Install/McmodInstallerTests.cs.

func eq[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// Step 2: "self" and "zip" placements.

func TestInstallFileFormWritesOnlyThePlacedFile(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("PeEditor.exe", "PeEditor.exe"))},
		src("lip.yaml", "{}", "PeEditor.exe", "exe", "README.md", "readme"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", owned, []string{"PeEditor.exe"})
	eq(t, "PeEditor.exe", h.read("PeEditor.exe"), "exe")
	eq(t, "lip.yaml exists", h.fileExists("lip.yaml"), false)
	eq(t, "README.md exists", h.fileExists("README.md"), false)
}

func TestInstallFileFormNestedWritesAtDest(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("bin/sub/a.dll", "tools/x.dll"))},
		src("bin/sub/a.dll", "a"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", owned, []string{"tools/x.dll"})
	eq(t, "tools/x.dll", h.read("tools/x.dll"), "a")
}

func TestInstallDirectoryFormWritesFilesAtTheirPathRelativeToSrc(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("assets/", "resource_packs/pack/"), place("", "copy/"))},
		src("lip.yaml", "{}", "assets/a.txt", "a", "assets/sub/b.txt", "b", "assets/lip.yaml", "nested"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "a", h.read("resource_packs/pack/a.txt"), "a")
	eq(t, "b", h.read("resource_packs/pack/sub/b.txt"), "b")
	eq(t, "nested", h.read("resource_packs/pack/lip.yaml"), "nested")
	eq(t, "copy a", h.read("copy/assets/a.txt"), "a")
	eq(t, "copy nested", h.read("copy/assets/lip.yaml"), "nested")
	// The package file of the package is never written (protocol/workspace.md §4 step 2).
	eq(t, "copy/lip.yaml exists", h.fileExists("copy/lip.yaml"), false)
	eq(t, "owned count", len(owned), 6)
}

func TestInstallDirectoryFormDestRootWritesIntoWorkspaceRoot(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("defaults/", ""))},
		src("defaults/server.properties", "props", "defaults/a/b.txt", "b"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", sorted(owned), []string{"a/b.txt", "server.properties"})
	eq(t, "props", h.read("server.properties"), "props")
}

func TestInstallGlobFormKeepsPathsBelowTheFixedPrefix(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("bin/**/*.dll", "tools/pack/"))},
		src("bin/a.dll", "a", "bin/x/b.dll", "b", "bin/c.pdb", "c", "other/d.dll", "d"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", sorted(owned), []string{"tools/pack/a.dll", "tools/pack/x/b.dll"})
	eq(t, "b", h.read("tools/pack/x/b.dll"), "b")
}

func TestInstallGlobFormFixedPrefixPartOfSegmentIsNotStripped(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("lib/a*/x.txt", "out/"))}, src("lib/ab/x.txt", "x"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"out/ab/x.txt"})
}

func TestInstallGlobFormMatchingNothingWritesNothing(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("*.dll", ""))}, src("a.txt", "a"))
	before := h.snapshot()

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", len(owned), 0)
	h.assertUnchanged(before)
}

func TestInstallGlobFormMatchingPackageFileNeverWritesIt(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("*.yaml", "cfg/"))}, src("lip.yaml", "{}", "a.yaml", "a"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"cfg/a.yaml"})
	eq(t, "cfg/lip.yaml exists", h.fileExists("cfg/lip.yaml"), false)
}

func TestInstallToothJsonBesidePackageFileIsWritten(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("*.json", "cfg/"))},
		src("lip.yaml", "{}", "tooth.json", "{}", "a.json", "a"))

	eq(t, "owned", sorted(h.mustInstall(p, h.opts())), []string{"cfg/a.json", "cfg/tooth.json"})
}

func TestInstallToothJsonOfArchiveWithoutPackageFileIsNeverWritten(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("*.json", "cfg/"))}, src("tooth.json", "{}", "a.json", "a"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"cfg/a.json"})
	eq(t, "cfg/tooth.json exists", h.fileExists("cfg/tooth.json"), false)
}

func TestInstallSrcNotInArchiveThrowsWithoutChanges(t *testing.T) {
	cases := []struct{ src, dest, msg string }{
		{"missing.txt", "out.txt", "places the file 'missing.txt' of asset 0, but the asset has no such file"},
		{"missing/", "out/", "places the directory 'missing/' of asset 0, but the asset has no file below it"},
		{"a.txt/", "out/", "places the directory 'a.txt/'"},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt"), place(c.src, c.dest))},
				src("a.txt", "a"))
			before := h.snapshot()

			_, err := h.install(p, h.opts())

			wantInstallError(t, err, "Cannot install example.com/pack 1.0.0: its package file "+c.msg)
			h.assertUnchanged(before)
		})
	}
}

func TestInstallLaterPlacementSameDestWins(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("one.txt", "out.txt"), place("two.txt", "out.txt"))},
		src("one.txt", "1", "two.txt", "2"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"out.txt"})
	eq(t, "out.txt", h.read("out.txt"), "2")
}

func TestInstallLaterAssetSameDestWins(t *testing.T) {
	h := newHarness(t)
	h.assets["https://example.com/a.zip"] = src("one.txt", "zip")
	p := h.pkg(pkgmodel.Entry{Assets: []pkgmodel.Asset{
		{Type: pkgmodel.AssetZip, URLs: []string{"https://example.com/a.zip"},
			Place: []pkgmodel.Placement{place("one.txt", "out.txt")}},
		{Type: pkgmodel.AssetSelf, Place: []pkgmodel.Placement{place("one.txt", "out.txt")}},
	}}, src("one.txt", "self"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"out.txt"})
	eq(t, "out.txt", h.read("out.txt"), "self")
}

func TestInstallLaterWriteSameTargetOtherCaseWinsOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("paths are case-sensitive here")
	}
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("one.txt", "Out.txt"), place("two.txt", "OUT.txt"))},
		src("one.txt", "1", "two.txt", "2"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"OUT.txt"})
	eq(t, "out.txt", h.read("out.txt"), "2")
}

func TestInstallZipAssetPlacesFilesOfTheDownloadedArchive(t *testing.T) {
	h := newHarness(t)
	h.assets["https://example.com/bds.zip"] = src("bedrock_server.exe", "bds", "worlds/a.mcworld", "w",
		"lip.yaml", "zip's")
	p := h.pkg(pkgmodel.Entry{Assets: []pkgmodel.Asset{
		{Type: pkgmodel.AssetZip, URLs: []string{"https://mirror.invalid/bds.zip", "https://example.com/bds.zip"},
			Place: []pkgmodel.Placement{place("", "")}},
		{Type: pkgmodel.AssetSelf, Place: []pkgmodel.Placement{place("defaults/", "")}},
	}}, src("lip.yaml", "{}", "defaults/server.properties", "props"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned", sorted(owned), []string{"bedrock_server.exe", "lip.yaml", "server.properties", "worlds/a.mcworld"})
	eq(t, "bds", h.read("bedrock_server.exe"), "bds")
	eq(t, "props", h.read("server.properties"), "props")
	// Only the package file of the package itself is never written.
	eq(t, "lip.yaml", h.read("lip.yaml"), "zip's")
	eq(t, "opened", h.opened, [][]string{{"https://mirror.invalid/bds.zip", "https://example.com/bds.zip"}})
}

func TestInstallZipAssetFromRealZipSource(t *testing.T) {
	h := newHarness(t)
	z, err := archive.NewZipBytesSource(zipBytes(t, "b.txt", "b", "a.txt", "a"), "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	h.assets["https://example.com/a.zip"] = z
	p := h.pkg(pkgmodel.Entry{Assets: []pkgmodel.Asset{{Type: pkgmodel.AssetZip,
		URLs: []string{"https://example.com/a.zip"}, Place: []pkgmodel.Placement{place("", "out/")}}}}, nil)

	// Keys in entry order, not sorted.
	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"out/b.txt", "out/a.txt"})
	eq(t, "a", h.read("out/a.txt"), "a")
}

func TestInstallAssetUnavailableThrowsWithoutChangesOrScripts(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{
		Assets: []pkgmodel.Asset{{Type: pkgmodel.AssetZip, URLs: []string{"https://example.com/gone.zip"},
			Place: []pkgmodel.Placement{place("", "")}}},
		Hooks: pkgmodel.Hooks{PreInstall: []string{"pre"}},
	}, nil)
	before := h.snapshot()

	_, err := h.install(p, h.opts())

	ie := wantInstallError(t, err, `Cannot install example.com/pack 1.0.0: lip could not obtain asset 0`+
		` (type "zip", from https://example.com/gone.zip) of the package, so nothing was changed.`)
	var unavailable *archive.UnavailableError
	if !errors.As(ie, &unavailable) {
		t.Errorf("cause %v", ie.Err)
	}
	h.assertUnchanged(before)
	eq(t, "hook calls", len(h.calls), 0)
}

// Step 2: "mcmod" assets.

func TestInstallMcmodWritesWholeArchiveIntoModDirectory(t *testing.T) {
	h := newHarness(t)
	source := legacyMoneyFiles()

	owned := h.mustInstall(h.legacyMoney(pkgmodel.Entry{}, source, ""), h.opts())

	eq(t, "owned", owned, []string{"mods/LegacyMoney/manifest.json", "mods/LegacyMoney/LegacyMoney.dll",
		"mods/LegacyMoney/lang/en_US.json"})
	eq(t, "dll", h.read("mods/LegacyMoney/LegacyMoney.dll"), "dll")
	eq(t, "lang", h.read("mods/LegacyMoney/lang/en_US.json"), "{}")
	eq(t, "prefetched", sorted(source.prefetched), []string{"LegacyMoney.dll", "lang/en_US.json", "manifest.json"})
	eq(t, "plugins exists", h.dirExists("plugins"), false)
}

func TestInstallMcmodManifestIsWrittenAsItIs(t *testing.T) {
	for _, manifest := range []string{`{"name":"LegacyMoney"}`,
		`{"name":"LegacyMoney","dependencies":[{"name":"Other"}],"extra":1}`} {
		h := newHarness(t)
		p := h.legacyMoney(pkgmodel.Entry{Deps: []pkgmodel.Dep{{Key: "example.com/a", Range: "^1.0.0"}}},
			src("manifest.json", manifest, "LegacyMoney.dll", "dll"), "")

		owned := h.mustInstall(p, h.opts())

		eq(t, "manifest", h.read("mods/LegacyMoney/manifest.json"), manifest)
		eq(t, "owned", owned, []string{"mods/LegacyMoney/manifest.json", "mods/LegacyMoney/LegacyMoney.dll"})
	}
}

func TestInstallMcmodWithoutManifestThrowsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{}, src("LegacyMoney.dll", "dll", "sub/manifest.json", "{}"), "")
	before := h.snapshot()

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, `Cannot install example.com/legacymoney 0.22.0: its asset of type "mcmod" has no`+
		` manifest.json at the root of its archive, so lip cannot tell the mod name.`)
	h.assertUnchanged(before)
}

func TestInstallInvalidModManifestThrowsWithoutChanges(t *testing.T) {
	cases := map[string]string{
		"not json":                `lip could not read the manifest.json of its asset of type "mcmod"`,
		"[]":                      `but it is a JSON array.`,
		`{"entry":"x.dll"}`:       `but it is an object without "name".`,
		`{"name":1}`:              `but it is an object whose "name" is a number.`,
		`{"name":".."}`:           `but it is an object whose "name" is "..".`,
		`{"name":"a/b"}`:          `but it is an object whose "name" is "a/b".`,
		`{"name":"x","name":"y"}`: `lip could not read the manifest.json`,
	}
	for manifest, msg := range cases {
		t.Run(manifest, func(t *testing.T) {
			h := newHarness(t)
			p := h.legacyMoney(pkgmodel.Entry{}, src("manifest.json", manifest, "x.dll", "dll"), "")
			before := h.snapshot()

			_, err := h.install(p, h.opts())

			wantInstallError(t, err, msg)
			h.assertUnchanged(before)
		})
	}
}

func TestInstallModNameFromCallback(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{}, nil, "")
	p.ModName = func(context.Context) (string, error) { return "Renamed", nil }

	owned := h.mustInstall(p, h.opts())

	eq(t, "owned[0]", owned[0], "mods/Renamed/manifest.json")
}

func TestInstallSelfPlacementIntoModDirectoryAfterMcmodWins(t *testing.T) {
	h := newHarness(t)
	h.assets[legacyMoneyURL] = legacyMoneyFiles()
	p := h.pkg(pkgmodel.Entry{Assets: []pkgmodel.Asset{
		{Type: pkgmodel.AssetMcmod, URLs: []string{legacyMoneyURL}},
		{Type: pkgmodel.AssetSelf, Place: []pkgmodel.Placement{place("patch.dll", "mods/LegacyMoney/LegacyMoney.dll")}},
	}}, src("patch.dll", "patched"))

	owned := h.mustInstall(p, h.opts())

	eq(t, "dll", h.read("mods/LegacyMoney/LegacyMoney.dll"), "patched")
	eq(t, "owned count", len(owned), 3)
}

// Write rules.

func TestInstallDestInLipDirectoryThrows(t *testing.T) {
	for _, dest := range []string{".lip/workspace.json", ".lip", ".LIP/x.txt", "./.lip/x"} {
		t.Run(dest, func(t *testing.T) {
			if runtime.GOOS != "windows" && strings.HasPrefix(dest, ".LIP") {
				t.Skip("case-sensitive")
			}
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", dest))}, src("a.txt", "a"))
			before := h.snapshot()

			_, err := h.install(p, h.opts())

			wantInstallError(t, err, "inside .lip, which is reserved for lip.")
			h.assertUnchanged(before)
		})
	}
}

func TestInstallUnsafeDestThrows(t *testing.T) {
	for _, dest := range []string{"../x.txt", "a/../../x.txt", `a\b.txt`, "C:/x.txt", "c:x", "/abs.txt",
		"a//b.txt", "dir/", ".", "./."} {
		t.Run(dest, func(t *testing.T) {
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", dest))}, src("a.txt", "a"))
			before := h.snapshot()

			_, err := h.install(p, h.opts())

			wantInstallError(t, err, "its package file places a file at '"+dest+"', which is not a valid path"+
				" inside the workspace (a relative path with '/' separators and without '..').")
			h.assertUnchanged(before)
		})
	}
}

func TestInstallAliasingDestOnWindowsThrows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("such names can be written elsewhere")
	}
	for _, dest := range []string{"trailing./a.txt", "a.txt ", "config/a:b.txt"} {
		t.Run(dest, func(t *testing.T) {
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", dest))}, src("a.txt", "a"))
			before := h.snapshot()

			_, err := h.install(p, h.opts())

			wantInstallError(t, err, "which is not a valid path on Windows (a name in it ends with '.' or a"+
				" space, or contains ':').")
			h.assertUnchanged(before)
		})
	}
}

func TestInstallDestNamedLikeLipOrGitFileWrites(t *testing.T) {
	for _, dest := range []string{"lip_lock.json", "tooth_lock.json", "tooth.json", "lip.yaml", ".git/hooks/pre-commit"} {
		t.Run(dest, func(t *testing.T) {
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", dest))}, src("a.txt", "a"))

			eq(t, "owned", h.mustInstall(p, h.opts()), []string{dest})
			eq(t, "content", h.read(dest), "a")
		})
	}
}

func TestInstallDestWithDotSegmentWritesAndOwnsThePathWithoutIt(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "x/./a.txt"))}, src("a.txt", "a"))

	eq(t, "owned", h.mustInstall(p, h.opts()), []string{"x/a.txt"})
	eq(t, "content", h.read("x/a.txt"), "a")
}

func TestInstallSchemaValidDestOutsideWindowsWrites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot write such names as they are")
	}
	for _, dest := range []string{"config/a:b.txt", "trailing./a.txt", "a.txt "} {
		h := newHarness(t)
		p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", dest))}, src("a.txt", "a"))

		eq(t, "owned", h.mustInstall(p, h.opts()), []string{dest})
		eq(t, "content", h.read(dest), "a")
	}
}

func TestInstallOverwritesExistingFileBacksItUpAsHiddenSibling(t *testing.T) {
	h := newHarness(t)
	h.addFile("dir/existing.txt", "original")
	h.addFile("dir/.existing.txt.bak", "someone else's")
	source := src("a.txt", "a", "b.txt", "b")
	var duringInstall []string
	var backup string
	hidden := false
	source.onOpen = func(key string) {
		if key == "b.txt" {
			entries, _ := os.ReadDir(h.full("dir"))
			for _, e := range entries {
				duringInstall = append(duringInstall, e.Name())
			}
			backup = h.read("dir/.existing.txt.bak1")
			attrHidden, _ := hiddenAttr(h.full("dir/.existing.txt.bak1"))
			hidden = attrHidden
		}
	}
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "dir/existing.txt"), place("b.txt", "dir/b.txt"))}, source)

	h.mustInstall(p, h.opts())

	if !slices.Contains(duringInstall, ".existing.txt.bak1") {
		t.Errorf("during install: %v", duringInstall)
	}
	eq(t, "backup", backup, "original")
	if runtime.GOOS == "windows" {
		eq(t, "hidden", hidden, true)
	}
	entries, _ := os.ReadDir(h.full("dir"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	eq(t, "files", sorted(names), []string{".existing.txt.bak", "b.txt", "existing.txt"})
	eq(t, "other backup", h.read("dir/.existing.txt.bak"), "someone else's")
	eq(t, "existing", h.read("dir/existing.txt"), "a")
}

func TestInstallPackageWritesBackupNameBackupAvoidsIt(t *testing.T) {
	h := newHarness(t)
	h.addFile("foo", "original")
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "foo"), place("b.txt", ".foo.bak"))},
		src("a.txt", "a", "b.txt", "b"))

	h.mustInstall(p, h.opts())

	eq(t, "foo", h.read("foo"), "a")
	eq(t, ".foo.bak", h.read(".foo.bak"), "b")
	eq(t, ".foo.bak1 exists", h.fileExists(".foo.bak1"), false)
}

func TestInstallPrefetchFailsThrowsBeforeScripts(t *testing.T) {
	h := newHarness(t)
	source := legacyMoneyFiles()
	source.failPrefetch = true
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"pre"}}}, source, "")

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, "Cannot install example.com/legacymoney 0.22.0: lip could not get the files of"+
		" the package, so nothing was changed.")
	eq(t, "hook calls", len(h.calls), 0)
	eq(t, "mods exists", h.dirExists("mods"), false)
}

func TestInstallPrefetchThroughDeferredSource(t *testing.T) {
	h := newHarness(t)
	inner := src("a.txt", "a")
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt"))},
		archive.Defer(func() (archive.Source, error) { return inner, nil }))

	h.mustInstall(p, h.opts())

	eq(t, "prefetched", inner.prefetched, []string{"a.txt"})
}

func TestInstallFileOwnedByAnotherPackageThrowsAndLeavesWorkspaceIdentical(t *testing.T) {
	h := newHarness(t)
	h.addFile("PeEditor.exe", "other's")
	h.addFile("unrelated.txt", "keep")
	other := locked(h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil), []string{"PeEditor.exe"})
	p := h.pkg(pkgmodel.Entry{
		Assets: selfAsset(place("new.txt", "new.txt"), place("PeEditor.exe", "PeEditor.exe")),
		Hooks:  pkgmodel.Hooks{PreInstall: []string{"echo pre"}},
	}, src("PeEditor.exe", "exe", "new.txt", "n"))
	before := h.snapshot()

	_, err := h.install(p, h.opts(other))

	wantInstallError(t, err, "Cannot install example.com/pack 1.0.0: it would overwrite files that belong to"+
		" other installed packages: PeEditor.exe (owned by example.com/other). Uninstall those packages"+
		" first, or ask the authors of the packages to resolve the conflict.")
	h.assertUnchanged(before)
	eq(t, "hook calls", len(h.calls), 0)
}

func TestInstallPreservedFileOwnedByAnotherPackageThrows(t *testing.T) {
	h := newHarness(t)
	h.addFile("cfg.ini", "other's")
	other := locked(h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil), []string{"cfg.ini"})
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg.ini", "cfg.ini")), Preserve: []string{"cfg.ini"}},
		src("cfg.ini", "default"))

	_, err := h.install(p, h.opts(other))

	wantInstallError(t, err, "cfg.ini (owned by example.com/other)")
	eq(t, "cfg.ini", h.read("cfg.ini"), "other's")
}

func TestInstallOwnershipComparedOnNormalizedPaths(t *testing.T) {
	h := newHarness(t)
	lockedPath := `mods\LegacyMoney\LegacyMoney.dll`
	if runtime.GOOS == "windows" {
		lockedPath = `Mods\LegacyMoney\LegacyMoney.DLL`
	}
	other := locked(h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil), []string{lockedPath})

	_, err := h.install(h.legacyMoney(pkgmodel.Entry{}, nil, ""), h.opts(other))

	wantInstallError(t, err, "(owned by example.com/other)")
	eq(t, "mods exists", h.dirExists("mods"), false)
}

func TestInstallFileOwnedBySamePackageInLockIsNotAConflict(t *testing.T) {
	h := newHarness(t)
	old := locked(h.legacyMoney(pkgmodel.Entry{}, nil, "0.21.0"), []string{"mods/LegacyMoney/LegacyMoney.dll"})

	owned := h.mustInstall(h.legacyMoney(pkgmodel.Entry{}, nil, ""), h.opts(old))

	if !slices.Contains(owned, "mods/LegacyMoney/LegacyMoney.dll") {
		t.Errorf("owned %v", owned)
	}
}

func TestInstallPreservedExistingFileKeepsItAndDoesNotOwnIt(t *testing.T) {
	h := newHarness(t)
	h.addFile("mods/LegacyMoney/config/config.json", "user")
	source := src("manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "LegacyMoney.dll", "dll",
		"config/config.json", "default", "config/other.json", "default")
	p := h.legacyMoney(pkgmodel.Entry{Preserve: []string{"mods/LegacyMoney/config/"}}, source, "")

	owned := h.mustInstall(p, h.opts())

	eq(t, "kept", h.read("mods/LegacyMoney/config/config.json"), "user")
	eq(t, "other", h.read("mods/LegacyMoney/config/other.json"), "default")
	eq(t, "owned", owned, []string{"mods/LegacyMoney/manifest.json", "mods/LegacyMoney/LegacyMoney.dll",
		"mods/LegacyMoney/config/other.json"})
	if slices.Contains(source.prefetched, "config/config.json") {
		t.Error("kept file prefetched")
	}
}

func TestInstallPreserveRelativeToModDirectoryDoesNotMatch(t *testing.T) {
	h := newHarness(t)
	h.addFile("mods/LegacyMoney/config/config.json", "user")
	p := h.legacyMoney(pkgmodel.Entry{Preserve: []string{"config/*.json"}},
		src("manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "config/config.json", "default"), "")

	owned := h.mustInstall(p, h.opts())

	eq(t, "config", h.read("mods/LegacyMoney/config/config.json"), "default")
	if !slices.Contains(owned, "mods/LegacyMoney/config/config.json") {
		t.Errorf("owned %v", owned)
	}
}

func TestInstallPreservedFileOwnedBySamePackageIsKeptAndNotOwned(t *testing.T) {
	h := newHarness(t)
	h.addFile("mods/LegacyMoney/config/config.json", "user")
	old := locked(h.legacyMoney(pkgmodel.Entry{}, nil, "0.21.0"), []string{"mods/LegacyMoney/config/config.json"})
	p := h.legacyMoney(pkgmodel.Entry{Preserve: []string{"mods/LegacyMoney/config/*.json"}},
		src("manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "config/config.json", "default"), "")

	owned := h.mustInstall(p, h.opts(old))

	eq(t, "owned", owned, []string{"mods/LegacyMoney/manifest.json"})
	eq(t, "config", h.read("mods/LegacyMoney/config/config.json"), "user")
}

func TestInstallPreservedNewFileIsWritten(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg/", "cfg/")), Preserve: []string{"cfg/*.ini"}},
		src("cfg/a.ini", "default", "cfg/b.ini", "b"))
	h.addFile("cfg/a.ini", "user")

	owned := h.mustInstall(p, h.opts())

	eq(t, "a", h.read("cfg/a.ini"), "user")
	eq(t, "b", h.read("cfg/b.ini"), "b")
	eq(t, "owned", owned, []string{"cfg/b.ini"})
}

func TestInstallTargetIsExistingDirectoryThrowsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addDir("mods/LegacyMoney/LegacyMoney.dll")
	before := h.snapshot()

	_, err := h.install(h.legacyMoney(pkgmodel.Entry{}, nil, ""), h.opts())

	wantInstallError(t, err, "it writes the file 'mods/LegacyMoney/LegacyMoney.dll', but a directory of that"+
		" name is in the way. Move or delete that directory, then try again.")
	h.assertUnchanged(before)
}

func TestInstallParentIsExistingFileThrowsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addFile("mods/LegacyMoney/lang", "file")
	before := h.snapshot()

	_, err := h.install(h.legacyMoney(pkgmodel.Entry{}, nil, ""), h.opts())

	wantInstallError(t, err, "it writes the file 'mods/LegacyMoney/lang/en_US.json', but"+
		" 'mods/LegacyMoney/lang' is a file, not a directory. Move or delete that file, then try again.")
	h.assertUnchanged(before)
}

// Hooks.

func TestInstallHooksRunInWorkspaceRootAroundWrites(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"pre1", "pre2"},
		PostInstall: []string{"post"}, PreUninstall: []string{"x"}}}, nil, "")
	var dllExisted []bool
	h.onRun = func(string) { dllExisted = append(dllExisted, h.fileExists("mods/LegacyMoney/LegacyMoney.dll")) }

	h.mustInstall(p, h.opts())

	eq(t, "calls", h.calls, []hookCall{{"pre1", h.root}, {"pre2", h.root}, {"post", h.root}})
	eq(t, "dll existed", dllExisted, []bool{false, false, true})
	eq(t, "hook output", h.rep.infos("ran pre1"), 1)
}

func TestInstallIgnoreScriptsRunsNoHooks(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"pre"},
		PostInstall: []string{"post"}}}, nil, "")
	o := h.opts()
	o.IgnoreScripts = true

	h.mustInstall(p, o)

	eq(t, "calls", len(h.calls), 0)
	eq(t, "dll exists", h.fileExists("mods/LegacyMoney/LegacyMoney.dll"), true)
}

func TestInstallPreInstallFailsThrowsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"fail"}}}, nil, "")
	h.fail["fail"] = true

	_, err := h.install(p, h.opts())

	ie := wantInstallError(t, err, "In the pre_install hook of example.com/legacymoney, the command 'fail'"+
		" failed. lip undoes the file changes of this operation, but not what hook commands changed.")
	var cf *CommandFailedError
	if !errors.As(ie, &cf) {
		t.Errorf("cause %v", ie.Err)
	}
	eq(t, "mods exists", h.dirExists("mods"), false)
}

func TestInstallWriteFailsMidwayRollsBackWrittenFiles(t *testing.T) {
	h := newHarness(t)
	h.addFile("existing.txt", "original")
	source := src("a.txt", "a", "b.txt", "b", "c.txt", "c")
	source.failing["c.txt"] = true
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "existing.txt"), place("b.txt", "new/dir/b.txt"),
		place("c.txt", "c.txt"))}, source)
	before := h.snapshot()

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, "Cannot install example.com/pack: lip cannot write 'c.txt'.")
	h.assertUnchanged(before)
}

func TestInstallPostInstallFailsRollsBackWrittenFiles(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PostInstall: []string{"fail"}}}, nil, "")
	h.fail["fail"] = true
	before := h.snapshot()

	_, err := h.install(p, h.opts())

	wantInstallError(t, err, "In the post_install hook")
	h.assertUnchanged(before)
}

func TestInstallDryRunReportsPlanWithoutChanges(t *testing.T) {
	h := newHarness(t)
	source := legacyMoneyFiles()
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreInstall: []string{"pre"},
		PostInstall: []string{"post"}}}, source, "")
	h.addFile("keep.txt", "k")
	before := h.snapshot()
	o := h.opts()
	o.DryRun = true

	owned := h.mustInstall(p, o)

	eq(t, "owned count", len(owned), 3)
	h.assertUnchanged(before)
	eq(t, "calls", len(h.calls), 0)
	eq(t, "would write dll", h.rep.infos("Would write mods/LegacyMoney/LegacyMoney.dll"), 1)
	eq(t, "prefetched", len(source.prefetched), 0)
}

func TestInstallDryRunReportsKeptFiles(t *testing.T) {
	h := newHarness(t)
	h.addFile("cfg/a.ini", "user")
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("cfg/", "cfg/")), Preserve: []string{"cfg/*.ini"}},
		src("cfg/a.ini", "default", "cfg/b.ini", "b"))
	o := h.opts()
	o.DryRun = true

	h.mustInstall(p, o)

	eq(t, "lines", h.rep.lines, []string{"info: Would keep cfg/a.ini (preserved)", "info: Would write cfg/b.ini"})
}

// Uninstalling.

func TestUninstallModDeletesOwnedFilesAndPrunesEmptyDirectories(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{}, nil, "")
	owned := h.mustInstall(p, h.opts())
	h.addFile("mods/Other/x.dll", "x")
	l := locked(p, owned)

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "mods/LegacyMoney exists", h.dirExists("mods/LegacyMoney"), false)
	eq(t, "mods exists", h.dirExists("mods"), true)
	eq(t, "other", h.fileExists("mods/Other/x.dll"), true)
}

func TestUninstallUsesTheRecordedEntryOnly(t *testing.T) {
	h := newHarness(t)
	h.addFile("a.txt", "a")
	h.addFile("keep.txt", "k")
	h.addFile("logs/x.log", "x")
	entry := &pkgmodel.Entry{
		Platform: "server", Runtime: "win-x64",
		Assets:   []pkgmodel.Asset{{Type: pkgmodel.AssetZip, URLs: []string{"https://example.com/gone.zip"}}},
		Preserve: []string{"keep.txt"},
		Purge:    []string{"logs/"},
		Hooks:    pkgmodel.Hooks{PreUninstall: []string{"pre"}},
	}
	l := locked(&Package{ID: "example.com/pack", Version: "1.0.0"}, []string{"a.txt", "keep.txt"})

	h.mustUninstall(&l, entry, h.opts())

	eq(t, "a exists", h.fileExists("a.txt"), false)
	eq(t, "keep exists", h.fileExists("keep.txt"), true)
	eq(t, "logs exists", h.dirExists("logs"), false)
	eq(t, "calls", h.calls, []hookCall{{"pre", h.root}})
	eq(t, "opened", len(h.opened), 0)
}

func TestUninstallPreexistingEmptyDirectoryIsKept(t *testing.T) {
	h := newHarness(t)
	h.addDir("mods/LegacyMoney/empty")
	h.addDir("unrelated")
	p := h.legacyMoney(pkgmodel.Entry{}, nil, "")
	l := locked(p, h.mustInstall(p, h.opts()))

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "empty", h.dirExists("mods/LegacyMoney/empty"), true)
	eq(t, "unrelated", h.dirExists("unrelated"), true)
	eq(t, "lang", h.dirExists("mods/LegacyMoney/lang"), false)
}

func TestUninstallPreserveKeepsMatchingOwnedFiles(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Preserve: []string{"mods/LegacyMoney/config/"}},
		src("manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "LegacyMoney.dll", "dll",
			"config/config.json", "default"), "")
	l := locked(p, h.mustInstall(p, h.opts()))

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "dll", h.fileExists("mods/LegacyMoney/LegacyMoney.dll"), false)
	eq(t, "manifest", h.fileExists("mods/LegacyMoney/manifest.json"), false)
	eq(t, "config", h.read("mods/LegacyMoney/config/config.json"), "default")
}

func TestUninstallFileOwnedByAnotherPackageIsKept(t *testing.T) {
	h := newHarness(t)
	h.addFile("shared.txt", "s")
	p := h.pkg(pkgmodel.Entry{}, nil)
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	self := locked(p, []string{"shared.txt"})

	h.mustUninstall(&self, p.Entry, h.opts(self, locked(other, []string{"shared.txt"})))

	eq(t, "shared", h.fileExists("shared.txt"), true)
}

func TestUninstallRecordedPathsWithDotSegmentsKeepLipAndOthersFiles(t *testing.T) {
	h := newHarness(t)
	h.addFile(".lip/lockx", "l")
	h.addFile("shared.txt", "s")
	h.addFile("own.txt", "o")
	p := h.pkg(pkgmodel.Entry{}, nil)
	other := h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil)
	recorded := []string{"./.lip/lockx", "./shared.txt", "./own.txt"}
	self := locked(p, recorded)

	h.mustUninstall(&self, p.Entry, h.opts(self, locked(other, []string{"shared.txt"})))

	eq(t, ".lip/lockx", h.fileExists(".lip/lockx"), true)
	eq(t, "shared", h.fileExists("shared.txt"), true)
	eq(t, "own", h.fileExists("own.txt"), false)
}

func TestUninstallPurgeDirectoryPatternDeletesRecursivelyExceptOthers(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Purge: []string{"mods/LegacyMoney/data/", "mods/LegacyMoney/*.log"}}, nil, "")
	owned := h.mustInstall(p, h.opts())
	h.addFile("mods/LegacyMoney/data/db/money.db", "db")
	h.addFile("mods/LegacyMoney/data/top.txt", "t")
	h.addDir("mods/LegacyMoney/data/emptysub")
	h.addFile("mods/LegacyMoney/data/foreign.txt", "f")
	h.addFile("mods/LegacyMoney/run.log", "log")
	h.addFile("mods/LegacyMoney/keep.txt", "k")
	other := locked(h.pkgOf("example.com/other", "1.0.0", pkgmodel.Entry{}, nil),
		[]string{"mods/LegacyMoney/data/foreign.txt"})
	self := locked(p, owned)

	h.mustUninstall(&self, p.Entry, h.opts(self, other))

	eq(t, "db dir", h.dirExists("mods/LegacyMoney/data/db"), false)
	eq(t, "emptysub", h.dirExists("mods/LegacyMoney/data/emptysub"), false)
	eq(t, "top", h.fileExists("mods/LegacyMoney/data/top.txt"), false)
	eq(t, "run.log", h.fileExists("mods/LegacyMoney/run.log"), false)
	eq(t, "dll", h.fileExists("mods/LegacyMoney/LegacyMoney.dll"), false)
	eq(t, "foreign", h.read("mods/LegacyMoney/data/foreign.txt"), "f")
	eq(t, "keep", h.read("mods/LegacyMoney/keep.txt"), "k")
}

func TestUninstallPurgeRelativeToWorkspaceRoot(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("PeEditor.exe", "PeEditor.exe")),
		Purge: []string{"PeEditor.log", "cache/"}}, src("PeEditor.exe", "exe"))
	l := locked(p, h.mustInstall(p, h.opts()))
	h.addFile("PeEditor.log", "log")
	h.addFile("sub/PeEditor.log", "log")
	h.addFile("cache/a/b.bin", "b")
	h.addFile("bedrock_server.exe", "bds")

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "exe", h.fileExists("PeEditor.exe"), false)
	eq(t, "log", h.fileExists("PeEditor.log"), false)
	eq(t, "sub log", h.fileExists("sub/PeEditor.log"), true)
	eq(t, "cache", h.dirExists("cache"), false)
	eq(t, "bds", h.fileExists("bedrock_server.exe"), true)
	eq(t, "root", dirExists(h.root), true)
}

func TestUninstallPurgeNamingLayoutDirectoriesRemovesNothingThroughThem(t *testing.T) {
	for _, pattern := range []string{"mods/", "plugins/", "*/", "plugins", "MODS/"} {
		t.Run(pattern, func(t *testing.T) {
			h := newHarness(t)
			p := h.pkg(pkgmodel.Entry{Purge: []string{pattern}}, nil)
			h.addFile("mods/Other/other.dll", "dll")
			l := locked(p, nil)

			h.mustUninstall(&l, p.Entry, h.opts())

			if pattern == "MODS/" && runtime.GOOS != "windows" {
				return
			}
			eq(t, "other.dll", h.fileExists("mods/Other/other.dll"), true)
			eq(t, "mods", h.dirExists("mods"), true)
		})
	}
}

func TestUninstallPurgeAliasingOnWindowsMatchesNothing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("such names are ordinary here")
	}
	h := newHarness(t)
	h.addFile("mods/Other/other.dll", "dll")
	h.addFile(".lip/journalx", "j")
	h.addFile("own.txt", "o")
	p := h.pkg(pkgmodel.Entry{Purge: []string{"mods./", ".lip./journalx", "own.txt "}}, nil)
	l := locked(p, []string{"own.txt."})

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "other.dll", h.fileExists("mods/Other/other.dll"), true)
	eq(t, "journalx", h.fileExists(".lip/journalx"), true)
	eq(t, "own.txt", h.fileExists("own.txt"), true)
}

func TestUninstallPurgeMatchingLipDirectoryLeavesIt(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt")),
		Purge: []string{".lip/", ".lip/workspace.json", "*/", "**/*.json"}}, src("a.txt", "a"))
	owned := h.mustInstall(p, h.opts())
	h.addFile(".lip/workspace.json", "lock")
	h.addFile("logs/a.json", "x")
	l := locked(p, append(owned, ".lip/workspace.json"))

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "lock", h.read(".lip/workspace.json"), "lock")
	eq(t, "a", h.fileExists("a.txt"), false)
	eq(t, "logs/a.json", h.fileExists("logs/a.json"), false)
}

func TestUninstallPurgeGlobsMatchOnlyAsDeepAsThePattern(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt")),
		Purge: []string{"*.log", "logs/**/*.txt"}}, src("a.txt", "a"))
	l := locked(p, h.mustInstall(p, h.opts()))
	h.addFile("top.log", "x")
	h.addFile("worlds/deep.log", "keep")
	h.addFile("logs/a.txt", "x")
	h.addFile("logs/2026/b.txt", "x")
	h.addFile("logs/keep.bin", "keep")

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "top.log", h.fileExists("top.log"), false)
	eq(t, "deep.log", h.fileExists("worlds/deep.log"), true)
	eq(t, "logs/a.txt", h.fileExists("logs/a.txt"), false)
	eq(t, "logs/2026", h.dirExists("logs/2026"), false)
	eq(t, "keep.bin", h.fileExists("logs/keep.bin"), true)
}

func TestUninstallPurgeNeverFollowsLinks(t *testing.T) {
	h := newHarness(t)
	outside := filepath.Join(filepath.Dir(h.root), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "x.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.addDir("data")
	if err := makeDirLink(outside, h.full("data/link")); err != nil {
		t.Skipf("cannot create a link: %v", err)
	}
	p := h.pkg(pkgmodel.Entry{Purge: []string{"data/**/*.log"}}, nil)
	l := locked(p, nil)

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "outside x.log", fileExists(filepath.Join(outside, "x.log")), true)
}

func TestUninstallMissingOwnedFileIsSkipped(t *testing.T) {
	h := newHarness(t)
	h.addFile("a.txt", "a")
	p := h.pkg(pkgmodel.Entry{}, nil)
	l := locked(p, []string{"gone.txt", "a.txt"})

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "a", h.fileExists("a.txt"), false)
}

func TestUninstallFileDeletedByHookIsSkipped(t *testing.T) {
	h := newHarness(t)
	h.addFile("a.txt", "a")
	p := h.pkg(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreUninstall: []string{"rm"}}}, nil)
	h.onRun = func(string) { os.Remove(h.full("a.txt")) }
	l := locked(p, []string{"a.txt"})

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "a", h.fileExists("a.txt"), false)
}

func TestUninstallHooksRunInWorkspaceRootAroundDeletion(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreUninstall: []string{"pre"},
		PostUninstall: []string{"post"}, PostInstall: []string{"x"}}}, nil, "")
	o := h.opts()
	o.IgnoreScripts = true
	l := locked(p, h.mustInstall(p, o))
	var dllExisted []bool
	h.onRun = func(string) { dllExisted = append(dllExisted, h.fileExists("mods/LegacyMoney/LegacyMoney.dll")) }

	h.mustUninstall(&l, p.Entry, h.opts())

	eq(t, "calls", h.calls, []hookCall{{"pre", h.root}, {"post", h.root}})
	eq(t, "dll existed", dllExisted, []bool{true, false})
}

func TestUninstallIgnoreScriptsRunsNoHooks(t *testing.T) {
	h := newHarness(t)
	p := h.pkg(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreUninstall: []string{"pre"}, PostUninstall: []string{"post"}}}, nil)
	o := h.opts()
	o.IgnoreScripts = true
	l := locked(p, nil)

	h.mustUninstall(&l, p.Entry, o)

	eq(t, "calls", len(h.calls), 0)
}

func TestUninstallDryRunReportsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	p := h.legacyMoney(pkgmodel.Entry{Hooks: pkgmodel.Hooks{PreUninstall: []string{"pre"}}}, nil, "")
	l := locked(p, h.mustInstall(p, h.opts()))
	before := h.snapshot()
	o := h.opts()
	o.DryRun = true

	h.mustUninstall(&l, p.Entry, o)

	h.assertUnchanged(before)
	eq(t, "calls", len(h.calls), 0)
	eq(t, "would delete", h.rep.infos("Would delete"), 3)
}

func TestUpdateUninstallOldThenInstallNewPreservesConfigAndReplacesFiles(t *testing.T) {
	h := newHarness(t)
	preserve := []string{"mods/LegacyMoney/config/config.json"}
	v1 := h.legacyMoney(pkgmodel.Entry{Preserve: preserve}, src(
		"manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "LegacyMoney.dll", "v1",
		"old.dll", "old", "config/config.json", "default1"), "0.21.0")
	lockedV1 := locked(v1, h.mustInstall(v1, h.opts()))
	h.addFile("mods/LegacyMoney/config/config.json", "user")
	v2 := h.legacyMoney(pkgmodel.Entry{Preserve: preserve}, src(
		"manifest.json", `{"name":"LegacyMoney","dependencies":[]}`, "LegacyMoney.dll", "v2",
		"config/config.json", "default2"), "")

	h.mustUninstall(&lockedV1, v1.Entry, h.opts(lockedV1))
	ownedV2 := h.mustInstall(v2, h.opts())

	eq(t, "dll", h.read("mods/LegacyMoney/LegacyMoney.dll"), "v2")
	eq(t, "old", h.fileExists("mods/LegacyMoney/old.dll"), false)
	eq(t, "config", h.read("mods/LegacyMoney/config/config.json"), "user")
	eq(t, "owned", ownedV2, []string{"mods/LegacyMoney/manifest.json", "mods/LegacyMoney/LegacyMoney.dll"})
}

func TestUpdateOwnedFileOverwrittenByNewVersionIsOwnedAgain(t *testing.T) {
	h := newHarness(t)
	v1 := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt"))}, src("a.txt", "1"))
	lockedV1 := locked(v1, h.mustInstall(v1, h.opts()))
	v2 := h.pkgOf("example.com/pack", "2.0.0", pkgmodel.Entry{Assets: selfAsset(place("a.txt", "a.txt"))}, src("a.txt", "2"))

	h.mustUninstall(&lockedV1, v1.Entry, h.opts(lockedV1))
	ownedV2 := h.mustInstall(v2, h.opts())

	eq(t, "owned", ownedV2, []string{"a.txt"})
	eq(t, "a", h.read("a.txt"), "2")
}

// One operation planned against one view (McmodService.Execute): an update
// in one transaction, uninstalling the old version first.
func TestUpdateInOneOperationPlansAgainstTheView(t *testing.T) {
	h := newHarness(t)
	v1 := h.pkg(pkgmodel.Entry{Assets: selfAsset(place("a/", "dir/"))}, src("a/x.txt", "1", "a/y.txt", "1"))
	lockedV1 := locked(v1, h.mustInstall(v1, h.opts()))
	v2 := h.pkgOf("example.com/pack", "2.0.0", pkgmodel.Entry{Assets: selfAsset(place("a/", "dir/"))}, src("a/x.txt", "2"))

	view := NewView(nil)
	un, err := h.inst.PlanUninstall(context.Background(), view, h.opts(lockedV1), &lockedV1, v1.Entry)
	if err != nil {
		t.Fatal(err)
	}
	in, err := h.inst.PlanInstall(context.Background(), view, h.opts(), v2)
	if err != nil {
		t.Fatal(err)
	}
	// The uninstall deletes dir/ when it empties, so a write into it must
	// keep it: the view has it again.
	if err := h.apply(context.Background(), nil, un, in); err != nil {
		t.Fatal(err)
	}

	eq(t, "x", h.read("dir/x.txt"), "2")
	eq(t, "y", h.fileExists("dir/y.txt"), false)
}
