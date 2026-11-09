package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/install"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// Ports of ToothLockMigrationTests.cs and ToothInstallTests.cs (lip
// v0.38.0-beta.1), the measured progress of the engine and FormatError.

const otherID = "example.com/other"

// addToothLock writes a tooth_lock.json: app (explicitly installed,
// depending on lib) before lib, as lip v0.x may have listed them, their tags
// and their files in the workspace.
func (h *harness) addInstalledLibAndApp() (libCommit, appCommit string) {
	libCommit = h.addTag(libID, "1.0.0", "tooth.json", tooth(libID, "1.0.0", "lib/lib.dll", "", ""), "file.dll", "lib")
	appDeps := `{ "` + libID + `": ">=1.0.0" }`
	appCommit = h.addTag(appID, "1.0.0", "tooth.json", tooth(appID, "1.0.0", "app/app.dll", appDeps, ""), "file.dll", "app")
	h.addFile("lib/lib.dll", "lib")
	h.addFile("app/app.dll", "app")
	member := func(manifest string, locked bool, files ...string) string {
		quoted := make([]string, len(files))
		for i, f := range files {
			quoted[i] = fmt.Sprintf("%q", f)
		}
		return fmt.Sprintf(`{ "files": [ %s ], "locked": %v, "manifest": %s, "variant": "" }`,
			strings.Join(quoted, ", "), locked, manifest)
	}
	h.addFile("tooth_lock.json", `{ "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "packages": [ `+
		member(tooth(appID, "1.0.0", "app/app.dll", appDeps, ""), true, h.path("app/app.dll"))+", "+
		member(tooth(libID, "1.0.0", "lib/lib.dll", "", ""), false, "lib/lib.dll", "../x.dll", "")+` ] }`)
	h.addTag(otherID, "1.0.0", "lip.toml", lipJSON(otherID, "1.0.0", selfAsset("o.txt", "o.txt")), "o.txt", "o")
	return libCommit, appCommit
}

func TestListToothLockIsReadInMemoryInInstallationOrder(t *testing.T) {
	h := newHarness(t)
	h.addInstalledLibAndApp()
	before := h.read("tooth_lock.json")
	equal(t, h.summary(), []string{libID + "@1.0.0 dep", appID + "@1.0.0 explicit"})
	if h.read("tooth_lock.json") != before || h.hasLock() || len(h.git.listedRefs) != 0 {
		t.Fatal("changed or resolved")
	}
}

func TestInstallToothLockWritesDerivedWorkspaceFileAndDeletesToothLock(t *testing.T) {
	h := newHarness(t)
	libCommit, appCommit := h.addInstalledLibAndApp()
	h.must(h.install(otherID))
	if h.exists("tooth_lock.json") {
		t.Fatal("tooth_lock.json kept")
	}
	lf := h.lock()
	if lf.Side != "server" || lf.Platform != testPlatform {
		t.Fatal(lf.Side, lf.Platform)
	}
	var got []string
	for _, p := range lf.Packages {
		got = append(got, fmt.Sprintf("%s %v %s %s", pkgIdentity(&p), p.Explicit, p.Ref, p.Commit))
	}
	equal(t, got, []string{libID + " false " + libID + " " + libCommit, appID + " true " + appID + " " + appCommit,
		otherID + " true " + otherID + " " + lf.Packages[2].Commit})
	equal(t, lf.Packages[0].Files, []string{"lib/lib.dll"})
	equal(t, lf.Packages[1].Files, []string{"app/app.dll"})
	if !h.rec.hasInfo("Migrating tooth_lock.json of lip v0.x to the state file.") {
		t.Fatal(h.rec.infos)
	}

	// Uninstalling the migrated package deletes its files.
	h.must(h.uninstall(appID))
	if h.exists("app/app.dll") || h.exists("lib/lib.dll") {
		t.Fatal("files kept")
	}
}

func TestInstallToothLockCommitOfTagCannotBeObtainedFailsAndChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.addInstalledLibAndApp()
	h.git.failRefs[repoURL(libID)] = errors.New("offline")
	before := h.snapshot()
	err := h.install(otherID)
	if err == nil || !strings.Contains(err.Error(), "tooth_lock.json") {
		t.Fatalf("err = %v", err)
	}
	after := h.snapshot()
	delete(after, ".lip")
	equalMaps(t, after, before)
}

func TestInstallToothLockDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	h.addInstalledLibAndApp()
	before := h.snapshot()
	h.must(h.e.Install(context.Background(), []string{otherID}, true, false, false))
	equalMaps(t, h.snapshot(), before)
}

// On the client, a tooth.json's label "client" is the variant "client" and
// its label "" the variant "" (lip 26d8295).
func TestClientEnvironmentInstallsTheLabelOfTheVariant(t *testing.T) {
	h := newHarness(t)
	h.side = ""
	h.restart()
	h.addTag(libID, "1.0.0", "tooth.json", `{ "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
	  "tooth": "`+libID+`", "version": "1.0.0", "variants": [
	    { "label": "", "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "s.dll", "dest": "server.dll" } ] } ] },
	    { "label": "client", "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "c.dll", "dest": "client.dll" } ] } ] } ] }`,
		"s.dll", "s", "c.dll", "c")
	h.must(h.install(libID + "#client"))
	if !h.exists("client.dll") || h.exists("server.dll") {
		t.Fatal(h.snapshot())
	}
	h.must(h.uninstall(libID + "#client"))
	if h.exists("client.dll") {
		t.Fatal("kept")
	}
	h.must(h.install(libID))
	if !h.exists("server.dll") || h.exists("client.dll") {
		t.Fatal(h.snapshot())
	}
}

func TestProgressBars(t *testing.T) {
	h := newHarness(t)
	h.addLibAndApp(true, false)
	h.must(h.install(appID))
	stages := map[report.Stage][]report.Progress{}
	for _, p := range h.rec.progress {
		stages[p.Stage] = append(stages[p.Stage], p)
	}
	resolve := stages[report.StageResolve]
	if len(resolve) == 0 || resolve[0].Message != "Resolving packages" || resolve[0].Total != 1 {
		t.Fatalf("resolve = %+v", resolve)
	}
	var deps []report.Progress
	for _, p := range resolve {
		if p.Message == "Resolving dependencies" {
			deps = append(deps, p)
		}
	}
	if len(deps) < 2 || deps[0].Total != 0 || !deps[len(deps)-1].Done || deps[len(deps)-1].Current == 0 {
		t.Fatalf("solve bar = %+v", deps)
	}
	task := stages[report.StageTask]
	last := task[len(task)-1]
	if task[0].Total != 2 || !last.Done || last.Current != 2 {
		t.Fatalf("task = %+v", task)
	}
	if len(stages[report.StagePlace]) == 0 {
		t.Fatal("no place bar")
	}
	for id, ps := range groupByID(h.rec.progress) {
		if !ps[len(ps)-1].Done {
			t.Errorf("bar %s not done: %+v", id, ps)
		}
	}
}

func groupByID(ps []report.Progress) map[string][]report.Progress {
	out := map[string][]report.Progress{}
	for _, p := range ps {
		out[p.ID] = append(out[p.ID], p)
	}
	return out
}

func TestDownloadProgressAndCacheClean(t *testing.T) {
	h := newHarness(t)
	h.addZip("x.zip", "lip.toml", lipJSON("example.com/x", "1.0.0",
		`"assets": [ { "type": "zip", "urls": [ "https://example.com/a.zip" ], "place": [ { "src": "a", "dest": "a" } ] } ]`))
	h.addDownload("https://example.com/a.zip", zipOf("a", "content"))
	h.must(h.install("x.zip"))
	var download []report.Progress
	for _, p := range h.rec.progress {
		if p.Stage == report.StageDownload {
			download = append(download, p)
		}
	}
	if len(download) == 0 || !download[len(download)-1].Done || download[0].Message != "Downloading https://example.com/a.zip" {
		t.Fatalf("download = %+v", download)
	}

	h.rec.progress = nil
	h.must(h.e.CacheClean(context.Background()))
	if _, err := os.Stat(h.cacheDir); err == nil {
		t.Fatal("cache kept")
	}
	ps := h.rec.progress
	if len(ps) == 0 || ps[0].Stage != report.StageCacheClean || !ps[len(ps)-1].Done ||
		ps[len(ps)-1].Current != ps[len(ps)-1].Total {
		t.Fatalf("clean = %+v", ps)
	}
}

func TestTreeSourcePackProgressBars(t *testing.T) {
	rec := &recorder{}
	n := 0
	g := &remoteGit{rep: rec, ids: func() string { n++; return fmt.Sprint(n) }}
	ts := &treeSource{key: "example.com/x@1.0.0", git: g}
	ts.message.Store(ptr("Listing files of example.com/x@1.0.0"))
	fn := ts.progress(&ts.message)
	fn(gitproto.PackProgress{Attempt: 1, URL: "u"})
	fn(gitproto.PackProgress{Attempt: 1, URL: "u", Objects: 1, Total: 2, Bytes: 10})
	fn(gitproto.PackProgress{Attempt: 1, URL: "u", Objects: 2, Total: 2, Bytes: 20, Done: true})
	fn(gitproto.PackProgress{Attempt: 2, URL: "u", Bytes: 5})
	ps := rec.progress
	if len(ps) != 4 || ps[0].ID != "1" || ps[3].ID != "2" || ps[2].Current != 2 || !ps[2].Done ||
		ps[0].Stage != report.StageGitFetch || ps[0].Package != "example.com/x@1.0.0" ||
		ps[3].Message != "Listing files of example.com/x@1.0.0 (5 bytes received)" {
		t.Fatalf("%+v", ps)
	}
}

func TestProviderValidatesDownloadsAndFallsBackFromTheProxy(t *testing.T) {
	h := newHarness(t)
	// The proxy serves an error page; the original URL the archive.
	h.addDownload("https://proxy.example/o/r/releases/a.zip", []byte("<html>"))
	h.addDownload("https://github.com/o/r/releases/a.zip", zipOf("f", "1"))
	p := newProvider(h.git, fetch.NewHTTPCache(h.cacheDir, nil), "https://proxy.example", h.e.client)
	src, err := p.OpenAsset(context.Background(), []string{"https://github.com/o/r/releases/a.zip"})
	if err != nil {
		t.Fatal(FormatError(err))
	}
	keys, _ := archive.ListKeys(src)
	equal(t, keys, []string{"f"})

	// Neither works: the reasons name what was downloaded.
	_, err = p.OpenAsset(context.Background(), []string{"https://github.com/o/r/releases/b.zip"})
	var ue *archive.UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %T %v", err, err)
	}
	msg := FormatError(err)
	if !strings.Contains(msg, "Could not download https://github.com/o/r/releases/b.zip, neither through the GitHub"+
		" proxy (https://proxy.example/o/r/releases/b.zip) nor directly.") {
		t.Fatal(msg)
	}

	h.addDownload("https://example.com/bad.zip", []byte("not a zip"))
	_, err = p.Open(context.Background(), ref.Ref{Kind: ref.KindURL, Target: "https://example.com/bad.zip"})
	if err == nil || !strings.Contains(FormatError(err), "The file downloaded from https://example.com/bad.zip") {
		t.Fatalf("%v", FormatError(err))
	}
}

func TestProviderResolveGitRefMessages(t *testing.T) {
	h := newHarness(t)
	p := newProvider(h.git, fetch.NewHTTPCache(h.cacheDir, nil), "", h.e.client)
	g, _ := ref.ParseGitRef("branch:main")
	_, err := p.ResolveGitRef(context.Background(), libID, g)
	if err == nil || err.Error() != libID+" has no branch 'main'. Check the spelling, and that it exists in the"+
		" repository https://"+libID+".git." {
		t.Fatalf("err = %v", err)
	}
	h.git.failRefs[repoURL(libID)] = errors.New("offline")
	_, err = p.ResolveGitRef(context.Background(), libID, g)
	if !isUnavailable(err) || err.Error() != "Could not look up the branch 'main' of "+libID+" (https://"+libID+".git)." {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.ListVersions(context.Background(), libID); !isUnavailable(err) ||
		err.Error() != "Could not list the versions of "+libID+" (the tags of https://"+libID+".git)." {
		t.Fatalf("err = %v", err)
	}
}

func TestFirstCandidateFailureMessage(t *testing.T) {
	h := newHarness(t)
	err := h.install(libID)
	want := "No package found for '" + libID + "' that can be installed in this workspace (server, win-x64). Every" +
		" place lip looked failed:\n  - " + libID + ": the repository has no version tags (tags named like 'v1.2.3')," +
		" so it has no version to install; a branch, tag or commit can be named with '@branch:<name>'," +
		" '@tag:<name>' or '@rev:<commit>'\nCheck the spelling of the package and the version you asked for."
	if err == nil || err.Error() != want || !errors.Is(err, ErrNoPackageFound) {
		t.Fatalf("err = %q", err)
	}
	if err := h.install(appID); !errors.Is(err, ErrNoPackageFound) {
		t.Fatalf("err = %v", err)
	}
	h.addLib("1.0.0", false)
	if err := h.install(libID); errors.Is(err, ErrNoPackageFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestFormatError(t *testing.T) {
	inner := &gitproto.StatusError{Verb: "GET", URL: "https://x", StatusCode: 404}
	err := &Error{Msg: "Cannot do it with https://user:token@example.com/a?token=s&service=git-upload-pack.",
		Err: &fetch.AggregateError{Message: "summary", Errs: []error{
			&archive.UnavailableError{Message: "first", Err: inner}, errors.New("second")}}}
	got := FormatError(err)
	if !strings.HasPrefix(got, "Cannot do it with https://***@example.com/a?token=***&service=git-upload-pack.\n"+
		"  caused by: summary\n  caused by: first\n  caused by: ") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "\nhint: The repository or file does not exist, or is private.") {
		t.Fatal(got)
	}
	if got := FormatError(&graph.ResolutionError{Message: "no set"}); !strings.Contains(got, "--no-dependencies") {
		t.Fatal(got)
	}
	if got := FormatError(&install.Error{Msg: "x"}); got != "x" {
		t.Fatal(got)
	}
}
