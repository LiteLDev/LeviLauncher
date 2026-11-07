package graph

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// The candidates and resolvers of GraphSolverTests.cs (TestCandidates,
// FakeResolver) and GraphSolverLazyTests.cs (CountingResolver).

// parseRef reads the refs of the tests: a URL, or a remote repository
// "<target>[#variant][@gitref]".
func parseRef(text string) Ref {
	if strings.HasPrefix(text, "https://") {
		return Ref{Text: text, Target: text}
	}
	r := Ref{Text: text, Spec: true}
	target := text
	if at := strings.Index(target, "@"); at >= 0 {
		r.Pinned = true
		target = target[:at]
	}
	if hash := strings.Index(target, "#"); hash >= 0 {
		r.Variant = target[hash+1:]
		target = target[:hash]
	}
	r.Target = target
	return r
}

type fakeCandidate struct {
	id, variant string
	version     semver.Version
	source      Ref
	archiveKey  string
	commit      string
	deps        []Dependency
	derived     bool

	mod       func() (string, error)
	loaded    bool
	modName   string
	modErr    error
	downloads *[]string
	modURL    string
}

func (c *fakeCandidate) Identity() string {
	if c.variant == "" {
		return c.id
	}
	return c.id + "#" + c.variant
}
func (c *fakeCandidate) Version() semver.Version    { return c.version }
func (c *fakeCandidate) Variant() string            { return c.variant }
func (c *fakeCandidate) Commit() string             { return c.commit }
func (c *fakeCandidate) ArchiveKey() string         { return c.archiveKey }
func (c *fakeCandidate) Source() Ref                { return c.source }
func (c *fakeCandidate) Derived() bool              { return c.derived }
func (c *fakeCandidate) Dependencies() []Dependency { return c.deps }
func (c *fakeCandidate) ModName(context.Context) (string, error) {
	if c.mod == nil {
		return "", nil
	}
	if !c.loaded {
		c.loaded = true
		if c.downloads != nil {
			*c.downloads = append(*c.downloads, c.modURL)
		}
		c.modName, c.modErr = c.mod()
	}
	return c.modName, c.modErr
}

type pkgOpts struct {
	source    string
	variant   string
	deps      []Dependency
	mod       string
	mcmod     func(label string) (string, error) // a mod manifest that fails or names a mod
	legacy    bool
	downloads *[]string
}

// pkg is TestCandidates.Make.
func pkg(id, version string, o pkgOpts) *fakeCandidate {
	source := o.source
	if source == "" {
		source = id
		if o.variant != "" {
			source += "#" + o.variant
		}
	}
	ref := parseRef(source)
	key := ref.Text + "@" + version
	sum := sha1.Sum([]byte(Fold(key)))
	c := &fakeCandidate{
		id: id, variant: o.variant, version: semver.MustParse(version), source: ref,
		archiveKey: key, commit: hex.EncodeToString(sum[:]), deps: o.deps, derived: o.legacy,
		downloads: o.downloads, modURL: fmt.Sprintf("https://example.com/%s/%s.mcmod", id, version),
	}
	label := id + " " + version
	switch {
	case o.mod != "":
		name := o.mod
		c.mod = func() (string, error) { return name, nil }
	case o.mcmod != nil:
		c.mod = func() (string, error) { return o.mcmod(label) }
	}
	return c
}

// Mod manifests that cannot be read, as Candidate.GetModManifest reports
// them.
func missingManifest(label string) (string, error) {
	return "", fmt.Errorf("Cannot install %s: its asset of type \"mcmod\" has no manifest.json at"+
		" the root of its archive, so lip cannot tell the mod name. This is a problem of the"+
		" package; report it to its author.", label)
}

func invalidManifest(label string) (string, error) {
	return "", fmt.Errorf("Cannot install %s: the manifest.json of its asset of type \"mcmod\" must"+
		" be a JSON object whose \"name\" is the mod name", label)
}

func unavailableAsset(label string) (string, error) {
	return "", fmt.Errorf("Cannot install %s: lip could not read the manifest.json of its asset of"+
		" type \"mcmod\" to tell the mod name.: %w", label, errors.New("404"))
}

func dep(source string, rng string, compat bool) Dependency {
	return Dependency{Source: parseRef(source), Range: mustRange(rng), ClientCompat: compat}
}

func mustRange(s string) Range {
	r, err := semver.ParseRange(s)
	if err != nil {
		panic(err)
	}
	return Range{r}
}

func request(source string) Request { return Request{Source: parseRef(source)} }

type resolveCall struct {
	ref    string
	rng    string
	compat bool
}

// fakeResolver maps a source, as written, to its candidates in order, and
// filters them by range like the real resolver.
type fakeResolver struct {
	candidates map[string][]Candidate
	calls      []resolveCall
	produced   int
}

func newResolver() *fakeResolver { return &fakeResolver{candidates: map[string][]Candidate{}} }

func (r *fakeResolver) add(c *fakeCandidate) {
	r.candidates[c.source.Text] = append(r.candidates[c.source.Text], c)
}

func (r *fakeResolver) addVersions(spec string, versions ...string) {
	for _, v := range versions {
		r.add(pkg(spec, v, pkgOpts{}))
	}
}

type countingSequence struct {
	r     *fakeResolver
	items []Candidate
	rng   Constraint
}

func (s *countingSequence) Next(context.Context) (Candidate, bool, error) {
	for len(s.items) > 0 {
		c := s.items[0]
		s.items = s.items[1:]
		if s.rng == nil || s.rng.Contains(c.Version()) {
			s.r.produced++
			return c, true, nil
		}
	}
	return nil, false, nil
}

func (r *fakeResolver) ResolveLazy(_ context.Context, source Ref, rng Constraint, compat bool,
	_ func(Diagnostic)) Sequence {
	call := resolveCall{ref: source.Text, compat: compat}
	if rng != nil {
		call.rng = rng.String()
	}
	r.calls = append(r.calls, call)
	return &countingSequence{r: r, items: r.candidates[source.Text], rng: rng}
}

func names(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Identity() + "@" + c.Version().String()
	}
	return out
}

func solve(t *testing.T, r Resolver, requests []Request, pinned ...Candidate) *Solution {
	t.Helper()
	s, err := Solve(context.Background(), Problem{Requests: requests, Pinned: pinned, Resolver: r})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func solveFails(t *testing.T, r Resolver, requests []Request, pinned ...Candidate) string {
	t.Helper()
	_, err := Solve(context.Background(), Problem{Requests: requests, Pinned: pinned, Resolver: r})
	var re *ResolutionError
	if !errors.As(err, &re) {
		t.Fatalf("want ResolutionError, got %v", err)
	}
	return re.Message
}

func equal(t *testing.T, want, got []string) {
	t.Helper()
	if strings.Join(want, "|") != strings.Join(got, "|") {
		t.Fatalf("want %v\ngot  %v", want, got)
	}
}
