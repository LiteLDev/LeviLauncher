package semver

import (
	"errors"
	"fmt"
	"math"
	"unicode"
)

// This file ports Semver.Parsing.GeneralRangeParser, StandardRangeParser and
// NpmRangeParser.

// maxRangeLength is SemVersionRange.MaxRangeLength.
const maxRangeLength = 2048

type rangeOperator int

const (
	opNone rangeOperator = iota + 1
	opEquals
	opLessThan
	opLessThanOrEqual
	opGreaterThan
	opGreaterThanOrEqual
	opTilde
	opCaret
)

var errTooLarge = errors.New("semver: version number too large for range bound")

var standardParsingOptions = parsingOptions{
	allowWildcardMajorMinorPatch: true,
	allowWildcardPrerelease:      true,
	isWildcard:                   func(c byte) bool { return c == '*' },
}

var npmParsingOptions = parsingOptions{
	allowWildcardMajorMinorPatch: true,
	missingVersionsAreWildcards:  true,
	isWildcard:                   func(c byte) bool { return c == 'x' || c == 'X' || c == '*' },
}

func isPossibleOperatorChar(c rune, allowMetadata bool) bool {
	return c == '=' || c == '<' || c == '>' || c == '~' || c == '^' ||
		(unicode.IsPunct(c) && !unicode.IsSpace(c) && c != '*') ||
		(unicode.IsSymbol(c) && (c != '+' || !allowMetadata))
}

func isPossibleVersionChar(c rune, allowMetadata bool) bool {
	return !unicode.IsSpace(c) &&
		(!isPossibleOperatorChar(c, allowMetadata) || c == '-' || c == '.' || (c == '+' && allowMetadata))
}

// splitOnOrOperator is GeneralRangeParser.SplitOnOrOperator.
func splitOnOrOperator(s []rune) [][]rune {
	var out [][]rune
	possiblyInSeparator := false
	start := 0
	for i, c := range s {
		isSep := c == '|'
		if possiblyInSeparator && isSep {
			possiblyInSeparator = false
			out = append(out, s[start:i-1])
			start = i + 1
		} else {
			possiblyInSeparator = isSep
		}
	}
	return append(out, s[start:])
}

// trimStartSpaces is StringSegment.TrimStartSpaces (only ' ').
func trimStartSpaces(s []rune) []rune {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

func trimStartWhitespace(s []rune) []rune {
	for len(s) > 0 && unicode.IsSpace(s[0]) {
		s = s[1:]
	}
	return s
}

// parseOptionalSpaces is GeneralRangeParser.ParseOptionalSpaces.
func parseOptionalSpaces(s []rune, rangeText string) ([]rune, error) {
	s = trimStartSpaces(s)
	if len(s) > 0 && unicode.IsSpace(s[0]) {
		return nil, fmt.Errorf("semver: invalid whitespace in range %q", rangeText)
	}
	return s, nil
}

// parseRangeVersion is GeneralRangeParser.ParseVersion: consume the
// version characters and parse them (metadata is removed).
func parseRangeVersion(s []rune, style styles, allowMetadata bool, opts *parsingOptions, maxLength int) ([]rune, Version, wildcardVersion, error) {
	end := 0
	for end < len(s) && isPossibleVersionChar(s[end], allowMetadata) {
		end++
	}
	v, wc, err := parseVersion(string(s[:end]), style, opts, maxLength)
	if err != nil {
		return nil, Version{}, wildcardNone, err
	}
	return s[end:], v.withoutMetadata(), wc, nil
}

func inc(n uint64) (uint64, error) {
	if n == math.MaxUint64 {
		return 0, errTooLarge
	}
	return n + 1, nil
}

func ptr(v Version) *Version { return &v }

func checkRangeLength(s string, maxLength int) error {
	if utf16Len(s) > maxLength {
		return fmt.Errorf("semver: exceeded maximum length of %d for range %q", maxLength, s)
	}
	return nil
}

// --- standard syntax ---

func parseStandardRange(s string, includeAllPrerelease bool, maxLength int) ([]unbrokenRange, error) {
	if err := checkRangeLength(s, maxLength); err != nil {
		return nil, err
	}
	var out []unbrokenRange
	for _, seg := range splitOnOrOperator([]rune(s)) {
		r, err := parseStandardUnbroken(seg, s, includeAllPrerelease, maxLength)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseStandardUnbroken(seg []rune, rangeText string, includeAllPrerelease bool, maxLength int) (unbrokenRange, error) {
	seg, err := parseOptionalSpaces(seg, rangeText)
	if err != nil {
		return unbrokenRange{}, err
	}
	if len(seg) == 0 {
		return unbrokenRange{}, fmt.Errorf("semver: range %q is missing a comparison", rangeText)
	}
	var left, right bound
	for len(seg) > 0 {
		seg, err = parseStandardComparison(seg, rangeText, &includeAllPrerelease, maxLength, &left, &right)
		if err != nil {
			return unbrokenRange{}, err
		}
	}
	return newUnbroken(left, right, includeAllPrerelease), nil
}

func parseStandardOperator(seg []rune) ([]rune, rangeOperator, error) {
	end := 0
	for end < len(seg) && isPossibleOperatorChar(seg[end], false) {
		end++
	}
	op, seg := seg[:end], seg[end:]
	if len(op) == 0 {
		return seg, opNone, nil
	}
	if len(op) > 2 || (len(op) == 2 && op[1] != '=') {
		return nil, 0, fmt.Errorf("semver: invalid operator %q", string(op))
	}
	orEqual := len(op) == 2
	switch {
	case op[0] == '=' && !orEqual:
		return seg, opEquals, nil
	case op[0] == '<' && orEqual:
		return seg, opLessThanOrEqual, nil
	case op[0] == '<':
		return seg, opLessThan, nil
	case op[0] == '>' && orEqual:
		return seg, opGreaterThanOrEqual, nil
	case op[0] == '>':
		return seg, opGreaterThan, nil
	case op[0] == '~' && !orEqual:
		return seg, opTilde, nil
	case op[0] == '^' && !orEqual:
		return seg, opCaret, nil
	}
	return nil, 0, fmt.Errorf("semver: invalid operator %q", string(op))
}

// caretUpper returns the exclusive upper bound version of a caret range.
func caretUpper(v Version, wildMinorPatch, wildPatch bool) (Version, error) {
	var out Version
	var err error
	switch {
	case v.Major != 0 || wildMinorPatch:
		out.Major, err = inc(v.Major)
	case v.Minor != 0 || wildPatch:
		out.Minor, err = inc(v.Minor)
	default:
		out.Patch, err = inc(v.Patch)
	}
	out.Prerelease = []string{"0"}
	return out, err
}

func nextMinorZero(v Version) (Version, error) {
	minor, err := inc(v.Minor)
	return Version{Major: v.Major, Minor: minor, Prerelease: []string{"0"}}, err
}

func nextMajorZero(v Version) (Version, error) {
	major, err := inc(v.Major)
	return Version{Major: major, Prerelease: []string{"0"}}, err
}

func parseStandardComparison(seg []rune, rangeText string, includeAllPrerelease *bool, maxLength int, left, right *bound) ([]rune, error) {
	seg, op, err := parseStandardOperator(seg)
	if err != nil {
		return nil, err
	}
	if seg, err = parseOptionalSpaces(seg, rangeText); err != nil {
		return nil, err
	}
	seg, v, wc, err := parseRangeVersion(seg, styleStrict, false, &standardParsingOptions, maxLength)
	if err != nil {
		return nil, err
	}
	if op != opNone && wc != wildcardNone {
		return nil, fmt.Errorf("semver: operator is combined with a wildcard version in range %q", rangeText)
	}
	if seg, err = parseOptionalSpaces(seg, rangeText); err != nil {
		return nil, err
	}

	switch op {
	case opEquals:
		*left = leftMax(*left, bound{ptr(v), true})
		*right = rightMin(*right, bound{ptr(v), true})
	case opGreaterThan:
		*left = leftMax(*left, bound{ptr(v), false})
	case opGreaterThanOrEqual:
		*left = leftMax(*left, bound{ptr(v), true})
	case opLessThan:
		*right = rightMin(*right, bound{ptr(v), false})
	case opLessThanOrEqual:
		*right = rightMin(*right, bound{ptr(v), true})
	case opCaret:
		*left = leftMax(*left, bound{ptr(v), true})
		upper, err := caretUpper(v, false, false)
		if err != nil {
			return nil, err
		}
		*right = rightMin(*right, bound{&upper, false})
	case opTilde:
		*left = leftMax(*left, bound{ptr(v), true})
		upper, err := nextMinorZero(v)
		if err != nil {
			return nil, err
		}
		*right = rightMin(*right, bound{&upper, false})
	case opNone:
		prereleaseWildcard := wc&wildcardPrerelease != 0
		*includeAllPrerelease = *includeAllPrerelease || prereleaseWildcard
		wc &^= wildcardPrerelease
		if wc != wildcardNone && v.IsPrerelease() {
			return nil, fmt.Errorf("semver: prerelease is not supported with a wildcard version in range %q", rangeText)
		}
		switch wc {
		case wildcardNone:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			upper, err := standardPrereleaseWildcardUpperBound(v, prereleaseWildcard)
			if err != nil {
				return nil, err
			}
			*right = rightMin(*right, upper)
		case wildcardMajorMinorPatch:
			// No further bound is placed on the left and right bounds.
		case wildcardMinorPatch:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			upper, err := nextMajorZero(v)
			if err != nil {
				return nil, err
			}
			*right = rightMin(*right, bound{&upper, false})
		case wildcardPatch:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			upper, err := nextMinorZero(v)
			if err != nil {
				return nil, err
			}
			*right = rightMin(*right, bound{&upper, false})
		default:
			return nil, fmt.Errorf("semver: unexpected wildcard in range %q", rangeText)
		}
	}
	return seg, nil
}

func standardWildcardLowerBound(v Version, prereleaseWildcard bool) bound {
	if prereleaseWildcard {
		pre := make([]string, 0, len(v.Prerelease)+1)
		pre = append(pre, v.Prerelease...)
		v.Prerelease = append(pre, "0")
	}
	return bound{ptr(v), true}
}

func standardPrereleaseWildcardUpperBound(v Version, prereleaseWildcard bool) (bound, error) {
	if !prereleaseWildcard {
		return bound{ptr(v), true}, nil
	}
	if v.IsPrerelease() {
		pre := make([]string, len(v.Prerelease))
		copy(pre, v.Prerelease)
		pre[len(pre)-1] = nextIdentifier(pre[len(pre)-1])
		return bound{&Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prerelease: pre}, false}, nil
	}
	patch, err := inc(v.Patch)
	if err != nil {
		return bound{}, err
	}
	return bound{&Version{Major: v.Major, Minor: v.Minor, Patch: patch, Prerelease: []string{"0"}}, false}, nil
}

// --- npm syntax ---

// npmStyles is NpmRangeParser.StandardRangeOptions converted to styles
// (AllowLowerV | OptionalMinorPatch; AllowMetadata is handled separately).
const npmStyles = styleAllowLowerV | styleOptionalMinorPatch

func parseNpmRange(s string, includeAllPrerelease bool, maxLength int) ([]unbrokenRange, error) {
	if err := checkRangeLength(s, maxLength); err != nil {
		return nil, err
	}
	var out []unbrokenRange
	for _, seg := range splitOnOrOperator([]rune(s)) {
		r, err := parseNpmUnbroken(seg, s, includeAllPrerelease, maxLength)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseNpmUnbroken(seg []rune, rangeText string, includeAllPrerelease bool, maxLength int) (unbrokenRange, error) {
	var left, right bound
	if seg1, seg2, ok := splitOnHyphenRangeSeparator(seg); ok {
		v1, wc1, err := parseNpmHyphenSegment(seg1, rangeText, maxLength)
		if err != nil {
			return unbrokenRange{}, err
		}
		v2, wc2, err := parseNpmHyphenSegment(seg2, rangeText, maxLength)
		if err != nil {
			return unbrokenRange{}, err
		}
		npmWildcardLowerBound(includeAllPrerelease, &left, v1, wc1)
		if err := npmWildcardUpperBound(&right, v2, wc2); err != nil {
			return unbrokenRange{}, err
		}
	} else {
		seg = trimStartWhitespace(seg)
		if len(seg) == 0 {
			return newUnbroken(bound{}, bound{}, includeAllPrerelease), nil
		}
		for len(seg) > 0 {
			var err error
			seg, err = parseNpmComparison(seg, rangeText, includeAllPrerelease, maxLength, &left, &right)
			if err != nil {
				return unbrokenRange{}, err
			}
		}
	}
	return newUnbroken(left, right, includeAllPrerelease), nil
}

// splitOnHyphenRangeSeparator is NpmRangeParser.TrySplitOnHyphenRangeSeparator.
func splitOnHyphenRangeSeparator(seg []rune) ([]rune, []rune, bool) {
	for i := 1; i < len(seg)-1; i++ {
		if seg[i] == '-' && unicode.IsSpace(seg[i-1]) && unicode.IsSpace(seg[i+1]) {
			return seg[:i-1], seg[i+2:], true
		}
	}
	return nil, nil, false
}

func parseNpmHyphenSegment(seg []rune, rangeText string, maxLength int) (Version, wildcardVersion, error) {
	seg = trimStartWhitespace(seg)
	if len(seg) == 0 {
		return Version{}, 0, fmt.Errorf("semver: missing version in hyphen range %q", rangeText)
	}
	if !isPossibleVersionChar(seg[0], true) {
		return Version{}, 0, fmt.Errorf("semver: unexpected %q in hyphen range %q", string(seg[0]), rangeText)
	}
	seg, v, wc, err := parseNpmVersion(seg, rangeText, maxLength)
	if err != nil {
		return Version{}, 0, err
	}
	seg = trimStartWhitespace(seg)
	if len(seg) != 0 {
		return Version{}, 0, fmt.Errorf("semver: unexpected %q in hyphen range %q", string(seg), rangeText)
	}
	return v, wc, nil
}

func parseNpmVersion(seg []rune, rangeText string, maxLength int) ([]rune, Version, wildcardVersion, error) {
	seg, v, wc, err := parseRangeVersion(seg, npmStyles, true, &npmParsingOptions, maxLength)
	if err != nil {
		return nil, Version{}, 0, err
	}
	if wc != wildcardNone && v.IsPrerelease() {
		return nil, Version{}, 0, fmt.Errorf("semver: prerelease is not supported with a wildcard version in range %q", rangeText)
	}
	return seg, v, wc, nil
}

func parseNpmOperator(seg []rune) ([]rune, rangeOperator, error) {
	end := 0
	for end < len(seg) && isPossibleOperatorChar(seg[end], true) {
		end++
	}
	op, seg := seg[:end], seg[end:]
	if len(op) == 0 {
		return seg, opNone, nil
	}
	if len(op) > 2 || (len(op) == 2 && op[1] != '=' && !(op[0] == '~' && op[1] == '>')) {
		return nil, 0, fmt.Errorf("semver: invalid operator %q", string(op))
	}
	orEqual := len(op) == 2 && op[1] == '='
	switch {
	case op[0] == '=' && !orEqual:
		return seg, opEquals, nil
	case op[0] == '<' && orEqual:
		return seg, opLessThanOrEqual, nil
	case op[0] == '<':
		return seg, opLessThan, nil
	case op[0] == '>' && orEqual:
		return seg, opGreaterThanOrEqual, nil
	case op[0] == '>':
		return seg, opGreaterThan, nil
	case op[0] == '~' && !orEqual:
		return seg, opTilde, nil
	case op[0] == '^' && !orEqual:
		return seg, opCaret, nil
	}
	return nil, 0, fmt.Errorf("semver: invalid operator %q", string(op))
}

func parseNpmComparison(seg []rune, rangeText string, includeAllPrerelease bool, maxLength int, left, right *bound) ([]rune, error) {
	seg, op, err := parseNpmOperator(seg)
	if err != nil {
		return nil, err
	}
	seg = trimStartWhitespace(seg)
	seg, v, wc, err := parseNpmVersion(seg, rangeText, maxLength)
	if err != nil {
		return nil, err
	}
	seg = trimStartWhitespace(seg)

	switch op {
	case opGreaterThan:
		return seg, npmGreaterThan(includeAllPrerelease, left, right, v, wc)
	case opGreaterThanOrEqual:
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
	case opLessThan:
		npmLessThan(right, v, wc)
	case opLessThanOrEqual:
		return seg, npmWildcardUpperBound(right, v, wc)
	case opCaret:
		if wc == wildcardMajorMinorPatch {
			return seg, nil
		}
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		upper, err := caretUpper(v, wc == wildcardMinorPatch, wc == wildcardPatch)
		if err != nil {
			return nil, err
		}
		*right = rightMin(*right, bound{&upper, false})
	case opTilde:
		if wc == wildcardMajorMinorPatch {
			return seg, nil
		}
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		var upper Version
		if wc == wildcardMinorPatch {
			upper, err = nextMajorZero(v)
		} else {
			upper, err = nextMinorZero(v)
		}
		if err != nil {
			return nil, err
		}
		*right = rightMin(*right, bound{&upper, false})
	case opEquals, opNone:
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		return seg, npmWildcardUpperBound(right, v, wc)
	}
	return seg, nil
}

func npmGreaterThan(includeAllPrerelease bool, left, right *bound, v Version, wc wildcardVersion) error {
	var pre []string
	if includeAllPrerelease {
		pre = []string{"0"}
	}
	inclusive := false
	switch wc {
	case wildcardMajorMinorPatch:
		// No version matches.
		*right = bound{ptr(minVersion), false}
		return nil
	case wildcardMinorPatch:
		major, err := inc(v.Major)
		if err != nil {
			return err
		}
		v = Version{Major: major, Prerelease: pre}
		inclusive = true
	case wildcardPatch:
		minor, err := inc(v.Minor)
		if err != nil {
			return err
		}
		v = Version{Major: v.Major, Minor: minor, Prerelease: pre}
		inclusive = true
	}
	*left = leftMax(*left, bound{ptr(v), inclusive})
	return nil
}

func npmLessThan(right *bound, v Version, wc wildcardVersion) {
	if wc != wildcardNone {
		// Wildcard places are already filled with zeros.
		v.Prerelease = []string{"0"}
	}
	*right = rightMin(*right, bound{ptr(v), false})
}

func npmWildcardLowerBound(includeAllPrerelease bool, left *bound, v Version, wc wildcardVersion) {
	switch wc {
	case wildcardMajorMinorPatch:
		return
	case wildcardMinorPatch, wildcardPatch:
		if includeAllPrerelease {
			v.Prerelease = []string{"0"}
		}
	}
	*left = leftMax(*left, bound{ptr(v), true})
}

func npmWildcardUpperBound(right *bound, v Version, wc wildcardVersion) error {
	switch wc {
	case wildcardNone:
		*right = rightMin(*right, bound{ptr(v), true})
	case wildcardMinorPatch:
		upper, err := nextMajorZero(v)
		if err != nil {
			return err
		}
		*right = rightMin(*right, bound{&upper, false})
	case wildcardPatch:
		upper, err := nextMinorZero(v)
		if err != nil {
			return err
		}
		*right = rightMin(*right, bound{&upper, false})
	}
	return nil
}
