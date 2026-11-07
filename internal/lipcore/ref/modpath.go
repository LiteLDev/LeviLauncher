package ref

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ModulePathError is Module.InvalidPathError of lip's Golang.Org.X.Mod
// (src/Golang.Org.X.Mod/Module.cs, v0.38.0-beta.1): Go's "malformed module
// path" error worded for people. Reason is the inner exception's message,
// the reason alone.
type ModulePathError struct {
	Kind   string
	Path   string
	Reason string
}

func (e *ModulePathError) Error() string {
	return fmt.Sprintf("The %s path \"%s\" is not valid: %s.", e.Kind, e.Path, e.Reason)
}

// Unwrap returns the reason alone, like the C# inner exception.
func (e *ModulePathError) Unwrap() error { return reasonError(e.Reason) }

type reasonError string

func (e reasonError) Error() string { return string(e) }

// CheckModulePath ports Module.CheckPath (golang.org/x/mod/module.CheckPath)
// with the messages of lip 0.38. It returns nil or a *ModulePathError.
func CheckModulePath(path string) error {
	mk := func(reason string) error {
		return &ModulePathError{Kind: "module", Path: path, Reason: reason}
	}

	if reason := checkPathInternal(path); reason != "" {
		return mk(reason)
	}
	i := strings.IndexByte(path, '/')
	if i < 0 {
		i = len(path)
	}
	if i == 0 {
		return mk("it starts with '/'")
	}
	first := path[:i]
	if !strings.Contains(first, ".") {
		return mk(fmt.Sprintf("its first path element \"%s\" has no dot, but the first element"+
			" must be a host name such as github.com", first))
	}
	if path[0] == '-' {
		return mk("its first path element starts with '-'")
	}
	// checkPathInternal rejected every non-ASCII character, so bytes are chars.
	for j := 0; j < i; j++ {
		if !firstPathOK(path[j]) {
			return mk(fmt.Sprintf("its first path element \"%s\" contains %s, but a host name has"+
				" only lowercase ASCII letters, digits, '-' and '.'", first, showRune(rune(path[j]))))
		}
	}
	if _, _, ok := SplitPathVersion(path); !ok {
		return mk("the major version suffix at its end is invalid (it must be /v2 or higher" +
			" without a leading zero or a dot, or .v0 or higher for gopkg.in)")
	}
	return nil
}

// SplitPathVersion ports Module.SplitPathVersion.
func SplitPathVersion(path string) (prefix, pathMajor string, ok bool) {
	if strings.HasPrefix(path, "gopkg.in/") {
		return splitGopkgIn(path)
	}

	i := len(path)
	dot := false
	for i > 0 && (path[i-1] >= '0' && path[i-1] <= '9' || path[i-1] == '.') {
		if path[i-1] == '.' {
			dot = true
		}
		i--
	}
	if i <= 1 || i == len(path) || path[i-1] != 'v' || path[i-2] != '/' {
		return path, "", true
	}
	prefix, pathMajor = path[:i-2], path[i-2:]
	if dot || len(pathMajor) <= 2 || pathMajor[2] == '0' || pathMajor == "/v1" {
		return path, "", false
	}
	return prefix, pathMajor, true
}

func splitGopkgIn(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "gopkg.in/") {
		return path, "", false
	}
	i := len(path)
	if strings.HasSuffix(path, "-unstable") {
		i -= len("-unstable")
	}
	for i > 0 && path[i-1] >= '0' && path[i-1] <= '9' {
		i--
	}
	if i <= 1 || path[i-1] != 'v' || path[i-2] != '.' {
		return path, "", false
	}
	prefix, pathMajor := path[:i-2], path[i-2:]
	if len(pathMajor) <= 2 || pathMajor[2] == '0' && pathMajor != ".v0" {
		return path, "", false
	}
	return prefix, pathMajor, true
}

var badWindowsNames = []string{
	"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
}

// checkPathInternal ports ModuleInternal.CheckPath; "" means valid.
func checkPathInternal(path string) string {
	if path == "" {
		return "it is empty"
	}
	if path[0] == '-' {
		return "it starts with '-'"
	}
	if strings.Contains(path, "//") {
		return "it contains '//'"
	}
	if path[len(path)-1] == '/' {
		return "it ends with '/'"
	}
	elemStart := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if r := checkElem(path[elemStart:i]); r != "" {
				return r
			}
			elemStart = i + 1
		}
	}
	return checkElem(path[elemStart:])
}

// checkElem ports ModuleInternal.CheckElem; "" means valid.
func checkElem(elem string) string {
	if elem == "" {
		return "one of its path elements is empty"
	}
	if strings.Count(elem, ".") == len(elem) {
		return fmt.Sprintf("the path element \"%s\" consists only of dots", elem)
	}
	if elem[0] == '.' {
		return fmt.Sprintf("the path element \"%s\" starts with a dot", elem)
	}
	// C# checks the last UTF-16 char; a trailing non-ASCII char is not '.'.
	if elem[len(elem)-1] == '.' {
		return fmt.Sprintf("the path element \"%s\" ends with a dot", elem)
	}
	for _, r := range elem {
		if !modPathOK(r) {
			return fmt.Sprintf("the path element \"%s\" contains %s, but only ASCII letters,"+
				" digits, '-', '.', '_' and '~' are allowed", elem, showRune(r))
		}
	}

	short := elem
	if dot := strings.IndexByte(short, '.'); dot >= 0 {
		short = short[:dot]
	}
	for _, bad := range badWindowsNames {
		if strings.EqualFold(bad, short) {
			return fmt.Sprintf("the path element \"%s\" uses \"%s\", a name that Windows reserves",
				elem, short)
		}
	}

	if tilde := strings.LastIndexByte(short, '~'); tilde >= 0 && tilde < len(short)-1 {
		digits := true
		for _, c := range []byte(short[tilde+1:]) {
			if c < '0' || c > '9' {
				digits = false
				break
			}
		}
		if digits {
			return fmt.Sprintf("the path element \"%s\" ends with '~' followed by digits", elem)
		}
	}
	return ""
}

// showRune ports ModuleInternal.Show, which works on UTF-16 chars: a
// printable ASCII char quoted, otherwise its code point. A character
// outside the BMP shows as its high surrogate, the first char C# sees; an
// invalid UTF-8 byte (no C# equivalent) shows as U+FFFD.
func showRune(r rune) string {
	if r >= ' ' && r < 0x7f {
		return fmt.Sprintf("'%c'", r)
	}
	if r > 0xFFFF && r <= utf8.MaxRune {
		r = 0xD800 + (r-0x10000)>>10
	}
	return fmt.Sprintf("the character U+%04X", r)
}

func firstPathOK(c byte) bool {
	return c == '-' || c == '.' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z'
}

func modPathOK(r rune) bool {
	if r < 0x80 {
		return r == '-' || r == '.' || r == '_' || r == '~' ||
			'0' <= r && r <= '9' || 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z'
	}
	return false
}
