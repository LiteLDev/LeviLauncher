package glob

import (
	"errors"
	"math/rand"
	"testing"
)

// Ported from DotNet.Glob 3.1.3 src/DotNet.Glob.Tests/GlobTests.cs
// (Does_Not_Match).
var dotnetDoesNotMatch = []struct {
	pattern string
	subject []string
}{
	{"literal", []string{"fliteral", "foo/literal", "literals", "literals/foo"}},
	{"path/hats*nd", []string{"path/hatsblahn", "path/hatsblahndt"}},
	{"path/?atstand", []string{"path/moatstand", "path/batstands"}},
	{"/**/file.csv", []string{"/file.txt"}},
	{"/*file.txt", []string{"/folder"}},
	{"Shock* 12", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"*Shock* 12", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"*ave*2", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"*ave 12", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"*ave 12", []string{"wave 12/"}},
	{`C:\THIS_IS_A_DIR\**\somefile.txt`, []string{`C:\THIS_IS_A_DIR\awesomefile.txt`}},
	{`C:\name\**`, []string{`C:\name.ext`, `C:\name_longer.ext`}},
	{"Bumpy/**/AssemblyInfo.cs", []string{"Bumpy.Test/Properties/AssemblyInfo.cs"}},
	{`C:\sources\x-y 1\BIN\DEBUG\COMPILE\**\MSVC*120.DLL`, []string{`C:\sources\x-y 1\BIN\DEBUG\COMPILE\ANTLR3.RUNTIME.DLL`}},
	{"literal1", []string{"LITERAL1"}},
	{"*ral*", []string{"LITERAL1"}},
	{"[list]s", []string{"LS", "iS", "Is"}},
	{"range/[a-b][C-D]", []string{"range/ac", "range/Ad", "range/BD"}},
	{"abc/**", []string{"abcd"}},
	{`**\segment1\**\segment2\**`, []string{`C:\test\segment1\src\segment2`}},
	{"**/.*", []string{"foobar."}},
	{"**/~*", []string{"/"}},
}

// Ported from DotNet.Glob 3.1.3 src/DotNet.Glob.Tests/GlobTests.cs (IsMatch).
var dotnetIsMatch = []struct {
	pattern string
	subject []string
}{
	{"literal", []string{"literal"}},
	{"a/literal", []string{"a/literal"}},
	{"path/*atstand", []string{"path/fooatstand"}},
	{"path/hats*nd", []string{"path/hatsforstand"}},
	{"path/?atstand", []string{"path/hatstand"}},
	{"path/?atstand?", []string{"path/hatstands"}},
	{"p?th/*a[bcd]", []string{"pAth/fooooac"}},
	{"p?th/*a[bcd]b[e-g]a[1-4]", []string{"pAth/fooooacbfa2"}},
	{"p?th/*a[bcd]b[e-g]a[1-4][!wxyz]", []string{"pAth/fooooacbfa2v"}},
	{"p?th/*a[bcd]b[e-g]a[1-4][!wxyz][!a-c][!1-3].*", []string{"pAth/fooooacbfa2vd4.txt"}},
	{"path/**/somefile.txt", []string{"path/foo/bar/baz/somefile.txt"}},
	{"p?th/*a[bcd]b[e-g]a[1-4][!wxyz][!a-c][!1-3].*", []string{"pGth/yGKNY6acbea3rm8."}},
	{"/**/file.*", []string{"/folder/file.csv"}},
	{"/**/file.*", []string{"/file.txt"}},
	{"**/file.*", []string{"/file.txt"}},
	{"/*file.txt", []string{"/file.txt"}},
	{`C:\THIS_IS_A_DIR\*`, []string{`C:\THIS_IS_A_DIR\somefile`}},
	{"/DIR1/*/*", []string{"/DIR1/DIR2/file.txt"}},
	{"~/*~3", []string{"~/abc123~3"}},
	{`**\Shock* 12`, []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{`**\*ave*2`, []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"**", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12`}},
	{"**", []string{`HKEY_LOCAL_MACHINE\SOFTWARE\Adobe\Shockwave 12.txt`}},
	{"Stuff, *", []string{"Stuff, x"}},
	{`"Stuff*`, []string{`"Stuff`}},
	{"path/**/somefile.txt", []string{"path//somefile.txt"}},
	{"**/app*.js", []string{"dist/app.js", "dist/app.a72ka8234.js"}},
	{"**/y", []string{"y"}},
	{"**/gfx/*.gfx", []string{`HKEY_LOCAL_MACHINE\gfx\foo.gfx`, "HKEY_LOCAL_MACHINE/gfx/foo.gfx"}},
	{"**/gfx/**/*.gfx", []string{`a_b\gfx\bar\foo.gfx`, "a_b/gfx/bar/foo.gfx"}},
	{`**\gfx\**\*.gfx`, []string{`a_b\gfx\bar\foo.gfx`, "a_b/gfx/bar/foo.gfx"}},
	{"/foo/bar!.baz", []string{"/foo/bar!.baz"}},
	{"/foo/bar[!!].baz", []string{"/foo/bar7.baz"}},
	{"/foo/bar[!]].baz", []string{"/foo/bar9.baz"}},
	{"/foo/bar[!?].baz", []string{"/foo/bar7.baz"}},
	{"/foo/bar[![].baz", []string{"/foo/bar7.baz"}},
	{`C:\myergen\[[]a]tor`, []string{`C:\myergen\[a]tor`}},
	{`C:\myergen\[[]ator`, []string{`C:\myergen\[ator`}},
	{`C:\myergen\[[][]]ator`, []string{`C:\myergen\[]ator`}},
	{`C:\myergen[*]ator`, []string{`C:\myergen*ator`}},
	{`C:\myergen[*][]]ator`, []string{`C:\myergen*]ator`}},
	{`C:\myergen[*]]ator`, []string{`C:\myergen*ator`, `C:\myergen]ator`}},
	{`C:\myergen[?]ator`, []string{`C:\myergen?ator`}},
	{`/path[\]hatstand`, []string{`/path\hatstand`}},
	{`**\[#!]*\**`, []string{`#test3`, `#test3\`, `\#test3\foo`, `\#test3`}},
	{`**\[#!]*`, []string{`#test3`, "#this is a comment", `\#test3`}},
	{`[#!]*\**`, []string{"#this is a comment"}},
	{`[#!]*`, []string{`#test3`, "#this is a comment"}},
	{"abc/**", []string{"abc/def/hij.txt"}},
	{"a/**/b", []string{"a/b"}},
	{"abc/**", []string{"abc/def"}},
	{"/some/path/**/some.file*.exe", []string{"/some/path/some.file.exe"}},
	{"**/some/path/some.file*.exe", []string{"/some/path/some.file.exe"}},
}

// Cases mirroring how lip 0.37.0 uses globs: variant platforms
// (Glob.Parse(variant.Platform).IsMatch(platform)), placement src globs
// against archive keys, preserve_files against relative paths and bare file
// names, and remove_files against relative paths (directories as rel+"/").
var lipUsage = []struct {
	name    string
	pattern string
	subject string
	want    bool
}{
	{"platform wildcard", "win-*", "win-x64", true},
	{"platform wildcard arm", "win-*", "win-arm64", true},
	{"platform wildcard other os", "win-*", "linux-x64", false},
	{"platform exact", "win-x64", "win-x64", true},
	{"platform exact mismatch", "win-x64", "win-arm64", false},
	{"platform star", "*", "win-x64", true},
	{"platform case sensitive", "WIN-*", "win-x64", false},
	{"platform class", "win-[ax]*", "win-x64", true},
	{"platform question", "win-x??", "win-x64", true},

	{"placement plugins/** nested", "plugins/**", "plugins/a/b.dll", true},
	{"placement plugins/** direct", "plugins/**", "plugins/b.dll", true},
	{"placement plugins/** dir itself", "plugins/**", "plugins", false},
	{"placement plugins/** other dir", "plugins/**", "plugin/b.dll", false},
	{"placement plugins/**/*.dll", "plugins/**/*.dll", "plugins/x/y/z.dll", true},
	{"placement plugins/**/*.dll direct", "plugins/**/*.dll", "plugins/z.dll", true},
	{"placement plugins/**/*.dll other ext", "plugins/**/*.dll", "plugins/z.pdb", false},
	{"placement *.dll", "*.dll", "a.dll", true},
	{"placement *.dll no subdir", "*.dll", "sub/a.dll", false},
	{"placement *.dll case", "*.dll", "a.DLL", false},
	{"placement **/*.dll root", "**/*.dll", "a.dll", true},
	{"placement **/*.dll nested", "**/*.dll", "x/y/a.dll", true},
	{"placement backslash pattern", `plugins\*`, "plugins/x", true},
	{"placement backslash subject", "plugins/*", `plugins\x`, true},

	{"preserve config/*.json", "config/*.json", "config/a.json", true},
	{"preserve config/*.json nested", "config/*.json", "config/sub/a.json", false},
	{"preserve config/*.json other ext", "config/*.json", "config/a.toml", false},
	{"preserve bare file name", "*.json", "a.json", true},
	{"preserve bare file name vs path", "*.json", "config/a.json", false},
	{"preserve config/**", "config/**", "config/sub/a.json", true},

	{"remove dir trailing slash", "logs/", "logs/", true},
	{"remove dir trailing slash vs file", "logs/", "logs", false},
	{"remove logs/** vs dir", "logs/**", "logs/", true},
	{"remove logs/** vs nested", "logs/**", "logs/a/b.txt", true},
	{"remove logs/* vs dir", "logs/*", "logs/", true},
	{"remove logs/* vs file", "logs/*", "logs/a.log", true},
	{"remove logs/* vs nested", "logs/*", "logs/a/b.log", false},
	{"remove */ vs dir", "*/", "cache/", true},
	{"remove */ vs file", "*/", "cache", false},

	{"star empty suffix", "a*", "a", true},
	{"double star empty", "**", "", false},
	{"double star single", "**", "a", true},
}

func TestDotNetGlobIsMatch(t *testing.T) {
	for _, tc := range dotnetIsMatch {
		g := MustCompile(tc.pattern)
		for _, s := range tc.subject {
			if !g.Match(s) {
				t.Errorf("Compile(%q).Match(%q) = false, want true", tc.pattern, s)
			}
		}
	}
}

func TestDotNetGlobDoesNotMatch(t *testing.T) {
	for _, tc := range dotnetDoesNotMatch {
		g := MustCompile(tc.pattern)
		for _, s := range tc.subject {
			if g.Match(s) {
				t.Errorf("Compile(%q).Match(%q) = true, want false", tc.pattern, s)
			}
		}
	}
}

func TestLipUsage(t *testing.T) {
	for _, tc := range lipUsage {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(tc.pattern, tc.subject); got != tc.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.subject, got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	// Ported from GlobTests.To_String_Returns_Pattern.
	const pattern = "p?th/*a[bcd]b[e-g]/**/a[1-4][!wxyz][!a-c][!1-3].*"
	g := MustCompile(pattern)
	if g.String() != pattern {
		t.Errorf("String() = %q, want %q", g.String(), pattern)
	}
	if g.Format() != pattern {
		t.Errorf("Format() = %q, want %q", g.Format(), pattern)
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		pattern, want string
	}{
		{"plugins/**", "plugins/**"},
		{`a\**\b`, `a\**\b`},
		{"**", "**"},
		{"[abc", "[abc]"},
		{"[a-zA-Z]", "[a-z]"},
		{"[!0-9]x", "[!0-9]x"},
		{"[a]]", "[a]]"},
		{"[", "[[]"},
		{"[!", "[!!]"},
		{"日本*.txt", "日本*.txt"},
	}
	for _, tc := range tests {
		g := MustCompile(tc.pattern)
		if got := g.Format(); got != tc.want {
			t.Errorf("Compile(%q).Format() = %q, want %q", tc.pattern, got, tc.want)
		}
		if got := g.String(); got != tc.pattern {
			t.Errorf("Compile(%q).String() = %q", tc.pattern, got)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		pattern string
		wantErr bool
	}{
		{"", true},
		{"[a-]", true},
		{"[a-", true},
		{"[0-]x", true},
		{"[!a-]", true},
		{"[a-]]", false}, // range a..']'
		{"[", false},
		{"[abc", false},
		{"]", false},
		{"***", false},
		{"**x", false},
		{"a", false},
	}
	for _, tc := range tests {
		_, err := Compile(tc.pattern)
		if (err != nil) != tc.wantErr {
			t.Errorf("Compile(%q) err = %v, wantErr %v", tc.pattern, err, tc.wantErr)
		}
	}
	if _, err := Compile(""); !errors.Is(err, ErrEmptyPattern) {
		t.Errorf("Compile(\"\") err = %v, want ErrEmptyPattern", err)
	}
	if Match("[a-]", "a") {
		t.Error("Match with invalid pattern returned true")
	}
}

func TestQuirks(t *testing.T) {
	tests := []struct {
		pattern, subject string
		want             bool
	}{
		// Only the first range of a class is used.
		{"[a-zA-Z]", "q", true},
		{"[a-zA-Z]", "Q", false},
		// Unclosed class is a character list.
		{"[abc", "b", true},
		// "]]" inside a class is an escaped ']'.
		{"[a]]", "]", true},
		{"[a]]", "a", true},
		// Non-ASCII is matched per UTF-16 code unit.
		{"?.txt", "é.txt", true},
		{"?.txt", "😀.txt", false},
		{"??.txt", "😀.txt", true},
		{"日本*.txt", "日本語.txt", true},
		// Distinct invalid UTF-8 bytes stay distinct.
		{"\xff", "\xfe", false},
		{"\xff", "\xff", true},
		{"?", "\xff", true},
	}
	for _, tc := range tests {
		if got := Match(tc.pattern, tc.subject); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.subject, got, tc.want)
		}
	}
}

// TestNoPanic checks that matching never panics on arbitrary input.
func TestNoPanic(t *testing.T) {
	alphabet := []rune("ab/\\*?[]!-.0é")
	rng := rand.New(rand.NewSource(1))
	gen := func(n int) string {
		r := make([]rune, rng.Intn(n))
		for i := range r {
			r[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(r)
	}
	for i := 0; i < 20000; i++ {
		p, s := gen(10), gen(12)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on Match(%q, %q): %v", p, s, r)
				}
			}()
			Match(p, s)
		}()
	}
}
