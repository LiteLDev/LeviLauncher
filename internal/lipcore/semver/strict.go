package semver

import (
	"cmp"
	"fmt"
	"strings"
)

// This file is the lip 0.38 surface of the package: strict versions,
// ranges with their original text (VersionRange), exact constraints and
// version tags. It ports
//   - src/Lip.Core/Mcmod/VersionRange.cs (VersionRange,
//     ExactVersionConstraint),
//   - src/Lip.Core/Mcmod/Archives/ArchiveProvider.cs (ParseVersionTag and
//     the order of ListVersions),
//   - SemVersion.Equals / PrecedenceComparer / SortOrderComparer of
//     WalkerCodeRanger Semver 3.0.0,
// at lip tag v0.38.0-beta.1 (protocol/package.md §1, §2.1, §4).

// ParseStrict parses s as SemVersion.Parse(s, SemVersionStyles.Strict):
// SemVer 2.0 with no leading "v", no leading zeros, no surrounding
// whitespace and all of major, minor and patch.
func ParseStrict(s string) (Version, error) { return Parse(s) }

// Equal is SemVersion.Equals: major, minor, patch, prerelease and build
// metadata identifiers are all equal. Versions that differ only in metadata
// are not Equal but have equal precedence (ComparePrecedence == 0).
func (v Version) Equal(o Version) bool { return v.equal(o) }

// ComparePrecedence is SemVersion.PrecedenceComparer.Compare: SemVer 2.0
// precedence, build metadata ignored. Returns -1, 0 or 1.
func ComparePrecedence(a, b Version) int { return a.Compare(b) }

// CompareSortOrder is SemVersion.SortOrderComparer.Compare: precedence
// first; at equal precedence the metadata identifiers are compared
// ordinally one by one, and then the shorter list (no metadata first)
// sorts before the longer. For parsed versions (whose numeric prerelease
// identifiers carry no leading zeros) it is 0 exactly when a.Equal(b).
func CompareSortOrder(a, b Version) int {
	if c := a.Compare(b); c != 0 {
		return c
	}
	n := min(len(a.Metadata), len(b.Metadata))
	for i := 0; i < n; i++ {
		if c := strings.Compare(a.Metadata[i], b.Metadata[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.Metadata), len(b.Metadata))
}

// Constraint is IVersionConstraint (src/Lip.Core/Mcmod/Contracts.cs): what
// a request or dependency puts on the version of a package. VersionRange
// and ExactVersionConstraint implement it.
type Constraint interface {
	// Contains is IsSatisfiedBy.
	Contains(v Version) bool
	String() string
}

// VersionRangeSyntax is VersionRange.Syntax: how a range is written, for
// messages.
const VersionRangeSyntax = "A version range is written like 1.2.3, >=1.2.0 <2.0.0," +
	" ^1.2.0, ~1.2.0, 1.* or *, with SemVer versions without a leading 'v', and" +
	" alternatives separated by ||."

// VersionRange is lip's VersionRange (protocol/package.md §1): a string
// that SemVersionRange.Parse of Semver 3.0.0 accepts with its default
// options (standard syntax, not npm), with the matching semantics of that
// range. The text is kept exactly as written: String, cache keys and
// messages use it, never the normalized form.
//
// The zero VersionRange has Text "" and contains no version; use
// ParseVersionRange to obtain one.
type VersionRange struct {
	// Text is the range exactly as written.
	Text string
	// Range is the parsed range (SemVersionRange).
	Range Range
}

// RangeError is the FormatException VersionRange.Parse throws: Error() is
// lip's message, and Unwrap returns the parser's reason (the C# inner
// exception).
type RangeError struct {
	Text string
	Err  error
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("The version range '%s' is not valid. %s", e.Text, VersionRangeSyntax)
}

func (e *RangeError) Unwrap() error { return e.Err }

// ParseVersionRange is VersionRange.Parse: SemVersionRange.Parse(s) with
// the default options; on failure the error is a *RangeError.
func ParseVersionRange(s string) (VersionRange, error) {
	r, err := ParseRange(s)
	if err != nil {
		return VersionRange{}, &RangeError{Text: s, Err: err}
	}
	return VersionRange{Text: s, Range: r}, nil
}

// MustParseVersionRange is ParseVersionRange that panics on error.
func MustParseVersionRange(s string) VersionRange {
	r, err := ParseVersionRange(s)
	if err != nil {
		panic(err)
	}
	return r
}

// Contains is VersionRange.IsSatisfiedBy (SemVersionRange.Contains).
func (r VersionRange) Contains(v Version) bool { return r.Range.Contains(v) }

// String is VersionRange.ToString: the text as written.
func (r VersionRange) String() string { return r.Text }

// ExactVersionConstraint is lip's ExactVersionConstraint: it accepts
// exactly the versions with the same SemVer precedence as Version (build
// metadata is ignored). Used to look up one version of a repository.
// Construct it with ExactVersion; the zero value is the constraint "=0.0.0".
type ExactVersionConstraint struct {
	Version Version
}

// ExactVersion returns the ExactVersionConstraint for v.
func ExactVersion(v Version) ExactVersionConstraint { return ExactVersionConstraint{Version: v} }

// Contains is ExactVersionConstraint.IsSatisfiedBy.
func (c ExactVersionConstraint) Contains(v Version) bool { return c.Version.Compare(v) == 0 }

// String is ExactVersionConstraint.ToString: "=" and the version, metadata
// included.
func (c ExactVersionConstraint) String() string { return "=" + c.Version.String() }

var (
	_ Constraint = VersionRange{}
	_ Constraint = ExactVersionConstraint{}
)

const (
	// TagsRefPrefix is the prefix of tag ref names.
	TagsRefPrefix = "refs/tags/"
	// VersionTagRefPrefix is the prefix of version tag ref names
	// (protocol/package.md §2.1: a tag named "v" followed by a version).
	VersionTagRefPrefix = TagsRefPrefix + "v"
)

// ParseVersionTag is ArchiveProvider.ParseVersionTag: refName is a full ref
// name; it is a version tag when it is "refs/tags/v" followed by a version
// that parses strictly (so "v1.2", "v01.2.3", "V1.2.3" and "1.2.3" are
// not). Build metadata is kept.
func ParseVersionTag(refName string) (Version, bool) {
	rest, ok := strings.CutPrefix(refName, VersionTagRefPrefix)
	if !ok {
		return Version{}, false
	}
	v, err := Parse(rest)
	if err != nil {
		return Version{}, false
	}
	return v, true
}

// TagVersion is a version tag: its version and its tag name below
// refs/tags/ (e.g. "v2.0.0+b").
type TagVersion struct {
	Version Version
	Tag     string
}

// CompareTagVersions orders version tags the way ArchiveProvider.ListVersions
// lists them (protocol/package.md §4): highest precedence first, and at
// equal precedence the ordinally greater tag name first. Tag is the name
// below refs/tags/ of a ref ParseVersionTag accepted, so it is ASCII and the
// byte order here equals C#'s UTF-16 ordinal order. For use with
// slices.SortFunc / SortStableFunc.
func CompareTagVersions(a, b TagVersion) int {
	if c := b.Version.Compare(a.Version); c != 0 {
		return c
	}
	return strings.Compare(b.Tag, a.Tag)
}
