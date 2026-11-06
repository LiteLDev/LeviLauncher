// Package glob implements glob matching compatible with DotNet.Glob 3.1.3
// using default options, as used by lip 0.37.0 for variant platforms,
// file placements, preserve_files and remove_files.
//
// Semantics: case-sensitive; "*" matches any run of characters except path
// separators ('/' and '\'); "**" matches across directories; "?" matches one
// non-separator character; "[abc]", "[a-z]" and "[!abc]" character classes;
// '/' and '\' in a pattern each match either separator.
package glob

import "errors"

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("glob: not implemented")

// Glob is a compiled pattern.
type Glob struct {
	pattern string
	// Implementation fields may be added.
}

// Compile parses pattern.
func Compile(pattern string) (*Glob, error) {
	return nil, ErrNotImplemented
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
	panic("glob: not implemented")
}

// String returns the original pattern (Glob.ToString round-trips it).
func (g *Glob) String() string { return g.pattern }

// Match compiles pattern and matches s; a pattern that fails to compile
// matches nothing.
func Match(pattern, s string) bool {
	g, err := Compile(pattern)
	if err != nil {
		return false
	}
	return g.Match(s)
}
