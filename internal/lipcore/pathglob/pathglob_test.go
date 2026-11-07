package pathglob

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Ported from tests/Lip.Core.Tests/Mcmod/PathGlobTests.cs (lip v0.38.0-beta.1),
// IsMatch_ReturnsExpected and IsMatch_UnclosedBracket_MatchesLiteralBracket.
var upstreamMatch = []struct {
	pattern, path string
	want          bool
}{
	{"config.json", "config.json", true},
	{"config.json", "a/config.json", false},
	{"*.json", "config.json", true},
	{"*.json", "a/config.json", false},
	{"**/*.json", "config.json", true},
	{"**/*.json", "a/b/config.json", true},
	{"a/**", "a", true},
	{"a/**", "a/b/c", true},
	{"a/*/c", "a/b/c", true},
	{"a/*/c", "a/b/x/c", false},
	{"config/", "config/", true},
	{"config/", "config/a.json", true},
	{"config/", "config/x/a.json", true},
	{"config/", "config", false},
	{"config", "config/", false},
	{"**/data/", "x/data/y.bin", true},
	{"**/data/", "data/", true},
	{"**/", "file.txt", true},
	{"**/", "a/b/file.txt", true},
	{"**/**/", "file.txt", true},
	{"x/**/", "file.txt", false},
	{"x/**/", "x/file.txt", true},
	{"a.?", "a.b", true},
	{"a.?", "a.?", true},
	{"a.?", "a.bc", false},
	{"*", ".env", true},
	{"config/*", "config/.secret", true},
	{"[ab].txt", "b.txt", true},
	{"[ab].txt", "c.txt", false},
	{"[!ab].txt", "c.txt", true},
	{"[!ab].txt", "a.txt", false},
	{"log[0-9].txt", "log7.txt", true},
	{"log[0-9].txt", "logx.txt", false},
	{"[[:digit:]][[:alpha:]]", "1a", true},
	{"[[:digit:]][[:alpha:]]", "a1", false},
	{"[]a]", "]", true},
	{"[!]a]", "b", true},
	{"[[.-.]]", "-", true},
	{"[[=x=]]", "x", true},
	{"a?c", "a/c", false},

	// IsMatch_UnclosedBracket_MatchesLiteralBracket
	{"foo[", "foo[", true},
	{"foo[", "foo", false},
	{"a[]", "a[]", true},
	{"[ab", "[ab", true},
	{"[ab", "a", false},
	{"[!a", "[!a", true},
	{"[^a", "[^a", true},
	{"*[", "x[", true},
	{"[[:alpha]", "[a", true},
	{"[a/b]", "[a/b]", true},
}

// Additional cases checked against lip 0.38.0-beta.1.
var extraMatch = []struct {
	pattern, path string
	want          bool
}{
	{"*.JSON", "a.json", false}, // case-sensitive everywhere
	{"A.json", "a.json", false},
	{"*", "", true},
	{"*", "/", false},
	{"**/", "/", true},
	{"**", "", true},
	{"**", "a/b/c", true},
	{"**", "a/", false},
	{"a/**/b", "a/b", true},
	{"a/**/b", "a/x/y/b", true},
	{"a/**/b", "a/x/y/c", false},
	{"a/", "a/b/", true},
	{"a/", "b/a/c", false},
	{"*/", "a/b/c.txt", true},
	{"*/", "c.txt", false},
	{"[[:upper:]]*", "Abc", true},
	{"[[:upper:]]*", "abc", false},
	{"[[:xdigit:]]", "F", true},
	{"[[:xdigit:]]", "g", false},
	{"[[:punct:]]", "[", true},
	{"[[:punct:]]", "a", false},
	{"[[:space:]]", "\v", true},
	{"[[:cntrl:]]", "\x7f", true},
	{"[[:alpha:]]", "é", false}, // ASCII classes only
	{"[!a]", "é", true},
	{"?", "é", true},
	{"?", "😀", false}, // one UTF-16 code unit
	{"??", "😀", true},
	{"[a-]", "-", true},
	{"[a-]", "b", false},
	{"[--0]", "/", false}, // '/' never appears in a segment anyway
	{"[--0]", ".", false}, // .NET: "[\--0]" is '-', '-', '0'
	{"[--0]", "0", true},
	{"[--0]", "-", true},
	{"[!--]", ",", true}, // the negated set '-', '-' (no range before ']')
	{"[!--]", "-", false},
	{"[ --]", ",", true}, // a range may end at '-'
	{"[ --]", ".", false},
	{"\xff*", "\xfe", false}, // invalid UTF-8 bytes stay distinct
	{"\xff*", "\xffa", true},
	{"[!]", "]", false},  // unclosed: literal "[!]"
	{"[!]", "[!]", true}, // unclosed: literal "[!]"
	{"[]", "[]", true},
	{"[[:a", "[[:a", true},
	{"[a[.b]", "[a.", true}, // first '[' is literal, then "[.b]" is a set
	{"[a[.b]", "[a[.b]", false},
	{"x*y*z", "xAyBz", true},
	{"x*y*z", "xAyB", false},
	{"*a*a*", "aa", true},
	{"*a*a*", "a", false},
	{".*", ".hidden", true},
	{"a.b", "aXb", false}, // '.' is literal
	{"a+b", "a+b", true},
	{"(a)", "(a)", true},
	{"$*^", "$x^", true},
}

func TestIsMatch(t *testing.T) {
	for _, tc := range append(append([]struct {
		pattern, path string
		want          bool
	}{}, upstreamMatch...), extraMatch...) {
		g, err := Compile(tc.pattern)
		if err != nil {
			t.Errorf("Compile(%q): %v", tc.pattern, err)
			continue
		}
		if got := g.IsMatch(tc.path); got != tc.want {
			t.Errorf("Compile(%q).IsMatch(%q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// Ported from Constructor_InvalidPattern_ThrowsFormatException, with the
// exact upstream messages.
func TestCompileInvalid(t *testing.T) {
	cases := []struct{ pattern, msg string }{
		{"", "The glob '' is not valid: it is empty."},
		{"/a", "The glob '/a' is not valid: it starts with '/', but a glob is a relative path."},
		{"a//b", "The glob 'a//b' is not valid: it contains '//', an empty path segment."},
		{`a\b`, `The glob 'a\b' is not valid: it contains '\', but path segments are separated by '/' and there is no escape character.`},
		{"[^a]", "The glob '[^a]' is not valid: a bracket expression starts with '^'; write '[!...]' to match any character not in a set."},
		{"[[:word:]]", "The glob '[[:word:]]' is not valid: '[:word:]' is not a character class (the classes are [:alnum:], [:alpha:], [:blank:], [:cntrl:], [:digit:], [:graph:], [:lower:], [:print:], [:punct:], [:space:], [:upper:], [:xdigit:])."},
		{"[z-a]", "The glob '[z-a]' is not valid: the range 'z-a' ends before it starts."},
		{"[[.ab.]]", "The glob '[[.ab.]]' is not valid: '[.ab.]' names no character (only single characters are collating elements)."},
		{"[[==]]", "The glob '[[==]]' is not valid: '[==]' names no character (only single characters are collating elements)."},
		{"a/", ""},
		{"a///", "The glob 'a///' is not valid: it contains '//', an empty path segment."},
		{"a//", "The glob 'a//' is not valid: it contains '//', an empty path segment."},
		{"/", "The glob '/' is not valid: it starts with '/', but a glob is a relative path."},
		{"[^a", ""}, // unclosed: no error
		{"[^]", "The glob '[^]' is not valid: a bracket expression starts with '^'; write '[!...]' to match any character not in a set."},
		{"[z-a", ""},      // unclosed: no error
		{"[[:word:]", ""}, // unclosed: no error
		{"[^[:word:]z-a]", "The glob '[^[:word:]z-a]' is not valid: a bracket expression starts with '^'; write '[!...]' to match any character not in a set."}, // first error wins
		{"[[:word:]z-a]", "The glob '[[:word:]z-a]' is not valid: '[:word:]' is not a character class (the classes are [:alnum:], [:alpha:], [:blank:], [:cntrl:], [:digit:], [:graph:], [:lower:], [:print:], [:punct:], [:space:], [:upper:], [:xdigit:])."},
		{"ok/[z-a]", "The glob 'ok/[z-a]' is not valid: the range 'z-a' ends before it starts."},
		{"**/x[z-a]", "The glob '**/x[z-a]' is not valid: the range 'z-a' ends before it starts."},
	}
	for _, tc := range cases {
		err := Validate(tc.pattern)
		if tc.msg == "" {
			if err != nil {
				t.Errorf("Validate(%q) = %v, want nil", tc.pattern, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error", tc.pattern)
			continue
		}
		if err.Error() != tc.msg {
			t.Errorf("Validate(%q) = %q, want %q", tc.pattern, err.Error(), tc.msg)
		}
		if !strings.Contains(err.Error(), "'"+tc.pattern+"'") {
			t.Errorf("Validate(%q) message does not quote the pattern", tc.pattern)
		}
	}
}

func TestFixedPrefix(t *testing.T) {
	cases := []struct{ pattern, want string }{
		{"a/b/c", "a/b/"},
		{"a/b/*.json", "a/b/"},
		{"a/*/c", "a/"},
		{"*/b", ""},
		{"config.json", ""},
		{"config/", "config/"},
		{"a/b/", "a/b/"},
		{"**/x", ""},
		{"a/[b]/c", "a/"},
		{"a/b?/c", "a/"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := FixedPrefix(tc.pattern); got != tc.want {
			t.Errorf("FixedPrefix(%q) = %q, want %q", tc.pattern, got, tc.want)
		}
	}
}

func TestHasMetaAndMatchesDirectories(t *testing.T) {
	for s, want := range map[string]bool{"a": false, "a*": true, "?": true, "[": true, "]": false, "a/b": false, "**": true} {
		if got := HasMeta(s); got != want {
			t.Errorf("HasMeta(%q) = %v, want %v", s, got, want)
		}
	}
	if !MustCompile("a/").MatchesDirectories() || MustCompile("a").MatchesDirectories() {
		t.Error("MatchesDirectories")
	}
	if MustCompile("a/*").String() != "a/*" {
		t.Error("String")
	}
}

// TestGoldenCorpus replays testdata/corpus.tsv, produced by running lip
// 0.38.0-beta.1's PathGlob over generated patterns and paths. Each line is
// "C<TAB>pattern<TAB>result" for compilation (result "ok" or the error
// message), "P<TAB>pattern<TAB>prefix" for FixedPrefix, or
// "M<TAB>pattern<TAB>path<TAB>0|1" for IsMatch. Fields are JSON strings.
func TestGoldenCorpus(t *testing.T) {
	f, err := os.Open("testdata/corpus.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	for sc.Scan() {
		fields := strings.Split(strings.TrimSuffix(sc.Text(), "\r"), "\t")
		n := len(fields)
		if fields[0] == "M" {
			n = 3 // the result is a bare 0 or 1
		}
		for i := 1; i < n && i < len(fields); i++ {
			fields[i] = unquote(t, fields[i])
		}
		count++
		switch fields[0] {
		case "C":
			got := "ok"
			if err := Validate(fields[1]); err != nil {
				got = err.Error()
			}
			if got != fields[2] {
				t.Errorf("Compile(%q) = %q, want %q", fields[1], got, fields[2])
			}
		case "P":
			if got := FixedPrefix(fields[1]); got != fields[2] {
				t.Errorf("FixedPrefix(%q) = %q, want %q", fields[1], got, fields[2])
			}
		case "M":
			want := fields[3] == "1"
			if got := MustCompile(fields[1]).IsMatch(fields[2]); got != want {
				t.Errorf("Compile(%q).IsMatch(%q) = %v, want %v", fields[1], fields[2], got, want)
			}
		default:
			t.Fatalf("bad corpus line %q", sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 500 {
		t.Fatalf("corpus has only %d cases", count)
	}
}

func unquote(t *testing.T, s string) string {
	t.Helper()
	var v string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad corpus field %q: %v", s, err)
	}
	return v
}

// Patterns with many "**" segments must not take exponential time.
func TestManyDoubleStars(t *testing.T) {
	g := MustCompile(strings.Repeat("**/", 12) + "x/")
	path := strings.Repeat("a/", 40) + "b"
	if g.IsMatch(path) {
		t.Fatal("unexpected match")
	}
	if !g.IsMatch(strings.Repeat("a/", 40) + "x/b") {
		t.Fatal("expected match")
	}
}
