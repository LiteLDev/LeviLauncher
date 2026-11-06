package solver

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// ---- fakes ----

// fv is a fake dotted-integer version; anything after '+' is build metadata
// and ignored by Compare (like SemVer precedence).
type fv string

func (v fv) parts() []int {
	s := string(v)
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var out []int
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

func (v fv) Compare(o Version) int {
	a, b := v.parts(), o.(fv).parts()
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func (v fv) String() string { return string(v) }

// fc is a fake constraint:
//
//	"*"            any version
//	"1.3.*"        prefix match on the dotted components
//	">=a <b"       half-open interval (either bound may be omitted)
//	"a"            exact (precedence-equal)
type fc string

func (c fc) String() string { return string(c) }

func (c fc) Contains(v Version) bool {
	s := string(c)
	switch {
	case s == "*":
		return true
	case strings.HasSuffix(s, ".*"):
		prefix := fv(strings.TrimSuffix(s, ".*")).parts()
		got := v.(fv).parts()
		for i, p := range prefix {
			if i >= len(got) || got[i] != p {
				return false
			}
		}
		return true
	case strings.HasPrefix(s, ">=") || strings.HasPrefix(s, "<"):
		for _, f := range strings.Fields(s) {
			switch {
			case strings.HasPrefix(f, ">="):
				if v.Compare(fv(f[2:])) < 0 {
					return false
				}
			case strings.HasPrefix(f, "<"):
				if v.Compare(fv(f[1:])) >= 0 {
					return false
				}
			}
		}
		return true
	default:
		return v.Compare(fv(s)) == 0
	}
}

func req(id, c string) Requirement { return Requirement{ID: id, Constraint: fc(c)} }

type fakeRegistry struct {
	versions map[string][]string      // id -> versions, oldest first
	deps     map[string][]Requirement // "id@version" -> deps
	errOn    string                   // id whose Versions call fails
	calls    map[string]int           // Versions calls per id
	depCalls map[string]int           // Dependencies calls per id@version
}

func (r *fakeRegistry) Versions(_ context.Context, id string) ([]Version, error) {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[id]++
	if id == r.errOn {
		return nil, errors.New("registry boom")
	}
	var out []Version
	for _, s := range r.versions[id] {
		out = append(out, fv(s))
	}
	return out, nil
}

func (r *fakeRegistry) Dependencies(_ context.Context, id string, v Version) ([]Requirement, error) {
	if r.depCalls == nil {
		r.depCalls = map[string]int{}
	}
	key := id + "@" + v.String()
	r.depCalls[key]++
	return r.deps[key], nil
}

func specs(sel []Selected) []string {
	out := make([]string, len(sel))
	for i, s := range sel {
		out[i] = s.ID + "@" + s.Version.String()
	}
	return out
}

const (
	ll       = "github.com/LiteLDev/LeviLamina#client"
	brd      = "github.com/LiteLDev/bedrock-runtime-data#client"
	preload  = "github.com/LiteLDev/preloader"
	prelink  = "github.com/LiteLDev/prelink"
	someMod  = "github.com/someone/mod"
	otherMod = "github.com/someone/other"
)

// ---- Solve ----

func TestSolve(t *testing.T) {
	tests := []struct {
		name    string
		reg     *fakeRegistry
		roots   []Requirement
		want    []string // topologically sorted id@version
		wantErr string
		logs    []string
	}{
		{
			// lip: Solve_NoDependencies_ReturnsEmpty. candidates is empty, so
			// Backtrack returns the empty selection immediately.
			name:  "no requirements",
			reg:   &fakeRegistry{},
			roots: nil,
			want:  []string{},
			logs:  []string{"Resolving dependencies for: "},
		},
		{
			// lip: Solve_SinglePackage_ReturnsPackage.
			name:  "single package",
			reg:   &fakeRegistry{versions: map[string][]string{"a": {"1.0.0"}}},
			roots: []Requirement{req("a", "*")},
			want:  []string{"a@1.0.0"},
			logs: []string{
				"Resolving dependencies for: a@[*]",
				"Considering a with 1 candidate version(s)",
			},
		},
		{
			// LeviLamina#client pinned exactly (as lip's InstallService does
			// for every explicit package). Selecting it discovers
			// bedrock-runtime-data (exact, 1 candidate) into the slot freed by
			// LeviLamina and preloader (1.3.*, 2 candidates) appended.
			// Fewest-first picks bedrock-runtime-data, then preloader newest
			// first (1.3.1), whose dependency prelink >=1.3.0 <2.0.0 leaves
			// {1.3.0, 1.3.2} -> 1.3.2. Selection order LL, brd, preloader,
			// prelink; the topological sort emits each dependency before its
			// dependant, visiting LL's deps in manifest order.
			name: "LeviLamina client with exact and wildcard deps",
			reg: &fakeRegistry{
				versions: map[string][]string{
					ll:      {"1.3.0", "1.4.0"},
					brd:     {"1.21.50", "1.21.60"},
					preload: {"1.2.0", "1.3.0", "1.3.1", "1.4.0"},
					prelink: {"1.3.0", "1.3.2", "2.0.0"},
				},
				deps: map[string][]Requirement{
					ll + "@1.4.0":      {req(brd, "1.21.50"), req(preload, "1.3.*")},
					preload + "@1.3.1": {req(prelink, ">=1.3.0 <2.0.0")},
				},
			},
			roots: []Requirement{req(ll, "1.4.0")},
			want:  []string{brd + "@1.21.50", prelink + "@1.3.2", preload + "@1.3.1", ll + "@1.4.0"},
			logs: []string{
				"Resolving dependencies for: " + ll + "@[1.4.0]",
				"Considering " + ll + " with 1 candidate version(s)",
				"Considering " + brd + " with 1 candidate version(s)",
				"Considering " + preload + " with 2 candidate version(s)",
				"Considering " + prelink + " with 2 candidate version(s)",
			},
		},
		{
			// Diamond: A -> B (>=1 <2), C (*); B -> D [1,3); C -> D >=2.
			// After A: B {1.0,1.1} in A's old slot, C {1.0} appended; C has
			// fewer candidates. C discovers D {2.0,2.5,3.0} into C's freed
			// slot. B (2) beats D (3); B@1.1 narrows D to {2.0,2.5}; D@2.5.
			// Selection order A, C, B, D -> topo: D, B, C, A.
			name: "diamond",
			reg: &fakeRegistry{
				versions: map[string][]string{
					"A": {"1.0.0"},
					"B": {"1.0.0", "1.1.0"},
					"C": {"1.0.0"},
					"D": {"1.0.0", "2.0.0", "2.5.0", "3.0.0"},
				},
				deps: map[string][]Requirement{
					"A@1.0.0": {req("B", ">=1.0.0 <2.0.0"), req("C", "*")},
					"B@1.1.0": {req("D", ">=1.0.0 <3.0.0")},
					"C@1.0.0": {req("D", ">=2.0.0")},
				},
			},
			roots: []Requirement{req("A", "1.0.0")},
			want:  []string{"D@2.5.0", "B@1.1.0", "C@1.0.0", "A@1.0.0"},
			logs: []string{
				"Resolving dependencies for: A@[1.0.0]",
				"Considering A with 1 candidate version(s)",
				"Considering C with 1 candidate version(s)",
				"Considering B with 2 candidate version(s)",
				"Considering D with 2 candidate version(s)",
			},
		},
		{
			// lip: Solve_Conflict_ThrowsException. Candidates A {1}, B {2}; tie
			// broken by insertion order -> A. A@1 requires B 1.0.0, which
			// empties B's candidate set -> no solution.
			name: "conflict with root requirement",
			reg: &fakeRegistry{
				versions: map[string][]string{"a": {"1.0.0"}, "b": {"1.0.0", "2.0.0"}},
				deps:     map[string][]Requirement{"a@1.0.0": {req("b", "1.0.0")}},
			},
			roots:   []Requirement{req("a", "*"), req("b", "2.0.0")},
			wantErr: "Cannot find a valid state to satisfy all dependencies.",
			logs: []string{
				"Resolving dependencies for: a@[*], b@[2.0.0]",
				"Considering a with 1 candidate version(s)",
			},
		},
		{
			// Two mods need disjoint preloader ranges: mod pulls preloader
			// 1.3.* (discovered), then other's 1.4.* intersects it to empty.
			name: "conflicting dependency ranges",
			reg: &fakeRegistry{
				versions: map[string][]string{
					someMod:  {"1.0.0"},
					otherMod: {"2.0.0"},
					preload:  {"1.3.0", "1.4.0"},
				},
				deps: map[string][]Requirement{
					someMod + "@1.0.0":  {req(preload, "1.3.*")},
					otherMod + "@2.0.0": {req(preload, "1.4.*")},
				},
			},
			roots:   []Requirement{req(someMod, "1.0.0"), req(otherMod, "2.0.0")},
			wantErr: "Cannot find a valid state to satisfy all dependencies.",
			logs: []string{
				"Resolving dependencies for: " + someMod + "@[1.0.0], " + otherMod + "@[2.0.0]",
				"Considering " + someMod + " with 1 candidate version(s)",
				"Considering " + preload + " with 1 candidate version(s)",
				"Considering " + otherMod + " with 1 candidate version(s)",
			},
		},
		{
			// A root range with no matching version gives an empty candidate
			// set; MinBy picks it (count 0) and the loop body never runs.
			name:    "root range matches nothing",
			reg:     &fakeRegistry{versions: map[string][]string{preload: {"1.2.0", "1.4.0"}}},
			roots:   []Requirement{req(preload, "1.3.*")},
			wantErr: "Cannot find a valid state to satisfy all dependencies.",
			logs: []string{
				"Resolving dependencies for: " + preload + "@[1.3.*]",
				"Considering " + preload + " with 0 candidate version(s)",
			},
		},
		{
			// Newest A@2 needs B 9.0.0 which does not exist (discovery yields
			// an empty set -> propagate fails); backtrack to A@1.
			name: "backtrack on failed discovery",
			reg: &fakeRegistry{
				versions: map[string][]string{"A": {"1.0.0", "2.0.0"}, "B": {"1.0.0"}},
				deps: map[string][]Requirement{
					"A@2.0.0": {req("B", "9.0.0")},
					"A@1.0.0": {req("B", "*")},
				},
			},
			roots: []Requirement{req("A", "*")},
			want:  []string{"B@1.0.0", "A@1.0.0"},
			logs: []string{
				"Resolving dependencies for: A@[*]",
				"Considering A with 2 candidate version(s)",
				"Considering B with 1 candidate version(s)",
			},
		},
		{
			// Failure two levels down: A@2 -> C; C@1 -> D 3.0.0 (missing), so
			// C has no viable version, Backtrack(C) returns null and A@1
			// (no deps) is tried.
			name: "backtrack from deeper level",
			reg: &fakeRegistry{
				versions: map[string][]string{
					"A": {"1.0.0", "2.0.0"},
					"C": {"1.0.0"},
					"D": {"1.0.0"},
				},
				deps: map[string][]Requirement{
					"A@2.0.0": {req("C", "*")},
					"C@1.0.0": {req("D", "3.0.0")},
				},
			},
			roots: []Requirement{req("A", "*")},
			want:  []string{"A@1.0.0"},
			logs: []string{
				"Resolving dependencies for: A@[*]",
				"Considering A with 2 candidate version(s)",
				"Considering C with 1 candidate version(s)",
			},
		},
		{
			// A dependency on an already-selected package is checked against
			// the selected version: A@2 -> B *, B@1 -> A 1.0.0 conflicts with
			// the selected A@2; backtrack to A@1 whose B@1 is satisfied. The
			// resulting A<->B cycle is then rejected by TopologicalSort,
			// visiting A first.
			name: "already selected conflict then cycle",
			reg: &fakeRegistry{
				versions: map[string][]string{"A": {"1.0.0", "2.0.0"}, "B": {"1.0.0"}},
				deps: map[string][]Requirement{
					"A@2.0.0": {req("B", "*")},
					"A@1.0.0": {req("B", "*")},
					"B@1.0.0": {req("A", "1.0.0")},
				},
			},
			roots:   []Requirement{req("A", "*")},
			wantErr: "Circular dependency detected involving package 'A'",
			logs: []string{
				"Resolving dependencies for: A@[*]",
				"Considering A with 2 candidate version(s)",
				"Considering B with 1 candidate version(s)", // B@1 fails against A@2
				"Considering B with 1 candidate version(s)", // rediscovered under A@1
			},
		},
		{
			// A package depending on itself is satisfied during propagation
			// (it is already selected) but is a cycle for the sort.
			name: "self dependency",
			reg: &fakeRegistry{
				versions: map[string][]string{"A": {"1.0.0"}},
				deps:     map[string][]Requirement{"A@1.0.0": {req("A", "*")}},
			},
			roots:   []Requirement{req("A", "*")},
			wantErr: "Circular dependency detected involving package 'A'",
			logs: []string{
				"Resolving dependencies for: A@[*]",
				"Considering A with 1 candidate version(s)",
			},
		},
		{
			// Multiple root requirements on one id intersect (RemoveWhere on
			// the first set). lip still fetches the versions for each one.
			name: "root requirements on same id intersect",
			reg:  &fakeRegistry{versions: map[string][]string{"P": {"1.2.0", "1.3.0", "1.3.5", "2.0.0"}}},
			roots: []Requirement{
				req("P", ">=1.0.0 <2.0.0"),
				req("P", "1.3.*"),
			},
			want: []string{"P@1.3.5"},
			logs: []string{
				"Resolving dependencies for: P@[>=1.0.0 <2.0.0], P@[1.3.*]",
				"Considering P with 2 candidate version(s)",
			},
		},
		{
			// Tie-break by Dictionary order with slot reuse: after R1 is
			// selected and removed, its dependency N fills R1's freed slot,
			// so N (1 candidate) is enumerated before R2 (1 candidate).
			name: "tie broken by dictionary slot reuse",
			reg: &fakeRegistry{
				versions: map[string][]string{"R1": {"1.0.0"}, "R2": {"1.0.0"}, "N": {"1.0.0"}},
				deps:     map[string][]Requirement{"R1@1.0.0": {req("N", "*")}},
			},
			roots: []Requirement{req("R1", "1.0.0"), req("R2", "1.0.0")},
			want:  []string{"N@1.0.0", "R1@1.0.0", "R2@1.0.0"},
			logs: []string{
				"Resolving dependencies for: R1@[1.0.0], R2@[1.0.0]",
				"Considering R1 with 1 candidate version(s)",
				"Considering N with 1 candidate version(s)",
				"Considering R2 with 1 candidate version(s)",
			},
		},
		{
			// lip quirk: candidate HashSets are shared across backtracking
			// levels (shallow Dictionary copy), so narrowing is never undone.
			// A@2 narrows root B {1,2} to {2} and discovers C into A's freed
			// slot; C (tie with B, earlier slot) is tried first and C@1's
			// B <2 empties the shared set. Back at A@1, B's set is still
			// empty, so A@1's B <2 fails too, even though A@1+B@1 is a
			// valid assignment. lip reports no solution; so do we.
			name: "shared candidate sets are not restored on backtrack",
			reg: &fakeRegistry{
				versions: map[string][]string{
					"A": {"1.0.0", "2.0.0"},
					"B": {"1.0.0", "2.0.0"},
					"C": {"1.0.0"},
				},
				deps: map[string][]Requirement{
					"A@2.0.0": {req("B", ">=2.0.0"), req("C", "*")},
					"A@1.0.0": {req("B", "<2.0.0")},
					"C@1.0.0": {req("B", "<2.0.0")},
				},
			},
			roots:   []Requirement{req("A", "*"), req("B", "*")},
			wantErr: "Cannot find a valid state to satisfy all dependencies.",
			logs: []string{
				"Resolving dependencies for: A@[*], B@[*]",
				"Considering A with 2 candidate version(s)",
				"Considering C with 1 candidate version(s)",
			},
		},
		{
			// Equal-precedence versions (differing only in metadata) are
			// tried in reverse registry order: Order() is stable and the
			// result is reversed.
			name:  "equal precedence newest-first is reverse insertion",
			reg:   &fakeRegistry{versions: map[string][]string{"M": {"1.0.0+a", "1.0.0+b"}}},
			roots: []Requirement{req("M", "*")},
			want:  []string{"M@1.0.0+b"},
			logs: []string{
				"Resolving dependencies for: M@[*]",
				"Considering M with 2 candidate version(s)",
			},
		},
		{
			name:    "registry error is returned",
			reg:     &fakeRegistry{errOn: "A"},
			roots:   []Requirement{req("A", "*")},
			wantErr: "registry boom",
			logs:    []string{"Resolving dependencies for: A@[*]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs []string
			logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }

			got, err := Solve(context.Background(), tt.reg, tt.roots, logf)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q (got %v)", err, tt.wantErr, specs(got))
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if s := specs(got); !reflect.DeepEqual(s, tt.want) {
					t.Errorf("result = %v, want %v", s, tt.want)
				}
			}
			if !reflect.DeepEqual(logs, tt.logs) {
				t.Errorf("logs =\n%q\nwant\n%q", logs, tt.logs)
			}
		})
	}
}

func TestSolveRegistryCalls(t *testing.T) {
	// lip calls GetAvailableVersions once per root requirement (even for a
	// duplicate id) and once per discovery; GetPackageManifest once per
	// tried version during search plus once per selected package when
	// building the dependency nodes. Caching is the registry's job.
	reg := &fakeRegistry{
		versions: map[string][]string{"A": {"1.0.0", "2.0.0"}, "B": {"1.0.0"}},
		deps: map[string][]Requirement{
			"A@2.0.0": {req("B", "9.0.0")},
			"A@1.0.0": {req("B", "*")},
		},
	}
	if _, err := Solve(context.Background(), reg, []Requirement{req("A", "*"), req("A", "<3.0.0")}, nil); err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"A": 2, "B": 2}; !reflect.DeepEqual(reg.calls, want) {
		t.Errorf("Versions calls = %v, want %v", reg.calls, want)
	}
	if want := map[string]int{"A@2.0.0": 1, "A@1.0.0": 2, "B@1.0.0": 2}; !reflect.DeepEqual(reg.depCalls, want) {
		t.Errorf("Dependencies calls = %v, want %v", reg.depCalls, want)
	}
}

func TestSolveSelectedRequirements(t *testing.T) {
	reg := &fakeRegistry{
		versions: map[string][]string{"A": {"1.0.0"}, "B": {"1.0.0"}},
		deps:     map[string][]Requirement{"A@1.0.0": {req("B", "*")}},
	}
	got, err := Solve(context.Background(), reg, []Requirement{req("A", "*")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].ID != "A" || len(got[1].Requirements) != 1 || got[1].Requirements[0].ID != "B" {
		t.Fatalf("unexpected result %+v", got)
	}
}

func TestSolveCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reg := &fakeRegistry{versions: map[string][]string{"A": {"1.0.0"}}}
	if _, err := Solve(ctx, reg, []Requirement{req("A", "*")}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRequirementString(t *testing.T) {
	if got := req(ll, "1.3.*").String(); got != ll+"@[1.3.*]" {
		t.Errorf("String() = %q", got)
	}
}

// ---- TopologicalSort ----

func node(id string, deps ...string) Selected {
	s := Selected{ID: id, Version: fv("1.0.0")}
	for _, d := range deps {
		s.Requirements = append(s.Requirements, req(d, "*"))
	}
	return s
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []Selected
		want    []string
		wantErr string
	}{
		{name: "empty", nodes: nil, want: []string{}},
		{
			name:  "independent nodes keep order",
			nodes: []Selected{node("c"), node("a"), node("b")},
			want:  []string{"c", "a", "b"},
		},
		{
			name:  "dependencies first, in requirement order",
			nodes: []Selected{node("app", "z", "y"), node("y"), node("z", "y")},
			want:  []string{"y", "z", "app"},
		},
		{
			name:  "requirements not in node set are ignored",
			nodes: []Selected{node("a", "missing"), node("b", "a")},
			want:  []string{"a", "b"},
		},
		{
			name: "diamond",
			nodes: []Selected{
				node("top", "left", "right"),
				node("left", "base"),
				node("right", "base"),
				node("base"),
			},
			want: []string{"base", "left", "right", "top"},
		},
		{
			// Visit x -> y -> z -> x: x is on the stack.
			name:    "cycle names first revisited package",
			nodes:   []Selected{node("x", "y"), node("y", "z"), node("z", "x")},
			wantErr: "Circular dependency detected involving package 'x'",
		},
		{
			// Visit starts at y: y -> z -> x -> y.
			name:    "cycle detected from visit order",
			nodes:   []Selected{node("y", "z"), node("x", "y"), node("z", "x")},
			wantErr: "Circular dependency detected involving package 'y'",
		},
		{
			name:    "duplicate id",
			nodes:   []Selected{node("a"), node("a")},
			wantErr: "An item with the same key has already been added. Key: a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TopologicalSort(tt.nodes)
			if strings.HasPrefix(tt.wantErr, "Circular") {
				var ce *CycleError
				if !errors.As(err, &ce) {
					t.Fatalf("err = %v, want *CycleError", err)
				}
			}
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(got))
			for i, s := range got {
				ids[i] = s.ID
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Errorf("got %v, want %v", ids, tt.want)
			}
		})
	}
}

// ---- internals ----

func TestOrderedCandsSlotReuse(t *testing.T) {
	d := newOrderedCands()
	for _, id := range []string{"a", "b", "c"} {
		d.set(id, &candSet{})
	}
	d.remove("b")
	d.remove("a")
	d.set("x", &candSet{}) // fills a's slot (most recently freed)
	d.set("y", &candSet{}) // fills b's slot
	d.set("z", &candSet{}) // appended
	var got []string
	for _, s := range d.slots {
		if s.used {
			got = append(got, s.id)
		}
	}
	if want := []string{"x", "y", "c", "z"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}

	d.remove("y")
	c := d.clone()
	if len(c.slots) != 3 || len(c.free) != 0 {
		t.Errorf("clone not compacted: %+v", c)
	}
	xs, _ := d.get("x")
	cs, _ := c.get("x")
	if xs != cs {
		t.Error("clone must share candidate sets")
	}
}
