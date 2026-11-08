package graph

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// Ports tests/Lip.Core.Tests/Mcmod/Resolution/GraphSolverTests.cs and
// GraphSolverLazyTests.cs.

const leviLamina = "github.com/LiteLDev/LeviLamina"

func TestSolve_SingleRequest_PicksFirstCandidate(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/a/a", "2.0.0", "1.0.0")

	got := solve(t, r, []Request{request("github.com/a/a")}).Order

	equal(t, []string{"github.com/a/a@2.0.0"}, names(got))
}

func TestSolve_LeviLaminaWithManyVersionsAndTenMods_PicksNewestCommonVersion(t *testing.T) {
	r := newResolver()
	var versions []string
	for major := 2; major >= 0; major-- {
		for minor := 99; minor >= 0; minor-- {
			versions = append(versions, fmt.Sprintf("%d.%d.0", major, minor))
		}
	}
	r.addVersions(leviLamina, versions...)
	var requests []Request
	for i := range 10 {
		spec := fmt.Sprintf("github.com/mods/mod%d", i)
		r.add(pkg(spec, "2.0.0", pkgOpts{deps: []Dependency{
			dep(leviLamina, fmt.Sprintf(">=1.%d.0 <2.%d.0", i*5, i*5+50))}}))
		r.add(pkg(spec, "1.0.0", pkgOpts{deps: []Dependency{dep(leviLamina, "*")}}))
		requests = append(requests, request(spec))
	}
	// The newest Old needs an ancient LeviLamina, so Old must fall back to 1.0.0.
	r.add(pkg("github.com/mods/old", "2.0.0", pkgOpts{deps: []Dependency{dep(leviLamina, "<1.0.0")}}))
	r.add(pkg("github.com/mods/old", "1.0.0", pkgOpts{deps: []Dependency{
		dep(leviLamina, ">=2.0.0 <3.0.0")}}))
	requests = append(requests, request("github.com/mods/old"))

	got := names(solve(t, r, requests).Order)

	if got[0] != leviLamina+"@2.49.0" || !slices.Contains(got, "github.com/mods/old@1.0.0") ||
		len(got) != 12 {
		t.Fatal(got)
	}
	for i := range 10 {
		if !slices.Contains(got, fmt.Sprintf("github.com/mods/mod%d@2.0.0", i)) {
			t.Fatal(got)
		}
	}
	seen := map[resolveCall]bool{}
	for _, c := range r.calls {
		if seen[c] {
			t.Fatalf("resolved twice: %+v", c)
		}
		seen[c] = true
	}
}

func TestSolve_DefaultVariantDependsOnItsVariants_SolvesThreeIdentities(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/t/t#b", "*"), dep("github.com/t/t#a", "*")}}))
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{variant: "a"}))
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{variant: "b"}))

	got := solve(t, r, []Request{request("github.com/t/t")}).Order

	equal(t, []string{"github.com/t/t#a@1.0.0", "github.com/t/t#b@1.0.0", "github.com/t/t@1.0.0"},
		names(got))
}

func TestSolve_TwoVariantsRequested_SolvesBothWithSharedDependency(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{variant: "a",
		deps: []Dependency{dep("github.com/l/l", "*")}}))
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{variant: "b",
		deps: []Dependency{dep("github.com/l/l", "*")}}))
	r.addVersions("github.com/l/l", "1.0.0")

	g := solve(t, r, []Request{request("github.com/t/t#a"), request("github.com/t/t#b")})

	equal(t, []string{"github.com/l/l@1.0.0", "github.com/t/t#a@1.0.0", "github.com/t/t#b@1.0.0"},
		names(g.Order))
	if !slices.Equal(g.RequestSatisfiers, []int{1, 2}) || len(g.Dependencies[0]) != 0 ||
		!slices.Equal(g.Dependencies[1], []int{0}) || !slices.Equal(g.Dependencies[2], []int{0}) {
		t.Fatalf("%+v", g)
	}
}

func TestSolve_TwoSourcesOfSameIdentity_Fails(t *testing.T) {
	for _, otherID := range []string{"github.com/x/x", "github.com/X/x"} {
		r := newResolver()
		r.addVersions("github.com/x/x", "1.0.0")
		r.add(pkg(otherID, "1.0.0", pkgOpts{source: "https://example.com/x.zip"}))

		msg := solveFails(t, r, []Request{request("github.com/x/x"),
			request("https://example.com/x.zip")})

		want := otherID + "@1.0.0 (from https://example.com/x.zip@1.0.0) has the same identity as" +
			" github.com/x/x@1.0.0 (from github.com/x/x@1.0.0)"
		if !strings.Contains(msg, want) {
			t.Fatal(msg)
		}
	}
}

func TestSolve_SameIdentityClash_BacktracksToOtherVersion(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "2.0.0", pkgOpts{deps: []Dependency{
		dep("https://example.com/b.zip", "*")}}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{source: "https://example.com/b.zip"}))
	r.addVersions("github.com/b/b", "2.0.0")

	got := solve(t, r, []Request{request("github.com/a/a"), request("github.com/b/b")}).Order

	equal(t, []string{"github.com/a/a@1.0.0", "github.com/b/b@2.0.0"}, names(got))
}

func TestSolve_ModNameClash_BacktracksToOlderVersion(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "2.0.0", pkgOpts{mod: "Shared"}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "A"}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{mod: "sHARED"}))

	got := solve(t, r, []Request{request("github.com/a/a"), request("github.com/b/b")}).Order

	equal(t, []string{"github.com/a/a@1.0.0", "github.com/b/b@1.0.0"}, names(got))
}

func TestSolve_ModNameClashDeepInGraph_BacktracksEarlierChoice(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/x/x", "2.0.0", pkgOpts{deps: []Dependency{dep("github.com/y/y", "*")}}))
	r.add(pkg("github.com/x/x", "1.0.0", pkgOpts{}))
	r.add(pkg("github.com/y/y", "1.0.0", pkgOpts{mod: "Z"}))
	r.add(pkg("github.com/z/z", "1.0.0", pkgOpts{mod: "z"}))

	got := solve(t, r, []Request{request("github.com/x/x"), request("github.com/z/z")}).Order

	equal(t, []string{"github.com/x/x@1.0.0", "github.com/z/z@1.0.0"}, names(got))
}

func TestSolve_ModNameClashUnavoidable_ListsBlockedCandidates(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "Foo"}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{mod: "FOO"}))

	msg := solveFails(t, r, []Request{request("github.com/a/a"), request("github.com/b/b")})

	want := "You requested github.com/b/b (any version), but no version of it fits: " +
		"github.com/b/b@1.0.0 (from github.com/b/b@1.0.0) is a mod of the same mod name 'FOO' " +
		"as github.com/a/a@1.0.0 (from github.com/a/a@1.0.0)."
	if !strings.Contains(msg, want) {
		t.Fatal(msg)
	}
}

func TestSolve_ModAmongFilePackages_DownloadsOnlyItsMcmodAssetOnce(t *testing.T) {
	r := newResolver()
	var downloads []string
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "Same", downloads: &downloads}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{}))
	r.add(pkg("github.com/c/c", "1.0.0", pkgOpts{}))

	got := solve(t, r, []Request{request("github.com/a/a"), request("github.com/b/b"),
		request("github.com/c/c")}).Order

	if len(got) != 3 {
		t.Fatal(names(got))
	}
	equal(t, []string{"https://example.com/github.com/a/a/1.0.0.mcmod"}, downloads)
}

func TestSolve_InvalidModManifest_BlocksCandidate(t *testing.T) {
	for _, manifest := range []func(string) (string, error){missingManifest, invalidManifest} {
		r := newResolver()
		r.add(pkg("github.com/a/a", "2.0.0", pkgOpts{mcmod: manifest}))
		r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "A"}))

		got := solve(t, r, []Request{request("github.com/a/a")}).Order

		equal(t, []string{"github.com/a/a@1.0.0"}, names(got))
	}
}

func TestSolve_OnlyCandidateHasNoModManifest_FailsNamingIt(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mcmod: missingManifest}))

	msg := solveFails(t, r, []Request{request("github.com/a/a")})

	if !strings.Contains(msg, "Cannot install github.com/a/a 1.0.0: its asset of type \"mcmod\""+
		" has no manifest.json") {
		t.Fatal(msg)
	}
}

func TestSolve_McmodAssetCannotBeObtained_BlocksCandidate(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "2.0.0", pkgOpts{mcmod: unavailableAsset}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "A"}))

	got := solve(t, r, []Request{request("github.com/a/a")}).Order

	equal(t, []string{"github.com/a/a@1.0.0"}, names(got))
}

func TestSolve_McmodAssetCannotBeObtained_KeepsCause(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mcmod: unavailableAsset}))

	_, err := Solve(context.Background(), Problem{Requests: []Request{request("github.com/a/a")},
		Resolver: r})

	re, ok := err.(*ResolutionError)
	if !ok || len(re.Causes) != 1 || !strings.Contains(re.Causes[0].Error(), "404") {
		t.Fatal(err)
	}
}

func TestSolve_InstalledModOfSameModName_BlocksCandidate(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/a/a", "2.0.0", pkgOpts{mod: "Taken"}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "A"}))
	installed := pkg("github.com/other/other", "1.0.0", pkgOpts{mod: "taken"})

	got := solve(t, r, []Request{request("github.com/a/a")}, installed).Order

	equal(t, []string{"github.com/a/a@1.0.0", "github.com/other/other@1.0.0"}, names(got))
}

func TestSolve_InstalledModsOfSameModName_Fails(t *testing.T) {
	a := pkg("github.com/a/a", "1.0.0", pkgOpts{mod: "Same"})
	b := pkg("github.com/b/b", "1.0.0", pkgOpts{mod: "same"})

	msg := solveFails(t, newResolver(), nil, a, b)

	if !strings.Contains(msg, "are mods of the same mod name") {
		t.Fatal(msg)
	}
}

func TestSolve_InstalledPackagesOfSameIdentity_Fails(t *testing.T) {
	a := pkg("github.com/a/a", "1.0.0", pkgOpts{})
	b := pkg("github.com/A/a", "2.0.0", pkgOpts{})

	msg := solveFails(t, newResolver(), nil, a, b)

	if !strings.Contains(msg, "have the same identity") {
		t.Fatal(msg)
	}
}

func TestSolve_InstalledWithSameIdentity_BlocksRequest(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/x/x", "2.0.0")
	installed := pkg("github.com/X/x", "1.0.0", pkgOpts{})

	msg := solveFails(t, r, []Request{request("github.com/x/x")}, installed)

	if !strings.Contains(msg, "github.com/x/x@2.0.0 (from github.com/x/x@2.0.0) has the same"+
		" identity as github.com/X/x@1.0.0 (from github.com/X/x@1.0.0) (installed)") {
		t.Fatal(msg)
	}
}

func TestSolve_InstalledSamePackage_SatisfiesRequestAndInstalledDependencies(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/x/x", "2.0.0", "1.0.0")
	r.addVersions("github.com/dep/dep", "1.0.0")
	installedX := pkg("github.com/x/x", "1.0.0", pkgOpts{})
	installedP := pkg("github.com/p/p", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/dep/dep", "*")}})

	got := solve(t, r, []Request{request("github.com/x/x")}, installedX, installedP).Order

	equal(t, []string{"github.com/dep/dep@1.0.0", "github.com/p/p@1.0.0", "github.com/x/x@1.0.0"},
		names(got))
	if got[2] != Candidate(installedX) {
		t.Fatal("not the installed candidate")
	}
}

func TestSolve_SamePackageSatisfiesSeveralDependencies_AddsItOnce(t *testing.T) {
	r := newResolver()
	lower := strings.ToLower(leviLamina)
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{
		dep(leviLamina, "*"), dep(lower, "*")}}))
	r.addVersions(leviLamina, "1.0.0")
	r.add(pkg(leviLamina, "1.0.0", pkgOpts{source: lower}))

	got := solve(t, r, []Request{request("github.com/a/a"), request(leviLamina)}).Order

	equal(t, []string{leviLamina + "@1.0.0", "github.com/a/a@1.0.0"}, names(got))
}

func TestSolve_DependencyWithoutRefBeforeRef_PackageOfRefSatisfiesBoth(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/s/s", "2.0.0", "1.0.0")
	r.add(pkg("github.com/s/s", "2.5.0", pkgOpts{source: "github.com/s/s@branch:main"}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/s/s", "*"), dep("github.com/b/b", "*")}}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/s/s@branch:main", "*")}}))

	g := solve(t, r, []Request{request("github.com/a/a")})

	equal(t, []string{"github.com/s/s@2.5.0", "github.com/b/b@1.0.0", "github.com/a/a@1.0.0"},
		names(g.Order))
	deps := slices.Sorted(slices.Values(g.Dependencies[2]))
	if !slices.Equal(deps, []int{0, 1}) {
		t.Fatal(g.Dependencies)
	}
}

func TestSolve_RequestsWithAndWithoutRef_PackageOfRefStandsIn(t *testing.T) {
	for _, refFirst := range []bool{true, false} {
		r := newResolver()
		rev := "github.com/s/s@rev:" + strings.Repeat("a", 40)
		r.addVersions("github.com/s/s", "2.0.0", "1.0.0")
		r.add(pkg("github.com/s/s", "2.5.0", pkgOpts{source: rev}))
		requests := []Request{request(rev), request("github.com/s/s")}
		if !refFirst {
			slices.Reverse(requests)
		}

		g := solve(t, r, requests)

		equal(t, []string{"github.com/s/s@2.5.0"}, names(g.Order))
		if !slices.Equal(g.RequestSatisfiers, []int{0, 0}) {
			t.Fatal(g.RequestSatisfiers)
		}
	}
}

func TestSolve_DependencyWithoutRefOutsideVersionOfRef_Fails(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/s/s", "2.0.0", "1.0.0")
	r.add(pkg("github.com/s/s", "2.5.0", pkgOpts{source: "github.com/s/s@branch:main"}))
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/s/s", "<2.0.0"), dep("github.com/b/b", "*")}}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/s/s@branch:main", "*")}}))

	solveFails(t, r, []Request{request("github.com/a/a")})
}

func TestSolve_TwoRefsOfSameRepository_DoNotStandInForEachOther(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/s/s", "2.5.0", pkgOpts{source: "github.com/s/s@branch:main"}))
	r.add(pkg("github.com/s/s", "1.0.0", pkgOpts{source: "github.com/s/s@1.0.0"}))

	solveFails(t, r, []Request{request("github.com/s/s@branch:main"), request("github.com/s/s@1.0.0")})
}

func TestSolve_PackageOfRef_DoesNotStandInForOtherVariant(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/s/s", "2.5.0", pkgOpts{source: "github.com/s/s@branch:main"}))
	r.add(pkg("github.com/s/s", "2.0.0", pkgOpts{variant: "lua"}))

	got := solve(t, r, []Request{request("github.com/s/s@branch:main"),
		request("github.com/s/s#lua")}).Order

	equal(t, []string{"github.com/s/s@2.5.0", "github.com/s/s#lua@2.0.0"}, names(got))
}

func TestSolve_InstalledPackageOfRef_DoesNotStandInForAnotherVariant(t *testing.T) {
	r := newResolver()
	r.add(pkg("github.com/t/t", "1.0.0", pkgOpts{variant: "client"}))
	r.add(pkg("github.com/r/r", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/t/t#client", "*")}}))
	installed := pkg("github.com/t/t", "2.5.0", pkgOpts{source: "github.com/t/t@branch:main"})

	got := solve(t, r, []Request{request("github.com/r/r")}, installed).Order

	equal(t, []string{"github.com/t/t@2.5.0", "github.com/t/t#client@1.0.0", "github.com/r/r@1.0.0"},
		names(got))
}

func TestSolve_Unsatisfiable_FailsWithFailingDependency(t *testing.T) {
	r := newResolver()
	r.addVersions(leviLamina, "1.2.0", "1.1.0")
	r.add(pkg("github.com/mods/mod", "1.0.0", pkgOpts{deps: []Dependency{
		dep(leviLamina, ">=99.0.0")}}))

	msg := solveFails(t, r, []Request{request("github.com/mods/mod")})

	want := "Cannot find versions of the requested packages and their dependencies that can all" +
		" be installed together:\n  - github.com/mods/mod@1.0.0 requires " + leviLamina +
		" >=99.0.0, but no version of it is available."
	if !strings.HasPrefix(msg, want) {
		t.Fatal(msg)
	}
}

type diagnosingResolver struct{ count int }

func (r *diagnosingResolver) ResolveLazy(_ context.Context, source Ref, _ Constraint,
	onDiagnostic func(Diagnostic)) Sequence {
	for i := range r.count {
		onDiagnostic(Diagnostic{Message: fmt.Sprintf("reason %d", i), Cause: fmt.Errorf("cause %d", i)})
	}
	return &SliceSequence{}
}

func TestSolve_NoCandidate_ListsDiagnosticsAndKeepsCauses(t *testing.T) {
	_, err := Solve(context.Background(), Problem{Requests: []Request{request("github.com/x/x")},
		Resolver: &diagnosingResolver{count: 7}})

	re := err.(*ResolutionError)
	want := "Cannot find versions of the requested packages and their dependencies that can all" +
		" be installed together:\n  - You requested github.com/x/x (any version), but no version" +
		" of it is available:\n      - reason 0\n      - reason 1\n      - reason 2\n      -" +
		" reason 3\n      - reason 4\n      ... and 2 more"
	if re.Message != want || len(re.Causes) != 7 {
		t.Fatalf("%q %v", re.Message, re.Causes)
	}
}

func TestSolve_ManyFailures_ListsTenReasons(t *testing.T) {
	r := newResolver()
	var requests []Request
	for i := range 12 {
		spec := fmt.Sprintf("github.com/m/m%02d", i)
		requests = append(requests, request(spec))
	}

	msg := solveFails(t, r, requests)

	// The first request fails; the search backs out at once, recording one reason.
	if !strings.HasSuffix(msg, "  - You requested github.com/m/m00 (any version), but no version"+
		" of it is available.") {
		t.Fatal(msg)
	}
}

func TestSolve_ManyPackagesAndVersions_CompletesQuickly(t *testing.T) {
	r := newResolver()
	for i := range 50 {
		spec := fmt.Sprintf("github.com/p/p%d", i)
		for v := 99; v >= 0; v-- {
			var deps []Dependency
			for j := i + 1; j <= min(i+2, 49); j++ {
				deps = append(deps, dep(fmt.Sprintf("github.com/p/p%d", j), "<90.0.0"))
			}
			r.add(pkg(spec, fmt.Sprintf("%d.0.0", v), pkgOpts{deps: deps}))
		}
	}
	start := time.Now()

	got := names(solve(t, r, []Request{request("github.com/p/p0")}).Order)

	if len(got) != 50 || got[0] != "github.com/p/p49@89.0.0" || got[49] != "github.com/p/p0@99.0.0" {
		t.Fatal(got)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestSolve_DeepUnsatisfiableChain_FailsQuickly(t *testing.T) {
	r := newResolver()
	for i := range 50 {
		spec := fmt.Sprintf("github.com/p/p%d", i)
		for v := 99; v >= 0; v-- {
			next := fmt.Sprintf("github.com/p/p%d", i+1)
			if i == 49 {
				next = "github.com/missing/missing"
			}
			r.add(pkg(spec, fmt.Sprintf("%d.0.0", v), pkgOpts{deps: []Dependency{dep(next, "*")}}))
		}
	}
	start := time.Now()

	msg := solveFails(t, r, []Request{request("github.com/p/p0")})

	if !strings.Contains(msg, "github.com/p/p49@99.0.0 requires github.com/missing/missing *, "+
		"but no version of it is available") {
		t.Fatal(msg)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestSolve_LongDependencyChain_DoesNotOverflowStack(t *testing.T) {
	const length = 5000
	r := newResolver()
	for i := range length {
		var deps []Dependency
		if i+1 < length {
			deps = []Dependency{dep(fmt.Sprintf("github.com/c/c%d", i+1), "*")}
		}
		r.add(pkg(fmt.Sprintf("github.com/c/c%d", i), "1.0.0", pkgOpts{deps: deps}))
	}

	got := names(solve(t, r, []Request{request("github.com/c/c0")}).Order)

	if len(got) != length || got[0] != fmt.Sprintf("github.com/c/c%d@1.0.0", length-1) ||
		got[length-1] != "github.com/c/c0@1.0.0" {
		t.Fatal(len(got))
	}
}

func TestSolve_Cancelled_StopsSearch(t *testing.T) {
	const length = 300
	r := newResolver()
	for i := range length {
		r.add(pkg(fmt.Sprintf("github.com/c/c%d", i), "1.0.0", pkgOpts{deps: []Dependency{
			dep(fmt.Sprintf("github.com/c/c%d", i+1), "*")}}))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Solve(ctx, Problem{Requests: []Request{request("github.com/c/c0")}, Resolver: r})

	if err != context.Canceled {
		t.Fatal(err)
	}
}

func TestSolve_DependencyCycle_OrdersCycleByLowercaseIdentity(t *testing.T) {
	r := newResolver()
	r.add(pkg("example.com/Zed", "1.0.0", pkgOpts{deps: []Dependency{dep("example.com/beta", "*")}}))
	r.add(pkg("example.com/beta", "1.0.0", pkgOpts{deps: []Dependency{dep("example.com/Alpha", "*")}}))
	r.add(pkg("example.com/Alpha", "1.0.0", pkgOpts{deps: []Dependency{
		dep("example.com/beta", "*"), dep("example.com/Base", "*")}}))
	r.add(pkg("example.com/Base", "1.0.0", pkgOpts{}))

	got := solve(t, r, []Request{request("example.com/Zed")}).Order

	equal(t, []string{"example.com/Base@1.0.0", "example.com/Alpha@1.0.0", "example.com/beta@1.0.0",
		"example.com/Zed@1.0.0"}, names(got))
}

func TestInstallOrder_Cycle_OrdersByLowercaseIdentityNotOrdinal(t *testing.T) {
	b := pkg("example.com/B", "1.0.0", pkgOpts{})
	a := pkg("example.com/a", "1.0.0", pkgOpts{})
	variant := pkg("example.com/a", "1.0.0", pkgOpts{variant: "x"})

	got := InstallOrder([]Candidate{b, variant, a}, func(c Candidate) []Candidate {
		switch c {
		case Candidate(a):
			return []Candidate{b, variant}
		case Candidate(b):
			return []Candidate{variant}
		}
		return []Candidate{a}
	})

	equal(t, []string{"example.com/a@1.0.0", "example.com/a#x@1.0.0", "example.com/B@1.0.0"},
		names(got))
}

func TestInstallOrder_DisconnectedPackages_OrdersByLowercaseIdentity(t *testing.T) {
	b := pkg("example.com/b", "1.0.0", pkgOpts{})
	a := pkg("example.com/A", "1.0.0", pkgOpts{})
	c := pkg("example.com/C", "1.0.0", pkgOpts{})

	got := InstallOrder([]Candidate{c, b, a}, func(Candidate) []Candidate { return nil })

	equal(t, []string{"example.com/A@1.0.0", "example.com/b@1.0.0", "example.com/C@1.0.0"}, names(got))
}

func TestInstallOrder_DependencyOutsideSet_IsIgnored(t *testing.T) {
	a := pkg("example.com/a", "1.0.0", pkgOpts{})
	outside := pkg("example.com/z", "1.0.0", pkgOpts{})
	b := pkg("example.com/b", "1.0.0", pkgOpts{})

	got := InstallOrder([]Candidate{b, a}, func(c Candidate) []Candidate {
		if c == Candidate(a) {
			return []Candidate{b, outside}
		}
		return nil
	})

	equal(t, []string{"example.com/b@1.0.0", "example.com/a@1.0.0"}, names(got))
}

func TestInstallOrder_SameKeyTwice_CountsOnce(t *testing.T) {
	a1 := pkg("example.com/a", "1.0.0", pkgOpts{})
	a2 := pkg("example.com/a", "1.0.0", pkgOpts{})

	got := InstallOrder([]Candidate{a1, a2}, func(Candidate) []Candidate { return nil })

	if len(got) != 1 || got[0] != Candidate(a1) {
		t.Fatal(names(got))
	}
}

func TestInstallOrderIndices_SelfEdgesAndOutOfRange_AreIgnored(t *testing.T) {
	got := InstallOrderIndices(3, func(i int) []int {
		return map[int][]int{0: {0, 2, 7, -1}, 1: {1}, 2: {}}[i]
	})

	if !slices.Equal(got, []int{1, 2, 0}) {
		t.Fatal(got)
	}
}

// GraphSolverLazyTests.cs

const base = "github.com/base/base"

func addCount(r *fakeResolver, spec string, count int, deps []Dependency, mod func(int) string) {
	for i := count - 1; i >= 0; i-- {
		o := pkgOpts{deps: deps}
		if mod != nil {
			o.mod = mod(i)
		}
		r.add(pkg(spec, fmt.Sprintf("%d.0.0", i), o))
	}
}

func TestSolve_RequestWithManyVersions_ProducesOnlyTheFirstCandidate(t *testing.T) {
	r := newResolver()
	addCount(r, base, 300, nil, nil)

	got := solve(t, r, []Request{request(base)}).Order

	if len(got) != 1 || got[0].Version().String() != "299.0.0" || r.produced != 1 {
		t.Fatal(names(got), r.produced)
	}
}

func TestSolve_DependencySatisfiedByInstalledMember_DoesNotEnumerateOtherVersions(t *testing.T) {
	r := newResolver()
	addCount(r, base, 300, nil, nil)
	addCount(r, "github.com/mod/mod", 1, []Dependency{dep(base, "*")}, nil)
	installed := pkg(base, "5.0.0", pkgOpts{})

	got := solve(t, r, []Request{request("github.com/mod/mod")}, installed).Order

	equal(t, []string{base + "@5.0.0", "github.com/mod/mod@0.0.0"}, names(got))
	if r.produced > 3 {
		t.Fatalf("produced %d candidates", r.produced)
	}
}

func TestSolve_NewestVersionsClash_ProducesCandidatesOnlyUntilOneFits(t *testing.T) {
	r := newResolver()
	addCount(r, base, 300, nil, func(v int) string {
		if v >= 295 {
			return "Taken"
		}
		return "Base"
	})
	installed := pkg("github.com/other/other", "1.0.0", pkgOpts{mod: "taken"})

	got := solve(t, r, []Request{request(base)}, installed).Order

	if got[0].Version().String() != "294.0.0" || r.produced != 6 {
		t.Fatal(names(got), r.produced)
	}
}

// Preferred candidates and RequestCandidate (lip b46fb4d, 8d287f7).

func solvePreferred(r Resolver, requests []Request, pinned, preferred []Candidate) (*Solution, error) {
	return Solve(context.Background(), Problem{Requests: requests, Pinned: pinned, Preferred: preferred,
		Resolver: r})
}

func TestSolve_Preferred_IsTriedFirstWhereItFits(t *testing.T) {
	// a needs d >=1 and e >=1; b needs e ^2. d 1.0.0 and e 1.0.0 are
	// preferred.
	r := newResolver()
	r.addVersions("github.com/d/d", "2.0.0", "1.0.0")
	r.addVersions("github.com/e/e", "2.0.0", "1.0.0")
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{
		dep("github.com/d/d", ">=1.0.0"), dep("github.com/e/e", ">=1.0.0")}}))
	r.add(pkg("github.com/b/b", "1.0.0", pkgOpts{deps: []Dependency{dep("github.com/e/e", "^2.0.0")}}))
	preferredD := pkg("github.com/d/d", "1.0.0", pkgOpts{})
	preferredE := pkg("github.com/e/e", "1.0.0", pkgOpts{})

	s, err := solvePreferred(r, []Request{request("github.com/a/a"), request("github.com/b/b")}, nil,
		[]Candidate{preferredE, preferredD})
	if err != nil {
		t.Fatal(err)
	}

	// d keeps its version; e cannot.
	equal(t, []string{"github.com/d/d@1.0.0", "github.com/e/e@2.0.0", "github.com/a/a@1.0.0",
		"github.com/b/b@1.0.0"}, names(s.Order))
	if s.Order[0] != Candidate(preferredD) {
		t.Fatal("not the preferred candidate")
	}
}

func TestRequestCandidate_SamePackage_ReturnsTheCandidateOfTheRequest(t *testing.T) {
	r := newResolver()
	r.addVersions("github.com/x/x", "2.0.0", "1.0.0")
	installed := pkg("github.com/x/x", "1.0.0", pkgOpts{})
	ofRef := pkg("github.com/x/x", "1.0.0", pkgOpts{source: "github.com/x/x@branch:main"})
	ctx := context.Background()

	got, err := RequestCandidate(ctx, r, request("github.com/x/x"), installed)
	if err != nil || got == nil || got.Source().Text != "github.com/x/x" {
		t.Fatalf("%v %v", got, err)
	}
	if got, err := RequestCandidate(ctx, r, request("github.com/x/x"), ofRef); err != nil ||
		got != Candidate(ofRef) {
		t.Fatalf("%v %v", got, err)
	}
	if got, err := RequestCandidate(ctx, r, Request{Source: parseRef("github.com/x/x"),
		Range: mustRange(">=2.0.0")}, installed); err != nil || got != nil {
		t.Fatalf("%v %v", got, err)
	}
}

func TestSolve_InstalledPackageOfRefWithoutVersionInRange_DoesNotStandIn(t *testing.T) {
	// The package at the branch is in the range of A's dependency, but no
	// version tag of the repository (if any) is, so there is no candidate for
	// it to stand in for.
	for _, tag := range []string{"1.0.0", ""} {
		t.Run(tag, func(t *testing.T) {
			r := newResolver()
			if tag != "" {
				r.addVersions("github.com/x/b", tag)
			}
			r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{dep("github.com/x/b", ">=1.1.0")}}))
			installed := pkg("github.com/x/b", "1.2.0", pkgOpts{source: "github.com/x/b@branch:main"})

			solveFails(t, r, []Request{request("github.com/a/a")}, installed)
			_, err := solvePreferred(r, []Request{request("github.com/a/a")}, nil, []Candidate{installed})
			var re *ResolutionError
			if !errors.As(err, &re) {
				t.Fatalf("want ResolutionError, got %v", err)
			}
		})
	}
}

func TestSolve_InstalledPackageOfRefWithVersionInRange_StandsIn(t *testing.T) {
	// A version of the repository is in the range, so the package at the
	// branch stands in for it.
	r := newResolver()
	r.addVersions("github.com/x/b", "1.1.0", "1.0.0")
	r.add(pkg("github.com/a/a", "1.0.0", pkgOpts{deps: []Dependency{dep("github.com/x/b", ">=1.1.0")}}))
	installed := pkg("github.com/x/b", "1.2.0", pkgOpts{source: "github.com/x/b@branch:main"})

	kept := solve(t, r, []Request{request("github.com/a/a")}, installed).Order
	preferred, err := solvePreferred(r, []Request{request("github.com/a/a")}, nil, []Candidate{installed})
	if err != nil {
		t.Fatal(err)
	}

	if kept[0] != Candidate(installed) || preferred.Order[0] != Candidate(installed) {
		t.Fatalf("%v %v", names(kept), names(preferred.Order))
	}
}

func TestRequestCandidate_PackageOfRef_StandsInOnlyWithVersionInRange(t *testing.T) {
	for _, tc := range []struct {
		tag      string
		standsIn bool
	}{{"1.0.0", false}, {"", false}, {"1.1.0", true}} {
		t.Run(tc.tag, func(t *testing.T) {
			r := newResolver()
			if tc.tag != "" {
				r.addVersions("github.com/x/b", tc.tag)
			}
			ofRef := pkg("github.com/x/b", "1.2.0", pkgOpts{source: "github.com/x/b@branch:main"})

			got, err := RequestCandidate(context.Background(), r, Request{Source: parseRef("github.com/x/b"),
				Range: mustRange(">=1.1.0")}, ofRef)

			if err != nil {
				t.Fatal(err)
			}
			if tc.standsIn != (got == Candidate(ofRef)) || (!tc.standsIn && got != nil) {
				t.Fatalf("got %v", got)
			}
		})
	}
}
