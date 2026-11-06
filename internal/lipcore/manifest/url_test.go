package manifest

import "testing"

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
