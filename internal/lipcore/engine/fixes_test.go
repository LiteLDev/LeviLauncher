package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// Ports of the tests McmodServiceTests.cs gained after v0.38.0-beta.1 with
// lip's engine fixes: keeping the versions of dependency packages that fit
// (b46fb4d, d6484ba), update (c4daa6b, 54f2a69, 4204ac7, 94f5362, 2d7a555),
// installation order of --no-dependencies (38433a1, b3cf923, 0df168f),
// preserve and purge decided when a step runs (91f5e3a, 8c9d597, 186c6a7),
// the journal before hook commands (0fa2c4c), resolution causes (15dfe48) and
// the layout made only when an operation may change the workspace (0c5cf46,
// 8ef0abe, 1dc3a6c).

// addPackage tags version of id with a package file placing <id>.txt, with
// the dependencies (key, range pairs).
func (h *harness) addPackage(id, version string, deps ...string) {
	var members []string
	for i := 0; i+1 < len(deps); i += 2 {
		members = append(members, `"`+deps[i]+`": "`+deps[i+1]+`"`)
	}
	entry := ""
	if len(members) > 0 {
		entry = `"deps": { ` + strings.Join(members, ", ") + ` }`
	}
	h.addTag(id, version, "lip.toml", lipJSON(id, version, entry))
}

func (h *harness) noDepsInstall(args ...string) error {
	return h.e.Install(context.Background(), args, false, false, true)
}

func (h *harness) journalExists() bool { return h.exists(".lip/journal") }

func (h *harness) dirExists(rel string) bool {
	info, err := os.Stat(h.path(rel))
	return err == nil && info.IsDir()
}

// unchangedThroughLink is the workspace as it matters when mods is a link,
// which the snapshot cannot walk: the state file and the files a package
// writes.
func (h *harness) unchangedThroughLink(stateBefore string) {
	h.t.Helper()
	state := ""
	if h.hasLock() {
		state = h.read(".lip/state.json")
	}
	if state != stateBefore || h.exists("lib/lib.dll") || h.journalExists() {
		h.t.Fatalf("changed: state %q, lib.dll %v, journal %v", state, h.exists("lib/lib.dll"), h.journalExists())
	}
}

// installedOrder is the identities of the state file, in order.
func (h *harness) installedOrder() []string {
	var out []string
	for _, p := range h.packages() {
		out = append(out, p.Identity)
	}
	return out
}

// installing is the "Installing" messages, in order.
func (h *harness) installing() []string {
	var out []string
	for _, m := range h.rec.infos {
		if strings.HasPrefix(m, "Installing") {
			out = append(out, m)
		}
	}
	return out
}

func (h *harness) dependencyPackages() []string {
	var out []string
	for _, p := range h.packages() {
		if !p.Explicit {
			out = append(out, p.Identity+"@"+p.Version)
		}
	}
	return out
}

func TestInstallDependencyPackageThatFitsTheRequestKeepsItsVersion(t *testing.T) {
	// lib 1.0.0 is a dependency package; lib 1.1.0 is newer.
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID + "@1.0.0"))
	h.must(h.install(appID + "@1.0.0"))
	h.must(h.uninstall(libID))
	h.restart()

	h.must(h.install(libID))

	// It only becomes explicitly installed (protocol/cli.md §5.1).
	lib := findPackage(t, h.lock(), libID)
	if !lib.Explicit || stringMember(lib.Manifest, "version") != "1.0.0" || lib.Ref != libID {
		t.Fatalf("lib = %+v", lib)
	}
	if h.read("lib/lib.dll") != "lib 1.0.0" {
		t.Fatal(h.read("lib/lib.dll"))
	}
}

func TestInstallDependencyPackagesChosenAgainKeepTheInstalledVersionsThatFit(t *testing.T) {
	// a needs d and e, installed at 1.0.0; b needs e ^2.0.0.
	h := newHarness(t)
	h.addPackage("example.com/d", "1.0.0")
	h.addPackage("example.com/e", "1.0.0")
	h.addPackage("example.com/a", "1.0.0", "example.com/d", ">=1.0.0", "example.com/e", ">=1.0.0")
	h.must(h.install("example.com/a"))
	h.addPackage("example.com/d", "2.0.0")
	h.addPackage("example.com/e", "2.0.0")
	h.addPackage("example.com/b", "1.0.0", "example.com/e", "^2.0.0")
	h.restart()

	h.must(h.install("example.com/b"))

	// Only e changes version (protocol/cli.md §5.1).
	equal(t, h.dependencyPackages(), []string{"example.com/d@1.0.0", "example.com/e@2.0.0"})
}

func TestInstallNoDependenciesOtherVersionKeepsInstallationOrder(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID+"@1.0.0", libID+"@1.0.0"))

	h.must(h.noDepsInstall(libID + "@1.1.0"))

	// lib still satisfies the dependency of app, so it comes first
	// (protocol/workspace.md §5, §7.1).
	equal(t, h.installedOrder(), []string{libID, appID})
}

func TestInstallNoDependenciesOfADependencyAndItsDependentInstallsInInstallationOrder(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)

	// app is named before lib, which satisfies its dependency.
	h.must(h.noDepsInstall(appID+"@1.0.0", libID+"@1.0.0"))

	equal(t, h.installedOrder(), []string{libID, appID})
	installing := h.installing()
	if len(installing) != 2 || !strings.Contains(installing[0], "'"+libID+"'") ||
		!strings.Contains(installing[1], "'"+appID+"'") {
		t.Fatalf("installing = %v", installing)
	}
}

func TestInstallNoDependenciesOfACycleThroughAnInstalledPackageInstallsInInstallationOrder(t *testing.T) {
	// zeta needs c, c needs alpha and alpha needs zeta; c is installed.
	const alpha, c, zeta = "example.com/cycle/alpha", "example.com/cycle/c", "example.com/cycle/zeta"
	h := newHarness(t)
	h.addPackage(zeta, "1.0.0", c, "1.0.0")
	h.addPackage(c, "1.0.0", alpha, "1.0.0")
	h.addPackage(alpha, "1.0.0", zeta, "1.0.0")
	h.must(h.noDepsInstall(c))
	h.rec.infos = nil

	h.must(h.noDepsInstall(zeta, alpha))

	// The three are a cycle, installed and recorded in ascending order of
	// identities (protocol/workspace.md §5, §7.1).
	installing := h.installing()
	if len(installing) != 2 || !strings.Contains(installing[0], "'"+alpha+"'") ||
		!strings.Contains(installing[1], "'"+zeta+"'") {
		t.Fatalf("installing = %v", installing)
	}
	equal(t, h.installedOrder(), []string{alpha, c, zeta})
}

func TestUninstallNoDependenciesOfAMemberOfACycleRecordsTheRestInInstallationOrder(t *testing.T) {
	// a needs c, c needs b and b needs a, a cycle recorded as a, b, c.
	const a, b, c = "example.com/cycle/a", "example.com/cycle/b", "example.com/cycle/c"
	h := newHarness(t)
	h.addPackage(a, "1.0.0", c, "1.0.0")
	h.addPackage(c, "1.0.0", b, "1.0.0")
	h.addPackage(b, "1.0.0", a, "1.0.0")
	h.must(h.install(a))
	equal(t, h.installedOrder(), []string{a, b, c})

	h.must(h.e.Uninstall(context.Background(), []string{b}, false, false, true))

	// Without b, c satisfies the dependency of a and comes first
	// (protocol/workspace.md §5, §7.1).
	equal(t, h.installedOrder(), []string{c, a})
}

func TestInstallPreservedFileDeletedByPreInstallRecordsItAsOwned(t *testing.T) {
	// The file matching preserve exists when the operation is planned, but
	// pre_install deletes it, so it is written and owned
	// (protocol/workspace.md §4).
	h := newHarness(t)
	h.addFile("cfg.ini", "user")
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", selfAsset("cfg.ini", "cfg.ini")+
		`, "preserve": [ "cfg.ini" ], "hooks": { "pre_install": [ "reset" ] }`), "cfg.ini", "default")
	h.onRun["reset"] = func() { os.Remove(h.path("cfg.ini")) }

	h.must(h.install(libID))

	if h.read("cfg.ini") != "default" {
		t.Fatal(h.read("cfg.ini"))
	}
	equal(t, single(t, h.lock()).Files, []string{"cfg.ini"})
}

func TestInstallDependencyWithInvalidPackageFileKeepsTheCause(t *testing.T) {
	// app needs lib, whose only tag has a package file that is no valid TOML.
	h := newHarness(t)
	h.addTag(libID, "1.0.0", "lip.toml", "id: [")
	h.addApp("1.0.0", "1.0.0")

	err := h.install(appID)

	// The failure keeps the error behind it, for the failure report.
	var re *graph.ResolutionError
	if !errors.As(err, &re) || len(re.Causes) == 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallPreInstallBeforeAnyChangeRunsWithTheJournal(t *testing.T) {
	// mods exists, so pre_install is the first thing the operation does; the
	// journal exists before it runs (protocol/workspace.md §7.4 steps 5, 6).
	h := newHarness(t)
	os.MkdirAll(h.path("mods"), 0o755)
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", selfAsset("a.txt", "a.txt")+
		`, "hooks": { "pre_install": [ "check" ] }`), "a.txt", "a")
	journalExisted := false
	h.onRun["check"] = func() { journalExisted = h.journalExists() }

	h.must(h.install(libID))

	if !journalExisted || h.journalExists() {
		t.Fatalf("journal existed %v, exists %v", journalExisted, h.journalExists())
	}
}

func TestUninstallOnlyHookCommandsRunsThemWithTheJournal(t *testing.T) {
	// lib owns no file, so uninstalling it changes nothing but runs
	// pre_uninstall (protocol/workspace.md §7.4 step 4 does not apply).
	h := newHarness(t)
	os.MkdirAll(h.path("mods"), 0o755)
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", `"hooks": { "pre_uninstall": [ "check" ] }`))
	h.must(h.install(libID))
	journalExisted := false
	h.onRun["check"] = func() { journalExisted = h.journalExists() }

	h.must(h.uninstall(libID))

	if !journalExisted || h.journalExists() {
		t.Fatalf("journal existed %v, exists %v", journalExisted, h.journalExists())
	}
	if len(h.lock().Packages) != 0 {
		t.Fatal(h.lock().Packages)
	}
}

// addCfgLibAndApp: lib keeps cfg.ini by preserve, but its pre_install
// deletes it; app needs lib and writes cfg.ini too.
func (h *harness) addCfgLibAndApp(appHooks string) {
	h.addFile("cfg.ini", "user")
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", selfAsset("cfg.ini", "cfg.ini")+
		`, "preserve": [ "cfg.ini" ], "hooks": { "pre_install": [ "reset" ] }`), "cfg.ini", "lib")
	h.addTag(appID, "1.0.0", "lip.toml", lipJSON(appID, "1.0.0", `"deps": { "`+libID+`": "1.0.0" }, `+
		selfAsset("cfg.ini", "cfg.ini")+appHooks), "cfg.ini", "app")
	h.onRun["reset"] = func() { os.Remove(h.path("cfg.ini")) }
}

func TestInstallPreservedFileWrittenAfterHookThenWrittenByAnotherFailsAndUndoes(t *testing.T) {
	// lib keeps cfg.ini as planned, but its pre_install deletes it, so lib
	// writes and owns it; app, installed after lib, then writing it is a
	// failure (protocol/workspace.md §4).
	h := newHarness(t)
	h.addCfgLibAndApp("")

	err := h.install(appID)

	var ie *install.Error
	if !errors.As(err, &ie) || !strings.Contains(ie.Msg, "cfg.ini") {
		t.Fatalf("err = %v", err)
	}
	if h.hasLock() || h.exists("cfg.ini") || h.journalExists() {
		t.Fatalf("lock %v, cfg.ini %v, journal %v", h.hasLock(), h.exists("cfg.ini"), h.journalExists())
	}
}

func TestInstallWriteOfFileWrittenByEarlierPackageFailsAtTheWrite(t *testing.T) {
	// app writing cfg.ini, which lib wrote after its pre_install deleted it,
	// is the failure itself (protocol/workspace.md §4), so app's post_install
	// never runs.
	h := newHarness(t)
	h.addCfgLibAndApp(`, "hooks": { "post_install": [ "app-post" ] }`)

	err := h.install(appID)

	var ie *install.Error
	if !errors.As(err, &ie) || !strings.Contains(ie.Msg, "'cfg.ini', which belongs to "+libID) {
		t.Fatalf("err = %v", err)
	}
	if h.ran("app-post") != 0 || h.hasLock() || h.exists("cfg.ini") {
		t.Fatalf("app-post %d, lock %v, cfg.ini %v", h.ran("app-post"), h.hasLock(), h.exists("cfg.ini"))
	}
}

// addServerLibAndApp: lib purges server.exe; app needs lib and its
// post_uninstall restores server.exe, as LeviLamina restores
// bedrock_server.exe.
func (h *harness) addServerLibAndApp(libHooks string) {
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", selfAsset("lib.dll", "lib/lib.dll")+
		`, "purge": [ "server.exe" ]`+libHooks), "lib.dll", "lib")
	h.addTag(appID, "1.0.0", "lip.toml", lipJSON(appID, "1.0.0", `"deps": { "`+libID+`": "1.0.0" }, `+
		selfAsset("app.dll", "app/app.dll")+`, "hooks": { "post_uninstall": [ "restore" ] }`), "app.dll", "app")
}

func TestUninstallPurgeMatchesFileThatEarlierHookRestoredDeletesIt(t *testing.T) {
	// app's post_uninstall restores server.exe before lib, uninstalled after
	// it, purges it (protocol/workspace.md §6 rule 4).
	h := newHarness(t)
	h.addServerLibAndApp("")
	h.must(h.install(appID))
	h.onRun["restore"] = func() { h.addFile("server.exe", "server") }

	h.must(h.uninstall(appID))

	if h.ran("restore") != 1 || len(h.lock().Packages) != 0 || h.exists("server.exe") || h.dirExists("lib") {
		t.Fatalf("restore %d, packages %v, server.exe %v, lib %v", h.ran("restore"), h.lock().Packages,
			h.exists("server.exe"), h.dirExists("lib"))
	}
}

func TestUninstallPurgeMatchesFileThatHookCreatedFailureRestoresIt(t *testing.T) {
	// lib purges server.exe, which app's post_uninstall restores; lib's
	// post_uninstall then fails, so the operation is undone, the restored
	// file included.
	h := newHarness(t)
	h.addServerLibAndApp(`, "hooks": { "post_uninstall": [ "fail" ] }`)
	h.must(h.install(appID))
	h.onRun["restore"] = func() { h.addFile("server.exe", "server") }
	h.failing["fail"] = true

	if err := h.uninstall(appID); err == nil {
		t.Fatal("no error")
	}

	if len(h.lock().Packages) != 2 || h.read("server.exe") != "server" || !h.exists("lib/lib.dll") {
		t.Fatalf("packages %d, lib.dll %v", len(h.lock().Packages), h.exists("lib/lib.dll"))
	}
}

func zipPackage(id, version, file, content string) []string {
	return []string{"lip.toml", lipJSON(id, version, selfAsset(file, file)), file, content}
}

func TestUpdateNoArgumentsSourceGivesOtherIdentityReplacesThePackage(t *testing.T) {
	// The local ZIP file now holds a package of another id.
	h := newHarness(t)
	h.addZip("x.zip", zipPackage("example.com/x", "1.0.0", "x.txt", "x")...)
	h.must(h.install("x.zip"))
	h.addZip("x.zip", zipPackage("example.com/y", "1.0.0", "y.txt", "y")...)

	h.must(h.update())

	// It differs in identity (protocol/workspace.md §5 "Updating").
	equal(t, h.summary(), []string{"example.com/y@1.0.0 explicit"})
	if h.exists("x.txt") || h.read("y.txt") != "y" {
		t.Fatal("files")
	}
}

func TestUpdateNoArgumentsTwoSourcesGiveSameIdentityFails(t *testing.T) {
	// a.zip now holds b 2.0.0, while b.zip still holds b 1.0.0.
	h := newHarness(t)
	h.addZip("a.zip", zipPackage("example.com/a", "1.0.0", "a.txt", "a")...)
	h.addZip("b.zip", zipPackage("example.com/b", "1.0.0", "b.txt", "b1")...)
	h.must(h.install("a.zip", "b.zip"))
	h.addZip("a.zip", zipPackage("example.com/b", "2.0.0", "b.txt", "b2")...)

	// Both requests cannot be satisfied by one package of that identity
	// (protocol/workspace.md §3).
	var re *graph.ResolutionError
	if err := h.update(); !errors.As(err, &re) {
		t.Fatalf("err = %v", err)
	}
	equal(t, h.summary(), []string{"example.com/a@1.0.0 explicit", "example.com/b@1.0.0 explicit"})
	if h.read("b.txt") != "b1" {
		t.Fatal(h.read("b.txt"))
	}
}

func TestUpdateNoArgumentsTwoSourcesGiveSamePackageReplacesWithIt(t *testing.T) {
	// a.zip now holds the package b.zip holds.
	h := newHarness(t)
	b := zipPackage("example.com/b", "1.0.0", "b.txt", "b")
	h.addZip("a.zip", zipPackage("example.com/a", "1.0.0", "a.txt", "a")...)
	h.addZip("b.zip", b...)
	h.must(h.install("a.zip", "b.zip"))
	source := findPackage(t, h.lock(), "example.com/b").Ref
	h.addZip("a.zip", b...)

	h.must(h.update())

	// One package satisfies both requests; b is not replaced, so it keeps its
	// source (protocol/workspace.md §5).
	equal(t, h.summary(), []string{"example.com/b@1.0.0 explicit"})
	if h.exists("a.txt") || single(t, h.lock()).Ref != source {
		t.Fatalf("a.txt %v, ref %s", h.exists("a.txt"), single(t, h.lock()).Ref)
	}
}

func TestUpdateNoArgumentsSourceGivesPackageKeptAsItIsFails(t *testing.T) {
	// a.zip now holds b 2.0.0, while b.zip, which b 1.0.0 was installed
	// from, is gone.
	h := newHarness(t)
	h.addZip("a.zip", zipPackage("example.com/a", "1.0.0", "a.txt", "a")...)
	path := h.addZip("b.zip", zipPackage("example.com/b", "1.0.0", "b.txt", "b1")...)
	h.must(h.install("a.zip", "b.zip"))
	h.addZip("a.zip", zipPackage("example.com/b", "2.0.0", "b.txt", "b2")...)
	os.Remove(path)

	// b is kept as it is (protocol/cli.md §5.3), so a's request for b 2.0.0
	// cannot be satisfied (protocol/workspace.md §3).
	var re *graph.ResolutionError
	if err := h.update(); !errors.As(err, &re) {
		t.Fatalf("err = %v", err)
	}
	equal(t, h.summary(), []string{"example.com/a@1.0.0 explicit", "example.com/b@1.0.0 explicit"})
	if h.read("b.txt") != "b1" {
		t.Fatal(h.read("b.txt"))
	}
}

func TestUpdateNewestVersionNeedsNewerDependencyPackageUpdatesBoth(t *testing.T) {
	for _, withArgument := range []bool{true, false} {
		t.Run(map[bool]string{true: "with argument", false: "without arguments"}[withArgument], func(t *testing.T) {
			// lib 1.0.0 is a dependency package of app 1.0.0; app 2.0.0 needs
			// lib >=1.1.0.
			h := newHarness(t)
			h.addLibAndApp(false, false)
			h.must(h.install(libID + "@1.0.0"))
			h.must(h.install(appID))
			h.must(h.uninstall(libID))
			h.addApp("2.0.0", ">=1.1.0")
			h.restart()

			if withArgument {
				h.must(h.update(appID))
			} else {
				h.must(h.update())
			}

			// A remote repository without a ref gives its highest version tag,
			// and the dependency package gives way (protocol/workspace.md §5,
			// protocol/cli.md §5.3).
			equal(t, h.summary(), []string{libID + "@1.1.0 dep", appID + "@2.0.0 explicit"})
			if h.read("lib/lib.dll") != "lib 1.1.0" {
				t.Fatal(h.read("lib/lib.dll"))
			}
		})
	}
}

func TestUpdateDependencyPackagesThatFitKeepTheirVersions(t *testing.T) {
	for _, withArgument := range []bool{true, false} {
		t.Run(map[bool]string{true: "with argument", false: "without arguments"}[withArgument], func(t *testing.T) {
			// a needs d, installed at 1.0.0; then a 1.1.0 and d 2.0.0 are
			// tagged.
			h := newHarness(t)
			h.addPackage("example.com/d", "1.0.0")
			h.addPackage("example.com/a", "1.0.0", "example.com/d", ">=1.0.0")
			h.must(h.install("example.com/a"))
			h.addPackage("example.com/d", "2.0.0")
			h.addPackage("example.com/a", "1.1.0", "example.com/d", ">=1.0.0")
			h.restart()

			if withArgument {
				h.must(h.update("example.com/a"))
			} else {
				h.must(h.update())
			}

			// d is updated only as far as Section 3 requires (protocol/cli.md
			// §5.3).
			equal(t, h.summary(), []string{"example.com/d@1.0.0 dep", "example.com/a@1.1.0 explicit"})
		})
	}
}

func TestUpdateLocalPathOfSameVersionWithOtherDependenciesKeepsTheRecordedOnes(t *testing.T) {
	for _, withArgument := range []bool{true, false} {
		t.Run(map[bool]string{true: "with argument", false: "without arguments"}[withArgument], func(t *testing.T) {
			// The local package needs lib; then its package file drops the
			// dependency.
			h := newHarness(t)
			h.addLib("1.0.0", false)
			h.addFile("src/x.txt", "x")
			h.addFile("src/lip.toml", lipJSON("example.com/local", "1.0.0",
				`"deps": { "`+libID+`": ">=1.0.0" }, `+selfAsset("x.txt", "x.txt")))
			h.must(h.install("./src"))
			h.addFile("src/lip.toml", lipJSON("example.com/local", "1.0.0", selfAsset("x.txt", "x.txt")))
			before := h.read(".lip/state.json")

			if withArgument {
				h.must(h.update("./src"))
			} else {
				h.must(h.update())
			}

			// The package is not replaced, so its recorded package file, which
			// needs lib, stays (protocol/workspace.md §3 rule 2 and §5).
			equal(t, h.summary(), []string{libID + "@1.0.0 dep", "example.com/local@1.0.0 explicit"})
			if after := h.read(".lip/state.json"); after != before {
				t.Fatalf("state file changed:\n%s\n%s", before, after)
			}
			if h.read("lib/lib.dll") != "lib 1.0.0" {
				t.Fatal(h.read("lib/lib.dll"))
			}
		})
	}
}

func TestUpdateTwoArgumentsOfOneIdentityGivingTheInstalledPackageRecordsTheRef(t *testing.T) {
	// lib 1.0.0 is installed from lib, which gives it again.
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.must(h.install(libID))

	h.must(h.update(libID, libID+"@1.0.0"))

	// Only its own candidates satisfy lib@1.0.0 (protocol/workspace.md §3).
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.0.0" || p.Ref != libID+"@1.0.0" {
		t.Fatalf("p = %+v", p)
	}
}

func TestUpdateTwoArgumentsOfOneIdentityNoPackageSatisfiesFails(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(libID))

	// No one package satisfies both requests (protocol/workspace.md §3).
	var re *graph.ResolutionError
	if err := h.update(libID+"@1.0.0", libID+"@1.1.0"); !errors.As(err, &re) {
		t.Fatalf("err = %v", err)
	}
	p := single(t, h.lock())
	if stringMember(p.Manifest, "version") != "1.1.0" || p.Ref != libID {
		t.Fatalf("p = %+v", p)
	}
}

// Layout: mods is created, and checked for a link, only when an operation
// may change the workspace (protocol/workspace.md §1, §2).

// addEmptyLib: version 1.0.0 of lib with no asset; entry holds extra
// members of its entry.
func (h *harness) addEmptyLib(entry string) {
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", entry))
}

// linkMods makes mods a link to the directory shared.
func (h *harness) linkMods() {
	h.t.Helper()
	if err := os.MkdirAll(h.path("shared"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := makeDirLink(h.path("shared"), h.path("mods")); err != nil {
		h.t.Skipf("cannot create a link: %v", err)
	}
}

func TestInstallPackageWritingNothingCreatesNoMods(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib("")

	h.must(h.install(libID))

	if h.dirExists("mods") || h.journalExists() || len(h.lock().Packages) != 1 {
		t.Fatal("mods or journal created")
	}
}

func TestInstallOnlyKeptPreservedFileModsIsALinkSucceeds(t *testing.T) {
	// The only file the package places exists and matches preserve, and no
	// hook command runs, so it is kept and the operation makes no change
	// (protocol/workspace.md §2, §4).
	h := newHarness(t)
	h.addFile("cfg.ini", "user")
	h.addTag(libID, "1.0.0", "lip.toml", lipJSON(libID, "1.0.0", selfAsset("cfg.ini", "cfg.ini")+
		`, "preserve": [ "cfg.ini" ]`), "cfg.ini", "default")
	h.linkMods()

	h.must(h.install(libID))

	if h.read("cfg.ini") != "user" || h.journalExists() || len(single(t, h.lock()).Files) != 0 {
		t.Fatal("changed")
	}
}

func TestInstallOnlyHookCommandsIgnoredCreatesNoMods(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib(`"hooks": { "pre_install": [ "check" ] }`)

	h.must(h.e.Install(context.Background(), []string{libID}, false, true, false))

	if h.dirExists("mods") || h.ran("check") != 0 || len(h.lock().Packages) != 1 {
		t.Fatal("mods created or hook ran")
	}
}

func TestInstallOnlyHookCommandsRunsThemWithoutCreatingMods(t *testing.T) {
	// Hook commands alone are no change, but they run with the journal
	// (protocol/workspace.md §7.4).
	h := newHarness(t)
	h.addEmptyLib(`"hooks": { "pre_install": [ "check" ] }`)
	journalExisted := false
	h.onRun["check"] = func() { journalExisted = h.journalExists() }

	h.must(h.install(libID))

	if !journalExisted || h.dirExists("mods") || h.journalExists() {
		t.Fatalf("journal existed %v, mods %v, journal %v", journalExisted, h.dirExists("mods"), h.journalExists())
	}
}

func TestInstallPackageWritingNothingModsIsALinkSucceeds(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib("")
	h.linkMods()

	h.must(h.install(libID))

	if len(h.lock().Packages) != 1 {
		t.Fatal(h.lock().Packages)
	}
}

func TestInstallPackageWritingNothingDryRunReportsNoMods(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib("")

	h.must(h.e.Install(context.Background(), []string{libID}, true, false, false))

	if h.rec.hasInfo("Dry run: would create the directory mods/") || h.dirExists("mods") {
		t.Fatal("reported mods")
	}
}

func TestInstallPackageWritingFilesCreatesMods(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)

	h.must(h.install(libID))

	if !h.dirExists("mods") || !h.exists("lib/lib.dll") {
		t.Fatal("not created")
	}
}

func TestInstallPackageWritingFilesModsIsALinkThrowsWithoutChanges(t *testing.T) {
	h := newHarness(t)
	h.addLib("1.0.0", false)
	h.linkMods()

	err := h.install(libID)

	var le *install.LayoutError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v", err)
	}
	h.unchangedThroughLink("")
}

func TestUninstallPackageOwningNothingCreatesNoMods(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib("")
	h.must(h.install(libID))

	h.must(h.uninstall(libID))

	if h.dirExists("mods") || h.journalExists() || len(h.lock().Packages) != 0 {
		t.Fatal("mods or journal created")
	}
}

func TestUninstallPackageOwningNothingModsIsALinkSucceeds(t *testing.T) {
	h := newHarness(t)
	h.addEmptyLib("")
	h.must(h.install(libID))
	h.linkMods()

	h.must(h.uninstall(libID))

	if len(h.lock().Packages) != 0 {
		t.Fatal(h.lock().Packages)
	}
}

func TestUninstallPackageWithPurgeAndHooksModsIsALinkThrowsWithoutChanges(t *testing.T) {
	// purge matches when it runs (protocol/workspace.md §6 rule 4), after
	// hook commands that may create what it matches, so it may change the
	// workspace even when nothing matches now.
	h := newHarness(t)
	h.addEmptyLib(`"purge": [ "cache/" ], "hooks": { "pre_uninstall": [ "make-cache" ] }`)
	h.must(h.install(libID))
	h.linkMods()
	before := h.read(".lip/state.json")

	err := h.uninstall(libID)

	var le *install.LayoutError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v", err)
	}
	h.unchangedThroughLink(before)
	if h.ran("make-cache") != 0 {
		t.Fatal("hook ran")
	}
}

func TestUninstallPackageWithPurgeMatchingNothingNoHooksCreatesNoMods(t *testing.T) {
	// Without hook commands the workspace stays as planned, and purge matches
	// nothing.
	h := newHarness(t)
	h.addEmptyLib(`"purge": [ "cache/" ]`)
	h.must(h.install(libID))
	h.linkMods()

	h.must(h.uninstall(libID))

	if len(h.lock().Packages) != 0 || h.journalExists() {
		t.Fatal("not uninstalled cleanly")
	}
}

// makeDirLink makes link a directory link to target: a symbolic link, or a
// junction where Windows does not allow the process to create one.
func makeDirLink(target, link string) error {
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr != nil {
			return fmt.Errorf("%v; mklink /J: %v %s", err, jerr, out)
		}
		return nil
	}
	return err
}

// Downloads are read into memory once and validated there (lip cfaa7a1).

func TestOpenCacheEntryDeletedAfterOpenStillServesTheOpenedArchive(t *testing.T) {
	const url = "https://example.com/a.zip"
	h := newHarness(t)
	h.addDownload(url, zipOf("x.txt", "x"))
	source, err := ref.Parse(url, false)
	if err != nil {
		t.Fatal(err)
	}
	a, err := h.e.newProvider().Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}

	// Another process may delete cache entries at any time
	// (protocol/userspace.md §3).
	if err := os.Remove(h.e.cache.Path(fetch.HTTPKey(url))); err != nil {
		t.Fatal(err)
	}

	equal(t, a.Source.Keys(), []string{"x.txt"})
	if got := readSource(t, a.Source, "x.txt"); got != "x" {
		t.Fatal(got)
	}
}

func TestOpenAssetCacheEntryReplacedAfterOpenStillServesTheOpenedArchive(t *testing.T) {
	const url = "https://example.com/a.zip"
	h := newHarness(t)
	h.addDownload(url, zipOf("a.txt", "a"))
	asset, err := h.e.newProvider().OpenAsset(context.Background(), []string{url})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(h.e.cache.Path(fetch.HTTPKey(url)), zipOf("b.txt", "b"), 0o644); err != nil {
		t.Fatal(err)
	}

	equal(t, asset.Keys(), []string{"a.txt"})
	if got := readSource(t, asset, "a.txt"); got != "a" {
		t.Fatal(got)
	}
}

func readSource(t *testing.T, s archive.Source, key string) string {
	t.Helper()
	r, err := s.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
