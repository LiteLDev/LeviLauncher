package engine

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lockfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

var ctx = context.Background()

// seed registers a common package graph:
//
//	a 1.0.0 -> b >=1.0.0 ; a 2.0.0 -> c * ; a 3.0.0-beta (no deps)
//	b 1.0.0, 1.1.0 ; c 1.0.0 ; d 1.0.0 ; e#client 1.0.0 -> b 1.0.0
func seed(h *harness) {
	h.reg.add("example.com/a", "1.0.0", "", "example.com/b", ">=1.0.0")
	h.reg.add("example.com/a", "2.0.0", "", "example.com/c", "*")
	h.reg.add("example.com/a", "3.0.0-beta", "")
	h.reg.add("example.com/b", "1.0.0", "")
	h.reg.add("example.com/b", "1.1.0", "")
	h.reg.add("example.com/c", "1.0.0", "")
	h.reg.add("example.com/d", "1.0.0", "")
	h.reg.add("example.com/e", "1.0.0", "client", "example.com/b#", "1.0.0")
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name      string
		pre       []string // installed first (explicitly)
		install   []string
		wantErr   string
		wantLock  string
		wantCalls string
		wantSaves int
		wantMsgs  []string
	}{
		{
			name:      "fresh without deps",
			install:   []string{"example.com/d@1.0.0"},
			wantLock:  "example.com/d@1.0.0!",
			wantCalls: "install:d/1.0.0.txt",
			wantSaves: 2, // add + explicitness reconcile, like lip
			wantMsgs: []string{
				"info: Listing files is the registry's job",
				"info: Resolving dependencies for: example.com/d@1.0.0",
				"info: Installing package example.com/d version 1.0.0",
				"info: Marking package 'example.com/d' version 1.0.0 as explicitly installed.",
			},
		},
		{
			name:      "with deps picks newest dependency",
			install:   []string{"example.com/a@1.0.0"},
			wantLock:  "example.com/b@1.1.0 example.com/a@1.0.0!",
			wantCalls: "install:b/1.1.0.txt install:a/1.0.0.txt",
			wantSaves: 3,
			wantMsgs: []string{
				"info: Installing package example.com/b version 1.1.0",
				"info: Installing package example.com/a version 1.0.0",
			},
		},
		{
			name:      "flexible id takes latest including prereleases",
			install:   []string{"example.com/a"},
			wantLock:  "example.com/a@3.0.0-beta!",
			wantCalls: "install:a/3.0.0-beta.txt",
			wantSaves: 2,
			wantMsgs:  []string{"info: Getting available versions for package 'example.com/a'..."},
		},
		{
			name:      "variant dependency with empty label normalizes id",
			install:   []string{"example.com/e#client@1.0.0"},
			wantLock:  "example.com/b@1.0.0 example.com/e#client@1.0.0!",
			wantCalls: "install:b/1.0.0.txt install:e/1.0.0.txt",
			wantSaves: 3,
		},
		{
			name:     "already explicit",
			pre:      []string{"example.com/a@1.0.0"},
			install:  []string{"example.com/a@2.0.0"},
			wantErr:  "Cannot install package 'example.com/a' because it is already explicitly installed.",
			wantLock: "example.com/b@1.1.0 example.com/a@1.0.0!",
		},
		{
			name:      "dependency promoted to explicit",
			pre:       []string{"example.com/a@1.0.0"},
			install:   []string{"example.com/b@1.1.0"},
			wantLock:  "example.com/b@1.1.0! example.com/a@1.0.0!",
			wantCalls: "",
			wantSaves: 1,
			wantMsgs:  []string{"info: Marking package 'example.com/b' version 1.1.0 as explicitly installed."},
		},
		{
			name:     "duplicate ids",
			install:  []string{"example.com/a@1.0.0", "example.com/a"},
			wantErr:  "Multiple packages with ID 'example.com/a' were specified. Please ensure that each package ID is unique.",
			wantLock: "",
		},
		{
			name:     "unparsable",
			install:  []string{"bad@pkg@x"},
			wantErr:  "Failed to parse package 'bad@pkg@x'. (Invalid package spec: bad@pkg@x) (Invalid package id: bad@pkg@x) (local archive packages are not supported) (remote archive URL packages are not supported)",
			wantLock: "",
		},
		{
			name:     "missing version tag",
			install:  []string{"example.com/a@9.9.9"},
			wantErr:  "Package example.com/a@9.9.9 not found",
			wantLock: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			seed(h)
			if len(tt.pre) > 0 {
				if err := h.e.Install(ctx, tt.pre, false, false, false); err != nil {
					t.Fatal(err)
				}
			}
			h.calls, h.lock.saves, h.rep.msgs = nil, nil, nil

			err := h.e.Install(ctx, tt.install, false, false, false)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := summary(h.state(t)); got != tt.wantLock {
				t.Errorf("lock = %q, want %q", got, tt.wantLock)
			}
			if tt.wantErr != "" {
				return
			}
			if got := h.callSummary(); got != tt.wantCalls {
				t.Errorf("calls = %q, want %q", got, tt.wantCalls)
			}
			if len(h.lock.saves) != tt.wantSaves {
				t.Errorf("saves = %d, want %d", len(h.lock.saves), tt.wantSaves)
			}
			for _, m := range tt.wantMsgs {
				if strings.HasPrefix(m, "info: Listing files") {
					continue
				}
				if !h.rep.has(m) {
					t.Errorf("missing message %q in %q", m, h.rep.msgs)
				}
			}
		})
	}
}

func TestInstallLockSavedAfterEachStep(t *testing.T) {
	h := newHarness(t)
	seed(h)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := range h.lock.saves {
		got = append(got, summary(&h.lock.saves[i]))
	}
	want := []string{
		"example.com/b@1.1.0",
		"example.com/b@1.1.0 example.com/a@1.0.0!",
		"example.com/b@1.1.0 example.com/a@1.0.0!",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saves = %q, want %q", got, want)
	}
	s := h.state(t)
	a := s.Packages[1]
	if a.Variant != "" || a.Manifest.Tooth != "example.com/a" || len(a.Manifest.Variants) != 1 {
		t.Errorf("stored package = %+v", a)
	}
	if rel := relFiles(h.e.Dir, a.Files); !reflect.DeepEqual(rel, []string{"a/1.0.0.txt"}) {
		t.Errorf("files = %q", rel)
	}
}

func TestInstallManifestFixup(t *testing.T) {
	tests := []struct {
		name     string
		tooth    string
		version  string
		wantWarn bool
	}{
		{"mismatch warns", "example.com/other", "0.9.0", true},
		{"case only is silent", "Example.com/D", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			seed(h)
			m := h.reg.pkgs["example.com/d"]["1.0.0"].m
			m.Tooth, m.Version = tt.tooth, tt.version
			if err := h.e.Install(ctx, []string{"example.com/d@1.0.0"}, false, false, false); err != nil {
				t.Fatal(err)
			}
			warn := "warning: Manifest of package example.com/d version 1.0.0 declares '" + tt.tooth + "' version " + tt.version + "; using the requested package path and version."
			if h.rep.has(warn) != tt.wantWarn {
				t.Errorf("warning present = %v, want %v (%q)", !tt.wantWarn, tt.wantWarn, h.rep.msgs)
			}
			p := h.state(t).Packages[0]
			if p.Manifest.Tooth != "example.com/d" || p.Manifest.Version != "1.0.0" {
				t.Errorf("stored manifest tooth/version = %s %s", p.Manifest.Tooth, p.Manifest.Version)
			}
			if m.Tooth != tt.tooth {
				t.Errorf("registry manifest was mutated")
			}
		})
	}
}

func TestInstallDryRunChangesNothing(t *testing.T) {
	h := newHarness(t)
	seed(h)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, true, false, false); err != nil {
		t.Fatal(err)
	}
	if len(h.lock.saves) != 0 || len(h.calls) != 0 {
		t.Fatalf("dry run saved %d times, %d ops", len(h.lock.saves), len(h.calls))
	}
	for _, m := range []string{
		"info: Dry run: would install package example.com/b version 1.1.0",
		"info: Dry run: would install package example.com/a version 1.0.0",
		"info: Dry run: would mark package 'example.com/a' version 1.0.0 as explicitly installed.",
	} {
		if !h.rep.has(m) {
			t.Errorf("missing %q in %q", m, h.rep.msgs)
		}
	}
}

func TestInstallNoDependencies(t *testing.T) {
	h := newHarness(t)
	seed(h)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, true); err != nil {
		t.Fatal(err)
	}
	if got := summary(h.state(t)); got != "example.com/a@1.0.0!" {
		t.Errorf("lock = %q", got)
	}
	if !h.rep.has("warning: --no-dependencies is enabled. This operation may result in a broken workspace.") {
		t.Errorf("missing warning: %q", h.rep.msgs)
	}
	// Installing it again hits the per-package check.
	err := h.e.Install(ctx, []string{"example.com/a@2.0.0"}, false, false, true)
	want := "Cannot install package example.com/a version 2.0.0 because it is already installed with version 1.0.0"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestInstallFailureKeepsPreviousSteps(t *testing.T) {
	h := newHarness(t)
	seed(h)
	h.e.Hooks.NewPackageOps = newFailingOps(&h.calls, 1)
	err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, false)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
	if got := summary(h.state(t)); got != "example.com/b@1.1.0" {
		t.Errorf("lock = %q, want dependency kept", got)
	}
}

// failingOps fails the n-th (0-based) InstallPackage call.
type failingOps struct {
	fakeOps
	n, failAt int
}

func (o *failingOps) InstallPackage(ctx context.Context, v *manifest.Variant, open workspace.SourceOpener) ([]string, error) {
	defer func() { o.n++ }()
	if o.n == o.failAt {
		return nil, errors.New("boom")
	}
	return o.fakeOps.InstallPackage(ctx, v, open)
}

func newFailingOps(calls *[]opCall, failAt int) func(string, bool, report.Reporter) PackageOps {
	return func(dir string, _ bool, _ report.Reporter) PackageOps {
		return &failingOps{fakeOps: fakeOps{dir: dir, calls: calls}, failAt: failAt}
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name      string
		update    []string
		wantErr   string
		wantLock  string
		wantCalls string
	}{
		{
			name:      "to version with different deps uninstalls in reverse topo order",
			update:    []string{"example.com/a@2.0.0"},
			wantLock:  "example.com/c@1.0.0 example.com/a@2.0.0!",
			wantCalls: "uninstall:a/1.0.0.txt uninstall:b/1.1.0.txt install:c/1.0.0.txt install:a/2.0.0.txt",
		},
		{
			name:      "flexible",
			update:    []string{"example.com/a"},
			wantLock:  "example.com/a@3.0.0-beta!",
			wantCalls: "uninstall:a/1.0.0.txt uninstall:b/1.1.0.txt install:a/3.0.0-beta.txt",
		},
		{
			name:     "not installed",
			update:   []string{"example.com/d@1.0.0"},
			wantErr:  "Cannot update package 'example.com/d' because it is not currently installed.",
			wantLock: "example.com/b@1.1.0 example.com/a@1.0.0!",
		},
		{
			name:      "same version is a no-op",
			update:    []string{"example.com/a@1.0.0"},
			wantLock:  "example.com/b@1.1.0 example.com/a@1.0.0!",
			wantCalls: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			seed(h)
			if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, false); err != nil {
				t.Fatal(err)
			}
			h.calls = nil
			err := h.e.Update(ctx, tt.update, false, false)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := summary(h.state(t)); got != tt.wantLock {
				t.Errorf("lock = %q, want %q", got, tt.wantLock)
			}
			if got := h.callSummary(); tt.wantErr == "" && got != tt.wantCalls {
				t.Errorf("calls = %q, want %q", got, tt.wantCalls)
			}
		})
	}
}

func TestUpdateDependencyBecomesExplicit(t *testing.T) {
	h := newHarness(t)
	seed(h)
	h.reg.add("example.com/b", "1.0.0", "")
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	h.calls = nil
	if err := h.e.Update(ctx, []string{"example.com/b@1.0.0"}, false, false); err != nil {
		t.Fatal(err)
	}
	if got, want := summary(h.state(t)), "example.com/a@1.0.0! example.com/b@1.0.0!"; got != want {
		t.Errorf("lock = %q, want %q", got, want)
	}
	if got, want := h.callSummary(), "uninstall:b/1.1.0.txt install:b/1.0.0.txt"; got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

func TestUninstall(t *testing.T) {
	tests := []struct {
		name      string
		uninstall []string
		noDeps    bool
		dryRun    bool
		wantErr   string
		wantLock  string
		wantCalls string
	}{
		{
			name:      "removes orphaned deps in reverse topo order",
			uninstall: []string{"example.com/a"},
			wantLock:  "example.com/d@1.0.0!",
			wantCalls: "uninstall:a/1.0.0.txt uninstall:b/1.1.0.txt",
		},
		{
			name:      "not explicit",
			uninstall: []string{"example.com/b"},
			wantErr:   "Package 'example.com/b' is not explicitly installed and cannot be uninstalled without --no-dependencies.",
			wantLock:  "example.com/b@1.1.0 example.com/a@1.0.0! example.com/d@1.0.0!",
		},
		{
			name:      "no dependencies removes only the package",
			uninstall: []string{"example.com/a"},
			noDeps:    true,
			wantLock:  "example.com/b@1.1.0 example.com/d@1.0.0!",
			wantCalls: "uninstall:a/1.0.0.txt",
		},
		{
			name:      "no dependencies, not installed",
			uninstall: []string{"example.com/c"},
			noDeps:    true,
			wantErr:   "Cannot uninstall package example.com/c because it is not installed",
			wantLock:  "example.com/b@1.1.0 example.com/a@1.0.0! example.com/d@1.0.0!",
		},
		{
			name:      "dry run",
			uninstall: []string{"example.com/a"},
			dryRun:    true,
			wantLock:  "example.com/b@1.1.0 example.com/a@1.0.0! example.com/d@1.0.0!",
			wantCalls: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			seed(h)
			if err := h.e.Install(ctx, []string{"example.com/a@1.0.0", "example.com/d@1.0.0"}, false, false, false); err != nil {
				t.Fatal(err)
			}
			h.calls, h.lock.saves = nil, nil
			err := h.e.Uninstall(ctx, tt.uninstall, tt.dryRun, false, tt.noDeps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := summary(h.state(t)); got != tt.wantLock {
				t.Errorf("lock = %q, want %q", got, tt.wantLock)
			}
			if got := h.callSummary(); tt.wantErr == "" && got != tt.wantCalls {
				t.Errorf("calls = %q, want %q", got, tt.wantCalls)
			}
			if tt.dryRun {
				if len(h.lock.saves) != 0 {
					t.Errorf("dry run saved")
				}
				if !h.rep.has("info: Dry run: would uninstall package example.com/a version 1.0.0") {
					t.Errorf("missing dry-run message: %q", h.rep.msgs)
				}
			}
		})
	}
}

func TestUninstallPassesStoredFiles(t *testing.T) {
	h := newHarness(t)
	seed(h)
	if err := h.e.Install(ctx, []string{"example.com/d@1.0.0"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	want := h.state(t).Packages[0].Files
	h.calls = nil
	if err := h.e.Uninstall(ctx, []string{"example.com/d"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	if len(h.calls) != 1 || !reflect.DeepEqual(h.calls[0].files, want) {
		t.Fatalf("calls = %+v, want files %q", h.calls, want)
	}
	if got := summary(h.state(t)); got != "" {
		t.Errorf("lock = %q", got)
	}
}

func TestList(t *testing.T) {
	h := newHarness(t)
	pkg := func(path, version string, locked bool, deps ...string) lockfile.Package {
		var d manifest.Dependencies
		for _, x := range deps {
			d = append(d, manifest.Dependency{ID: x, Range: "*"})
		}
		return lockfile.Package{
			Locked: locked,
			Manifest: manifest.Manifest{Tooth: path, Version: version,
				Variants: []manifest.Variant{{Dependencies: d}}},
		}
	}
	s := lockfile.New()
	s.Packages = []lockfile.Package{
		pkg("x/top", "1.0.0", true, "x/mid", "x/lib"),
		pkg("x/other", "2.0.0", true),
		pkg("x/mid", "1.0.0", true, "x/lib"),
		pkg("x/lib", "1.0.0", false),
		pkg("x/dep2", "1.0.0", false, "x/lib"),
		pkg("x/dep1", "1.0.0", false),
	}
	s.Packages[4].Manifest.Variants[0].Dependencies = append(s.Packages[4].Manifest.Variants[0].Dependencies, manifest.Dependency{ID: "x/dep1", Range: "*"})
	if err := h.lock.save(h.e.Dir, s); err != nil {
		t.Fatal(err)
	}
	explicit, implicit, err := h.e.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"x/mid@1.0.0", "x/top@1.0.0", "x/other@2.0.0"}; !reflect.DeepEqual(explicit, want) {
		t.Errorf("explicit = %q, want %q", explicit, want)
	}
	if want := []string{"x/lib@1.0.0", "x/dep1@1.0.0", "x/dep2@1.0.0"}; !reflect.DeepEqual(implicit, want) {
		t.Errorf("implicit = %q, want %q", implicit, want)
	}
}

func TestListCycle(t *testing.T) {
	h := newHarness(t)
	s := lockfile.New()
	for _, p := range [][2]string{{"x/a", "x/b"}, {"x/b", "x/a"}} {
		s.Packages = append(s.Packages, lockfile.Package{Locked: true, Manifest: manifest.Manifest{
			Tooth: p[0], Version: "1.0.0",
			Variants: []manifest.Variant{{Dependencies: manifest.Dependencies{{ID: p[1], Range: "*"}}}},
		}})
	}
	_ = h.lock.save(h.e.Dir, s)
	_, _, err := h.e.List(ctx)
	if err == nil || err.Error() != "Circular dependency detected involving package 'x/a'" {
		t.Fatalf("err = %v", err)
	}
}

func TestBrokenLockAborts(t *testing.T) {
	h := newHarness(t)
	seed(h)
	h.e.Hooks.LoadState = func(string) (*lockfile.State, error) { return lockfile.New(), errors.New("bad json") }
	want := "Failed to load workspace state from 'tooth_lock.json': bad json"
	ops := map[string]func() error{
		"install":   func() error { return h.e.Install(ctx, []string{"example.com/d@1.0.0"}, false, false, false) },
		"nodeps":    func() error { return h.e.Install(ctx, []string{"example.com/d@1.0.0"}, false, false, true) },
		"update":    func() error { return h.e.Update(ctx, []string{"example.com/d@1.0.0"}, false, false) },
		"uninstall": func() error { return h.e.Uninstall(ctx, []string{"example.com/d"}, false, false, false) },
		"list": func() error {
			_, _, err := h.e.List(ctx)
			return err
		},
	}
	for name, op := range ops {
		if err := op(); err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", name, err, want)
		}
	}
	if len(h.lock.saves) != 0 || len(h.calls) != 0 {
		t.Errorf("saved %d times, %d ops after a broken lock", len(h.lock.saves), len(h.calls))
	}
}

func TestCompositeVersionsUnion(t *testing.T) {
	h := newHarness(t)
	seed(h)
	// Installed version not in the remote registry is still a candidate.
	s := lockfile.New()
	s.Packages = []lockfile.Package{{Locked: true, Manifest: manifest.Manifest{Tooth: "example.com/b", Version: "0.5.0", Variants: []manifest.Variant{{}}}}}
	_ = h.lock.save(h.e.Dir, s)
	arts := []artifact{{spec: manifest.PackageSpec{ID: manifest.PackageID{Path: "example.com/b"}, Version: "9.0.0"}}}
	reg := &compositeRegistry{e: h.e, regs: []pkgRegistry{artifactsRegistry{e: h.e, arts: arts}, h.e.baseRegistry()}}
	vs, err := reg.versions(ctx, manifest.PackageID{Path: "example.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range vs {
		got = append(got, v.String())
	}
	if want := []string{"0.5.0", "1.0.0", "1.1.0", "9.0.0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("versions = %q, want %q", got, want)
	}

	// Remote failure is swallowed when another registry answers.
	vs, err = h.e.baseRegistry().versions(ctx, manifest.PackageID{Path: "example.com/missing"})
	if err != nil || len(vs) != 0 {
		t.Errorf("versions = %v, %v", vs, err)
	}

	// Manifest errors are aggregated.
	_, err = h.e.baseRegistry().manifest(ctx, manifest.PackageSpec{ID: manifest.PackageID{Path: "example.com/zzz"}, Version: "1.0.0"})
	want := "Failed to retrieve package manifest for package example.com/zzz@1.0.0 from any registry. (Cannot get manifest of package 'example.com/zzz@1.0.0' because it is not installed.) (Package example.com/zzz@1.0.0 not found)"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v\nwant %s", err, want)
	}
}

func TestSolverAdapterMemoizesDependencies(t *testing.T) {
	h := newHarness(t)
	seed(h)
	calls := 0
	h.e.Hooks.ResolveVariant = func(m *manifest.Manifest, label string) (*manifest.Variant, error) {
		calls++
		return fakeResolveVariant(m, label)
	}
	a := &solverAdapter{e: h.e, reg: h.e.baseRegistry()}
	v, _ := parseFakeVersion("1.0.0")
	for i := 0; i < 3; i++ {
		reqs, err := a.Dependencies(ctx, "example.com/a", v)
		if err != nil || len(reqs) != 1 || reqs[0].ID != "example.com/b" {
			t.Fatalf("reqs = %v, %v", reqs, err)
		}
	}
	if calls != 1 {
		t.Errorf("ResolveVariant called %d times, want 1", calls)
	}
}
