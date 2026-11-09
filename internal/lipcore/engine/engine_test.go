package engine

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// Ports of McmodServiceTests.cs and PackageInstallTests.cs (lip
// v0.38.0-beta.1): install, uninstall, update and list through the whole
// pipeline of the harness.

const (
	libID = "example.com/lib"
	appID = "example.com/app"
)

func libPackageFile(version string, hook bool) string {
	entry := selfAsset("lib.dll", "lib/lib.dll")
	if hook {
		entry += `, "hooks": { "post_install": [ "lib-installed" ] }`
	}
	return lipJSON(libID, version, entry)
}

func (h *harness) addLib(version string, hook bool) string {
	return h.addTag(libID, version, "lip.toml", libPackageFile(version, hook), "lib.dll", "lib "+version)
}

func (h *harness) addApp(version, libRange string) {
	h.addTag(appID, version, "lip.toml", lipJSON(appID, version,
		`"deps": { "`+libID+`": "`+libRange+`" }, `+selfAsset("app.dll", "app/app.dll")), "app.dll", "app "+version)
}

// addLibAndApp: lib 1.0.0 and 1.1.0; app 1.0.0 (lib >=1.0.0) and 2.0.0 (lib
// >=1.1.0).
func (h *harness) addLibAndApp(withApp2, hook bool) {
	h.addLib("1.0.0", hook)
	h.addLib("1.1.0", hook)
	h.addApp("1.0.0", ">=1.0.0")
	if withApp2 {
		h.addApp("2.0.0", ">=1.1.0")
	}
}

// addVariantsPackage: version 1.0.0 of a package whose variants "a" and "b"
// place a.dll and b.dll.
func (h *harness) addVariantsPackage(id string) {
	entry := func(f string) string {
		return `[ { "side": "server", "platform": "win-x64", "assets": [ { "type": "self", "place": [ { "src": "` +
			f + `", "dest": "` + f + `" } ] } ] } ]`
	}
	h.addTag(id, "1.0.0", "lip.toml", `{ "format": 4, "id": "`+id+`", "version": "1.0.0", "variants": { "a": `+
		entry("a.dll")+`, "b": `+entry("b.dll")+` } }`, "a.dll", "a", "b.dll", "b")
}

// addLocalDirectory: a local directory holding a package example.com/local
// placing x.txt at out/x.txt.
func (h *harness) addLocalDirectory(rel, version, content string) {
	h.addFile(rel+"/lip.toml", lipJSON("example.com/local", version, selfAsset("x.txt", "out/x.txt")))
	h.addFile(rel+"/x.txt", content)
}

// addMod: version 1.0.0 of a mod whose asset of type "mcmod" is
// https://example.com/<name>.mcmod; entry holds extra members, each followed
// by a comma.
func (h *harness) addMod(id, name, entry string) {
	url := "https://example.com/" + name + ".mcmod"
	h.addDownload(url, mcmod(name))
	h.addTag(id, "1.0.0", "lip.toml", lipJSON(id, "1.0.0", entry+` "assets": [ { "type": "mcmod", "urls": [ "`+url+`" ] } ]`))
}

func findPackage(t *testing.T, lf *pkgmodel.LockFile, id string) *pkgmodel.LockedPackage {
	t.Helper()
	for i := range lf.Packages {
		if stringMember(lf.Packages[i].Manifest, "id") == id {
			return &lf.Packages[i]
		}
	}
	t.Fatalf("no package %s", id)
	return nil
}

func single(t *testing.T, lf *pkgmodel.LockFile) *pkgmodel.LockedPackage {
	t.Helper()
	if len(lf.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(lf.Packages))
	}
	return &lf.Packages[0]
}

func TestInstallRemoteRepositoryRecordsWorkspaceFile(t *testing.T) {
	h := newHarness(t)
	commit := h.addTag(libID, "1.0.0", "lip.toml", libPackageFile("1.0.0", false), "lib.dll", "lib 1.0.0")
	h.must(h.install(libID))

	if got := h.read("lib/lib.dll"); got != "lib 1.0.0" {
		t.Fatalf("lib.dll = %q", got)
	}
	lf := h.lock()
	if lf.Format != 4 || lf.Side != "server" || lf.Platform != testPlatform {
		t.Fatalf("lock = %d %s %s", lf.Format, lf.Side, lf.Platform)
	}
	p := single(t, lf)
	if stringMember(p.Manifest, "id") != libID || p.Variant != "" || stringMember(p.Manifest, "version") != "1.0.0" ||
		!p.Explicit || p.Ref != libID || p.Commit != commit {
		t.Fatalf("package = %+v", p)
	}
	equal(t, p.Files, []string{"lib/lib.dll"})
	want, _ := doc.ParseJSON([]byte(libPackageFile("1.0.0", false)), "lip.toml")
	if !doc.Equal(want, p.Manifest) {
		t.Fatalf("manifest = %s", doc.MarshalJSONCompact(p.Manifest))
	}
}

func TestInstallWritesTheMembersOfTheSchema(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0", selfAsset("x.txt", "x.txt")), "x.txt", "x")
	h.must(h.install(libID, "x.zip"))

	data, err := os.ReadFile(h.path(pkgmodel.WorkspaceFile))
	if err != nil {
		t.Fatal(err)
	}
	root, err := doc.ParseJSONFile(data)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, _ := root.(*doc.Object).Get("packages")
	list := pkgs.([]any)
	equal(t, list[0].(*doc.Object).Keys(), []string{"manifest", "variant", "explicit", "ref", "commit", "files"})
	equal(t, list[1].(*doc.Object).Keys(), []string{"manifest", "variant", "explicit", "ref", "files"})
}

func TestInstallLocalZipRecordsFileURISourceAndNoCommit(t *testing.T) {
	h := newHarness(t)
	h.addDownload("https://example.com/extra.zip", zipOf("y.txt", "y"))
	packageFile := lipJSON("example.com/pkg", "1.0.0", `"assets": [
	  { "type": "self", "place": [ { "src": "x.txt", "dest": "mods/pkg/x.txt" } ] },
	  { "type": "zip", "urls": [ "https://example.com/extra.zip" ], "place": [ { "src": "y.txt", "dest": "extra/y.txt" } ] } ]`)
	path := h.addZip("pkg.zip", "lip.toml", packageFile, "x.txt", "x")
	h.must(h.install("pkg.zip"))

	if h.read("mods/pkg/x.txt") != "x" || h.read("extra/y.txt") != "y" {
		t.Fatal("files not placed")
	}
	p := single(t, h.lock())
	r, err := ref.FromFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.String(), "file:") || p.Ref != r.String() || p.Commit != "" {
		t.Fatalf("ref = %q, commit = %q", p.Ref, p.Commit)
	}
	equal(t, p.Files, []string{"mods/pkg/x.txt", "extra/y.txt"})
}

func TestInstallNewestVersionOpensOnlyTheNewestTag(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 20; i++ {
		h.addLib("1."+strconv.Itoa(i)+".0", false)
	}
	h.must(h.install(libID))
	if h.read("lib/lib.dll") != "lib 1.19.0" {
		t.Fatal(h.read("lib/lib.dll"))
	}
	listed := map[string]bool{}
	for _, c := range h.git.treesListed() {
		listed[c] = true
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d trees", len(listed))
	}
}

func TestInstallWithDependencyInstallsDependencyPackage(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID + "@1.0.0"))
	equal(t, h.summary(), []string{libID + "@1.1.0 dep", appID + "@1.0.0 explicit"})
	lf := h.lock()
	equal(t, []string{lf.Packages[0].Ref, lf.Packages[1].Ref}, []string{libID, appID + "@1.0.0"})
	if !h.exists("lib/lib.dll") || !h.exists("app/app.dll") {
		t.Fatal("files missing")
	}
}

func TestInstallFormatThreeDependencyInstallsBoth(t *testing.T) {
	h := newHarness(t)
	h.addTag(libID, "1.0.0", "tooth.json", tooth(libID, "1.0.0", "lib/lib.dll", "", ""), "file.dll", "lib 1.0.0")
	h.addApp("1.0.0", ">=1.0.0")
	h.must(h.install(appID))

	lf := h.lock()
	if stringMember(lf.Packages[0].Manifest, "id") != libID || lf.Packages[0].Explicit || !lf.Packages[1].Explicit {
		t.Fatal(h.summary())
	}
	if h.read("lib/lib.dll") != "lib 1.0.0" {
		t.Fatal("lib not placed")
	}
	want, _ := doc.ParseJSON([]byte(`{ "format": 4, "id": "`+libID+`", "version": "1.0.0", "variants": { "": [ {
	  "side": "server", "platform": "win-x64",
	  "assets": [ { "type": "self", "place": [ { "src": "file.dll", "dest": "lib/lib.dll" } ] } ] } ] } }`), "x")
	if !doc.Equal(want, lf.Packages[0].Manifest) {
		t.Fatalf("manifest = %s", doc.MarshalJSONCompact(lf.Packages[0].Manifest))
	}
}

func TestInstallFormatThreeWithOtherVersionThanItsTagRecordsTheTagVersion(t *testing.T) {
	h := newHarness(t)
	h.addTag(libID, "1.2.0", "tooth.json", tooth(libID, "0.9.0", "lib/lib.dll", "", ""), "file.dll", "lib")
	h.must(h.install(libID))
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.2.0" {
		t.Fatal(stringMember(p.Manifest, "version"))
	}
	equal(t, h.summary(), []string{libID + "@1.2.0 explicit"})
	if !h.rec.hasWarning("using the tag version 1.2.0") {
		t.Fatalf("warnings = %v", h.rec.warnings)
	}
}

func TestInstallAlreadyExplicitFails(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID))
	err := h.install(libID)
	if !errors.Is(err, ErrAlreadyExplicitlyInstalled) || !strings.Contains(err.Error(), "already explicitly installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallTwoArgumentsOfOneIdentityFailsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	err := h.install(libID, libID+"@1.0.0")
	if err == nil || !strings.Contains(err.Error(), "both name the package") {
		t.Fatalf("err = %v", err)
	}
	if h.exists("lib/lib.dll") || h.hasLock() {
		t.Fatal("changed")
	}
}

func TestInstallDependencyPackageMarksItExplicit(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	h.must(h.install(libID + "@1.1.0"))
	lib := findPackage(t, h.lock(), libID)
	if !lib.Explicit || stringMember(lib.Manifest, "version") != "1.1.0" || lib.Ref != libID+"@1.1.0" {
		t.Fatalf("lib = %+v", lib)
	}
	if !h.rec.hasInfo("Marking package '" + libID + "' version 1.1.0 as explicitly installed.") {
		t.Fatalf("infos = %v", h.rec.infos)
	}
}

func TestInstallInstalledDependencyTooOldReplacesIt(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID + "@1.0.0"))
	h.must(h.install(appID + "@1.0.0"))
	h.must(h.uninstall(libID))
	equal(t, h.summary(), []string{libID + "@1.0.0 dep", appID + "@1.0.0 explicit"})

	h.must(h.update(appID + "@2.0.0"))
	equal(t, h.summary(), []string{libID + "@1.1.0 dep", appID + "@2.0.0 explicit"})
	if h.read("lib/lib.dll") != "lib 1.1.0" {
		t.Fatal(h.read("lib/lib.dll"))
	}
}

func TestInstallNoDependenciesInstallsOnlyTheRequest(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.e.Install(context.Background(), []string{appID}, false, false, true))
	equal(t, h.summary(), []string{appID + "@2.0.0 explicit"})
	if h.exists("lib/lib.dll") || !h.exists("app/app.dll") {
		t.Fatal("files")
	}
	if !h.rec.hasWarning("--no-dependencies is enabled") {
		t.Fatal(h.rec.warnings)
	}
}

func TestInstallNoDependenciesOfInstalledDependencyOnlyMarksIt(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, true)
	h.must(h.install(appID))
	before := *findPackage(t, h.lock(), libID)
	h.must(h.e.Install(context.Background(), []string{libID}, false, false, true))
	after := findPackage(t, h.lock(), libID)
	if !after.Explicit || after.Commit != before.Commit {
		t.Fatalf("after = %+v", after)
	}
	equal(t, after.Files, before.Files)
	if h.ran("lib-installed") != 1 {
		t.Fatalf("hook ran %d times", h.ran("lib-installed"))
	}
}

func TestInstallNoDependenciesOtherVersionReplacesInstalledPackageOnly(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID+"@1.0.0", libID+"@1.0.0"))
	h.must(h.e.Install(context.Background(), []string{libID + "@1.1.0"}, false, false, true))
	// lib still satisfies the dependency of app, so it comes first
	// (protocol/workspace.md §5, §7.1).
	equal(t, h.summary(), []string{libID + "@1.1.0 explicit", appID + "@1.0.0 explicit"})
	if h.read("lib/lib.dll") != "lib 1.1.0" {
		t.Fatal(h.read("lib/lib.dll"))
	}
}

func TestInstallNoDependenciesDryRunOfInstalledDependencyReportsMarkingIt(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	h.must(h.e.Install(context.Background(), []string{libID}, true, false, true))
	if !h.rec.hasInfo("Dry run: would mark package '" + libID + "'") {
		t.Fatal(h.rec.infos)
	}
	if findPackage(t, h.lock(), libID).Explicit {
		t.Fatal("marked")
	}
}

func TestInstallDryRunChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, true)
	before := h.snapshot()
	h.must(h.e.Install(context.Background(), []string{appID}, true, false, false))
	equalMaps(t, h.snapshot(), before)
	if _, err := os.Stat(h.path(".lip")); err == nil {
		t.Fatal(".lip created")
	}
	if len(h.commands) != 0 {
		t.Fatal(h.commands)
	}
	if !h.rec.hasInfo("Dry run: would install package '" + appID + "' version 2.0.0 from " + appID + "@2.0.0") {
		t.Fatal(h.rec.infos)
	}
}

func TestInstallIgnoreScriptsRunsNoHook(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, true)
	h.must(h.e.Install(context.Background(), []string{libID}, false, true, false))
	if !h.exists("lib/lib.dll") || len(h.commands) != 0 {
		t.Fatal(h.commands)
	}
}

func TestInstallVariantsTogetherRecordsVariantIdentities(t *testing.T) {
	h := newHarness(t)
	h.addVariantsPackage(libID)
	h.must(h.install(libID+"#a", libID+"#b"))
	lf := h.lock()
	for i, v := range []string{"a", "b"} {
		p := &lf.Packages[i]
		if pkgIdentity(p) != libID+"#"+v || p.Variant != v || p.Ref != libID+"#"+v {
			t.Fatalf("package %d = %+v", i, p)
		}
	}
	if h.read("a.dll") != "a" || h.read("b.dll") != "b" {
		t.Fatal("files")
	}
}

func pkgIdentity(p *pkgmodel.LockedPackage) string {
	return ref.Identity(stringMember(p.Manifest, "id"), p.Variant)
}

func TestInstallBranchRefRecordsCommitAndUpdateFollowsHead(t *testing.T) {
	h := newHarness(t)
	first := h.git.addBranch(libID, "main", "lip.toml", libPackageFile("1.2.0", false), "lib.dll", "head 1")
	h.must(h.install(libID + "@branch:main"))
	installed := *single(t, h.lock())
	second := h.git.addBranch(libID, "main", "lip.toml", libPackageFile("1.2.0", false), "lib.dll", "head 2")
	h.restart()
	h.must(h.update())

	if stringMember(installed.Manifest, "version") != "1.2.0" || installed.Ref != libID+"@branch:main" ||
		installed.Commit != first {
		t.Fatalf("installed = %+v", installed)
	}
	if h.read("lib/lib.dll") != "head 2" {
		t.Fatal(h.read("lib/lib.dll"))
	}
	updated := single(t, h.lock())
	if updated.Commit != second || updated.Ref != libID+"@branch:main" {
		t.Fatalf("updated = %+v", updated)
	}
}

func TestInstallTagAndRevRefsUsePackageFileVersionAndExactCommit(t *testing.T) {
	h := newHarness(t)
	h.git.addTagName(libID, "nightly", "lip.toml", libPackageFile("3.0.0-nightly", false), "lib.dll", "nightly")
	rev := h.git.addBranch(appID, "dev", "lip.toml", lipJSON(appID, "0.1.0", ""))
	h.git.addBranch(appID, "dev", "lip.toml", lipJSON(appID, "0.2.0", ""))
	h.must(h.install(libID+"@tag:nightly", appID+"@rev:"+rev))
	lf := h.lock()
	app, lib := findPackage(t, lf, appID), findPackage(t, lf, libID)
	if stringMember(app.Manifest, "version") != "0.1.0" || app.Ref != appID+"@rev:"+rev {
		t.Fatalf("app = %+v", app)
	}
	if stringMember(lib.Manifest, "version") != "3.0.0-nightly" || lib.Ref != libID+"@tag:nightly" {
		t.Fatalf("lib = %+v", lib)
	}
}

func TestInstallRecordedSideOfOtherKindFailsWithoutChanges(t *testing.T) {
	for _, tc := range []struct{ engine, recorded, says string }{
		{"server", "client", "lip manages server workspaces only"},
		{"client", "server", "LeviLauncher manages client workspaces only"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			h := newHarness(t)
			h.side = tc.engine
			h.restart()
			h.addLibAndApp(true, false)
			h.addFile(".lip/state.json", `{ "format": 4, "side": "`+tc.recorded+`", "platform": "win-x64", "packages": [] }`)
			before := h.snapshot()
			err := h.install(libID)
			if err == nil || !strings.Contains(err.Error(), tc.says) {
				t.Fatalf("err = %v", err)
			}
			equalMaps(t, h.snapshot(), before)
		})
	}
}

func TestInstallPlatformOptionRecordsPlatform(t *testing.T) {
	h := newHarness(t)
	h.addTag(libID, "1.0.0", "lip.toml", `{ "format": 4, "id": "`+libID+`", "version": "1.0.0", "variants": { "": [
	  { "side": "server", "platform": "win-x64", "assets": [ { "type": "self", "place": [ { "src": "win.dll", "dest": "lib.dll" } ] } ] },
	  { "side": "server", "platform": "linux-x64", "assets": [ { "type": "self", "place": [ { "src": "linux.so", "dest": "lib.so" } ] } ] } ] } }`,
		"win.dll", "win", "linux.so", "linux")
	h.withPlatform("linux-x64")
	h.must(h.install(libID))
	if h.read("lib.so") != "linux" || h.exists("lib.dll") || h.lock().Platform != "linux-x64" {
		t.Fatal("platform")
	}
}

func TestInstallClientSideByDefault(t *testing.T) {
	h := newHarness(t)
	h.side = ""
	h.restart()
	h.addTag(libID, "1.0.0", "lip.toml", lipJSONFor(libID, "1.0.0", "client", selfAsset("lib.dll", "lib/lib.dll")),
		"lib.dll", "lib")
	h.must(h.install(libID))
	if lf := h.lock(); lf.Side != "client" {
		t.Fatal(lf.Side)
	}
}

func TestUninstallExplicitPackageRemovesItsUnneededDependencies(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	h.must(h.uninstall(appID))
	if len(h.lock().Packages) != 0 || h.exists("lib/lib.dll") || h.exists("app/app.dll") {
		t.Fatal("not removed")
	}
}

func TestUninstallDependencyOfAnotherPackageKeepsItAsDependency(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID+"@1.1.0", appID))
	h.must(h.uninstall(libID))
	if findPackage(t, h.lock(), libID).Explicit || !h.exists("lib/lib.dll") {
		t.Fatal("lib")
	}
	if !h.rec.hasInfo("Package '" + libID + "' is still required by other packages; keeping it as a dependency.") {
		t.Fatal(h.rec.infos)
	}
}

func TestUninstallDependencyPackageFailsUnlessNoDependencies(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	if err := h.uninstall(libID); err == nil || !strings.Contains(err.Error(), "was installed as a dependency") {
		t.Fatalf("err = %v", err)
	}
	h.must(h.e.Uninstall(context.Background(), []string{libID}, false, false, true))
	equal(t, h.summary(), []string{appID + "@2.0.0 explicit"})
	if h.exists("lib/lib.dll") || !h.exists("app/app.dll") {
		t.Fatal("files")
	}
}

func TestUninstallNoDependenciesOfExplicitPackageKeepsItsDependencies(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	h.must(h.e.Uninstall(context.Background(), []string{appID}, false, false, true))
	equal(t, h.summary(), []string{libID + "@1.1.0 dep"})
	if !h.exists("lib/lib.dll") || h.exists("app/app.dll") {
		t.Fatal("files")
	}
}

func TestUninstallNotInstalledFails(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID))
	if err := h.uninstall(appID); err == nil || !strings.Contains(err.Error(), "is not installed in this workspace") {
		t.Fatalf("err = %v", err)
	}
	single(t, h.lock())
}

func TestUninstallMatching(t *testing.T) {
	t.Run("identity in other case", func(t *testing.T) {
		h := newHarness(t)
		h.addLibAndApp(true, false)
		h.must(h.install(libID))
		h.must(h.uninstall("Example.COM/Lib"))
		if len(h.lock().Packages) != 0 {
			t.Fatal("kept")
		}
	})
	t.Run("source without variant matches every variant", func(t *testing.T) {
		h := newHarness(t)
		h.addVariantsPackage(libID)
		h.must(h.install(libID+"#a", libID+"#b"))
		h.must(h.uninstall(libID))
		if len(h.lock().Packages) != 0 || h.exists("a.dll") || h.exists("b.dll") {
			t.Fatal("kept")
		}
	})
	t.Run("identity with variant removes only that variant", func(t *testing.T) {
		h := newHarness(t)
		h.addVariantsPackage(libID)
		h.must(h.install(libID+"#a", libID+"#b"))
		h.must(h.uninstall(libID + "#a"))
		if h.exists("a.dll") || h.read("b.dll") != "b" {
			t.Fatal("files")
		}
		equal(t, h.summary(), []string{libID + "#b@1.0.0 explicit"})
	})
	t.Run("source with ref matches only that ref", func(t *testing.T) {
		h := newHarness(t)
		h.addLibAndApp(true, false)
		h.must(h.install(libID + "@1.0.0"))
		if err := h.uninstall(libID + "@1.1.0"); err == nil {
			t.Fatal("other ref matched")
		}
		h.must(h.uninstall("example.com/LIB@1.0.0"))
		if len(h.lock().Packages) != 0 || h.exists("lib/lib.dll") {
			t.Fatal("kept")
		}
	})
	t.Run("source without ref matches every ref", func(t *testing.T) {
		h := newHarness(t)
		h.git.addBranch(libID, "main", "lip.toml", libPackageFile("1.5.0", false), "lib.dll", "x")
		h.must(h.install(libID + "@branch:main"))
		h.must(h.uninstall(libID))
		if len(h.lock().Packages) != 0 {
			t.Fatal("kept")
		}
	})
	for _, arg := range []string{"./src", "src"} {
		t.Run("local path "+arg, func(t *testing.T) {
			h := newHarness(t)
			h.addLocalDirectory("src", "1.0.0", "x")
			h.must(h.install("./src"))
			h.must(h.uninstall(arg))
			if len(h.lock().Packages) != 0 || h.exists("out/x.txt") {
				t.Fatal("kept")
			}
		})
	}
	t.Run("deleted local path fails", func(t *testing.T) {
		h := newHarness(t)
		path := h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0", selfAsset("x.txt", "x.txt")), "x.txt", "x")
		h.must(h.install("x.zip"))
		os.Remove(path)
		before := h.snapshot()
		if err := h.uninstall("./x.zip"); err == nil {
			t.Fatal("uninstalled")
		}
		equalMaps(t, h.snapshot(), before)
	})
	t.Run("file URI argument fails", func(t *testing.T) {
		h := newHarness(t)
		h.addLocalDirectory("src", "1.0.0", "x")
		h.must(h.install("./src"))
		source := single(t, h.lock()).Ref
		before := h.snapshot()
		if err := h.uninstall(source); err == nil {
			t.Fatal("uninstalled")
		}
		equalMaps(t, h.snapshot(), before)
	})
}

func TestUpdateNoArgumentsUpdatesExplicitPackagesToNewest(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(false, false)
	h.must(h.install(appID))
	h.addApp("2.0.0", ">=1.1.0")
	h.restart()
	h.must(h.update())
	equal(t, h.summary(), []string{libID + "@1.1.0 dep", appID + "@2.0.0 explicit"})
	if h.read("app/app.dll") != "app 2.0.0" {
		t.Fatal(h.read("app/app.dll"))
	}
}

func TestUpdateNothingDiffersReplacesNothing(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, true)
	h.must(h.install(libID))
	h.restart()
	h.must(h.update())
	h.must(h.update(libID))
	if h.ran("lib-installed") != 1 {
		t.Fatal(h.ran("lib-installed"))
	}
	equal(t, h.summary(), []string{libID + "@1.1.0 explicit"})
}

func TestUpdateTagMovedToOtherCommitReplacesPackage(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.must(h.install(libID))
	moved := h.addTag(libID, "1.0.0", "lip.toml", libPackageFile("1.0.0", false), "lib.dll", "lib moved")
	h.restart()
	h.must(h.update())
	if h.read("lib/lib.dll") != "lib moved" || single(t, h.lock()).Commit != moved {
		t.Fatal("not replaced")
	}
}

func TestUpdateBranchAtTheSameCommitReplacesNothing(t *testing.T) {
	h := newHarness(t)
	commit := h.git.addBranch(libID, "main", "lip.toml", libPackageFile("1.0.0", true), "lib.dll", "head")
	h.must(h.install(libID + "@branch:main"))
	h.restart()
	h.must(h.update())
	if h.ran("lib-installed") != 1 || single(t, h.lock()).Commit != commit {
		t.Fatal("replaced")
	}
}

func TestInstallNoDependenciesOfSameVersionAtOtherCommitReplacesIt(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", true)
	h.must(h.install(libID))
	moved := h.addTag(libID, "1.0.0", "lip.toml", libPackageFile("1.0.0", true), "lib.dll", "lib moved")
	h.restart()
	h.must(h.e.Install(context.Background(), []string{libID}, false, false, true))
	if h.read("lib/lib.dll") != "lib moved" || single(t, h.lock()).Commit != moved || h.ran("lib-installed") != 2 {
		t.Fatal("not replaced")
	}
}

func TestInstallInstalledPackageWhoseTagMovedIsKeptWithItsRecordedPackageFile(t *testing.T) {
	h := newHarness(t)
	h.addMod("example.com/a", "A", "")
	h.must(h.install("example.com/a"))
	h.addDownload("https://example.com/moved.mcmod", mcmod("B"))
	h.addTag("example.com/a", "1.0.0", "lip.toml", lipJSON("example.com/a", "1.0.0",
		`"assets": [ { "type": "mcmod", "urls": [ "https://example.com/moved.mcmod" ] } ]`))
	h.addMod("example.com/b", "B", "")
	h.restart()
	h.must(h.install("example.com/b"))
	if !h.exists("mods/A/A.dll") || !h.exists("mods/B/B.dll") {
		t.Fatal(h.snapshot())
	}
	if !strings.Contains(string(doc.MarshalJSONCompact(h.lock().Packages[0].Manifest)), "https://example.com/A.mcmod") {
		t.Fatal("manifest changed")
	}
}

func TestUpdateVersionRefWhoseTagMovedKeepsTheRecordedCommit(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.must(h.install(libID + "@1.0.0"))
	h.addTag(libID, "1.0.0", "lip.toml", libPackageFile("1.0.0", false), "lib.dll", "moved")
	h.addLib("1.1.0", false)
	h.restart()
	h.must(h.update())
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.0.0" || p.Ref != libID+"@1.0.0" || h.read("lib/lib.dll") != "lib 1.0.0" {
		t.Fatalf("p = %+v", p)
	}
}

func TestUpdateSourceWithoutRefMovesBackToVersionTags(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.git.addBranch(libID, "main", "lip.toml", libPackageFile("1.5.0", false), "lib.dll", "head")
	h.must(h.install(libID + "@branch:main"))
	h.must(h.update(libID))
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.1.0" || p.Ref != libID || !p.Explicit || h.read("lib/lib.dll") != "lib 1.1.0" {
		t.Fatalf("p = %+v", p)
	}
}

func TestUpdateAssetChangedReplacesNothing(t *testing.T) {
	h := newHarness(t)
	h.addDownload("https://example.com/data.zip", zipOf("d.txt", "1"))
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0",
		`"assets": [ { "type": "zip", "urls": [ "https://example.com/data.zip" ], "place": [ { "src": "d.txt", "dest": "d.txt" } ] } ]`))
	h.must(h.install(libID))
	h.addDownload("https://example.com/data.zip", zipOf("d.txt", "2"))
	os.RemoveAll(h.cacheDir)
	h.restart()
	h.must(h.update())
	if h.read("d.txt") != "1" {
		t.Fatal("replaced")
	}
}

func TestUpdateLocalZipOfSameVersionReplacesNothing(t *testing.T) {
	h := newHarness(t)
	h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0", selfAsset("x.txt", "x.txt")), "x.txt", "v1")
	h.must(h.install("x.zip"))
	h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0", selfAsset("x.txt", "x.txt")), "x.txt", "v2")
	h.must(h.update())
	if h.read("x.txt") != "v1" {
		t.Fatal("replaced")
	}
}

func TestUpdateLocalDirectoryOfSameVersionReplacesNothing(t *testing.T) {
	h := newHarness(t)
	h.addLocalDirectory("src", "1.0.0", "v1")
	h.must(h.install("./src"))
	h.addLocalDirectory("src", "1.0.0", "v2")
	h.must(h.update())
	if h.read("out/x.txt") != "v1" {
		t.Fatal("replaced")
	}
	h.addLocalDirectory("src", "1.0.1", "v3")
	h.must(h.update())
	if h.read("out/x.txt") != "v3" || stringMember(single(t, h.lock()).Manifest, "version") != "1.0.1" {
		t.Fatal("not replaced")
	}
}

func TestUpdateLocalPathGoneKeepsPackageAndWarns(t *testing.T) {
	h := newHarness(t)
	path := h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0", selfAsset("x.txt", "x.txt")), "x.txt", "v1")
	h.must(h.install(path))
	os.Remove(path)
	h.must(h.update())
	if h.read("x.txt") != "v1" || !h.rec.hasWarning("does not exist any more") {
		t.Fatal(h.rec.warnings)
	}
	single(t, h.lock())
}

func TestUpdateDependencyBecomesExplicitlyInstalled(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	h.must(h.update(libID))
	if !findPackage(t, h.lock(), libID).Explicit {
		t.Fatal("not explicit")
	}
}

func TestUpdateTwoArgumentsOfOneIdentitySatisfiesBoth(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID))
	// lib 1.0.0 of lib@1.0.0 stands in for the request of lib
	// (protocol/workspace.md §3).
	h.must(h.update(libID, libID+"@1.0.0"))
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.0.0" || p.Ref != libID+"@1.0.0" {
		t.Fatalf("p = %+v", p)
	}
	if h.read("lib/lib.dll") != "lib 1.0.0" {
		t.Fatal(h.read("lib/lib.dll"))
	}
}

func TestUpdateNotInstalledFails(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	if err := h.update(appID); err == nil || !strings.Contains(err.Error(), "is not installed in this workspace") {
		t.Fatalf("err = %v", err)
	}
	if h.hasLock() {
		t.Fatal("state file written")
	}
}

func TestInstallModWithDependencyWritesItsManifestAsItIs(t *testing.T) {
	h := newHarness(t)
	h.addMod("example.com/base", "Base", "")
	h.addMod("example.com/top", "Top", `"deps": { "example.com/base": "^1.0.0" },`)
	h.must(h.install("example.com/top"))
	if !strings.Contains(h.read("mods/Top/manifest.json"), `"name": "Top"`) || !h.exists("mods/Base/manifest.json") {
		t.Fatal(h.snapshot())
	}
}

func TestInstallTwoModsOfOneModNameFails(t *testing.T) {
	h := newHarness(t)
	h.addMod("example.com/a", "Same", "")
	h.addDownload("https://example.com/other.mcmod", mcmod("same"))
	h.addTag("example.com/b", "1.0.0", "lip.toml", lipJSON("example.com/b", "1.0.0",
		`"assets": [ { "type": "mcmod", "urls": [ "https://example.com/other.mcmod" ] } ]`))
	err := h.install("example.com/a", "example.com/b")
	if err == nil {
		t.Fatal("installed")
	}
	if h.hasLock() {
		t.Fatal("changed")
	}
}

func TestListReturnsPackagesInInstallationOrder(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.addVariantsPackage("example.com/var")
	h.must(h.install(appID, "example.com/var#a"))
	equal(t, h.summary(), []string{libID + "@1.1.0 dep", appID + "@2.0.0 explicit", "example.com/var#a@1.0.0 explicit"})
	explicit, implicit, err := h.e.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	equal(t, explicit, []string{appID + "@2.0.0", "example.com/var#a@1.0.0"})
	equal(t, implicit, []string{libID + "@1.1.0"})
	ps := h.packages()
	if ps[2].ID != "example.com/var" || ps[2].Variant != "a" || ps[2].Ref != "example.com/var#a" {
		t.Fatalf("package = %+v", ps[2])
	}
}

func TestListNewWorkspaceIsEmpty(t *testing.T) {
	h := newHarness(t)
	explicit, implicit, err := h.e.List(context.Background())
	if err != nil || len(explicit) != 0 || len(implicit) != 0 || explicit == nil || implicit == nil {
		t.Fatalf("%v %v %v", explicit, implicit, err)
	}
	if _, err := os.Stat(h.path(".lip")); err == nil {
		t.Fatal(".lip created")
	}
}

func TestInstallInstalledPackageWithoutEntryForThePlatformFailsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.must(h.install(libID))
	h.addTag("example.com/any", "1.0.0", "lip.toml", `{ "format": 4, "id": "example.com/any", "version": "1.0.0",
	  "variants": { "": [ { "side": "server", "platform": "linux-x64" } ] } }`)
	before := h.snapshot()
	h.withPlatform("linux-x64")
	err := h.install("example.com/any")
	if err == nil || !strings.Contains(err.Error(), "'"+libID+"'") ||
		!strings.Contains(err.Error(), "The workspace was installed for the platform win-x64") {
		t.Fatalf("err = %v", err)
	}
	equalMaps(t, h.snapshot(), before)
}

func TestOperationsInvalidWorkspaceFileFailWithoutChanges(t *testing.T) {
	for _, content := range []string{
		`{ "format": "bf45b1", "side": "server", "packages": [] }`,
		`{ "format": 4, "platform": "server", "runtime": "win-x64", "packages": [] }`,
		`{ "format_version": 3, "packages": [] }`,
		`{ "format": 4, "side": "server", "platform": "win-x64", "packages": [ { } ] }`,
		"format = 4\n",
	} {
		h := newHarness(t)
		h.addLibAndApp(true, false)
		h.addFile(".lip/state.json", content)
		before := h.snapshot()
		if h.install(libID) == nil || h.uninstall(libID) == nil || h.update() == nil {
			t.Fatal("an operation succeeded")
		}
		if _, _, err := h.e.List(context.Background()); err == nil {
			t.Fatal("list succeeded")
		}
		var lfe *pkgfile.LockFormatError
		if err := h.install(libID); !errors.As(err, &lfe) {
			t.Fatalf("err = %T %v", err, err)
		}
		equalMaps(t, h.snapshot(), before)
	}
}
