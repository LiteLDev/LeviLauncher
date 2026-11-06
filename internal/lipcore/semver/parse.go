package semver

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// styles mirrors SemVersionStyles.
type styles uint32

const (
	styleStrict             styles = 0
	styleAllowLeadingZeros  styles = 1
	styleAllowLeadingWS     styles = 1 << 1
	styleAllowTrailingWS    styles = 1 << 2
	styleAllowLowerV        styles = 1 << 3
	styleAllowUpperV        styles = 1 << 4
	styleOptionalPatch      styles = 1 << 5
	styleOptionalMinorPatch styles = 1<<6 | styleOptionalPatch
	styleAny                styles = math.MaxUint32
)

func (s styles) has(f styles) bool { return s&f == f }

// wildcardVersion mirrors Semver.Parsing.WildcardVersion.
type wildcardVersion uint8

const (
	wildcardNone            wildcardVersion = 0
	wildcardPrerelease      wildcardVersion = 1 << 0
	wildcardPatch           wildcardVersion = 1 << 1
	wildcardMinor           wildcardVersion = 1 << 2
	wildcardMajor           wildcardVersion = 1 << 3
	wildcardMinorPatch                      = wildcardMinor | wildcardPatch
	wildcardMajorMinorPatch                 = wildcardMajor | wildcardMinorPatch
)

// parsingOptions mirrors SemVersionParsingOptions.
type parsingOptions struct {
	allowWildcardMajorMinorPatch bool
	allowWildcardPrerelease      bool
	missingVersionsAreWildcards  bool
	isWildcard                   func(byte) bool
}

var noWildcards = parsingOptions{isWildcard: func(byte) bool { return false }}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlphaOrHyphen(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '-'
}

// utf16Len is the length .NET would report for s.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// charAt returns the (possibly multi-byte) character starting at s[i] for
// error messages.
func charAt(s string, i int) string {
	r, _ := utf8.DecodeRuneInString(s[i:])
	return string(r)
}

// parseVersion ports SemVersionParser.Parse.
func parseVersion(version string, style styles, opts *parsingOptions, maxLength int) (Version, wildcardVersion, error) {
	wildcard := wildcardNone
	fail := func(format string, args ...any) (Version, wildcardVersion, error) {
		return Version{}, wildcardNone, fmt.Errorf("semver: "+format, args...)
	}

	if len(version) == 0 {
		return fail("empty string is not a valid version")
	}
	if utf16Len(version) > maxLength {
		return fail("exceeded maximum length of %d for %q", maxLength, version)
	}

	main := strings.TrimLeftFunc(version, unicode.IsSpace)
	if main == "" {
		return fail("whitespace is not a valid version")
	}
	if len(main) < len(version) && !style.has(styleAllowLeadingWS) {
		return fail("version %q has leading whitespace", version)
	}
	withTrailing := len(main)
	main = strings.TrimRightFunc(main, unicode.IsSpace)
	hasTrailingWS := withTrailing > len(main)

	main, metadataSeg, hasMetadata := strings.Cut(main, "+")
	mmp, prereleaseSeg, hasPrerelease := strings.Cut(main, "-")

	if mmp != "" {
		switch mmp[0] {
		case 'v':
			if !style.has(styleAllowLowerV) {
				return fail("leading 'v' in %q", version)
			}
			mmp = mmp[1:]
		case 'V':
			if !style.has(styleAllowUpperV) {
				return fail("leading 'V' in %q", version)
			}
			mmp = mmp[1:]
		}
	}

	allowLeadingZeros := style.has(styleAllowLeadingZeros)
	numbers := strings.Split(mmp, ".")
	var v Version

	parseNumber := func(kind string, idx int, optional, wildcardRequired bool) (uint64, bool, error) {
		if idx >= len(numbers) {
			if !optional {
				return 0, false, fmt.Errorf("semver: %s version missing in %q", kind, version)
			}
			return 0, opts.missingVersionsAreWildcards, nil
		}
		return parseVersionNumber(kind, version, numbers[idx], allowLeadingZeros, wildcardRequired, opts)
	}

	var majorWild, minorWild, patchWild bool
	var err error
	if v.Major, majorWild, err = parseNumber("Major", 0, false, false); err != nil {
		return Version{}, wildcardNone, err
	}
	if majorWild {
		wildcard |= wildcardMajorMinorPatch
	}
	minorOptional := style.has(styleOptionalMinorPatch) || majorWild
	if v.Minor, minorWild, err = parseNumber("Minor", 1, minorOptional, majorWild); err != nil {
		return Version{}, wildcardNone, err
	}
	if minorWild {
		wildcard |= wildcardMinorPatch
	}
	patchOptional := style.has(styleOptionalPatch) || majorWild || minorWild
	if v.Patch, patchWild, err = parseNumber("Patch", 2, patchOptional, minorWild); err != nil {
		return Version{}, wildcardNone, err
	}
	if patchWild {
		wildcard |= wildcardPatch
	}
	if len(numbers) > 3 {
		fourth := numbers[3]
		if len(fourth) > 0 && isDigit(fourth[0]) {
			return fail("fourth version number in %q", version)
		}
		return fail("the prerelease identifiers should be prefixed by '-' instead of '.' in %q", version)
	}

	if hasPrerelease {
		ids, isWildcard, err := parsePrerelease(version, prereleaseSeg, allowLeadingZeros, opts)
		if err != nil {
			return Version{}, wildcardNone, err
		}
		v.Prerelease = ids
		if isWildcard {
			wildcard |= wildcardPrerelease
		}
	}

	if hasMetadata {
		for _, id := range strings.Split(metadataSeg, ".") {
			if id == "" {
				return fail("missing metadata identifier in %q", version)
			}
			for i := 0; i < len(id); i++ {
				if c := id[i]; !isAlphaOrHyphen(c) && !isDigit(c) {
					return fail("invalid character '%s' in metadata identifier in %q", charAt(id, i), version)
				}
			}
			v.Metadata = append(v.Metadata, id)
		}
	}

	if hasTrailingWS && !style.has(styleAllowTrailingWS) {
		return fail("version %q has trailing whitespace", version)
	}
	return v, wildcard, nil
}

func parseVersionNumber(kind, version, segment string, allowLeadingZeros, wildcardRequired bool, opts *parsingOptions) (uint64, bool, error) {
	if segment == "" {
		return 0, false, fmt.Errorf("semver: %s version missing in %q", kind, version)
	}
	if opts.allowWildcardMajorMinorPatch && opts.isWildcard(segment[0]) {
		if len(segment) > 1 {
			return 0, true, fmt.Errorf("semver: %s version is a wildcard and should contain only 1 character in %q", kind, version)
		}
		return 0, true, nil
	}
	if wildcardRequired {
		return 0, false, fmt.Errorf("semver: %s version should be a wildcard because the preceding version is a wildcard in %q", kind, version)
	}
	withZeros := len(segment)
	segment = trimLeadingZeros(segment)
	for i := 0; i < len(segment); i++ {
		if !isDigit(segment[i]) {
			return 0, false, fmt.Errorf("semver: %s version contains invalid character '%s' in %q", kind, charAt(segment, i), version)
		}
	}
	if !allowLeadingZeros && withZeros > len(segment) {
		return 0, false, fmt.Errorf("semver: %s version has leading zero in %q", kind, version)
	}
	n, ok := parseUint(segment)
	if !ok {
		return 0, false, fmt.Errorf("semver: %s version %q is too large in %q", kind, segment, version)
	}
	return n, false, nil
}

func parseUint(digits string) (uint64, bool) {
	var n uint64
	for i := 0; i < len(digits); i++ {
		d := uint64(digits[i] - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}

func parsePrerelease(version, segment string, allowLeadingZeros bool, opts *parsingOptions) ([]string, bool, error) {
	var ids []string
	isWildcard := false
	for _, id := range strings.Split(segment, ".") {
		if isWildcard {
			return nil, false, fmt.Errorf("semver: prerelease identifier follows wildcard prerelease identifier in %q", version)
		}
		if id == "" {
			return nil, false, fmt.Errorf("semver: missing prerelease identifier in %q", version)
		}
		isNumeric := true
		for i := 0; i < len(id); i++ {
			c := id[i]
			switch {
			case isAlphaOrHyphen(c):
				isNumeric = false
			case opts.allowWildcardPrerelease && opts.isWildcard(c):
				isWildcard = true
			case !isDigit(c):
				return nil, false, fmt.Errorf("semver: invalid character '%s' in prerelease identifier in %q", charAt(id, i), version)
			}
		}
		if isWildcard {
			if len(id) > 1 {
				return nil, false, fmt.Errorf("semver: prerelease version is a wildcard and should contain only 1 character in %q", version)
			}
			continue
		}
		if isNumeric && id[0] == '0' && len(id) > 1 {
			if !allowLeadingZeros {
				return nil, false, fmt.Errorf("semver: leading zero in prerelease identifier in version %q", version)
			}
			id = trimLeadingZeros(id)
		}
		ids = append(ids, id)
	}
	return ids, isWildcard, nil
}
