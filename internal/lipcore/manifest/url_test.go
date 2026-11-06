package manifest

import (
	"strings"
	"testing"
)

// Expected values were produced with Flurl 4.0.0 on .NET 10:
// Url.Parse(in).ToString().
func TestNormalizeURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://github.com/a/b/releases/download/v1/x.zip", "https://github.com/a/b/releases/download/v1/x.zip"},
		{" https://x.com/a ", "https://x.com/a"},
		{"\t", ""},
		{"HTTPS://Example.COM/A?b=c d#f", "https://example.com/A?b=c%20d#f"},
		{"https://x.com/a b", "https://x.com/a%20b"},
		{"https://x.com:443/a", "https://x.com:443/a"},
		{"http://x.com:80/a", "http://x.com:80/a"},
		{"https://x.com:8080", "https://x.com:8080"},
		{"https://x.com/%E4%B8%AD", "https://x.com/%E4%B8%AD"},
		{"https://x.com/%e4%b8%ad", "https://x.com/%E4%B8%AD"},
		{"https://x.com/中", "https://x.com/%E4%B8%AD"},
		{"not a url", "not%20a%20url"},
		{"/a b", "/a%20b"},
		{"a/b", "a/b"},
		{"//x.com/a", "//x.com/a"},
		{"https://x.com", "https://x.com"},
		{"https://x.com/", "https://x.com/"},
		{"https://x.com?a=1", "https://x.com?a=1"},
		{"https://x.com/a?", "https://x.com/a?"},
		{"https://x.com/a#", "https://x.com/a#"},
		{"https://x.com/a?x=1&y", "https://x.com/a?x=1&y"},
		{"https://user:p@x.com/", "https://user:p@x.com/"},
		{"https://x.com/a/../b", "https://x.com/b"},
		{"https://X.com/a/./b", "https://x.com/a/b"},
		{"https://x.com/a/b/..", "https://x.com/a/"},
		{"https://x.com/./a", "https://x.com/a"},
		{"https://x.com//a//b", "https://x.com//a//b"},
		{"https://x.com/a/", "https://x.com/a/"},
		{"https://x.com/a//", "https://x.com/a//"},
		{"https://x.com/a?q=a+b", "https://x.com/a?q=a+b"},
		{"https://x.com/a?q=%20", "https://x.com/a?q=%20"},
		{"https://x.com/a\\b", "https://x.com/a/b"},
		{"https://x.com/a|b{c}^d`e\"f<g>", "https://x.com/a%7Cb%7Bc%7D%5Ed%60e%22f%3Cg%3E"},
		{"https://x.com/%zz", "https://x.com/%25zz"},
		{"https://x.com/%", "https://x.com/%25"},
		{"https://x.com/a#b c", "https://x.com/a#b%20c"},
		{"https://x.com/a#中", "https://x.com/a#%E4%B8%AD"},
		{"https://x.com/a?x=中&y=a b", "https://x.com/a?x=%E4%B8%AD&y=a%20b"},
		{"https://x.com/a?x='y'", "https://x.com/a?x='y'"},
		{"https://x.com/a;b=c,d!e$f*g(h)", "https://x.com/a;b=c,d!e$f*g(h)"},
		{"https://x.com/a?b=c&b=d&&e", "https://x.com/a?b=c&b=d&&e"},
		{"https://x.com/a?=x", "https://x.com/a?=x"},
		{"https://x.com/a?b==c", "https://x.com/a?b==c"},
		{"ftp://x.com/a b", "ftp://x.com/a%20b"},
		{"https://x.com/~a_b-c.d", "https://x.com/~a_b-c.d"},
		{"https://x.com/a%2Fb", "https://x.com/a%2Fb"},
		{"https://x.com/a%41b", "https://x.com/aAb"},
		{"https://x.com/a?b=%zz", "https://x.com/a?b=%25zz"},
		{"https://x.com/a?b=c%2Bd", "https://x.com/a?b=c%2Bd"},
		{"https://x.com/a?b=c#d?e/f", "https://x.com/a?b=c#d?e/f"},
		{"https://x.com/a?b=%E4%B8%AD", "https://x.com/a?b=%E4%B8%AD"},
		{"https://[::1]:80/a", "https://[::1]:80/a"},
		{"https://x.com/a@b:c", "https://x.com/a@b:c"},
		{"https://x.com/a?b=c@d:e/f?g", "https://x.com/a?b=c@d:e/f?g"},
		{"https://x.com/a?b", "https://x.com/a?b"},
		{"https://x.com/a?b=", "https://x.com/a?b="},
		{"https://x.com/a?b=c+d e", "https://x.com/a?b=c+d%20e"},
		{"https://x.com/a b?c d#e f", "https://x.com/a%20b?c%20d#e%20f"},
		{"https://xn--fiq228c.com/a", "https://xn--fiq228c.com/a"},
		{"https://中文.com/a", "https://中文.com/a"},
		{"https://x.com/a?b=c;d", "https://x.com/a?b=c;d"},
		{"https://x.com/a?%41=1", "https://x.com/a?A=1"},
		{"https://x.com/a?b=c/d?e", "https://x.com/a?b=c/d?e"},
		{"https://x.com/a%2bb", "https://x.com/a%2bb"},
		{"https://x.com/a%2fb", "https://x.com/a%2fb"},
		{"https://x.com/a?b=%2f", "https://x.com/a?b=%2f"},
		{"https://x.com/%e9t%e9", "https://x.com/%e9t%e9"},
		{"https://x.com/%e4%b8%ad%2f", "https://x.com/%E4%B8%AD%2f"},
		{"https://github.com/o/r/releases/download/v1.0.0%2bbuild/x.zip", "https://github.com/o/r/releases/download/v1.0.0%2bbuild/x.zip"},
		{"https://x.com/%ef%bf%bd", "https://x.com/%ef%bf%bd"},
		{"https://x.com/a%c2%80b?c=%ee%80%80#%ee%80%80", "https://x.com/a%c2%80b?c=%EE%80%80#%ee%80%80"},
		{"https://x.com/%c2%80%c3%a9", "https://x.com/%c2%80%C3%A9"},
		{"https://x.com/%ee%80%80%e4%b8%ad?q=%c2%80%ee%80%80", "https://x.com/%ee%80%80%E4%B8%AD?q=%c2%80%EE%80%80"},
		{"/a%ee%80%80?q=%ee%80%80", "/a%ee%80%80?q=%EE%80%80"},
		// Not checked against Flurl: edge cases of the "uppercase only
		// complete valid UTF-8 sequences" rule the cases above establish.
		{"https://x.com/%c3%a9", "https://x.com/%C3%A9"},
		{"https://x.com/%f0%9f%98%80x", "https://x.com/%F0%9F%98%80x"},
		{"https://x.com/%e4%b8x", "https://x.com/%e4%b8x"},
		{"https://x.com/%c0%80", "https://x.com/%c0%80"},
		{"https://x.com/%ed%a0%80", "https://x.com/%ed%a0%80"},
		{"https://x.com/%a9%e9", "https://x.com/%a9%e9"},
		{"https://x.com/%e9%c3%a9", "https://x.com/%e9%C3%A9"},
		{"https://x.com/%c3", "https://x.com/%c3"},
		{"https://x.com/%c3%", "https://x.com/%c3%25"},
	}
	for _, tt := range tests {
		got := NormalizeURL(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if again := NormalizeURL(got); again != got {
			t.Errorf("NormalizeURL not idempotent for %q: %q -> %q", tt.in, got, again)
		}
	}
}

// TestNormalizeURLIRIRange covers every boundary of .NET's
// IriHelper.CheckIriUnicodeRange: an escaped UTF-8 character is
// uppercased only when System.Uri decodes it, which for private-use code
// points depends on the component. Expected values were produced with
// Flurl 4.0.0 on .NET 10 for https://x.com/X, https://x.com/a?b=X and
// https://x.com/a#X, where X is the escape.
func TestNormalizeURLIRIRange(t *testing.T) {
	tests := []struct {
		esc               string
		path, query, frag bool // whether the escape is uppercased there
	}{
		{"%c2%80", false, false, false},       // U+0080, C1 control
		{"%c2%9f", false, false, false},       // U+009F
		{"%c2%a0", true, true, true},          // U+00A0
		{"%e2%80%8e", true, true, true},       // U+200E, bidi control
		{"%ed%9f%bf", true, true, true},       // U+D7FF
		{"%ee%80%80", false, true, false},     // U+E000, private use
		{"%ef%a3%bf", false, true, false},     // U+F8FF
		{"%ef%a4%80", true, true, true},       // U+F900
		{"%ef%b7%8f", true, true, true},       // U+FDCF
		{"%ef%b7%90", false, false, false},    // U+FDD0, noncharacter
		{"%ef%b7%af", false, false, false},    // U+FDEF
		{"%ef%b7%b0", true, true, true},       // U+FDF0
		{"%ef%bf%af", true, true, true},       // U+FFEF
		{"%ef%bf%b0", false, false, false},    // U+FFF0
		{"%ef%bf%bd", false, false, false},    // U+FFFD
		{"%ef%bf%bf", false, false, false},    // U+FFFF
		{"%f0%90%80%80", true, true, true},    // U+10000
		{"%f0%9f%98%80", true, true, true},    // U+1F600
		{"%f0%9f%bf%bd", true, true, true},    // U+1FFFD
		{"%f0%9f%bf%be", false, false, false}, // U+1FFFE
		{"%f0%9f%bf%bf", false, false, false}, // U+1FFFF
		{"%f3%9f%bf%bf", false, false, false}, // U+DFFFF
		{"%f3%a0%80%80", false, false, false}, // U+E0000, tags
		{"%f3%a0%80%81", false, false, false}, // U+E0001
		{"%f3%a0%bf%bf", false, false, false}, // U+E0FFF
		{"%f3%a1%80%80", true, true, true},    // U+E1000
		{"%f3%af%bf%bd", true, true, true},    // U+EFFFD
		{"%f3%af%bf%bf", false, false, false}, // U+EFFFF
		{"%f3%b0%80%80", false, true, false},  // U+F0000, plane 15
		{"%f3%bf%bf%bd", false, true, false},  // U+FFFFD
		{"%f4%80%80%80", false, true, false},  // U+100000, plane 16
		{"%f4%8f%bf%bd", false, true, false},  // U+10FFFD
		{"%f4%8f%bf%be", false, false, false}, // U+10FFFE
		{"%f4%8f%bf%bf", false, false, false}, // U+10FFFF
	}
	for _, tt := range tests {
		for _, c := range []struct {
			prefix string
			upper  bool
		}{
			{"https://x.com/", tt.path},
			{"https://x.com/a?b=", tt.query},
			{"https://x.com/a#", tt.frag},
		} {
			in := c.prefix + tt.esc
			want := in
			if c.upper {
				want = c.prefix + strings.ToUpper(tt.esc)
			}
			if got := NormalizeURL(in); got != want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
			}
		}
	}
}
