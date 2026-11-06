// Package solver ports lip 0.37.0's backtracking dependency solver
// (src/Lip.Core/Services/DependencySolver.cs, IDependencySolver.TopologicalSort).
//
// It is written against small interfaces so it does not depend on the
// concrete semver/manifest packages; package engine adapts them.
//
// The port is faithful (newest-first candidate order, fewest-candidates
// heuristic and its tie-break, log messages, error texts, output order,
// cycle detection, registry call pattern) with one deliberate difference:
//
// Narrowing is undone on backtrack. lip copies the candidate dictionary
// shallowly (new Dictionary(candidates)), so the HashSet values are the same
// objects at every level, and narrowing a set during propagation
// (HashSet.RemoveWhere) leaks into the enclosing levels and is never undone.
// That makes lip's search incomplete: it can report "Cannot find a valid
// state to satisfy all dependencies." although a solution exists. This
// package shares *candSet pointers between levels too, but never mutates a
// set once the search has started: propagation replaces a set it narrows
// with a narrowed copy in the current level's dictionary (copy-on-write), so
// every level sees exactly the candidates its own selections allow.
//
// The difference shows only after lip's search failed somewhere (a
// propagation conflict, or a version whose subtree has no solution) after
// narrowing a set that an enclosing level or a later sibling still uses; in
// lip that set then stays narrowed for the rest of the search. Every search
// without such a failure, in particular every search that succeeds without
// backtracking, gives the same selection, "Considering" log lines and
// registry calls as lip. After such a failure the rest of the search sees
// candidates lip had wrongly pruned, so:
//
//   - where lip reports no solution, a solution may now be found;
//   - where lip succeeds, a version lip had pruned may now be selected
//     instead of the one lip settled for: typically a newer one, as the
//     newest is tried first (lip picks B@1 where B@2 is valid);
//   - as candidate counts differ, the fewest-candidates heuristic may
//     consider packages in another order, so the "Considering" lines,
//     the selection, the topological output order and the registry calls
//     can differ from lip in that situation.
//
// A search that succeeds in lip never fails here: lip only selects valid
// assignments and this search is complete.
//
// lip quirks kept as they are:
//
//   - Ties in the fewest-candidates heuristic are broken by .NET Dictionary
//     enumeration order, which is insertion order except that an id added
//     after a removal reuses the removed slot. orderedCands emulates that.
//   - The solver itself does not cache registry calls; like lip it relies on
//     the Registry implementation to memoize (lip's GitRemotePackageRegistry
//     caches tags per repository and manifests per spec).
package solver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("solver: not implemented")

// ErrNoSolution is returned by Solve when no assignment satisfies all
// requirements. Its text is lip's InvalidOperationException message.
var ErrNoSolution = errors.New("Cannot find a valid state to satisfy all dependencies.")

// Version is an opaque comparable version.
type Version interface {
	// Compare returns -1, 0 or 1 by precedence.
	Compare(other Version) int
	String() string
}

// Constraint is a version range.
type Constraint interface {
	Contains(v Version) bool
	String() string
}

// CycleError is returned by TopologicalSort (and Solve) when the dependency
// graph has a cycle. Its text is lip's InvalidOperationException message.
type CycleError struct {
	// ID is the package that was reached again while on the visit stack.
	ID string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("Circular dependency detected involving package '%s'", e.ID)
}

// Requirement is "package id must satisfy constraint" (lip PackageReqt).
type Requirement struct {
	// ID is a package id string ("path" or "path#variant"); ids are compared
	// exactly ("x#client" and "x" are different packages), so callers must
	// pass the canonical form (lip PackageId.ToString: no trailing "#").
	ID string
	// Constraint must be non-nil.
	Constraint Constraint
}

// String formats the requirement like lip's PackageReqt.ToString:
// "<id>@[<range>]".
func (r Requirement) String() string { return r.ID + "@[" + r.Constraint.String() + "]" }

// Registry supplies candidates (lip IPackageRegistry + manifest variant deps).
type Registry interface {
	// Versions returns available versions of id, oldest first.
	Versions(ctx context.Context, id string) ([]Version, error)
	// Dependencies returns the requirements of id@v for the current platform
	// (resolved variant dependencies), in manifest order.
	Dependencies(ctx context.Context, id string, v Version) ([]Requirement, error)
}

// Selected is one package in the solution.
type Selected struct {
	ID      string
	Version Version
	// Requirements are the dependencies of this package (used for ordering).
	Requirements []Requirement
}

// Logf, if set, receives the same informational messages lip prints
// (e.g. "Resolving dependencies for: ...").
type Logf func(format string, args ...any)

// candSet is lip's HashSet<SemVersion> of candidate versions. Elements keep
// insertion order (HashSet enumeration order when only RemoveWhere is used)
// and are unique by String() (SemVersion.Equals compares every component,
// including metadata, i.e. the canonical string).
type candSet struct {
	vs []Version
}

func newCandSet(available []Version, c Constraint) *candSet {
	s := &candSet{}
	seen := make(map[string]struct{}, len(available))
	for _, v := range available {
		if !c.Contains(v) {
			continue
		}
		k := v.String()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		s.vs = append(s.vs, v)
	}
	return s
}

// narrowed is HashSet.RemoveWhere(v => !c.Contains(v)) without mutating s,
// since sets are shared between backtracking levels: it returns s itself
// when every element satisfies c, else a new set holding the satisfying
// elements in their original order.
func (s *candSet) narrowed(c Constraint) *candSet {
	for i, v := range s.vs {
		if c.Contains(v) {
			continue
		}
		out := &candSet{vs: make([]Version, i, len(s.vs)-1)}
		copy(out.vs, s.vs[:i])
		for _, w := range s.vs[i+1:] {
			if c.Contains(w) {
				out.vs = append(out.vs, w)
			}
		}
		return out
	}
	return s
}

// newestFirst is versions.Order(PrecedenceComparer).Reverse(): a stable
// ascending sort, reversed, so equal-precedence versions come out in reverse
// insertion order.
func (s *candSet) newestFirst() []Version {
	sorted := append([]Version(nil), s.vs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Compare(sorted[j]) < 0 })
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}
	return sorted
}

// orderedCands emulates Dictionary<PackageId, HashSet<SemVersion>> including
// its enumeration order: entries are enumerated by slot index, Remove leaves
// a hole that is pushed on a LIFO free list, and Add fills the most recently
// freed hole before appending. Copying (new Dictionary(d)) compacts slots.
type orderedCands struct {
	slots []candSlot
	free  []int
	index map[string]int
}

type candSlot struct {
	id   string
	set  *candSet
	used bool
}

func newOrderedCands() *orderedCands {
	return &orderedCands{index: map[string]int{}}
}

func (d *orderedCands) count() int { return len(d.index) }

func (d *orderedCands) get(id string) (*candSet, bool) {
	i, ok := d.index[id]
	if !ok {
		return nil, false
	}
	return d.slots[i].set, true
}

// set is the indexer setter: replace in place when present, else add.
func (d *orderedCands) set(id string, s *candSet) {
	if i, ok := d.index[id]; ok {
		d.slots[i].set = s
		return
	}
	var i int
	if n := len(d.free); n > 0 {
		i = d.free[n-1]
		d.free = d.free[:n-1]
		d.slots[i] = candSlot{id: id, set: s, used: true}
	} else {
		i = len(d.slots)
		d.slots = append(d.slots, candSlot{id: id, set: s, used: true})
	}
	d.index[id] = i
}

func (d *orderedCands) remove(id string) {
	i, ok := d.index[id]
	if !ok {
		return
	}
	delete(d.index, id)
	d.slots[i] = candSlot{}
	d.free = append(d.free, i)
}

// clone is a shallow, compacting copy: the *candSet values are shared, which
// is safe because sets are never mutated during the search (see narrowed).
func (d *orderedCands) clone() *orderedCands {
	c := &orderedCands{
		slots: make([]candSlot, 0, d.count()),
		index: make(map[string]int, d.count()),
	}
	for _, s := range d.slots {
		if !s.used {
			continue
		}
		c.index[s.id] = len(c.slots)
		c.slots = append(c.slots, s)
	}
	return c
}

// minByCount is Enumerable.MinBy(x => x.Value.Count): the first entry in
// enumeration order with the smallest count. d must be non-empty.
func (d *orderedCands) minByCount() (string, *candSet) {
	var (
		bestID  string
		bestSet *candSet
		found   bool
	)
	for _, s := range d.slots {
		if !s.used {
			continue
		}
		if !found || len(s.set.vs) < len(bestSet.vs) {
			bestID, bestSet, found = s.id, s.set, true
		}
	}
	return bestID, bestSet
}

// selection is Dictionary<PackageId, SemVersion> for selected versions.
// Entries are never removed, so enumeration order is insertion order.
type selection struct {
	ids []string
	vs  map[string]Version
}

func (s *selection) with(id string, v Version) *selection {
	n := &selection{
		ids: make([]string, len(s.ids), len(s.ids)+1),
		vs:  make(map[string]Version, len(s.vs)+1),
	}
	copy(n.ids, s.ids)
	for k, x := range s.vs {
		n.vs[k] = x
	}
	if _, ok := n.vs[id]; !ok {
		n.ids = append(n.ids, id)
	}
	n.vs[id] = v
	return n
}

type solverState struct {
	reg  Registry
	logf Logf
}

func (st *solverState) printInfo(format string, args ...any) {
	if st.logf != nil {
		st.logf(format, args...)
	}
}

// Solve finds one version per package satisfying all requirements as lip
// does, except that narrowing is undone on backtrack (see the package doc):
// candidate sets per id; backtracking picks the unresolved id
// with the fewest candidates and tries the newest version first; selecting
// a version propagates its dependencies (an already-selected dependency must
// satisfy the range, otherwise the candidate set is intersected; a new
// dependency's candidates are fetched from the registry and filtered). It
// does not prefer installed versions. The result is topologically sorted
// (dependencies first, stable with respect to discovery order); a cycle is
// an error ("Circular dependency detected involving package '<id>'").
//
// When no solution exists the error is ErrNoSolution. Registry errors are
// returned unchanged.
func Solve(ctx context.Context, reg Registry, roots []Requirement, logf Logf) ([]Selected, error) {
	st := &solverState{reg: reg, logf: logf}

	reqStrs := make([]string, len(roots))
	for i, r := range roots {
		reqStrs[i] = r.String()
	}
	st.printInfo("Resolving dependencies for: %s", strings.Join(reqStrs, ", "))

	candidates := newOrderedCands()
	for _, req := range roots {
		// lip fetches available versions even when the id is already present.
		available, err := reg.Versions(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		if existing, ok := candidates.get(req.ID); ok {
			// Pick the intersection of existing candidates and new constraints.
			// set replaces the value in place, so the id keeps its slot.
			candidates.set(req.ID, existing.narrowed(req.Constraint))
		} else {
			candidates.set(req.ID, newCandSet(available, req.Constraint))
		}
	}

	solution, err := st.backtrack(ctx, candidates, &selection{vs: map[string]Version{}})
	if err != nil {
		return nil, err
	}
	if solution == nil {
		return nil, ErrNoSolution
	}

	nodes := make([]Selected, 0, len(solution.ids))
	for _, id := range solution.ids {
		v := solution.vs[id]
		deps, err := reg.Dependencies(ctx, id, v)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, Selected{ID: id, Version: v, Requirements: deps})
	}

	return TopologicalSort(nodes)
}

// backtrack ports DependencySolver.Backtrack. A nil selection with a nil
// error means "no solution in this branch". Each tried version works on its
// own clone of candidates, and propagation never mutates a shared set, so
// nothing narrowed while trying one version is seen when trying the next.
func (st *solverState) backtrack(ctx context.Context, candidates *orderedCands, selected *selection) (*selection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if candidates.count() == 0 {
		return selected, nil
	}

	// Fail-first heuristic: Pick candidate with fewest version options.
	nextID, versions := candidates.minByCount()

	st.printInfo("Considering %s with %d candidate version(s)", nextID, len(versions.vs))

	// Prefer newest versions. The ordering is materialized once, before the
	// first iteration, as Reverse() buffers the sequence.
	for _, version := range versions.newestFirst() {
		nextSelected := selected.with(nextID, version)

		nextCandidates := candidates.clone()
		nextCandidates.remove(nextID)

		ok, err := st.tryPropagate(ctx, nextID, version, nextSelected, nextCandidates)
		if err != nil {
			return nil, err
		}
		if ok {
			result, err := st.backtrack(ctx, nextCandidates, nextSelected)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}
	}

	return nil, nil
}

// tryPropagate ports DependencySolver.TryPropagate.
func (st *solverState) tryPropagate(ctx context.Context, id string, v Version, currentSelection *selection, currentCandidates *orderedCands) (bool, error) {
	deps, err := st.reg.Dependencies(ctx, id, v)
	if err != nil {
		return false, err
	}

	for _, req := range deps {
		// Conflict with already selected package?
		if selectedVer, ok := currentSelection.vs[req.ID]; ok {
			if !req.Constraint.Contains(selectedVer) {
				return false, nil
			}
			continue // Constraint satisfied.
		}

		// Constrain remaining candidates.
		if options, ok := currentCandidates.get(req.ID); ok {
			// lip narrows the set shared with the enclosing levels in place;
			// replacing it in this level's dictionary instead keeps the
			// narrowing from outliving a backtrack.
			options = options.narrowed(req.Constraint)
			currentCandidates.set(req.ID, options)
			if len(options.vs) == 0 {
				return false, nil
			}
		} else {
			// Discovery of new dependency.
			available, err := st.reg.Versions(ctx, req.ID)
			if err != nil {
				return false, err
			}
			compatible := newCandSet(available, req.Constraint)
			if len(compatible.vs) == 0 {
				return false, nil
			}
			currentCandidates.set(req.ID, compatible)
		}
	}

	return true, nil
}

// TopologicalSort orders nodes so dependencies come first, visiting nodes
// in the given order (IDependencySolver.TopologicalSort). Requirements whose
// id is not among nodes are ignored. Duplicate ids are an error (lip's
// ToDictionary throws).
func TopologicalSort(nodes []Selected) ([]Selected, error) {
	byID := make(map[string]int, len(nodes))
	for i, n := range nodes {
		if _, dup := byID[n.ID]; dup {
			return nil, fmt.Errorf("An item with the same key has already been added. Key: %s", n.ID)
		}
		byID[n.ID] = i
	}

	visited := make(map[string]bool, len(nodes))
	cycleStack := make(map[string]bool)
	sorted := make([]Selected, 0, len(nodes))

	var visit func(n Selected) error
	visit = func(n Selected) error {
		if visited[n.ID] {
			return nil
		}
		if cycleStack[n.ID] {
			return &CycleError{ID: n.ID}
		}
		cycleStack[n.ID] = true

		for _, req := range n.Requirements {
			if i, ok := byID[req.ID]; ok {
				if err := visit(nodes[i]); err != nil {
					return err
				}
			}
		}

		delete(cycleStack, n.ID)
		visited[n.ID] = true
		sorted = append(sorted, n)
		return nil
	}

	for _, n := range nodes {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return sorted, nil
}
