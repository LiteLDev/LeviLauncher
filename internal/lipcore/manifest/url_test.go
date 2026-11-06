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

// TestNormalizeURLIRITrigger covers when System.Uri runs IRI processing
// (Uri.CheckForUnicodeOrEscapedUnreserved over the whole URL) and what it
// then does to the escapes of each component, the userinfo included.
// Expected values were produced with Flurl 4.0.0 on .NET 10:
// Url.Parse(in).ToString().
func TestNormalizeURLIRITrigger(t *testing.T) {
	tests := []struct{ in, want string }{
		// Escapes of < > ^ { | } are uppercased only when something
		// anywhere in the URL triggers IRI processing.
		{"https://h.com/x%3cy", "https://h.com/x%3cy"},
		{"https://h.com/x%3cy?q=%c3%a9", "https://h.com/x%3Cy?q=%C3%A9"},
		{"https://h.com/x%3c%c3%a9", "https://h.com/x%3C%C3%A9"},
		// With IRI, System.Uri trims trailing spaces the IRI pass decoded in
		// the fragment (Flurl 4.0.0 / .NET 10 results).
		{"https://github.com/o/r/releases/download/v1.0.0/%e4%b8%ad.zip#sha%20", "https://github.com/o/r/releases/download/v1.0.0/%E4%B8%AD.zip#sha"},
		{"https://h.com/%C3%A9#%7F%20", "https://h.com/%C3%A9#%7F"},
		{"https://h.com/a%41#%20", "https://h.com/aA#"},
		{"https://h.com/a%41#x%20%20", "https://h.com/aA#x"},
		{"https://h.com/a%41#%20x", "https://h.com/aA#%20x"},
		{"https://h.com/a#%20", "https://h.com/a#%20"},
		{"https://h.com/a%41?q=%20#f", "https://h.com/aA?q=%20#f"},
		{"https://h.com/x%5e%c2%80", "https://h.com/x%5E%c2%80"},
		{"https://h.com/x%3cy#%c2%80", "https://h.com/x%3Cy#%c2%80"},
		{"https://h.com/x%3cy?q=%41", "https://h.com/x%3Cy?q=A"},
		{"https://h.com/x%3cy?q=%e9", "https://h.com/x%3Cy?q=%e9"},
		{"https://h.com/x%3c?q=%7e", "https://h.com/x%3C?q=~"},
		{"https://h.com/x%3cy?q=中", "https://h.com/x%3Cy?q=%E4%B8%AD"},
		{"https://中.com/x%3c", "https://中.com/x%3C"},
		{"https://h.com/x%3c%ee%80%80", "https://h.com/x%3C%ee%80%80"},
		{"https://h.com/x%3c?q=%ee%80%80", "https://h.com/x%3C?q=%EE%80%80"},
		{"https://h.com/x%3c?q=%3c#%3c%c3%a9", "https://h.com/x%3C?q=%3C#%3C%C3%A9"},
		{"https://h.com/a?%3c=%c3%a9", "https://h.com/a?%3C=%C3%A9"},
		{"https://H.com/%3c?%c3%a9", "https://h.com/%3C?%C3%A9"},
		{"https://h.com:443/x%3c%c3%a9", "https://h.com:443/x%3C%C3%A9"},
		{"https://h.com/x%3c\\y%c3%a9", "https://h.com/x%3C/y%C3%A9"},
		{"https://h.com/%c3%a9/../%3c", "https://h.com/%3C"},
		{"https://h.com/%5E%7b?a=%c3%a9", "https://h.com/%5E%7B?a=%C3%A9"},
		{"https://h.com/x%60%20%22%c3%a9", "https://h.com/x%60%20%22%C3%A9"},
		{"https://user:p@x.com/%e9%3c", "https://user:p@x.com/%e9%3C"},
		// Reserved and unsafe escapes keep their case.
		{"https://h.com/x%7b%7c%7d%3e%2f%3a%5b%5c?q=%e9", "https://h.com/x%7B%7C%7D%3E%2f%3a%5b%5c?q=%e9"},
		{"https://h.com/%c3%a9%3c%2f%5b%40%3f%23%3d%26", "https://h.com/%C3%A9%3C%2f%5b%40%3f%23%3d%26"},
		{"https://h.com/%3c%c3%a9%2f%21", "https://h.com/%3C%C3%A9%2f%21"},
		{"https://h.com/x%3c?q=a%2Fb%3c%c3%a9", "https://h.com/x%3C?q=a%2Fb%3C%C3%A9"},
		{"https://h.com/x%25%3c%c3%a9", "https://h.com/x%25%3C%C3%A9"},
		{"https://h.com/x%7f%3c%c3%a9", "https://h.com/x%7f%3C%C3%A9"},
		{"https://h.com/x%1a%3c%c3%a9", "https://h.com/x%1a%3C%C3%A9"},
		// Invalid hex after a '%' triggers too, unless the '%' is one of
		// the last two characters.
		{"https://h.com/%zz%3c", "https://h.com/%25zz%3C"},
		{"https://h.com/x%3c%zza", "https://h.com/x%3C%25zza"},
		{"https://h.com/x%3c?q=%zz", "https://h.com/x%3C?q=%25zz"},
		{"https://h.com/%3c%", "https://h.com/%3c%25"},
		{"https://h.com/%3c%4", "https://h.com/%3c%254"},
		{"https://h.com/x%3c%z", "https://h.com/x%3c%25z"},
		{" https://h.com/x%3c%z", "https://h.com/x%3c%25z"},
		// Relative URLs follow the same rules.
		{"/a%3c", "/a%3c"},
		{"/a%3c%c3%a9", "/a%3C%C3%A9"},
		{"a%21%3c%41", "a%21%3CA"},
		{"/a%2f%21%3c%c3%a9", "/a%2f%21%3C%C3%A9"},
		{"/a%21%c3%a9?b=%21%3c#%21", "/a%21%C3%A9?b=%21%3C#%21"},
		{"/a%3c%zz", "/a%3C%25zz"},
		{"//h.com/a%3c%c3%a9", "//h.com/a%3C%C3%A9"},
		// The userinfo is normalized like the path.
		{"https://u%c3%a9@h.com/a", "https://u%C3%A9@h.com/a"},
		{"https://u%c3%a9:p@h.com/a", "https://u%C3%A9:p@h.com/a"},
		{"https://u%e2%82%ac%e2%82@h.com/x", "https://u%E2%82%AC%e2%82@h.com/x"},
		{"https://u%3c%c3%a9@h.com/x", "https://u%3C%C3%A9@h.com/x"},
		{"https://u:p%3c%c3%a9@h.com/x", "https://u:p%3C%C3%A9@h.com/x"},
		{"https://u%c3%a9@h.com/a%3c?b=%3c#%3c", "https://u%C3%A9@h.com/a%3C?b=%3C#%3C"},
		{"https://u%3c%c3%a9@h.com/x?q=%3c", "https://u%3C%C3%A9@h.com/x?q=%3C"},
		{"https://u%3c@h.com/x", "https://u%3c@h.com/x"},
		{"https://u%3c@h.com", "https://u%3c@h.com"},
		{"https://u%3c@h.com/%c3%a9", "https://u%3C@h.com/%C3%A9"},
		{"https://u%3C@h.com/%c3%a9", "https://u%3C@h.com/%C3%A9"},
		{"https://u%3c@h.com/%41", "https://u%3C@h.com/A"},
		{"https://u%2f%3a%40%c3%a9@h.com/x", "https://u%2f%3a%40%C3%A9@h.com/x"},
		{"https://u%e9@h.com/x", "https://u%e9@h.com/x"},
		{"https://u%c2%80@h.com/x", "https://u%c2%80@h.com/x"},
		{"https://u%ee%80%80@h.com/x", "https://u%ee%80%80@h.com/x"},
		{"https://u;=!$&'()*+,@h.com/x", "https://u;=!$&'()*+,@h.com/x"},
		// Not Flurl's output: when the userinfo changes length .NET also
		// drops the character after the host (https://uA@h.coma,
		// https://u%20v@h.comx, ...); NormalizeURL keeps the path intact.
		{"https://u%41@h.com/a", "https://uA@h.com/a"},
		{"https://u%7e@h.com/x", "https://u~@h.com/x"},
		{"https://u v@h.com/x", "https://u%20v@h.com/x"},
		{"https://u\"^|{}`@h.com/x", "https://u%22%5E%7C%7B%7D%60@h.com/x"},
		{"https://ü%3c@h.com/x", "https://%C3%BC%3C@h.com/x"},
		{"https://u%zz%c3%a9@h.com/x", "https://u%25zz%C3%A9@h.com/x"},
		{"https://u%3c v@h.com/%c3%a9", "https://u%3C%20v@h.com/%C3%A9"},
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

// The IRI pass copies a '%' with invalid hex and the two characters after it
// as they are (IriHelper.EscapeUnescapeIri). Like Flurl, the result is not
// idempotent: parsing it again IRI-processes the escapes that were skipped.
// Expected values are Flurl 4.0.0 / .NET 10 output.
func TestNormalizeURLInvalidHexSkipsIRI(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://h.com/%e%c3%a9", "https://h.com/%25e%c3%a9"},
		{"https://h.com/a%%41", "https://h.com/a%25%41"},
		{"https://h.com/a%%3c%c3%a9", "https://h.com/a%25%3c%C3%A9"},
		{"https://h.com/x?q=%%c3%a9", "https://h.com/x?q=%25%c3%a9"},
		{"https://h.com/%c3%a9%%3c", "https://h.com/%C3%A9%25%3c"},
		{"https://h.com/%c3%a9#%z%20", "https://h.com/%C3%A9#%25z%20"},
	}
	for _, tt := range tests {
		if got := NormalizeURL(tt.in); got != tt.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
