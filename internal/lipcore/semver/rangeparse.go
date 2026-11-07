package semver

import (
	"fmt"
	"unicode"
	"unicode/utf16"
)

// This file ports Semver.Parsing.GeneralRangeParser, StandardRangeParser,
// NpmRangeParser and RangeError. Like the C# code it works on UTF-16 code
// units, so positions in messages and the classification of characters
// outside the BMP (surrogates are neither whitespace, punctuation nor
// symbols) are the library's.

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

// segment is a StringSegment: units [off, end) of the UTF-16 text buf.
type segment struct {
	buf      []uint16
	off, end int
}

func newSegment(s string) segment {
	buf := utf16.Encode([]rune(s))
	return segment{buf: buf, end: len(buf)}
}

func (s segment) len() int           { return s.end - s.off }
func (s segment) at(i int) uint16    { return s.buf[s.off+i] }
func (s segment) from(i int) segment { return segment{s.buf, s.off + i, s.end} }
func (s segment) slice(i, j int) segment {
	return segment{s.buf, s.off + i, s.off + j}
}
func (s segment) String() string { return string(utf16.Decode(s.buf[s.off:s.end])) }

// text is segment.Buffer: the whole range.
func (s segment) text() string { return string(utf16.Decode(s.buf)) }

// charRune is the rune of a UTF-16 code unit for classification; a
// surrogate is not a character of its own and classifies as nothing.
func charRune(c uint16) (rune, bool) {
	if utf16.IsSurrogate(rune(c)) {
		return 0, false
	}
	return rune(c), true
}

// isWhiteSpace is char.IsWhiteSpace.
func isWhiteSpace(c uint16) bool {
	r, ok := charRune(c)
	return ok && unicode.IsSpace(r)
}

// isPossibleOperatorChar is GeneralRangeParser.IsPossibleOperatorChar.
func isPossibleOperatorChar(c uint16, allowMetadata bool) bool {
	r, ok := charRune(c)
	if !ok {
		return false
	}
	return r == '=' || r == '<' || r == '>' || r == '~' || r == '^' ||
		(unicode.IsPunct(r) && !unicode.IsSpace(r) && r != '*') ||
		(unicode.IsSymbol(r) && (r != '+' || !allowMetadata))
}

// isPossibleVersionChar is GeneralRangeParser.IsPossibleVersionChar.
func isPossibleVersionChar(c uint16, allowMetadata bool) bool {
	return !isWhiteSpace(c) &&
		(!isPossibleOperatorChar(c, allowMetadata) || c == '-' || c == '.' || (c == '+' && allowMetadata))
}

// --- RangeError messages ---

func errTooLong(rangeText string, maxLength int) error {
	return formatError("Exceeded maximum length of %d for '%s'.", maxLength, limitLength(rangeText))
}

func errInvalidOperator(op segment) error {
	return formatError("Invalid operator '%s'.", limitLength(op.String()))
}

func errInvalidWhitespace(position int, rangeText string) error {
	return formatError("Invalid whitespace character at %d in '%s'. Only the ASCII space character is allowed.",
		position, limitLength(rangeText))
}

func errMissingComparison(position int, rangeText string) error {
	return formatError("Range is missing a comparison or limit at %d in '%s'.", position, limitLength(rangeText))
}

func errWildcardWithOperator(rangeText string) error {
	return formatError("Operator is combined with wildcards in '%s'.", limitLength(rangeText))
}

func errPrereleaseWithWildcard(rangeText string) error {
	return formatError("A wildcard major, minor, or patch is combined with a prerelease version in '%s'.",
		limitLength(rangeText))
}

func errUnexpectedInHyphenRange(unexpected string) error {
	return formatError("Unexpected characters in hyphen range '%s'.", unexpected)
}

func errMissingVersionInHyphenRange(rangeText string) error {
	return formatError("Missing a version number in hyphen range in '%s'.", limitLength(rangeText))
}

// --- GeneralRangeParser ---

// splitOnOrOperator is GeneralRangeParser.SplitOnOrOperator.
func splitOnOrOperator(s segment) []segment {
	var out []segment
	possiblyInSeparator := false
	start := 0
	for i := 0; i < s.len(); i++ {
		isSep := s.at(i) == '|'
		if possiblyInSeparator && isSep {
			possiblyInSeparator = false
			out = append(out, s.slice(start, i-1))
			start = i + 1
		} else {
			possiblyInSeparator = isSep
		}
	}
	return append(out, s.from(start))
}

// parseOptionalSpaces is GeneralRangeParser.ParseOptionalSpaces.
func parseOptionalSpaces(s segment) (segment, error) {
	i := 0
	for i < s.len() && s.at(i) == ' ' {
		i++
	}
	s = s.from(i)
	if s.len() > 0 && isWhiteSpace(s.at(0)) {
		return s, errInvalidWhitespace(s.off, s.text())
	}
	return s, nil
}

// parseOptionalWhitespace is GeneralRangeParser.ParseOptionalWhitespace.
func parseOptionalWhitespace(s segment) segment {
	i := 0
	for i < s.len() && isWhiteSpace(s.at(i)) {
		i++
	}
	return s.from(i)
}

// parseRangeVersion is GeneralRangeParser.ParseVersion: consume the
// version characters and parse them (metadata is removed).
func parseRangeVersion(s segment, style styles, allowMetadata bool, opts *parsingOptions, maxLength int) (segment, Version, wildcardVersion, error) {
	end := 0
	for end < s.len() && isPossibleVersionChar(s.at(end), allowMetadata) {
		end++
	}
	v, wc, err := parseVersion(s.slice(0, end).String(), style, opts, maxLength)
	if err != nil {
		return s, Version{}, wildcardNone, err
	}
	return s.from(end), v.withoutMetadata(), wc, nil
}

func ptr(v Version) *Version { return &v }

func checkRangeLength(s segment, maxLength int) error {
	if s.len() > maxLength {
		return errTooLong(s.text(), maxLength)
	}
	return nil
}

// parseOperator is StandardRangeParser.ParseOperator (npm false) and
// NpmRangeParser.ParseOperator (npm true, which also accepts "~>").
func parseOperator(s segment, npm bool) (segment, rangeOperator, error) {
	end := 0
	for end < s.len() && isPossibleOperatorChar(s.at(end), npm) {
		end++
	}
	op := s.slice(0, end)
	s = s.from(end)
	if op.len() == 0 {
		return s, opNone, nil
	}
	if op.len() > 2 || (op.len() == 2 && op.at(1) != '=' && !(npm && op.at(0) == '~' && op.at(1) == '>')) {
		return s, 0, errInvalidOperator(op)
	}
	orEqual := op.len() == 2 && op.at(1) == '='
	switch c := op.at(0); {
	case c == '=' && !orEqual:
		return s, opEquals, nil
	case c == '<' && orEqual:
		return s, opLessThanOrEqual, nil
	case c == '<':
		return s, opLessThan, nil
	case c == '>' && orEqual:
		return s, opGreaterThanOrEqual, nil
	case c == '>':
		return s, opGreaterThan, nil
	case c == '~' && !orEqual:
		return s, opTilde, nil
	case c == '^' && !orEqual:
		return s, opCaret, nil
	}
	return s, 0, errInvalidOperator(op)
}

// --- standard syntax ---

func parseStandardRange(s string, includeAllPrerelease bool, maxLength int) ([]unbrokenRange, error) {
	all := newSegment(s)
	if err := checkRangeLength(all, maxLength); err != nil {
		return nil, err
	}
	var out []unbrokenRange
	for _, seg := range splitOnOrOperator(all) {
		r, err := parseStandardUnbroken(seg, includeAllPrerelease, maxLength)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseStandardUnbroken(seg segment, includeAllPrerelease bool, maxLength int) (unbrokenRange, error) {
	seg, err := parseOptionalSpaces(seg)
	if err != nil {
		return unbrokenRange{}, err
	}
	if seg.len() == 0 {
		return unbrokenRange{}, errMissingComparison(seg.off, seg.text())
	}
	var left, right bound
	for seg.len() > 0 {
		seg, err = parseStandardComparison(seg, &includeAllPrerelease, maxLength, &left, &right)
		if err != nil {
			return unbrokenRange{}, err
		}
	}
	return newUnbroken(left, right, includeAllPrerelease), nil
}

// caretUpper returns the exclusive upper bound version of a caret range.
func caretUpper(v Version, wildMinorPatch, wildPatch bool) Version {
	var out Version
	switch {
	case !v.Major.IsZero() || wildMinorPatch:
		out.Major = v.Major.inc()
	case !v.Minor.IsZero() || wildPatch:
		out.Minor = v.Minor.inc()
	default:
		out.Patch = v.Patch.inc()
	}
	out.Prerelease = []string{"0"}
	return out
}

func nextMinorZero(v Version) Version {
	return Version{Major: v.Major, Minor: v.Minor.inc(), Prerelease: []string{"0"}}
}

func nextMajorZero(v Version) Version {
	return Version{Major: v.Major.inc(), Prerelease: []string{"0"}}
}

func parseStandardComparison(seg segment, includeAllPrerelease *bool, maxLength int, left, right *bound) (segment, error) {
	seg, op, err := parseOperator(seg, false)
	if err != nil {
		return seg, err
	}
	if seg, err = parseOptionalSpaces(seg); err != nil {
		return seg, err
	}
	seg, v, wc, err := parseRangeVersion(seg, styleStrict, false, &standardParsingOptions, maxLength)
	if err != nil {
		return seg, err
	}
	if op != opNone && wc != wildcardNone {
		return seg, errWildcardWithOperator(seg.text())
	}
	if seg, err = parseOptionalSpaces(seg); err != nil {
		return seg, err
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
		*right = rightMin(*right, bound{ptr(caretUpper(v, false, false)), false})
	case opTilde:
		*left = leftMax(*left, bound{ptr(v), true})
		*right = rightMin(*right, bound{ptr(nextMinorZero(v)), false})
	case opNone:
		prereleaseWildcard := wc&wildcardPrerelease != 0
		*includeAllPrerelease = *includeAllPrerelease || prereleaseWildcard
		wc &^= wildcardPrerelease
		if wc != wildcardNone && v.IsPrerelease() {
			return seg, errPrereleaseWithWildcard(seg.text())
		}
		switch wc {
		case wildcardNone:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			*right = rightMin(*right, standardPrereleaseWildcardUpperBound(v, prereleaseWildcard))
		case wildcardMajorMinorPatch:
			// No further bound is placed on the left and right bounds.
		case wildcardMinorPatch:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			*right = rightMin(*right, bound{ptr(nextMajorZero(v)), false})
		case wildcardPatch:
			*left = leftMax(*left, standardWildcardLowerBound(v, prereleaseWildcard))
			*right = rightMin(*right, bound{ptr(nextMinorZero(v)), false})
		default:
			panic(fmt.Sprintf("semver: unexpected wildcard %d", wc))
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

func standardPrereleaseWildcardUpperBound(v Version, prereleaseWildcard bool) bound {
	if !prereleaseWildcard {
		return bound{ptr(v), true}
	}
	if v.IsPrerelease() {
		pre := make([]string, len(v.Prerelease))
		copy(pre, v.Prerelease)
		pre[len(pre)-1] = nextIdentifier(pre[len(pre)-1])
		return bound{&Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prerelease: pre}, false}
	}
	return bound{&Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch.inc(), Prerelease: []string{"0"}}, false}
}

// --- npm syntax ---

// npmStyles is NpmRangeParser.StandardRangeOptions converted to styles
// (AllowLowerV | OptionalMinorPatch; AllowMetadata is handled separately).
const npmStyles = styleAllowLowerV | styleOptionalMinorPatch

func parseNpmRange(s string, includeAllPrerelease bool, maxLength int) ([]unbrokenRange, error) {
	all := newSegment(s)
	if err := checkRangeLength(all, maxLength); err != nil {
		return nil, err
	}
	var out []unbrokenRange
	for _, seg := range splitOnOrOperator(all) {
		r, err := parseNpmUnbroken(seg, includeAllPrerelease, maxLength)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseNpmUnbroken(seg segment, includeAllPrerelease bool, maxLength int) (unbrokenRange, error) {
	var left, right bound
	if seg1, seg2, ok := splitOnHyphenRangeSeparator(seg); ok {
		v1, wc1, err := parseNpmHyphenSegment(seg1, maxLength)
		if err != nil {
			return unbrokenRange{}, err
		}
		v2, wc2, err := parseNpmHyphenSegment(seg2, maxLength)
		if err != nil {
			return unbrokenRange{}, err
		}
		npmWildcardLowerBound(includeAllPrerelease, &left, v1, wc1)
		npmWildcardUpperBound(&right, v2, wc2)
	} else {
		seg = parseOptionalWhitespace(seg)
		if seg.len() == 0 {
			return newUnbroken(bound{}, bound{}, includeAllPrerelease), nil
		}
		for seg.len() > 0 {
			var err error
			seg, err = parseNpmComparison(seg, includeAllPrerelease, maxLength, &left, &right)
			if err != nil {
				return unbrokenRange{}, err
			}
		}
	}
	return newUnbroken(left, right, includeAllPrerelease), nil
}

// splitOnHyphenRangeSeparator is NpmRangeParser.TrySplitOnHyphenRangeSeparator.
func splitOnHyphenRangeSeparator(seg segment) (segment, segment, bool) {
	for i := 1; i < seg.len()-1; i++ {
		if seg.at(i) == '-' && isWhiteSpace(seg.at(i-1)) && isWhiteSpace(seg.at(i+1)) {
			return seg.slice(0, i-1), seg.from(i + 2), true
		}
	}
	return seg, seg, false
}

func parseNpmHyphenSegment(seg segment, maxLength int) (Version, wildcardVersion, error) {
	seg = parseOptionalWhitespace(seg)
	if seg.len() == 0 {
		return Version{}, 0, errMissingVersionInHyphenRange(seg.text())
	}
	if !isPossibleVersionChar(seg.at(0), true) {
		return Version{}, 0, errUnexpectedInHyphenRange(seg.slice(0, 1).String())
	}
	seg, v, wc, err := parseNpmVersion(seg, maxLength)
	if err != nil {
		return Version{}, 0, err
	}
	seg = parseOptionalWhitespace(seg)
	if seg.len() != 0 {
		return Version{}, 0, errUnexpectedInHyphenRange(seg.String())
	}
	return v, wc, nil
}

func parseNpmVersion(seg segment, maxLength int) (segment, Version, wildcardVersion, error) {
	seg, v, wc, err := parseRangeVersion(seg, npmStyles, true, &npmParsingOptions, maxLength)
	if err != nil {
		return seg, Version{}, 0, err
	}
	if wc != wildcardNone && v.IsPrerelease() {
		return seg, Version{}, 0, errPrereleaseWithWildcard(seg.text())
	}
	return seg, v, wc, nil
}

func parseNpmComparison(seg segment, includeAllPrerelease bool, maxLength int, left, right *bound) (segment, error) {
	seg, op, err := parseOperator(seg, true)
	if err != nil {
		return seg, err
	}
	seg = parseOptionalWhitespace(seg)
	seg, v, wc, err := parseNpmVersion(seg, maxLength)
	if err != nil {
		return seg, err
	}
	seg = parseOptionalWhitespace(seg)

	switch op {
	case opGreaterThan:
		npmGreaterThan(includeAllPrerelease, left, right, v, wc)
	case opGreaterThanOrEqual:
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
	case opLessThan:
		npmLessThan(right, v, wc)
	case opLessThanOrEqual:
		npmWildcardUpperBound(right, v, wc)
	case opCaret:
		if wc == wildcardMajorMinorPatch {
			return seg, nil
		}
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		upper := caretUpper(v, wc == wildcardMinorPatch, wc == wildcardPatch)
		*right = rightMin(*right, bound{&upper, false})
	case opTilde:
		if wc == wildcardMajorMinorPatch {
			return seg, nil
		}
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		upper := nextMinorZero(v)
		if wc == wildcardMinorPatch {
			upper = nextMajorZero(v)
		}
		*right = rightMin(*right, bound{&upper, false})
	case opEquals, opNone:
		npmWildcardLowerBound(includeAllPrerelease, left, v, wc)
		npmWildcardUpperBound(right, v, wc)
	}
	return seg, nil
}

func npmGreaterThan(includeAllPrerelease bool, left, right *bound, v Version, wc wildcardVersion) {
	var pre []string
	if includeAllPrerelease {
		pre = []string{"0"}
	}
	inclusive := false
	switch wc {
	case wildcardMajorMinorPatch:
		// No version matches.
		*right = bound{ptr(minVersion), false}
		return
	case wildcardMinorPatch:
		v = Version{Major: v.Major.inc(), Prerelease: pre}
		inclusive = true
	case wildcardPatch:
		v = Version{Major: v.Major, Minor: v.Minor.inc(), Prerelease: pre}
		inclusive = true
	}
	*left = leftMax(*left, bound{ptr(v), inclusive})
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

func npmWildcardUpperBound(right *bound, v Version, wc wildcardVersion) {
	switch wc {
	case wildcardNone:
		*right = rightMin(*right, bound{ptr(v), true})
	case wildcardMinorPatch:
		*right = rightMin(*right, bound{ptr(nextMajorZero(v)), false})
	case wildcardPatch:
		*right = rightMin(*right, bound{ptr(nextMinorZero(v)), false})
	}
}
