package graph

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	// MaxRecordedFailures bounds the failures kept for the failure message.
	MaxRecordedFailures = 100
	// MaxTracedSteps bounds the search steps traced.
	MaxTracedSteps = 500
	// LevelsPerYield: the search checks for cancellation every this many
	// decision levels.
	LevelsPerYield = 128
	// MaxReportedFailures bounds the reasons listed in the failure message,
	// and the causes kept.
	MaxReportedFailures = 10
	// MaxListedReasons bounds the diagnostics and blocked versions listed per
	// reason.
	MaxListedReasons = 5
)

// Problem is the input of Solve.
type Problem struct {
	Requests []Request
	// Pinned are installed packages to keep: members of the set as they are,
	// whose dependencies must be satisfied too.
	Pinned   []Candidate
	Resolver Resolver
	// Trace, when set, receives the trace lines of RunTrace.Write("resolve").
	Trace func(message string)
}

// Solution is SolvedGraph: the package set in installation order; for every
// member (by index into Order) the members satisfying its dependencies; and
// for every request the member satisfying it (-1 when none).
type Solution struct {
	Order             []Candidate
	Dependencies      [][]int
	RequestSatisfiers []int
}

// ResolutionError is DependencyResolutionException: no package set
// satisfies the requests. Causes are the errors behind the reasons (e.g.
// network failures).
type ResolutionError struct {
	Message string
	Causes  []error
}

func (e *ResolutionError) Error() string   { return e.Message }
func (e *ResolutionError) Unwrap() []error { return e.Causes }

// Solve finds the package set S of protocol/workspace.md §3 and returns it in
// the installation order of §5 (GraphSolver.SolveGraph). It fails with
// *ResolutionError when there is none; other errors come from the resolver,
// the candidates or ctx.
//
// The search is a depth-first backtracking search over the open
// requirements (the requests, then the dependencies of each member in the
// order members are added). For each open requirement the candidates are
// tried in their sequence order, so earlier candidates win. It uses
// conflict-directed backjumping and learns candidates that can never be part
// of S. Candidates are produced lazily; whether a member already satisfies a
// requirement is decided without enumerating every candidate (see
// satisfyingMember).
//
// Two members clash when they have the same identity, or are both mods of
// the same mod name. A member resolved from a remote repository with a ref
// stands in for the versions of that repository, but its candidates are not
// candidates of a source without the ref. When a requirement without a ref
// is decided before one with a ref for the same repository, its choice
// clashes with the package of the ref; once every candidate has failed so,
// the requirement is deferred to the end of the agenda, where that package
// may satisfy it.
func Solve(ctx context.Context, p Problem) (*Solution, error) {
	s := &search{
		ctx:           ctx,
		resolver:      p.Resolver,
		trace:         p.Trace,
		resolved:      map[resolveKey]*lazySequence{},
		choices:       map[resolveKey]*choices{},
		members:       newMemberMap(),
		dead:          map[PackageKey]string{},
		pinnedClashes: map[int]bool{},
		reported:      map[string]bool{},
	}
	return s.run(p.Requests, p.Pinned)
}

// resolveKey identifies a candidate sequence by what determines it: the
// source, the range and clientCompat.
type resolveKey struct {
	ref    string
	rng    string
	compat bool
}

func keyOf(source Ref, rng Constraint, compat bool) resolveKey {
	k := ""
	if rng != nil {
		k = rng.Key()
	}
	return resolveKey{source.Text, k, compat}
}

// requirement is an open obligation: a request (owner nil), or a dependency
// of a member. ownerLevel is the decision level that added the owner, or -1
// for requests and pinned owners.
type requirement struct {
	key         resolveKey
	owner       Candidate
	description string
	ownerLevel  int
	ref         Ref
	rng         Constraint
	compat      bool
	deferred    bool
}

// lazySequence is the candidate sequence of one Resolve call, produced on
// demand and shared by every requirement that resolves the same key, with
// the diagnostics of the paths that produced no candidate.
type lazySequence struct {
	start       func(*lazySequence) Sequence
	seq         Sequence
	items       []Candidate
	diagnostics []Diagnostic
	complete    bool
}

func (l *lazySequence) has(ctx context.Context, index int) (bool, error) {
	for len(l.items) <= index && !l.complete {
		if l.seq == nil {
			l.seq = l.start(l)
		}
		c, ok, err := l.seq.Next(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			l.items = append(l.items, c)
		} else {
			l.complete = true
		}
	}
	return index < len(l.items), nil
}

// choices are the candidates that satisfy a requirement, in preference
// order and without duplicates, loaded on demand.
type choices struct {
	sequence      *lazySequence
	keys          map[PackageKey]bool
	position      int
	candidates    []Candidate
	candidateKeys []PackageKey
	complete      bool
	// unsatisfiable is set once no candidate can be chosen whatever the other
	// members are.
	unsatisfiable bool
}

func (c *choices) has(ctx context.Context, index int) (bool, error) {
	for len(c.candidates) <= index && !c.complete {
		if err := c.loadNext(ctx); err != nil {
			return false, err
		}
	}
	return index < len(c.candidates), nil
}

func (c *choices) loadNext(ctx context.Context) error {
	for {
		ok, err := c.sequence.has(ctx, c.position)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		candidate := c.sequence.items[c.position]
		c.position++
		key := KeyOf(candidate)
		if !c.keys[key] {
			c.keys[key] = true
			c.candidates = append(c.candidates, candidate)
			c.candidateKeys = append(c.candidateKeys, key)
			return nil
		}
	}
	c.complete = true
	return nil
}

type member struct {
	candidate Candidate
	key       PackageKey
	level     int
}

// memberMap is Dictionary<string, Member> by folded identity, enumerated in
// the order .NET enumerates it: by entry slot, a removed entry's slot being
// reused by the next one added (most recently freed first).
type memberMap struct {
	slots []*member
	index map[string]int
	free  []int
}

func newMemberMap() *memberMap { return &memberMap{index: map[string]int{}} }

func (m *memberMap) get(identity string) *member {
	if i, ok := m.index[identity]; ok {
		return m.slots[i]
	}
	return nil
}

func (m *memberMap) add(identity string, v *member) {
	if n := len(m.free); n > 0 {
		i := m.free[n-1]
		m.free = m.free[:n-1]
		m.slots[i] = v
		m.index[identity] = i
		return
	}
	m.index[identity] = len(m.slots)
	m.slots = append(m.slots, v)
}

func (m *memberMap) remove(identity string) {
	i := m.index[identity]
	delete(m.index, identity)
	m.slots[i] = nil
	m.free = append(m.free, i)
}

func (m *memberMap) len() int { return len(m.index) }

// values is a snapshot of the members in enumeration order.
func (m *memberMap) values() []*member {
	out := make([]*member, 0, len(m.index))
	for _, v := range m.slots {
		if v != nil {
			out = append(out, v)
		}
	}
	return out
}

// nogood is a set of decision levels whose choices together caused a
// failure; nil means success.
type nogood map[int]bool

func (n nogood) union(o nogood) {
	for k := range o {
		n[k] = true
	}
}

type search struct {
	ctx         context.Context
	resolver    Resolver
	trace       func(string)
	tracedSteps int

	resolved map[resolveKey]*lazySequence
	choices  map[resolveKey]*choices
	members  *memberMap
	agenda   []*requirement

	// dead are candidates proven to fail whatever else is chosen, with the
	// reason.
	dead map[PackageKey]string
	// pinnedClashes are decision levels whose member blocked a candidate of a
	// pinned spec ref by its identity.
	pinnedClashes map[int]bool

	failures     []string
	unresolvable []string
	reported     map[string]bool
	causes       []error
}

func (s *search) traceLine(message string) {
	if s.trace != nil {
		s.trace(message)
	}
}

func (s *search) traceStep(message string) {
	s.tracedSteps++
	if s.tracedSteps <= MaxTracedSteps {
		s.traceLine(message)
	}
}

func (s *search) run(requests []Request, pinned []Candidate) (*Solution, error) {
	s.traceLine(fmt.Sprintf("Solving %d requests with %d installed packages kept",
		len(requests), len(pinned)))
	for _, r := range requests {
		s.agenda = append(s.agenda, &requirement{
			key:         keyOf(r.Source, r.Range, false),
			description: "You requested " + showRequirement(r.Source, r.Range),
			ownerLevel:  -1,
			ref:         r.Source,
			rng:         r.Range,
		})
	}

	for _, c := range pinned {
		key := KeyOf(c)
		s.traceLine("Keeping the installed package " + showSource(c))
		if other := s.members.get(key.Identity); other != nil {
			if other.key == key {
				continue
			}
			return nil, &ResolutionError{Message: fmt.Sprintf("Installed packages %s and %s have the"+
				" same identity, which the workspace file .lip/workspace.yaml must not record."+
				" Uninstall them with --no-dependencies and install the one you want again.",
				showSource(other.candidate), showSource(c))}
		}
		modName, err := s.modName(c)
		if err != nil {
			return nil, err
		}
		if modName != "" {
			for _, m := range s.members.values() {
				otherName, err := s.modName(m.candidate)
				if err != nil {
					return nil, err
				}
				if otherName != "" && namesEqual(modName, otherName) {
					return nil, &ResolutionError{Message: fmt.Sprintf("Installed packages %s and %s"+
						" are mods of the same mod name '%s', so they would share one mod"+
						" directory. Uninstall one of them with --no-dependencies.",
						showSource(m.candidate), showSource(c), modName)}
				}
			}
		}
		s.members.add(key.Identity, &member{c, key, -1})
		s.addEdges(c, -1, " (installed)")
	}

	failed, err := s.solve(0, 0)
	if err != nil {
		return nil, err
	}
	if s.tracedSteps > MaxTracedSteps {
		s.traceLine(fmt.Sprintf("%d more search steps not traced", s.tracedSteps-MaxTracedSteps))
	}
	if failed != nil {
		// Missing dependencies are usually the root cause; among the other failures, the
		// most recent ones are closest to the requests that finally failed.
		lines := append([]string(nil), s.unresolvable...)
		for i := len(s.failures) - 1; i >= 0; i-- {
			lines = append(lines, s.failures[i])
		}
		if len(lines) == 0 {
			lines = append(lines, "Every combination of versions has two packages of the same"+
				" identity, or two mods of the same mod name.")
		}
		s.traceLine(fmt.Sprintf("No package set found; %d reasons recorded", len(lines)))
		var b strings.Builder
		b.WriteString("Cannot find versions of the requested packages and their dependencies" +
			" that can all be installed together:")
		for i, l := range lines {
			if i == MaxReportedFailures {
				break
			}
			b.WriteString("\n  - " + l)
		}
		if len(lines) > MaxReportedFailures {
			fmt.Fprintf(&b, "\n  ... and %d more reasons", len(lines)-MaxReportedFailures)
		}
		return nil, &ResolutionError{Message: b.String(), Causes: s.causes}
	}

	// On success the agenda holds exactly the requests and the dependencies of every
	// member, each satisfied by a member.
	dependencies := map[PackageKey][]Candidate{}
	var requestSatisfiers []Candidate
	for _, r := range s.agenda {
		if r.deferred {
			continue
		}
		ch := s.getChoices(r)
		satisfier, err := s.satisfyingMember(r, ch)
		if err != nil {
			return nil, err
		}
		if satisfier == nil {
			if satisfier, err = s.memberAmongCandidates(ch); err != nil {
				return nil, err
			}
		}
		if r.owner != nil {
			if satisfier != nil {
				k := KeyOf(r.owner)
				dependencies[k] = append(dependencies[k], satisfier)
			}
		} else {
			requestSatisfiers = append(requestSatisfiers, satisfier)
		}
	}

	var set []Candidate
	for _, m := range s.members.values() {
		set = append(set, m.candidate)
	}
	order := InstallOrder(set, func(c Candidate) []Candidate { return dependencies[KeyOf(c)] })
	indexOf := map[PackageKey]int{}
	for i, c := range order {
		indexOf[KeyOf(c)] = i
	}
	names := make([]string, len(order))
	for i, c := range order {
		names[i] = showSource(c)
	}
	s.traceLine("Solved; installation order: " + strings.Join(names, ", "))

	solution := &Solution{Order: order}
	for _, c := range order {
		deps := []int{}
		seen := map[int]bool{}
		for _, d := range dependencies[KeyOf(c)] {
			i := indexOf[KeyOf(d)]
			if !seen[i] {
				seen[i] = true
				deps = append(deps, i)
			}
		}
		solution.Dependencies = append(solution.Dependencies, deps)
	}
	solution.RequestSatisfiers = []int{}
	for _, c := range requestSatisfiers {
		if c == nil {
			solution.RequestSatisfiers = append(solution.RequestSatisfiers, -1)
		} else {
			solution.RequestSatisfiers = append(solution.RequestSatisfiers, indexOf[KeyOf(c)])
		}
	}
	return solution, nil
}

// solve satisfies the open requirements from agenda position index on.
// Returns nil on success (the members then form S), or the decision levels
// whose choices together caused the failure (-1 is never included).
func (s *search) solve(index, level int) (nogood, error) {
	if level%LevelsPerYield == LevelsPerYield-1 {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
	}

	var r *requirement
	var ch *choices
	for {
		if index == len(s.agenda) {
			return nil, nil
		}
		r = s.agenda[index]
		ch = s.getChoices(r)
		satisfier, err := s.satisfyingMember(r, ch)
		if err != nil {
			return nil, err
		}
		if satisfier == nil {
			if !r.deferred || s.onlyDeferredAfter(index+1) {
				break
			}
			// A deferred requirement waits until every other open requirement is decided.
			s.agenda = append(s.agenda, r)
		}
		index++
	}

	// The nogood is the owner (the requirement exists because of it) plus the blame: the
	// levels responsible for each candidate being rejected.
	blame := nogood{}
	var blocked []string
	failedDeeper := 0
	delete(s.pinnedClashes, level)
	for i := 0; !ch.unsatisfiable; i++ {
		ok, err := ch.has(s.ctx, i)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		candidate := ch.candidates[i]
		key := ch.candidateKeys[i]
		if existing := s.members.get(key.Identity); existing != nil && existing.key == key {
			// A member the shortcut of satisfyingMember missed. Nothing is decided here,
			// so no nogood of the rest contains this level.
			rest, err := s.solve(index+1, level+1)
			if err != nil || rest == nil {
				return nil, err
			}
			blame.union(rest)
			if existing.level >= 0 {
				blame[existing.level] = true
			}
			failedDeeper++
			break
		}

		blockerLevel, reason, isBlocked, err := s.blocker(candidate, key)
		if err != nil {
			return nil, err
		}
		if isBlocked {
			if blockerLevel >= 0 {
				blame[blockerLevel] = true
				if isPinned(candidate) {
					if clash := s.members.get(key.Identity); clash != nil && clash.level == blockerLevel {
						s.pinnedClashes[blockerLevel] = true
					}
				}
			}
			s.traceStep(fmt.Sprintf("%s: rejected %s: %s", r.description, show(candidate), reason))
			blocked = append(blocked, reason)
			continue
		}

		s.traceStep(fmt.Sprintf("%s: trying %s", r.description, showSource(candidate)))
		agendaCount := len(s.agenda)
		s.members.add(key.Identity, &member{candidate, key, level})
		s.addEdges(candidate, level, "")

		result, err := s.solve(index+1, level+1)
		if err != nil || result == nil {
			return nil, err
		}

		s.traceStep(fmt.Sprintf("%s: %s did not work out; backtracking", r.description,
			show(candidate)))
		s.members.remove(key.Identity)
		s.agenda = s.agenda[:agendaCount]

		if !result[level] {
			// This choice played no part in the failure; trying others cannot help.
			return result, nil
		}
		delete(result, level)
		if len(result) == 0 {
			s.dead[key] = show(candidate) + " needs packages that cannot be installed"
		}
		blame.union(result)
		failedDeeper++
	}

	if !r.deferred && s.pinnedClashes[level] && r.ref.specWithoutRef() {
		// A pinned package clashed with every choice made here; it may stand in for this
		// requirement once it is a member. The deferred copy belongs to this level, so its
		// failure leads back here.
		agendaCount := len(s.agenda)
		deferred := *r
		deferred.ownerLevel = level
		deferred.deferred = true
		s.agenda = append(s.agenda, &deferred)
		result, err := s.solve(index+1, level+1)
		if err != nil || result == nil {
			return nil, err
		}
		s.agenda = s.agenda[:agendaCount]
		if !result[level] {
			return result, nil
		}
		delete(result, level)
		blame.union(result)
	}

	if len(blame) == 0 {
		// No member is to blame, so the same candidates fail for any owner.
		ch.unsatisfiable = true
	}
	s.recordFailure(r, ch, blocked, failedDeeper)
	if r.ownerLevel >= 0 {
		blame[r.ownerLevel] = true
	}
	return blame, nil
}

func (s *search) onlyDeferredAfter(index int) bool {
	for _, r := range s.agenda[index:] {
		if !r.deferred {
			return false
		}
	}
	return true
}

// blocker returns why a candidate cannot join the current members, with the
// decision level of the member responsible (-1 when no member or only fixed
// ones are); isBlocked is false when it can join.
func (s *search) blocker(c Candidate, key PackageKey) (level int, reason string, isBlocked bool,
	err error) {
	if dead, ok := s.dead[key]; ok {
		return -1, dead, true, nil
	}
	if same := s.members.get(key.Identity); same != nil {
		return same.level, fmt.Sprintf("%s has the same identity as %s%s", showSource(c),
			showSource(same.candidate), installedSuffix(same.level)), true, nil
	}

	modName, err := c.ModName(s.ctx)
	if err != nil {
		if isContextError(err) {
			return 0, "", false, err
		}
		// A missing or invalid mod manifest fails whatever the other members are.
		s.dead[key] = err.Error()
		s.addCause(err)
		return -1, err.Error(), true, nil
	}
	if modName == "" {
		return 0, "", false, nil
	}

	found := false
	for _, m := range s.members.values() {
		if found && m.level >= level {
			continue
		}
		other, err := s.modName(m.candidate)
		if err != nil {
			return 0, "", false, err
		}
		if other != "" && namesEqual(modName, other) {
			found = true
			level = m.level
			reason = fmt.Sprintf("%s is a mod of the same mod name '%s' as %s%s", showSource(c),
				modName, showSource(m.candidate), installedSuffix(m.level))
		}
	}
	return level, reason, found, nil
}

// satisfyingMember returns the member that satisfies a requirement
// (protocol/workspace.md §3: a member that is one of the requirement's
// candidates, or stands in for one), or nil. It avoids loading every
// candidate: the loaded candidates are checked first, then (for a remote
// repository) each member whose identity is that of a loaded candidate is
// looked up directly, resolving the repository for exactly its version.
func (s *search) satisfyingMember(r *requirement, ch *choices) (Candidate, error) {
	if s.members.len() == 0 {
		return nil, nil
	}
	for i := range ch.candidates {
		if m := s.members.get(ch.candidateKeys[i].Identity); m != nil && m.key == ch.candidateKeys[i] {
			return m.candidate, nil
		}
	}
	if r.ref.specWithoutRef() {
		for _, m := range s.members.values() {
			if isPinnedSatisfier(r, m.candidate) {
				return m.candidate, nil
			}
		}
	}
	if ch.complete {
		return nil, nil
	}

	// Only remote repositories can have long candidate sequences; load the others
	// completely.
	if !r.ref.Spec {
		for i := len(ch.candidates); ; i++ {
			ok, err := ch.has(s.ctx, i)
			if err != nil || !ok {
				return nil, err
			}
			if m := s.members.get(ch.candidateKeys[i].Identity); m != nil && m.key == ch.candidateKeys[i] {
				return m.candidate, nil
			}
		}
	}

	// Only members named like a candidate are looked up, so that members of unrelated
	// packages never make the requirement's repository open the tag of their version.
	if ok, err := ch.has(s.ctx, 0); err != nil || !ok {
		return nil, err
	}
	identities := map[string]bool{}
	for _, k := range ch.candidateKeys {
		identities[k.Identity] = true
	}
	for _, m := range s.members.values() {
		if !identities[m.key.Identity] {
			continue
		}
		is, err := s.isCandidate(r, m)
		if err != nil {
			return nil, err
		}
		if is {
			return m.candidate, nil
		}
	}
	return nil, nil
}

// memberAmongCandidates is the member that is one of the candidates, loading
// all of them.
func (s *search) memberAmongCandidates(ch *choices) (Candidate, error) {
	for i := 0; ; i++ {
		ok, err := ch.has(s.ctx, i)
		if err != nil || !ok {
			return nil, err
		}
		if m := s.members.get(ch.candidateKeys[i].Identity); m != nil && m.key == ch.candidateKeys[i] {
			return m.candidate, nil
		}
	}
}

// modName is the mod name of a candidate, or "" for a file package and for a
// mod whose mod manifest cannot be read (which installing it reports).
func (s *search) modName(c Candidate) (string, error) {
	name, err := c.ModName(s.ctx)
	if err != nil {
		if isContextError(err) {
			return "", err
		}
		return "", nil
	}
	return name, nil
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func isPinned(c Candidate) bool {
	src := c.Source()
	return src.Spec && src.Pinned
}

// isPinnedSatisfier: whether a candidate resolved from a remote repository
// with a ref stands in for a requirement without a ref (protocol/workspace.md
// §3): same repository (ignoring ASCII case) and variant, and a version in
// the requirement's range. A dependency of a tooth.json on "client" names
// the variant "" of a package that has a package file
// (protocol/compatibility.md §2).
func isPinnedSatisfier(r *requirement, c Candidate) bool {
	pinned := c.Source()
	if !pinned.Spec || !pinned.Pinned || !r.ref.specWithoutRef() {
		return false
	}
	if r.rng != nil && !r.rng.Contains(c.Version()) {
		return false
	}
	wanted := r.ref
	return namesEqual(wanted.Target, pinned.Target) &&
		(wanted.Variant == c.Variant() ||
			(r.compat && !c.Derived() && wanted.Variant == ClientLabel && c.Variant() == ""))
}

// isCandidate: whether a member is one of the candidates of a requirement,
// without enumerating the candidates of other versions.
func (s *search) isCandidate(r *requirement, m *member) (bool, error) {
	version := m.candidate.Version()
	if r.rng != nil && !r.rng.Contains(version) {
		return false, nil
	}
	rng := r.rng
	if r.ref.Spec {
		rng = ExactVersion{version}
	}
	seq := s.resolve(r.ref, rng, r.compat)
	for i := 0; ; i++ {
		ok, err := seq.has(s.ctx, i)
		if err != nil || !ok {
			return false, err
		}
		if m.key == KeyOf(seq.items[i]) {
			return true, nil
		}
	}
}

func (s *search) addEdges(owner Candidate, level int, suffix string) {
	for _, d := range owner.Dependencies() {
		s.agenda = append(s.agenda, &requirement{
			key:   keyOf(d.Source, d.Range, d.ClientCompat),
			owner: owner,
			description: fmt.Sprintf("%s%s requires %s", show(owner), suffix,
				showRequirement(d.Source, d.Range)),
			ownerLevel: level,
			ref:        d.Source,
			rng:        d.Range,
			compat:     d.ClientCompat,
		})
	}
}

func (s *search) getChoices(r *requirement) *choices {
	ch, ok := s.choices[r.key]
	if !ok {
		ch = &choices{sequence: s.resolve(r.ref, r.rng, r.compat), keys: map[PackageKey]bool{}}
		s.choices[r.key] = ch
	}
	return ch
}

func (s *search) resolve(ref Ref, rng Constraint, compat bool) *lazySequence {
	key := keyOf(ref, rng, compat)
	seq, ok := s.resolved[key]
	if !ok {
		seq = &lazySequence{start: func(l *lazySequence) Sequence {
			return s.resolver.ResolveLazy(s.ctx, ref, rng, compat, func(d Diagnostic) {
				l.diagnostics = append(l.diagnostics, d)
			})
		}}
		s.resolved[key] = seq
	}
	return seq
}

func (s *search) recordFailure(r *requirement, ch *choices, blocked []string, failedDeeper int) {
	if len(ch.candidates) > 0 && len(blocked) == 0 && failedDeeper == 0 {
		// Skipped: the candidates were already found unsatisfiable for another owner.
		s.addFailure(r.description + ", but none of its versions can be installed.")
		return
	}

	if len(ch.candidates) == 0 {
		diagnostics := ch.sequence.diagnostics
		var b strings.Builder
		b.WriteString(r.description + ", but no version of it is available")
		if len(diagnostics) == 0 {
			b.WriteString(".")
		} else {
			b.WriteString(":")
		}
		for i, d := range diagnostics {
			if i == MaxListedReasons {
				break
			}
			b.WriteString("\n      - " + d.Message)
		}
		if len(diagnostics) > MaxListedReasons {
			fmt.Fprintf(&b, "\n      ... and %d more", len(diagnostics)-MaxListedReasons)
		}
		line := b.String()
		seen := s.reported[line]
		if len(s.unresolvable) < MaxRecordedFailures && !s.reported[line] {
			s.reported[line] = true
			s.unresolvable = append(s.unresolvable, line)
			for _, d := range diagnostics {
				if d.Cause != nil {
					s.addCause(d.Cause)
				}
			}
		}
		if !seen {
			s.traceStep(line)
		}
		return
	}

	reasons := blocked[:min(len(blocked), MaxListedReasons):min(len(blocked), MaxListedReasons)]
	if len(blocked) > MaxListedReasons {
		reasons = append(reasons, fmt.Sprintf("%d more versions are blocked likewise",
			len(blocked)-MaxListedReasons))
	}
	if failedDeeper > 0 {
		if failedDeeper == 1 {
			reasons = append(reasons, "1 version needs packages that cannot be installed with it")
		} else {
			reasons = append(reasons, fmt.Sprintf("%d versions need packages that cannot be"+
				" installed with them", failedDeeper))
		}
	}
	s.addFailure(fmt.Sprintf("%s, but no version of it fits: %s.", r.description,
		strings.Join(reasons, "; ")))
}

// addCause keeps an error behind a failure, once.
func (s *search) addCause(cause error) {
	if len(s.causes) >= MaxReportedFailures {
		return
	}
	for _, c := range s.causes {
		if sameError(c, cause) {
			return
		}
	}
	s.causes = append(s.causes, cause)
}

// sameError is reference equality of exceptions: == where the dynamic type
// allows it.
func sameError(a, b error) bool {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	return ta == tb && ta.Comparable() && a == b
}

// addFailure keeps the most recent distinct failures.
func (s *search) addFailure(line string) {
	if s.reported[line] {
		return
	}
	s.reported[line] = true
	if len(s.failures) == MaxRecordedFailures {
		s.failures = s.failures[1:]
	}
	s.failures = append(s.failures, line)
}

func installedSuffix(level int) string {
	if level < 0 {
		return " (installed)"
	}
	return ""
}

func show(c Candidate) string { return c.Identity() + "@" + c.Version().String() }

// showSource is a candidate with where it comes from, to tell apart packages
// of one identity.
func showSource(c Candidate) string { return show(c) + " (from " + c.ArchiveKey() + ")" }

// showRequirement is a source with the versions a requirement accepts.
func showRequirement(source Ref, rng Constraint) string {
	if rng == nil {
		return source.Text + " (any version)"
	}
	return source.Text + " " + rng.String()
}
