// Package semver implements SemVer 2.0 versions and version ranges with the
// semantics lip gets from WalkerCodeRanger Semver 3.0.0 (SemVersion /
// SemVersionRange, "standard" range syntax, strict parsing). strict.go holds
// the lip 0.38 surface (ParseStrict, VersionRange, ExactVersionConstraint,
// ParseVersionTag).
//
// Version numbers have arbitrary size (BigInteger in the C# library, Number
// here), and parse errors carry the C# library's FormatException messages.
package semver

import (
	"cmp"
	"slices"
	"strings"
)

// maxVersionLength is SemVersion.MaxVersionLength.
const maxVersionLength = 1024

// Version is a SemVer 2.0 version.
type Version struct {
	Major, Minor, Patch Number
	// Prerelease identifiers, e.g. ["client", "11"] for "-client.11".
	Prerelease []string
	// Metadata (build) identifiers; ignored for precedence.
	Metadata []string
}

// Parse parses s strictly as SemVer 2.0 (SemVersion.Parse with
// SemVersionStyles.Strict): no "v" prefix, no leading zeros, all three
// components required.
func Parse(s string) (Version, error) {
	v, _, err := parseVersion(s, styleStrict, &noWildcards, maxVersionLength)
	return v, err
}

// ParseAny parses s like SemVersion.Parse with SemVersionStyles.Any (used by
// lip for tooth.json "version"): allows leading "v"/"V", leading zeros,
// missing minor/patch, surrounding whitespace.
func ParseAny(s string) (Version, error) {
	v, _, err := parseVersion(s, styleAny, &noWildcards, maxVersionLength)
	return v, err
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
	if c := v.Major.Compare(o.Major); c != 0 {
		return c
	}
	if c := v.Minor.Compare(o.Minor); c != 0 {
		return c
	}
	if c := v.Patch.Compare(o.Patch); c != 0 {
		return c
	}
	// A release has higher precedence than any prerelease.
	switch {
	case len(v.Prerelease) == 0 && len(o.Prerelease) == 0:
		return 0
	case len(v.Prerelease) == 0:
		return 1
	case len(o.Prerelease) == 0:
		return -1
	}
	n := min(len(v.Prerelease), len(o.Prerelease))
	for i := 0; i < n; i++ {
		if c := compareIdentifier(v.Prerelease[i], o.Prerelease[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(v.Prerelease), len(o.Prerelease))
}

// IsPrerelease reports whether v has prerelease identifiers.
func (v Version) IsPrerelease() bool { return len(v.Prerelease) > 0 }

// String formats v as SemVersion.ToString does ("1.2.3-a.1+b").
func (v Version) String() string {
	var b strings.Builder
	b.WriteString(v.Major.String())
	b.WriteByte('.')
	b.WriteString(v.Minor.String())
	b.WriteByte('.')
	b.WriteString(v.Patch.String())
	if len(v.Prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Prerelease, "."))
	}
	if len(v.Metadata) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Metadata, "."))
	}
	return b.String()
}

// Range is a version range in WalkerCodeRanger Semver 3 standard syntax:
// exact ("1.2.3"), wildcards ("*", "1.*", "1.3.*", "*-*"), caret, tilde,
// comparison operators and "||" unions (standard syntax has no hyphen
// ranges; those are npm syntax, see ParseNpmRange). Prerelease versions
// only satisfy a range if the matching comparator mentions a prerelease of
// the same major.minor.patch, unless the range includes all prereleases
// ("*-*" style).
//
// The zero Range is the empty range: it contains nothing and prints as "".
type Range struct {
	raw string
	// ranges are the normalized (sorted, merged, non-empty) unbroken ranges
	// whose union is the range.
	ranges []unbrokenRange
}

// ParseRange parses s as SemVersionRange.Parse(s) (strict, standard syntax).
func ParseRange(s string) (Range, error) {
	ranges, err := parseStandardRange(s, false, maxRangeLength)
	if err != nil {
		return Range{}, err
	}
	return newRange(ranges), nil
}

// ParseNpmRange parses s like SemVersionRange.ParseNpm (used only by the
// tooth.json v1/v2 migration for platform-level dependencies).
func ParseNpmRange(s string) (Range, error) {
	ranges, err := parseNpmRange(s, false, maxRangeLength)
	if err != nil {
		return Range{}, err
	}
	return newRange(ranges), nil
}

// Contains reports whether v satisfies r.
func (r Range) Contains(v Version) bool {
	for i := range r.ranges {
		if r.ranges[i].contains(v) {
			return true
		}
	}
	return false
}

// String returns the range text as lip would write it back
// (SemVersionRange.ToString()). Implementations should return the original
// text when it is already in normalized form.
func (r Range) String() string { return r.raw }

// newRange is SemVersionRange.Create: drop empty ranges, sort, merge.
func newRange(ranges []unbrokenRange) Range {
	ranges = normalizeRanges(ranges)
	parts := make([]string, len(ranges))
	for i := range ranges {
		parts[i] = ranges[i].String()
	}
	return Range{raw: strings.Join(parts, " || "), ranges: ranges}
}

// --- helpers ---

func (v Version) withoutMetadata() Version {
	v.Metadata = nil
	return v
}

func (v Version) prereleaseIsZero() bool {
	return len(v.Prerelease) == 1 && v.Prerelease[0] == "0"
}

func (v Version) majorMinorPatchEquals(o Version) bool {
	return v.Major == o.Major && v.Minor == o.Minor && v.Patch == o.Patch
}

// equal is SemVersion.Equals (structural, including metadata).
func (v Version) equal(o Version) bool {
	return v.majorMinorPatchEquals(o) &&
		slices.Equal(v.Prerelease, o.Prerelease) &&
		slices.Equal(v.Metadata, o.Metadata)
}

// isNumericIdentifier reports whether a prerelease identifier is numeric.
func isNumericIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// compareIdentifier is PrereleaseIdentifier.CompareTo: numeric identifiers
// compare numerically and are lower than alphanumeric ones, which compare
// ordinally.
func compareIdentifier(a, b string) int {
	an, bn := isNumericIdentifier(a), isNumericIdentifier(b)
	switch {
	case an && bn:
		a, b = trimLeadingZeros(a), trimLeadingZeros(b)
		if c := cmp.Compare(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

// nextIdentifier is PrereleaseIdentifier.NextIdentifier: the smallest
// identifier greater than id (numeric+1, otherwise id+"-").
func nextIdentifier(id string) string {
	if !isNumericIdentifier(id) {
		return id + "-"
	}
	d := []byte(trimLeadingZeros(id))
	for i := len(d) - 1; i >= 0; i-- {
		if d[i] != '9' {
			d[i]++
			return string(d)
		}
		d[i] = '0'
	}
	return "1" + string(d)
}

// trimLeadingZeros removes leading '0's but keeps at least one character.
func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}
