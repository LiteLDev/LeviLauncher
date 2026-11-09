package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Ports of ResolverTests.cs (lip v0.38.0-beta.1): Resolve(source, range,
// environment) (protocol/package.md §4), with package files and with
// tooth.json files (protocol/compatibility.md §2), over in-memory archives.

const (
	rSpec = "github.com/user/pkg"
	rURL  = "https://example.com/pkg.zip"
)

var (
	serverEnv = environment{Side: "server", Platform: "win-x64"}
	clientEnv = environment{Side: "client", Platform: "win-x64"}
)

func rLipJSON(id, version, variants string) string {
	return fmt.Sprintf(`{ "format": 4, "id": %q, "version": %q, "variants": { %s } }`, id, version, variants)
}

func simple4(id, version, members string) string {
	return rLipJSON(id, version, `"": [ `+rEntry("server", "win-x64", members)+` ]`)
}

func rEntry(side, platform, members string) string {
	if members != "" {
		members = ", " + members
	}
	return fmt.Sprintf(`{ "side": %q, "platform": %q%s }`, side, platform, members)
}

func marker(name string) string {
	return `"deps": { "example.com/marker/` + name + `": "*" }`
}

func tooth3(id, version string, variants ...string) string {
	return fmt.Sprintf(`{ "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
	  "tooth": %q, "version": %q, "variants": [ %s ] }`, id, version, strings.Join(variants, ", "))
}

func variant3(label, platform, m string) string {
	return fmt.Sprintf(`{ "label": %q, "platform": %q, "dependencies": { "example.com/marker/%s": ">=1.0.0" } }`,
		label, platform, m)
}

func markerOf(t *testing.T, c *candidate) string {
	t.Helper()
	if len(c.deps) != 1 {
		t.Fatalf("deps = %v", c.deps)
	}
	return strings.TrimPrefix(c.deps[0].Source.Target, "example.com/marker/")
}

// fakeArchives is FakeArchives: in-memory archives of URLs and of remote
// repositories with tags, branches and commits; each holds its lip.toml or
// tooth.json only.
type fakeArchives struct {
	urls      map[string]*string
	repos     map[string]*fakeRepo
	commits   int
	opened    []string
	resolved  []string
	listCount int
	failOnce  map[string]bool
}

type fakeRepo struct {
	tags, branches, commits map[string]string
}

func newFakeArchives() *fakeArchives {
	return &fakeArchives{urls: map[string]*string{}, repos: map[string]*fakeRepo{}, failOnce: map[string]bool{}}
}

func (f *fakeArchives) addURL(url string, content *string) { f.urls[url] = content }

func (f *fakeArchives) repo(spec string) *fakeRepo {
	k := ref.Fold(spec)
	if f.repos[k] == nil {
		f.repos[k] = &fakeRepo{tags: map[string]string{}, branches: map[string]string{}, commits: map[string]string{}}
	}
	return f.repos[k]
}

func (f *fakeArchives) addCommit(spec, content string) string {
	f.commits++
	c := fmt.Sprintf("%040x", f.commits)
	f.repo(spec).commits[c] = content
	return c
}

func (f *fakeArchives) addTag(spec, tag, content string) string {
	c := f.addCommit(spec, content)
	f.repo(spec).tags[tag] = c
	return c
}

func (f *fakeArchives) addBranch(spec, branch, content string) string {
	c := f.addCommit(spec, content)
	f.repo(spec).branches[branch] = c
	return c
}

func handle(key string, content *string, commit string) *archiveHandle {
	src := &memSource{files: map[string][]byte{}}
	name := "readme.txt"
	text := ""
	if content != nil {
		text = *content
		name = "lip.toml"
		if strings.Contains(text, `"format_version"`) {
			name = "tooth.json"
		} else {
			text = tomlFixture(name, text)
		}
	}
	src.keys = []string{name}
	src.files[name] = []byte(text)
	return &archiveHandle{Key: key, Source: src, Commit: commit}
}

func (f *fakeArchives) Open(_ context.Context, target ref.Ref) (*archiveHandle, error) {
	if f.failOnce[target.Target] {
		delete(f.failOnce, target.Target)
		return nil, unavailablef(nil, "timeout %s", target.Target)
	}
	content, ok := f.urls[target.Target]
	if !ok {
		return nil, unavailablef(nil, "404 %s", target.Target)
	}
	return handle(target.Target, content, ""), nil
}

func (f *fakeArchives) ListVersions(_ context.Context, spec string) ([]specVersion, error) {
	f.listCount++
	var out []specVersion
	for tag := range f.repo(spec).tags {
		if v, ok := semver.ParseVersionTag("refs/tags/" + tag); ok {
			out = append(out, specVersion{Version: v, Tag: tag})
		}
	}
	slices.SortFunc(out, func(a, b specVersion) int {
		return semver.CompareTagVersions(semver.TagVersion{Version: a.Version, Tag: a.Tag},
			semver.TagVersion{Version: b.Version, Tag: b.Tag})
	})
	return out, nil
}

func (f *fakeArchives) OpenVersion(_ context.Context, spec string, v specVersion) (*archiveHandle, error) {
	f.opened = append(f.opened, spec+"@"+v.Tag)
	c := f.repo(spec).tags[v.Tag]
	content := f.repo(spec).commits[c]
	return handle(spec+"@"+v.Version.String(), &content, c), nil
}

func (f *fakeArchives) ResolveGitRef(_ context.Context, spec string, g ref.GitRef) (string, error) {
	f.resolved = append(f.resolved, g.String())
	r := f.repo(spec)
	var c string
	switch g.Kind {
	case ref.GitRefVersion:
		c = r.tags["v"+g.Name]
	case ref.GitRefBranch:
		c = r.branches[g.Name]
	case ref.GitRefTag:
		c = r.tags[g.Name]
	default:
		if _, ok := r.commits[g.Name]; ok {
			c = g.Name
		}
	}
	if c == "" {
		return "", unavailablef(nil, "%s has no %s", spec, g)
	}
	return c, nil
}

func (f *fakeArchives) OpenCommit(_ context.Context, spec, commit string) (*archiveHandle, error) {
	f.opened = append(f.opened, spec+"@rev:"+commit)
	content := f.repo(spec).commits[commit]
	return handle(spec+"@rev:"+commit, &content, commit), nil
}

func (f *fakeArchives) OpenAsset(context.Context, []string) (archive.Source, error) {
	return nil, fmt.Errorf("not supported")
}

func (f *fakeArchives) Forget(string) error { return nil }

type resolveResult struct {
	candidates  []*candidate
	diagnostics []string
}

func resolveAll(t *testing.T, r *resolver, source string, rng graph.Constraint) resolveResult {
	t.Helper()
	s, err := ref.Parse(source, false)
	if err != nil {
		t.Fatal(err)
	}
	var res resolveResult
	seq := r.resolve(s, rng, func(d graph.Diagnostic) { res.diagnostics = append(res.diagnostics, d.Message) })
	for {
		c, err := seq.nextCandidate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if c == nil {
			return res
		}
		res.candidates = append(res.candidates, c)
	}
}

func rng(s string) graph.Constraint { return versionRange{semver.MustParseVersionRange(s)} }

func strp(s string) *string { return &s }

type resolverFixture struct {
	a        *fakeArchives
	warnings []string
}

func (f *resolverFixture) resolver(env environment) *resolver {
	return newResolver(context.Background(), f.a, env, func(w string) { f.warnings = append(f.warnings, w) })
}

func newFixture() *resolverFixture { return &resolverFixture{a: newFakeArchives()} }

func TestResolveURLSelectsEntryOfExactSideAndPlatform(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(rLipJSON("example.com/pkg", "1.0.0", `"": [ `+
		rEntry("server", "linux-x64", marker("linux"))+`, `+rEntry("client", "win-x64", marker("client"))+`, `+
		rEntry("server", "win-x64", marker("server"))+` ]`)))
	res := resolveAll(t, f.resolver(serverEnv), rURL, nil)
	if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != "server" {
		t.Fatalf("%+v", res)
	}
	c := res.candidates[0]
	if c.Identity() != "example.com/pkg" || c.ArchiveKey() != rURL || c.Commit() != "" {
		t.Fatalf("%+v", c)
	}
}

func TestResolveNoEntryForPlatformNoCandidate(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(simple4("example.com/pkg", "1.0.0", "")))
	res := resolveAll(t, f.resolver(environment{"server", "linux-x64"}), rURL, nil)
	if len(res.candidates) != 0 || len(res.diagnostics) != 1 ||
		res.diagnostics[0] != rURL+": its lip.toml has no entry of the default variant for the side server and the"+
			" platform linux-x64; it has entries for (server, win-x64); it does not support the platform linux-x64"+
			" (if the server runs on another platform and the workspace has no packages yet, pass it with"+
			" --platform)" {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolveUnknownVariantNoCandidate(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(rLipJSON("example.com/pkg", "1.0.0", `"": [ `+rEntry("server", "win-x64", "")+` ], "a": [ `+rEntry("server", "win-x64", "")+` ]`)))
	res := resolveAll(t, f.resolver(serverEnv), rURL+"#b", nil)
	if len(res.candidates) != 0 || !strings.HasSuffix(res.diagnostics[0], `its variants are the default variant, "a"`) {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolveVariantSelectsEntryOfVariantOfSource(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(rLipJSON("example.com/pkg", "1.0.0", `"": [ `+rEntry("server", "win-x64", marker("default"))+
		` ], "a/b": [ `+rEntry("server", "win-x64", marker("ab"))+` ]`)))
	for v, m := range map[string]string{"": "default", "a/b": "ab"} {
		source := rURL
		if v != "" {
			source += "#" + v
		}
		res := resolveAll(t, f.resolver(serverEnv), source, nil)
		if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != m || res.candidates[0].Variant() != v {
			t.Fatalf("%s: %+v", v, res)
		}
	}
}

func TestResolveArchiveNotAPackageNoCandidate(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, nil)
	res := resolveAll(t, f.resolver(serverEnv), rURL, nil)
	if len(res.candidates) != 0 || res.diagnostics[0] != rURL+": "+rURL+" is not a lip package: it has no lip.toml at its root." {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolveTransientOpenFailureIsRetriedByLaterResolve(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(simple4("example.com/pkg", "1.0.0", "")))
	f.a.failOnce[rURL] = true
	r := f.resolver(serverEnv)
	if res := resolveAll(t, r, rURL, nil); len(res.candidates) != 0 {
		t.Fatal("candidate despite failure")
	}
	if res := resolveAll(t, r, rURL, nil); len(res.candidates) != 1 {
		t.Fatal("failure kept")
	}
}

func TestResolveRemoteRepositoryTakesEveryVersionTagInPrecedenceOrder(t *testing.T) {
	f := newFixture()
	for _, v := range []string{"1.0.0", "2.0.0", "1.5.0-beta", "1.5.0", "not-a-version"} {
		f.a.addTag(rSpec, "v"+v, simple4(rSpec, v, ""))
	}
	res := resolveAll(t, f.resolver(serverEnv), rSpec, nil)
	var got []string
	for _, c := range res.candidates {
		got = append(got, c.version.String()+" "+c.ArchiveKey())
	}
	equal(t, got, []string{"2.0.0 " + rSpec + "@2.0.0", "1.5.0 " + rSpec + "@1.5.0", "1.5.0-beta " + rSpec + "@1.5.0-beta",
		"1.0.0 " + rSpec + "@1.0.0"})
}

func TestResolveRemoteRepositoryWithoutVersionTagsNoCandidate(t *testing.T) {
	f := newFixture()
	res := resolveAll(t, f.resolver(serverEnv), rSpec, nil)
	if len(res.candidates) != 0 || !strings.Contains(res.diagnostics[0], "has no version tags") {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolvePackageIDOfOtherRepositoryNoCandidate(t *testing.T) {
	f := newFixture()
	f.a.addTag(rSpec, "v1.0.0", simple4("github.com/user/other", "1.0.0", ""))
	res := resolveAll(t, f.resolver(serverEnv), rSpec, nil)
	if len(res.candidates) != 0 || res.diagnostics[0] != rSpec+"@1.0.0: its lip.toml declares the package id"+
		" github.com/user/other, not "+rSpec+", so it is not a package of this repository" {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolvePackageIDDiffersInCaseIsCandidateWithPackageIDOfPackageFile(t *testing.T) {
	f := newFixture()
	f.a.addTag(rSpec, "v1.0.0", simple4("github.com/User/Pkg", "1.0.0", ""))
	res := resolveAll(t, f.resolver(serverEnv), rSpec, nil)
	if len(res.candidates) != 1 || res.candidates[0].Identity() != "github.com/User/Pkg" {
		t.Fatalf("%+v", res)
	}
}

func TestResolveFormat4VersionDiffersFromTagVersionNoCandidate(t *testing.T) {
	for _, declared := range []string{"1.0.1", "1.0.0+build"} {
		f := newFixture()
		f.a.addTag(rSpec, "v1.0.0", simple4(rSpec, declared, ""))
		res := resolveAll(t, f.resolver(serverEnv), rSpec, nil)
		if len(res.candidates) != 0 || !strings.Contains(res.diagnostics[0], "not the tag version 1.0.0") {
			t.Fatalf("%q", res.diagnostics)
		}
	}
}

func TestResolveFormat3VersionDiffersFromTagVersionTakesTagVersionAndWarnsOnce(t *testing.T) {
	f := newFixture()
	f.a.addTag(rSpec, "v2.0.0", tooth3(rSpec, "1.0.0", variant3("", "", "x")))
	r := f.resolver(serverEnv)
	for i := 0; i < 2; i++ {
		res := resolveAll(t, r, rSpec, nil)
		if len(res.candidates) != 1 || res.candidates[0].version.String() != "2.0.0" ||
			stringMember(res.candidates[0].manifest, "version") != "2.0.0" {
			t.Fatalf("%+v", res)
		}
	}
	equal(t, f.warnings, []string{rSpec + "@2.0.0: its tooth.json declares the version 1.0.0, but it is tagged as" +
		" version 2.0.0; using the tag version 2.0.0."})
}

func TestResolveFormat3FromURLTakesDeclaredVersionWithoutWarning(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(tooth3(rSpec, "1.0.0", variant3("", "", "x"))))
	res := resolveAll(t, f.resolver(serverEnv), rURL, nil)
	if len(res.candidates) != 1 || res.candidates[0].version.String() != "1.0.0" || len(f.warnings) != 0 {
		t.Fatalf("%+v %v", res, f.warnings)
	}
}

func TestResolveRangeFiltersTagVersionsAndOpensOnlyThose(t *testing.T) {
	f := newFixture()
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		f.a.addTag(rSpec, "v"+v, simple4(rSpec, v, ""))
	}
	res := resolveAll(t, f.resolver(serverEnv), rSpec, rng("^1.0.0"))
	if len(res.candidates) != 2 || res.candidates[0].version.String() != "1.1.0" {
		t.Fatalf("%+v", res)
	}
	equal(t, f.a.opened, []string{rSpec + "@v1.1.0", rSpec + "@v1.0.0"})

	res = resolveAll(t, f.resolver(serverEnv), rSpec, rng(">=3.0.0"))
	if len(res.candidates) != 0 || res.diagnostics[0] != rSpec+": no version tag satisfies >=3.0.0; the repository"+
		" has the versions 2.0.0, 1.1.0, 1.0.0 (newest first)" {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestResolveLazyFirstCandidateOnlyOpensOnlyFirstTag(t *testing.T) {
	f := newFixture()
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		f.a.addTag(rSpec, "v"+v, simple4(rSpec, v, ""))
	}
	s, _ := ref.Parse(rSpec, false)
	c, err := f.resolver(serverEnv).resolve(s, nil, nil).nextCandidate(context.Background())
	if err != nil || c == nil || c.version.String() != "2.0.0" {
		t.Fatal(c, err)
	}
	equal(t, f.a.opened, []string{rSpec + "@v2.0.0"})
}

func TestResolveSameSourceTwiceListsAndReadsOnce(t *testing.T) {
	f := newFixture()
	f.a.addTag(rSpec, "v1.0.0", simple4(rSpec, "1.0.0", ""))
	r := f.resolver(serverEnv)
	reads := 0
	r.onRead = func(string) { reads++ }
	resolveAll(t, r, rSpec, nil)
	resolveAll(t, r, rSpec, nil)
	if f.a.listCount != 1 || reads != 1 || len(f.a.opened) != 1 {
		t.Fatalf("list %d, reads %d, opened %v", f.a.listCount, reads, f.a.opened)
	}
}

func TestResolveVersionRefOpensCommitOfTagByIdentity(t *testing.T) {
	f := newFixture()
	commit := f.a.addTag(rSpec, "v1.0.0", simple4(rSpec, "1.0.0", ""))
	res := resolveAll(t, f.resolver(serverEnv), rSpec+"@1.0.0", nil)
	if len(res.candidates) != 1 {
		t.Fatal(res.diagnostics)
	}
	c := res.candidates[0]
	if c.ArchiveKey() != rSpec+"@1.0.0" || c.Commit() != commit || c.source.String() != rSpec+"@1.0.0" {
		t.Fatalf("%+v", c)
	}
	equal(t, f.a.opened, []string{rSpec + "@rev:" + commit})
}

func TestResolveRefRequiresTagVersionOnlyForVersionTags(t *testing.T) {
	f := newFixture()
	f.a.addTag(rSpec, "v2.0.0", simple4(rSpec, "1.0.0", ""))
	f.a.addTag(rSpec, "nightly", simple4(rSpec, "1.0.0", ""))
	f.a.addBranch(rSpec, "main", simple4(rSpec, "1.0.0", ""))
	for source, ok := range map[string]bool{rSpec + "@tag:v2.0.0": false, rSpec + "@tag:nightly": true,
		rSpec + "@branch:main": true, rSpec + "@2.0.0": false} {
		res := resolveAll(t, f.resolver(serverEnv), source, nil)
		if (len(res.candidates) == 1) != ok {
			t.Fatalf("%s: %+v", source, res)
		}
	}
}

func TestResolveBranchRefResolvesOnceAndOpensCommit(t *testing.T) {
	f := newFixture()
	commit := f.a.addBranch(rSpec, "main", simple4(rSpec, "1.0.0", ""))
	r := f.resolver(serverEnv)
	resolveAll(t, r, rSpec+"@branch:main", nil)
	f.a.addBranch(rSpec, "main", simple4(rSpec, "2.0.0", ""))
	res := resolveAll(t, r, rSpec+"@branch:main", nil)
	if len(res.candidates) != 1 || res.candidates[0].Commit() != commit || res.candidates[0].ArchiveKey() != rSpec+"@rev:"+commit {
		t.Fatalf("%+v", res)
	}
	equal(t, f.a.resolved, []string{"branch:main"})
}

func TestResolveMissingRefNoCandidateWithDiagnostic(t *testing.T) {
	f := newFixture()
	res := resolveAll(t, f.resolver(serverEnv), rSpec+"@branch:none", nil)
	if len(res.candidates) != 0 || res.diagnostics[0] != rSpec+"@branch:none: "+rSpec+" has no branch:none" {
		t.Fatalf("%q", res.diagnostics)
	}
}

func TestPinCommitVersionRefResolvesToRecordedCommitAfterTagMoved(t *testing.T) {
	f := newFixture()
	recorded := f.a.addTag(rSpec, "v1.0.0", simple4(rSpec, "1.0.0", ""))
	f.a.addTag(rSpec, "v1.0.0", simple4(rSpec, "1.0.0", ""))
	r := f.resolver(serverEnv)
	for _, s := range []string{rSpec + "@1.0.0", rSpec + "@branch:main"} {
		src, _ := ref.Parse(s, false)
		r.PinCommit(src, strings.ToUpper(recorded))
	}
	res := resolveAll(t, r, "github.com/User/PKG@1.0.0", nil)
	if len(res.candidates) != 1 || res.candidates[0].Commit() != recorded || len(f.a.resolved) != 0 {
		t.Fatalf("%+v %v", res, f.a.resolved)
	}
	if res := resolveAll(t, r, rSpec+"@branch:main", nil); len(f.a.resolved) != 1 || len(res.candidates) != 0 {
		t.Fatal("branch ref pinned")
	}
}

func TestResolveFormat3DerivesEntryForEnvironment(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(tooth3(rSpec, "1.0.0", variant3("", "", "default"), variant3("client", "", "client"))))
	res := resolveAll(t, f.resolver(serverEnv), rURL, nil)
	if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != "default" {
		t.Fatalf("%+v", res)
	}
	// On the client side too, the label "" is the variant "" (lip 26d8295).
	res = resolveAll(t, f.resolver(clientEnv), rURL, nil)
	if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != "default" {
		t.Fatalf("%+v", res)
	}
	res = resolveAll(t, f.resolver(clientEnv), rURL+"#client", nil)
	if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != "client" {
		t.Fatalf("%+v", res)
	}
}

func TestResolveClientVariantIsItsOwnVariant(t *testing.T) {
	// A source with "#client" names the variant "client", of a package file
	// and of a tooth.json alike (lip 26d8295).
	f := newFixture()
	f.a.addURL(rURL, strp(rLipJSON(rSpec, "1.0.0", `"": [ `+rEntry("client", "win-x64", marker("default"))+
		` ], "client": [ `+rEntry("client", "win-x64", marker("client"))+` ]`)))
	r := f.resolver(clientEnv)
	for source, marker := range map[string]string{rURL + "#client": "client", rURL: "default"} {
		res := resolveAll(t, r, source, nil)
		if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != marker {
			t.Fatalf("%s: %+v", source, res)
		}
	}
	f.a.addURL("https://example.com/t.zip", strp(tooth3(rSpec, "1.0.0", variant3("", "", "default"),
		variant3("client", "", "client"))))
	for _, env := range []environment{serverEnv, clientEnv} {
		res := resolveAll(t, f.resolver(env), "https://example.com/t.zip#client", nil)
		if len(res.candidates) != 1 || markerOf(t, res.candidates[0]) != "client" ||
			res.candidates[0].Identity() != rSpec+"#client" {
			t.Fatalf("%s: %+v", env.Side, res)
		}
	}
}

func TestResolveOutsideRangeNoCandidate(t *testing.T) {
	f := newFixture()
	f.a.addURL(rURL, strp(simple4("example.com/pkg", "1.0.0", "")))
	res := resolveAll(t, f.resolver(serverEnv), rURL, rng(">=2.0.0"))
	if len(res.candidates) != 0 || res.diagnostics[0] != rURL+": version 1.0.0 does not satisfy >=2.0.0" {
		t.Fatalf("%q", res.diagnostics)
	}
}
