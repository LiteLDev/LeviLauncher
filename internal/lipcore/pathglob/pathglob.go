// Package pathglob implements the path patterns of lip 0.38 (lip.schema.toml
// "globs" and "place", protocol/workspace.md §4). It is a direct port of
// src/Lip.Core/Mcmod/PathGlob.cs at lip v0.38.0-beta.1.
//
// A pattern is matched segment by segment with the POSIX pattern matching
// notation as fnmatch applies it with FNM_PATHNAME and without FNM_PERIOD:
// '*' matches any run of characters other than '/' (a leading '.' included),
// '?' one character other than '/', and a bracket expression one character of
// its set ('[!...]' of the complement), with ranges, character classes,
// collating symbols and equivalence classes of the POSIX locale; a '[' that
// starts no complete bracket expression matches itself. There is no escape
// character. A segment that is exactly "**" matches zero or more segments,
// and a pattern ending in '/' matches directories together with every path
// below a matched directory.
//
// Paths are '/'-separated and relative; a directory path is written with a
// trailing '/'. Matching is case-sensitive on every OS.
//
// Upstream translates each pattern segment to a .NET regular expression; this
// port compiles the same translation to a token list and matches it over
// UTF-16 code units, like .NET chars, so '?' and bracket expressions match
// exactly one UTF-16 code unit and ranges compare code units. Segments without
// metacharacters are compared ordinally.
package pathglob

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// classRange is an inclusive range of UTF-16 code units.
type classRange struct{ lo, hi uint16 }

// classNames lists the POSIX character classes in upstream's dictionary
// order (used in error messages).
var classNames = []string{
	"alnum", "alpha", "blank", "cntrl", "digit", "graph",
	"lower", "print", "punct", "space", "upper", "xdigit",
}

func r(lo, hi byte) classRange { return classRange{uint16(lo), uint16(hi)} }

// classes are the ASCII members of the POSIX locale's character classes.
var classes = map[string][]classRange{
	"alnum":  {r('A', 'Z'), r('a', 'z'), r('0', '9')},
	"alpha":  {r('A', 'Z'), r('a', 'z')},
	"blank":  {r(' ', ' '), r('\t', '\t')},
	"cntrl":  {r(0x00, 0x1F), r(0x7F, 0x7F)},
	"digit":  {r('0', '9')},
	"graph":  {r(0x21, 0x7E)},
	"lower":  {r('a', 'z')},
	"print":  {r(0x20, 0x7E)},
	"punct":  {r('!', '/'), r(':', '@'), r('[', '`'), r('{', '~')},
	"space":  {r(' ', ' '), r('\t', '\t'), r('\n', '\n'), r('\v', '\v'), r('\f', '\f'), r('\r', '\r')},
	"upper":  {r('A', 'Z')},
	"xdigit": {r('0', '9'), r('A', 'F'), r('a', 'f')},
}

type tokenKind uint8

const (
	tokLiteral tokenKind = iota // one code unit equal to lit
	tokAny                      // '?': one code unit other than '/'
	tokStar                     // '*': any run of code units other than '/'
	tokClass                    // bracket expression
)

type token struct {
	kind    tokenKind
	lit     uint16
	negated bool
	ranges  []classRange
}

func (t *token) matches(c uint16) bool {
	switch t.kind {
	case tokLiteral:
		return c == t.lit
	case tokAny:
		return c != '/'
	case tokClass:
		in := false
		for _, rg := range t.ranges {
			if rg.lo <= c && c <= rg.hi {
				in = true
				break
			}
		}
		if t.negated {
			// The .NET class is "[^/...]".
			return !in && c != '/'
		}
		return in
	}
	return false
}

// Glob is a compiled path pattern.
type Glob struct {
	pattern            string
	matchesDirectories bool
	segments           []string
	// tokens[i] is nil when segments[i] is "**" or has no metacharacter.
	tokens [][]token
}

// Compile parses a pattern. The error text is upstream's FormatException
// message.
func Compile(pattern string) (*Glob, error) {
	var reason string
	switch {
	case pattern == "":
		reason = "it is empty"
	case strings.HasPrefix(pattern, "/"):
		reason = "it starts with '/', but a glob is a relative path"
	case strings.Contains(pattern, `\`):
		reason = "it contains '\\', but path segments are separated by '/' and there is no escape character"
	}
	if reason != "" {
		return nil, invalid(pattern, reason)
	}

	g := &Glob{pattern: pattern, matchesDirectories: strings.HasSuffix(pattern, "/")}
	body := pattern
	if g.matchesDirectories {
		body = pattern[:len(pattern)-1]
	}
	g.segments = strings.Split(body, "/")
	for _, seg := range g.segments {
		if seg == "" {
			return nil, invalid(pattern, "it contains '//', an empty path segment")
		}
	}

	g.tokens = make([][]token, len(g.segments))
	for i, seg := range g.segments {
		if seg == "**" || !HasMeta(seg) {
			continue
		}
		toks, err := translate(seg, pattern)
		if err != nil {
			return nil, err
		}
		g.tokens[i] = toks
	}
	return g, nil
}

// MustCompile is like Compile but panics on an invalid pattern.
func MustCompile(pattern string) *Glob {
	g, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return g
}

// Validate reports whether pattern is a valid glob, returning upstream's
// error message when it is not.
func Validate(pattern string) error {
	_, err := Compile(pattern)
	return err
}

func invalid(pattern, reason string) error {
	return fmt.Errorf("The glob '%s' is not valid: %s.", pattern, reason)
}

// Pattern returns the source pattern.
func (g *Glob) Pattern() string { return g.pattern }

// String returns the source pattern.
func (g *Glob) String() string { return g.pattern }

// MatchesDirectories reports whether the pattern ends with '/'.
func (g *Glob) MatchesDirectories() bool { return g.matchesDirectories }

// HasMeta reports whether s contains '*', '?' or '[', which make it a pattern
// (lip.schema.toml "place").
func HasMeta(s string) bool { return strings.ContainsAny(s, "*?[") }

// FixedPrefix returns the leading segments of a pattern that contain no
// metacharacter, excluding the last segment, each followed by '/'
// (protocol/workspace.md §4 step 2): "" when the first segment is a pattern.
func FixedPrefix(pattern string) string {
	segments := strings.Split(pattern, "/")
	var b strings.Builder
	for i := 0; i < len(segments)-1 && !HasMeta(segments[i]); i++ {
		b.WriteString(segments[i])
		b.WriteByte('/')
	}
	return b.String()
}

// IsMatch reports whether path is matched. A directory path must end with
// '/'.
func (g *Glob) IsMatch(path string) bool { return g.IsMatchBelow(path, 0) }

// IsMatchBelow is IsMatch(path, minSegments): a directory pattern matches a
// path below a directory only through a directory of at least minSegments
// segments (e.g. 2 for a path below mods, which a pattern matching mods alone
// does not reach).
func (g *Glob) IsMatchBelow(path string, minSegments int) bool {
	isDirectory := strings.HasSuffix(path, "/")
	body := path
	if isDirectory {
		body = path[:len(path)-1]
	}
	m := newMatcher(g, strings.Split(body, "/"))

	if !g.matchesDirectories {
		return !isDirectory && m.matchEnd(len(m.path))
	}

	// A directory pattern matches a directory and everything below it. The
	// last segment of a file path is the file itself, never a directory. The
	// empty prefix is the base directory, which only a pattern of "**"
	// segments matches (e.g. "**/" matches every path).
	maxPrefix := len(m.path) - 1
	if isDirectory {
		maxPrefix = len(m.path)
	}
	for length := minSegments; length <= maxPrefix; length++ {
		if m.matchEnd(length) {
			return true
		}
	}
	return false
}

// matcher evaluates upstream's MatchSegments for one path. Results are
// memoized (which does not change them) so that patterns with many "**"
// segments run in polynomial time.
type matcher struct {
	g    *Glob
	path []string
	// seg[p*n+i] caches whether pattern segment p matches path segment i:
	// 0 unknown, 1 no, 2 yes.
	seg []int8
	// memo[p*(n+1)+i] caches matchSegments(p, i, end) for the current end.
	memo []int8
	end  int
}

func newMatcher(g *Glob, path []string) *matcher {
	n := len(path)
	return &matcher{
		g:    g,
		path: path,
		seg:  make([]int8, len(g.segments)*n),
		memo: make([]int8, (len(g.segments)+1)*(n+1)),
	}
}

func (m *matcher) matchEnd(end int) bool {
	clear(m.memo)
	m.end = end
	return m.matchSegments(0, 0)
}

func (m *matcher) matchSegments(p, i int) bool {
	g := m.g
	if p == len(g.segments) {
		return i == m.end
	}
	key := p*(len(m.path)+1) + i
	if v := m.memo[key]; v != 0 {
		return v == 2
	}

	var ok bool
	switch {
	case g.segments[p] == "**":
		for k := i; k <= m.end && !ok; k++ {
			ok = m.matchSegments(p+1, k)
		}
	case i == m.end:
		ok = false
	default:
		ok = m.segmentMatches(p, i) && m.matchSegments(p+1, i+1)
	}

	if ok {
		m.memo[key] = 2
	} else {
		m.memo[key] = 1
	}
	return ok
}

func (m *matcher) segmentMatches(p, i int) bool {
	key := p*len(m.path) + i
	if v := m.seg[key]; v != 0 {
		return v == 2
	}
	var ok bool
	if toks := m.g.tokens[p]; toks != nil {
		ok = matchTokens(toks, toUTF16(m.path[i]))
	} else {
		ok = m.g.segments[p] == m.path[i]
	}
	if ok {
		m.seg[key] = 2
	} else {
		m.seg[key] = 1
	}
	return ok
}

// matchTokens matches a whole segment (anchored at both ends). Every token
// other than '*' consumes exactly one code unit, so backtracking to the most
// recent '*' is sufficient.
func matchTokens(toks []token, s []uint16) bool {
	ti, si := 0, 0
	starT, starS := -1, 0
	for si < len(s) {
		if ti < len(toks) {
			t := &toks[ti]
			if t.kind == tokStar {
				starT, starS = ti, si
				ti++
				continue
			}
			if t.matches(s[si]) {
				ti++
				si++
				continue
			}
		}
		if starT >= 0 {
			starS++
			si = starS
			ti = starT + 1
			continue
		}
		return false
	}
	for ti < len(toks) && toks[ti].kind == tokStar {
		ti++
	}
	return ti == len(toks)
}

// translate is upstream's Translate: the matcher of one pattern segment
// (which contains no '/').
func translate(segment, pattern string) ([]token, error) {
	u := toUTF16(segment)
	var toks []token
	for i := 0; i < len(u); i++ {
		switch c := u[i]; c {
		case '*':
			toks = append(toks, token{kind: tokStar})
		case '?':
			toks = append(toks, token{kind: tokAny})
		case '[':
			tok, closeIdx, err := translateBracket(u, i, pattern)
			if err != nil {
				return nil, err
			}
			if closeIdx < 0 {
				toks = append(toks, token{kind: tokLiteral, lit: '['})
			} else {
				toks = append(toks, tok)
				i = closeIdx
			}
		default:
			toks = append(toks, token{kind: tokLiteral, lit: c})
		}
	}
	return toks, nil
}

// translateBracket is upstream's TranslateBracket: it returns the class of the
// bracket expression starting at seg[start] and the index of its closing ']',
// or -1 when there is none. It fails when a complete bracket expression is
// invalid, e.g. starts with '^'.
func translateBracket(seg []uint16, start int, pattern string) (token, int, error) {
	i := start + 1
	negated := false
	errMsg := ""
	if i < len(seg) && seg[i] == '!' {
		negated = true
		i++
	} else if i < len(seg) && seg[i] == '^' {
		errMsg = "a bracket expression starts with '^'; write '[!...]' to match any character not in a set"
	}

	var set []classRange
	first := true
	for ; i < len(seg); i++ {
		c := seg[i]
		if c == ']' && !first {
			if errMsg != "" {
				return token{}, 0, invalid(pattern, errMsg)
			}
			return token{kind: tokClass, negated: negated, ranges: set}, i, nil
		}

		first = false
		if c == '[' && i+1 < len(seg) && (seg[i+1] == ':' || seg[i+1] == '.' || seg[i+1] == '=') {
			kind := seg[i+1]
			closeIdx := indexPair(seg, kind, ']', i+2)
			if closeIdx < 0 {
				return token{}, -1, nil
			}

			name := seg[i+2 : closeIdx]
			if kind == ':' {
				if members, ok := classes[str(name)]; ok {
					set = append(set, members...)
				} else if errMsg == "" {
					names := make([]string, len(classNames))
					for k, n := range classNames {
						names[k] = "[:" + n + ":]"
					}
					errMsg = fmt.Sprintf("'[:%s:]' is not a character class (the classes are %s)",
						str(name), strings.Join(names, ", "))
				}
			} else if len(name) == 1 {
				// In the POSIX locale a collating symbol or an equivalence
				// class is the character.
				set = append(set, classRange{name[0], name[0]})
			} else if errMsg == "" {
				k := string(rune(kind))
				errMsg = fmt.Sprintf("'[%s%s%s]' names no character (only single characters are collating elements)",
					k, str(name), k)
			}

			i = closeIdx + 1
			continue
		}

		if i+2 < len(seg) && seg[i+1] == '-' && seg[i+2] != ']' {
			end := seg[i+2]
			if end < c && errMsg == "" {
				errMsg = fmt.Sprintf("the range '%s-%s' ends before it starts", str(seg[i:i+1]), str(seg[i+2:i+3]))
			}
			switch {
			case end < c:
				// Invalid; reported if the bracket expression is closed.
			case c == '-':
				// Upstream emits "\--X", and .NET never starts a range at
				// an escaped '-': the set is '-', '-' and X.
				set = append(set, classRange{'-', '-'}, classRange{end, end})
			default:
				set = append(set, classRange{c, end})
			}
			i += 2
			continue
		}

		set = append(set, classRange{c, c})
	}

	return token{}, -1, nil
}

// indexPair returns the index of the first occurrence of the two code units
// a, b at or after from, or -1.
func indexPair(s []uint16, a, b uint16, from int) int {
	for j := from; j+1 < len(s); j++ {
		if s[j] == a && s[j+1] == b {
			return j
		}
	}
	return -1
}

func str(u []uint16) string { return string(utf16.Decode(u)) }

// toUTF16 converts s to UTF-16 code units like a .NET string. Each byte of
// invalid UTF-8 becomes the lone surrogate 0xDC00+b (as in
// internal/lipcore/glob), so distinct byte strings stay distinct and a
// metacharacter segment agrees with the byte-wise comparison of plain ones.
func toUTF16(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			out = append(out, 0xDC00+uint16(s[i]))
			i++
			continue
		}
		out = utf16.AppendRune(out, r)
		i += size
	}
	return out
}
