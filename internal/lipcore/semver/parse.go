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

// displayLimit is Semver.Utility.Display.Limit.
const displayLimit = 100

// limitLength is StringExtensions.LimitLength / ToStringLimitLength: text
// longer than 100 UTF-16 code units is cut to its first 97 and "..." is
// appended. A surrogate pair cut in half leaves a lone surrogate in .NET,
// which encodes as U+FFFD.
func limitLength(s string) string {
	if utf16Len(s) <= displayLimit {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if n+w > displayLimit-3 {
			if n < displayLimit-3 {
				b.WriteRune(utf8.RuneError)
			}
			break
		}
		b.WriteRune(r)
		n += w
	}
	b.WriteString("...")
	return b.String()
}

// charAt returns the character starting at s[i] as .NET formats the UTF-16
// char there: for a character outside the BMP that is a lone high
// surrogate, which encodes as U+FFFD.
func charAt(s string, i int) string {
	r, _ := utf8.DecodeRuneInString(s[i:])
	if r > 0xFFFF {
		r = utf8.RuneError
	}
	return string(r)
}

// FormatError is a version or range parse failure. Its message is the one
// of the FormatException Semver 3.0.0 throws for the same input (lip shows
// it as the cause of its own message).
type FormatError struct{ Message string }

func (e *FormatError) Error() string { return e.Message }

func formatError(format string, args ...any) error {
	return &FormatError{Message: fmt.Sprintf(format, args...)}
}

// parseVersion ports SemVersionParser.Parse.
func parseVersion(version string, style styles, opts *parsingOptions, maxLength int) (Version, wildcardVersion, error) {
	wildcard := wildcardNone
	// fail formats a message whose only argument is the version.
	fail := func(format string) (Version, wildcardVersion, error) {
		return Version{}, wildcardNone, formatError(format, limitLength(version))
	}

	if len(version) == 0 {
		return Version{}, wildcardNone, &FormatError{Message: "Empty string is not a valid version."}
	}
	if utf16Len(version) > maxLength {
		return Version{}, wildcardNone, formatError("Exceeded maximum length of %d for '%s'.", maxLength, limitLength(version))
	}

	main := strings.TrimLeftFunc(version, unicode.IsSpace)
	if main == "" {
		return Version{}, wildcardNone, &FormatError{Message: "Whitespace is not a valid version."}
	}
	if len(main) < len(version) && !style.has(styleAllowLeadingWS) {
		return fail("Version '%s' has leading whitespace.")
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
				return fail("Leading 'v' in '%s'.")
			}
			mmp = mmp[1:]
		case 'V':
			if !style.has(styleAllowUpperV) {
				return fail("Leading 'V' in '%s'.")
			}
			mmp = mmp[1:]
		}
	}

	allowLeadingZeros := style.has(styleAllowLeadingZeros)
	numbers := strings.Split(mmp, ".")
	var v Version

	parseNumber := func(kind string, idx int, optional, wildcardRequired bool) (Number, bool, error) {
		if idx >= len(numbers) {
			if !optional {
				return Number{}, false, formatError("%s version missing in '%s'.", kind, limitLength(version))
			}
			return Number{}, opts.missingVersionsAreWildcards, nil
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
			return fail("Fourth version number in '%s'.")
		}
		// "identfiers" is the library's spelling.
		return fail("The prerelease identfiers should be prefixed by '-' instead of '.' in '%s'.")
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
				return fail("Missing metadata identifier in '%s'.")
			}
			for i := 0; i < len(id); i++ {
				if c := id[i]; !isAlphaOrHyphen(c) && !isDigit(c) {
					return Version{}, wildcardNone, formatError("Invalid character '%s' in metadata identifier in '%s'.",
						charAt(id, i), limitLength(version))
				}
			}
			v.Metadata = append(v.Metadata, id)
		}
	}

	if hasTrailingWS && !style.has(styleAllowTrailingWS) {
		return fail("Version '%s' has trailing whitespace.")
	}
	return v, wildcard, nil
}

func parseVersionNumber(kind, version, segment string, allowLeadingZeros, wildcardRequired bool, opts *parsingOptions) (Number, bool, error) {
	if segment == "" {
		return Number{}, false, formatError("%s version missing in '%s'.", kind, limitLength(version))
	}
	if opts.allowWildcardMajorMinorPatch && opts.isWildcard(segment[0]) {
		if len(segment) > 1 {
			return Number{}, true, formatError("%s version is a wildcard and should contain only 1 character in '%s'.",
				kind, limitLength(version))
		}
		return Number{}, true, nil
	}
	if wildcardRequired {
		return Number{}, false, formatError("%s version should be a wildcard because the preceding version is a wildcard in '%s'.",
			kind, limitLength(version))
	}
	withZeros := len(segment)
	segment = trimLeadingZeros(segment)
	for i := 0; i < len(segment); i++ {
		if !isDigit(segment[i]) {
			return Number{}, false, formatError("%s version contains invalid character '%s' in '%s'.",
				kind, charAt(segment, i), limitLength(version))
		}
	}
	if !allowLeadingZeros && withZeros > len(segment) {
		return Number{}, false, formatError("%s version has leading zero in '%s'.", kind, limitLength(version))
	}
	return numberFromDigits(segment), false, nil
}

func parsePrerelease(version, segment string, allowLeadingZeros bool, opts *parsingOptions) ([]string, bool, error) {
	var ids []string
	isWildcard := false
	for _, id := range strings.Split(segment, ".") {
		if isWildcard {
			return nil, false, formatError("Prerelease identifier follows wildcard prerelease identifier in '%s'.", limitLength(version))
		}
		if id == "" {
			return nil, false, formatError("Missing prerelease identifier in '%s'.", limitLength(version))
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
				return nil, false, formatError("Invalid character '%s' in prerelease identifier in '%s'.",
					charAt(id, i), limitLength(version))
			}
		}
		if isWildcard {
			if len(id) > 1 {
				return nil, false, formatError("Prerelease version is a wildcard and should contain only 1 character in '%s'.",
					limitLength(version))
			}
			continue
		}
		if isNumeric && id[0] == '0' && len(id) > 1 {
			if !allowLeadingZeros {
				return nil, false, formatError("Leading zero in prerelease identifier in version '%s'.", limitLength(version))
			}
			id = trimLeadingZeros(id)
		}
		ids = append(ids, id)
	}
	return ids, isWildcard, nil
}
