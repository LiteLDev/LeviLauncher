// Package glob implements glob matching compatible with DotNet.Glob 3.1.3
// using default options, as used by lip 0.37.0 for variant platforms,
// file placements, preserve_files and remove_files.
//
// Semantics: case-sensitive; "*" matches any run of characters except path
// separators ('/' and '\'); "**" matches across directories; "?" matches one
// non-separator character; "[abc]", "[a-z]" and "[!abc]" character classes;
// '/' and '\' in a pattern each match either separator.
//
// The implementation is a direct port of DotNet.Glob's GlobTokeniser and
// GlobTokenEvaluator classes, including their quirks (for example "[a-zA-Z]"
// is the range a-z: only the first range of a class is honoured, an unclosed
// "[abc" is a character list, and "**" never matches the empty string).
// Patterns and subjects are processed as UTF-16 code units, like .NET chars.
package glob

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// ErrNotImplemented was returned by skeleton stubs; it is no longer used.
var ErrNotImplemented = errors.New("glob: not implemented")

// ErrEmptyPattern is returned by Compile for an empty pattern
// (Glob.Parse throws ArgumentNullException).
var ErrEmptyPattern = errors.New("glob: empty pattern")

// Glob is a compiled pattern.
type Glob struct {
	pattern string
	tokens  []token
	root    *compositeEvaluator
}

// Compile parses pattern.
//
// It fails for the empty pattern and for a malformed range such as "[a-]" or
// "[0-" (DotNet.Glob throws IndexOutOfRangeException for these). Every other
// string is a valid pattern.
func Compile(pattern string) (*Glob, error) {
	if pattern == "" {
		return nil, ErrEmptyPattern
	}
	toks, err := tokenise(toUTF16(pattern))
	if err != nil {
		return nil, fmt.Errorf("glob: invalid pattern %q: %w", pattern, err)
	}
	return &Glob{pattern: pattern, tokens: toks, root: newComposite(toks)}, nil
}

// MustCompile is Compile that panics on error.
func MustCompile(pattern string) *Glob {
	g, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return g
}

// Match reports whether s matches the whole pattern.
func (g *Glob) Match(s string) bool {
	ok, _ := g.root.isMatch(toUTF16(s), 0)
	return ok
}

// toUTF16 converts s to UTF-16 code units. Each byte of an invalid UTF-8
// sequence becomes its own lone low surrogate (0xDC00+b), so distinct invalid
// bytes stay distinct instead of all collapsing to U+FFFD.
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

// String returns the original pattern (Glob.ToString round-trips it).
func (g *Glob) String() string { return g.pattern }

// Format returns the pattern re-rendered from its tokens, exactly as
// DotNet.Glob's Glob.ToString (GlobTokenFormatter) does. lip writes globs
// back to JSON with ToString, so this is the text lip would serialise. It
// equals String() for every well-formed pattern, but differs for quirky
// ones (e.g. "[abc" formats as "[abc]", "[a-zA-Z]" as "[a-z]").
func (g *Glob) Format() string {
	var out []uint16
	for _, t := range g.tokens {
		out = t.format(out)
	}
	return string(utf16.Decode(out))
}

// Match compiles pattern and matches s; a pattern that fails to compile
// matches nothing.
func Match(pattern, s string) bool {
	g, err := Compile(pattern)
	if err != nil {
		return false
	}
	return g.Match(s)
}

func isSep(c uint16) bool { return c == '/' || c == '\\' }

// ---------------------------------------------------------------- tokens

type tokenKind int

const (
	tLiteral tokenKind = iota
	tPathSep
	tAnyChar
	tCharList
	tRange // LetterRangeToken / NumberRangeToken (identical when case-sensitive)
	tWildcard
	tWildcardDir
)

type token struct {
	kind    tokenKind
	value   []uint16 // literal text or character list
	sep     uint16   // path separator char (tPathSep)
	start   uint16   // range start
	end     uint16   // range end
	negated bool
	// tWildcardDir: optional leading/trailing separators (0 = absent).
	lead, trail uint16
}

func (t token) format(b []uint16) []uint16 {
	switch t.kind {
	case tLiteral:
		return append(b, t.value...)
	case tPathSep:
		return append(b, t.sep)
	case tAnyChar:
		return append(b, '?')
	case tWildcard:
		return append(b, '*')
	case tWildcardDir:
		if t.lead != 0 {
			b = append(b, t.lead)
		}
		b = append(b, '*', '*')
		if t.trail != 0 {
			b = append(b, t.trail)
		}
		return b
	case tCharList, tRange:
		b = append(b, '[')
		if t.negated {
			b = append(b, '!')
		}
		if t.kind == tCharList {
			b = append(b, t.value...)
		} else {
			b = append(b, t.start, '-', t.end)
		}
		return append(b, ']')
	}
	return b
}

// reader mirrors GlobStringReader: cur is the last character read, and a
// failed read leaves cur unchanged.
type reader struct {
	text []uint16
	pos  int // index of the next character to read
	cur  uint16
}

func (r *reader) read() bool {
	if r.pos >= len(r.text) {
		return false
	}
	r.cur = r.text[r.pos]
	r.pos++
	return true
}

func (r *reader) atEnd() bool { return r.pos >= len(r.text) }

func (r *reader) peek() uint16 {
	if r.atEnd() {
		return 0
	}
	return r.text[r.pos]
}

func isTokenStart(c uint16) bool { return c == '*' || c == '[' || c == '?' }

// char.IsLetter / char.IsDigit on a UTF-16 code unit (surrogates are neither).
func isLetter(c uint16) bool { return unicode.IsLetter(rune(c)) }
func isDigit(c uint16) bool  { return unicode.IsDigit(rune(c)) }

var errBadRange = errors.New("malformed character range")

func tokenise(pattern []uint16) ([]token, error) {
	r := &reader{text: pattern}
	var toks []token
	for r.read() {
		switch {
		case r.cur == '[':
			t, err := readRangeOrList(r)
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
		case r.cur == '?':
			toks = append(toks, token{kind: tAnyChar})
		case r.cur == '*' && r.peek() != '*':
			toks = append(toks, token{kind: tWildcard})
		case isSep(r.cur):
			toks = append(toks, token{kind: tPathSep, sep: r.cur})
		case r.cur == '*': // beginning of "**"
			var lead uint16
			if n := len(toks); n > 0 && toks[n-1].kind == tPathSep {
				lead = toks[n-1].sep
				toks = toks[:n-1]
			}
			r.read() // second '*'
			t := token{kind: tWildcardDir, lead: lead}
			if isSep(r.peek()) {
				r.read()
				t.trail = r.cur
			}
			toks = append(toks, t)
		default:
			buf := []uint16{r.cur}
			for !r.atEnd() {
				p := r.peek()
				if isTokenStart(p) || isSep(p) {
					break
				}
				r.read()
				buf = append(buf, r.cur)
			}
			toks = append(toks, token{kind: tLiteral, value: buf})
		}
	}
	return toks, nil
}

// readRangeOrList ports GlobTokeniser.ReadRangeOrListToken; r.cur is '['.
func readRangeOrList(r *reader) (token, error) {
	negated, isRange := false, false
	var buf []uint16
	if r.peek() == '!' {
		negated = true
		r.read()
	}
	if next := r.peek(); isLetter(next) || isDigit(next) {
		r.read()
		isRange = r.peek() == '-'
		buf = append(buf, r.cur)
	} else {
		r.read() // may fail at end of pattern, leaving cur unchanged
		buf = append(buf, r.cur)
	}
	if isRange {
		r.read() // skip '-'
	}
	for r.read() {
		if r.cur == ']' {
			// "]]" inside a class is an escaped ']'.
			if r.peek() != ']' {
				break
			}
		}
		buf = append(buf, r.cur)
	}
	if !isRange {
		return token{kind: tCharList, value: buf, negated: negated}, nil
	}
	if len(buf) < 2 {
		return token{}, errBadRange
	}
	return token{kind: tRange, start: buf[0], end: buf[1], negated: negated}, nil
}

// ---------------------------------------------------------------- evaluation

type evaluator interface {
	isMatch(s []uint16, pos int) (bool, int)
	minLen() int
	variable() bool
}

// fixedEvaluator handles every single-token, fixed-length evaluator:
// literal, path separator, '?', character list and ranges.
type fixedEvaluator struct{ t token }

func (e *fixedEvaluator) minLen() int {
	if e.t.kind == tLiteral {
		return len(e.t.value)
	}
	return 1
}

func (e *fixedEvaluator) variable() bool { return false }

func (e *fixedEvaluator) isMatch(s []uint16, pos int) (bool, int) {
	t := e.t
	if t.kind == tLiteral {
		n := 0
		for pos < len(s) && n < len(t.value) {
			if s[pos] != t.value[n] {
				return false, pos
			}
			pos++
			n++
		}
		return n >= len(t.value), pos
	}
	c := s[pos]
	pos++
	switch t.kind {
	case tPathSep:
		return isSep(c), pos
	case tAnyChar:
		return !isSep(c), pos
	case tCharList:
		in := false
		for _, x := range t.value {
			if x == c {
				in = true
				break
			}
		}
		return in != t.negated, pos
	case tRange:
		in := c >= t.start && c <= t.end
		return in != t.negated, pos
	}
	return false, pos
}

// compositeEvaluator ports CompositeTokenEvaluator.
type compositeEvaluator struct {
	evals []evaluator
	min   int
	vari  bool
}

func newComposite(toks []token) *compositeEvaluator {
	// Only a trailing wildcard's (empty) sub-evaluator is built from zero
	// tokens; it consumes a variable length.
	c := &compositeEvaluator{vari: len(toks) == 0}
	for i, t := range toks {
		switch t.kind {
		case tWildcard:
			c.add(&wildcardEvaluator{sub: newComposite(toks[i+1:])})
			return c
		case tWildcardDir:
			c.add(&wildcardDirEvaluator{t: t, sub: newComposite(toks[i+1:])})
			return c
		default:
			c.add(&fixedEvaluator{t: t})
		}
	}
	return c
}

func (c *compositeEvaluator) add(e evaluator) {
	c.evals = append(c.evals, e)
	c.min += e.minLen()
	if e.variable() {
		c.vari = true
	}
}

func (c *compositeEvaluator) minLen() int    { return c.min }
func (c *compositeEvaluator) variable() bool { return c.vari }

func (c *compositeEvaluator) isMatch(s []uint16, pos int) (bool, int) {
	newPos := pos
	if !c.vari {
		if len(s)-pos != c.min {
			return false, newPos
		}
	} else if len(s)-pos < c.min {
		return false, newPos
	}
	for _, e := range c.evals {
		var ok bool
		if ok, newPos = e.isMatch(s, newPos); !ok {
			return false, newPos
		}
	}
	// Faithful to DotNet.Glob (Length - 1, not Length).
	if newPos < len(s)-1 {
		return false, newPos
	}
	return true, newPos
}

// wildcardEvaluator ports WildcardTokenEvaluator ("*").
type wildcardEvaluator struct{ sub *compositeEvaluator }

func (e *wildcardEvaluator) minLen() int    { return e.sub.min }
func (e *wildcardEvaluator) variable() bool { return true }

func (e *wildcardEvaluator) isMatch(s []uint16, pos int) (bool, int) {
	if len(e.sub.evals) == 0 { // last token in the pattern
		if pos >= len(s) {
			return true, pos
		}
		for i := pos; i < len(s); i++ {
			if isSep(s[i]) {
				return false, pos
			}
		}
		return true, pos + len(s)
	}
	if !e.sub.vari {
		// The rest of the pattern has a fixed length, so "*" must consume
		// exactly up to that point, without crossing a separator.
		required := len(s) - e.sub.min
		for i := pos; i < required; i++ {
			if isSep(s[i]) {
				return false, pos
			}
		}
		return e.sub.isMatch(s, required)
	}
	maxPos := len(s) - 1
	if e.sub.min > 0 {
		maxPos = maxPos - e.sub.min + 1
	}
	newPos := pos
	for i := pos; i <= maxPos; i++ {
		var ok bool
		if ok, newPos = e.sub.isMatch(s, i); ok {
			return true, newPos
		}
		if isSep(s[i]) {
			return false, newPos
		}
	}
	return e.sub.min == 0, newPos
}

// wildcardDirEvaluator ports WildcardDirectoryTokenEvaluator ("**").
type wildcardDirEvaluator struct {
	t   token
	sub *compositeEvaluator
}

func (e *wildcardDirEvaluator) minLen() int    { return e.sub.min }
func (e *wildcardDirEvaluator) variable() bool { return true }

func (e *wildcardDirEvaluator) isMatch(s []uint16, pos int) (bool, int) {
	newPos := pos
	if len(s) <= pos || pos < 0 {
		return false, newPos
	}
	cur := s[pos]
	if e.t.lead != 0 {
		// "/**" requires a separator here.
		if !isSep(cur) {
			return false, newPos
		}
		pos++
	} else if isSep(cur) {
		// "**" optionally matches a leading separator.
		pos++
	}
	if e.sub.min == 0 {
		return true, len(s)
	}
	maxPos := len(s) - e.sub.min
	if pos > maxPos {
		return false, newPos
	}
	if !e.sub.vari {
		// Only full segments may be matched.
		if maxPos > 0 && !isSep(s[maxPos-1]) {
			return false, newPos
		}
		return e.sub.isMatch(s, maxPos)
	}
	cur = s[pos]
	matchedSep := false
	if e.t.trail != 0 && isSep(cur) {
		pos++
	}
	if pos == maxPos {
		return e.sub.isMatch(s, pos)
	}
	for pos <= maxPos {
		if pos == maxPos && !matchedSep {
			return false, newPos
		}
		var ok bool
		if ok, newPos = e.sub.isMatch(s, pos); ok {
			return true, newPos
		}
		if pos == maxPos {
			return false, newPos
		}
		// Advance to just past the next separator (or to maxPos).
		matchedSep = false
		for pos < maxPos {
			pos++
			if isSep(s[pos]) {
				matchedSep = true
				pos++
				break
			}
		}
	}
	return false, newPos
}
