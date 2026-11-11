package graph

import (
	"context"
	"slices"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// ClientLabel is ToothCompat.ClientLabel: the variant "client" that a
// dependency of a tooth.json names, which stands for the variant "" of a
// package that has a package file (protocol/compatibility.md §2).
const ClientLabel = "client"

// Ref is what the solver needs of a source (Ref.cs).
type Ref struct {
	// Text is Ref.ToString(); candidate sequences are keyed by it.
	Text string
	// Spec: the source is a remote repository (RefKind.Spec).
	Spec bool
	// Pinned: a remote repository with a git ref (branch, tag, commit), whose
	// package stands in for the versions of that repository.
	Pinned bool
	// Target is the repository of a Spec ref, compared ignoring ASCII case.
	Target string
	// Variant is the variant the ref names ("" for the default variant).
	Variant string
	// Compat tells apart refs of the same text that LeviLauncher's
	// temporary compatibility layer (engine, compat_legacy_variants.go)
	// resolves differently; candidate sequences are keyed by it too. Always
	// "" without that layer.
	Compat string
}

func (r Ref) String() string { return r.Text }

// specWithoutRef is `Ref is { Kind: RefKind.Spec, GitRef: null }`.
func (r Ref) specWithoutRef() bool { return r.Spec && !r.Pinned }

// Constraint is IVersionConstraint: the versions a requirement accepts. A nil
// Constraint accepts every version.
type Constraint interface {
	Contains(v semver.Version) bool
	// String is how failures show it.
	String() string
	// Key identifies the constraint in candidate sequence keys
	// (ResolveKey.RangeKey).
	Key() string
}

// Range is VersionRange.
type Range struct{ semver.Range }

func (r Range) Key() string { return "range:" + r.Range.String() }

// ExactVersion is ExactVersionConstraint: a version of the same precedence.
// The solver passes it to Resolver.ResolveLazy to look up exactly the version
// of a member (a resolver of a remote repository resolves that tag only).
type ExactVersion struct{ Version semver.Version }

func (e ExactVersion) Contains(v semver.Version) bool { return e.Version.Compare(v) == 0 }
func (e ExactVersion) String() string                 { return "=" + e.Version.String() }
func (e ExactVersion) Key() string                    { return "exact:" + e.Version.String() }

// Dependency is PackageDependency: a source and a range.
type Dependency struct {
	Source Ref
	Range  Constraint
}

// Request is PackageRequest.
type Request struct {
	Source Ref
	Range  Constraint
}

// Candidate is what the solver needs of a candidate (Contracts.cs Candidate,
// with its Package and Archive). Implementations are usually pointers, and
// are returned in the solution as given.
type Candidate interface {
	// Identity is Package.Identity: the package id, then '#' and the variant
	// when that is not "".
	Identity() string
	Version() semver.Version
	// Variant is Package.Variant.
	Variant() string
	// Commit is the commit of its archive, or "".
	Commit() string
	// ArchiveKey is ArchiveHandle.Key, shown to tell packages apart.
	ArchiveKey() string
	// Source is the ref it was resolved for.
	Source() Ref
	Dependencies() []Dependency
	// ModName is the mod name of a mod ("" for a package that is not a mod),
	// obtaining its asset of type mcmod. An error is McmodInstallException (a
	// missing or invalid mod manifest, an asset that cannot be obtained),
	// which makes the candidate fail whatever the other members are; only a
	// context error stops the search. Implementations cache the result, as
	// Candidate.GetModManifest does.
	ModName(ctx context.Context) (string, error)
}

// Diagnostic is ResolveDiagnostic: why a path produced no candidate.
type Diagnostic struct {
	Message string
	Cause   error
}

// Sequence is a lazily produced candidate sequence.
type Sequence interface {
	// Next returns the next candidate, or false at the end. An error stops the
	// search.
	Next(ctx context.Context) (Candidate, bool, error)
}

// Resolver is IResolver: Resolve(source, range, environment) of
// protocol/package.md §4, producing candidates lazily and reporting a
// diagnostic for every path that produced none.
type Resolver interface {
	ResolveLazy(ctx context.Context, source Ref, rng Constraint, onDiagnostic func(Diagnostic)) Sequence
}

// SliceSequence is a Sequence over a slice.
type SliceSequence []Candidate

func (s *SliceSequence) Next(context.Context) (Candidate, bool, error) {
	if len(*s) == 0 {
		return nil, false, nil
	}
	c := (*s)[0]
	*s = (*s)[1:]
	return c, true, nil
}

// Fold is PackageNames.Fold: lowercases ASCII letters only.
func Fold(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, name)
}

// namesEqual is PackageNames.AreEqual.
func namesEqual(a, b string) bool { return Fold(a) == Fold(b) }

// PackageKey is the "same package" identity of a candidate
// (protocol/workspace.md §5): its folded identity, version and lowercase
// commit.
type PackageKey struct {
	Identity string
	// Version is the version as text, which tells versions apart exactly as
	// SemVersion equality does.
	Version string
	Commit  string
}

// KeyOf is PackageKey.Of.
func KeyOf(c Candidate) PackageKey {
	return PackageKey{Fold(c.Identity()), c.Version().String(), strings.ToLower(c.Commit())}
}

// compareSortOrder is SemVersion.SortOrderComparer: precedence, then the
// metadata identifiers (ordinal; fewer first).
func compareSortOrder(a, b semver.Version) int {
	if c := a.Compare(b); c != 0 {
		return c
	}
	for i := 0; i < min(len(a.Metadata), len(b.Metadata)); i++ {
		if c := compareOrdinal(a.Metadata[i], b.Metadata[i]); c != 0 {
			return c
		}
	}
	return len(a.Metadata) - len(b.Metadata)
}

// compareOrdinal is StringComparer.Ordinal: by UTF-16 code units.
func compareOrdinal(a, b string) int {
	if isASCII(a) && isASCII(b) {
		return strings.Compare(a, b)
	}
	return slices.Compare(utf16Units(a), utf16Units(b))
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}
