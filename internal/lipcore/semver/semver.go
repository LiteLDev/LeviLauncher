// Package semver implements SemVer 2.0 versions and version ranges with the
// semantics lip 0.37.0 gets from WalkerCodeRanger Semver 3.0.0
// (SemVersion / SemVersionRange, "standard" range syntax, strict parsing).
package semver

import "errors"

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("semver: not implemented")

// Version is a SemVer 2.0 version.
type Version struct {
	Major, Minor, Patch uint64
	// Prerelease identifiers, e.g. ["client", "11"] for "-client.11".
	Prerelease []string
	// Metadata (build) identifiers; ignored for precedence.
	Metadata []string
}

// Parse parses s strictly as SemVer 2.0 (SemVersion.Parse with
// SemVersionStyles.Strict): no "v" prefix, no leading zeros, all three
// components required.
func Parse(s string) (Version, error) {
	return Version{}, ErrNotImplemented
}

// ParseAny parses s like SemVersion.Parse with SemVersionStyles.Any (used by
// lip for tooth.json "version"): allows leading "v"/"V", leading zeros,
// missing minor/patch, surrounding whitespace.
func ParseAny(s string) (Version, error) {
	return Version{}, ErrNotImplemented
}

// ParseTag converts a git tag name (without "refs/tags/") to a version the
// way lip's GitRemotePackageRegistry does: Go semver.Canonical (so "v1.2"
// becomes "v1.2.0" and "+build" is dropped), strip the leading "v", then
// Parse strictly. ok is false for tags that are not valid semver.
func ParseTag(tag string) (v Version, ok bool) {
	return Version{}, false
}

// MustParse is Parse that panics on error.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Compare compares precedence per SemVer 2.0 (metadata ignored); returns
// -1, 0 or 1.
func (v Version) Compare(o Version) int {
	panic("semver: not implemented")
}

// IsPrerelease reports whether v has prerelease identifiers.
func (v Version) IsPrerelease() bool { return len(v.Prerelease) > 0 }

// String formats v as SemVersion.ToString does ("1.2.3-a.1+b").
func (v Version) String() string {
	panic("semver: not implemented")
}

// Range is a version range in WalkerCodeRanger Semver 3 standard syntax:
// exact ("1.2.3"), wildcards ("*", "1.*", "1.3.*", "*-*"), caret, tilde,
// comparison operators, hyphen ranges and "||" unions. Prerelease versions
// only satisfy a range if the matching comparator mentions a prerelease of
// the same major.minor.patch, unless the range includes all prereleases
// ("*-*" style).
type Range struct {
	raw string
	// Implementation fields may be added.
}

// ParseRange parses s as SemVersionRange.Parse(s) (strict, standard syntax).
func ParseRange(s string) (Range, error) {
	return Range{}, ErrNotImplemented
}

// ParseNpmRange parses s like SemVersionRange.ParseNpm (used only by the
// tooth.json v1/v2 migration for platform-level dependencies).
func ParseNpmRange(s string) (Range, error) {
	return Range{}, ErrNotImplemented
}

// Exact returns the range matching exactly v (SemVersionRange.Equals(v));
// its String() is v.String().
func Exact(v Version) Range {
	panic("semver: not implemented")
}

// Contains reports whether v satisfies r.
func (r Range) Contains(v Version) bool {
	panic("semver: not implemented")
}

// String returns the range text as lip would write it back
// (SemVersionRange.ToString()). Implementations should return the original
// text when it is already in normalized form.
func (r Range) String() string { return r.raw }
