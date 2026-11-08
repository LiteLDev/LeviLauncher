package ref

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

// Ports of tests/Lip.Core.Tests/Mcmod/RefTests.cs (v0.38.0-beta.1).

func TestParseValidRefReturnsParts(t *testing.T) {
	for _, tc := range []struct {
		s       string
		kind    Kind
		target  string
		variant string
	}{
		{"https://example.com/x.zip", KindURL, "https://example.com/x.zip", ""},
		{"https://example.com/x.zip#a/b", KindURL, "https://example.com/x.zip", "a/b"},
		{"github.com/LiteLDev/LeviLamina", KindSpec, "github.com/LiteLDev/LeviLamina", ""},
		{"github.com/LiteLDev/LeviLamina#client", KindSpec, "github.com/LiteLDev/LeviLamina", "client"},
	} {
		r, err := Parse(tc.s, false)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.s, err)
		}
		if r.Kind != tc.kind || r.Target != tc.target || r.Variant != tc.variant || r.String() != tc.s {
			t.Errorf("Parse(%q) = %+v (%q)", tc.s, r, r.String())
		}
	}
}

func TestParseInvalidRef(t *testing.T) {
	for _, s := range []string{
		"github.com/a/b#",
		"github.com/a/b#UPPER",
		"./quickjs/",
		"./a/b.zip#lua",
		"http://example.com/x.zip",
		"file:///c/x.zip",
		"github.com/a/b@v1.0.0",
		"github.com/a/b@1.0",
		"github.com/a/b@branch:",
		"github.com/a/b@branch:a b",
		"github.com/a/b@tag:x.lock",
		"github.com/a/b@rev:0123abc",
		"github.com/a/b@rev:0123456789abcdef0123456789abcdef01234567\n",
		"github.com/a/b#lua\n",
		"github.com/a/b#c#d",
		"https://",
		"https://example.com/a b.zip",
		"https://example.com/é.zip",
		"https://example.com/%zz.zip",
		"https://example.com/a[1].zip",
		"https://example.com/a.zip?x[0]=1",
		"https://example.com/a.zip#lua#x",
		"",
	} {
		_, err := Parse(s, false)
		var pe *Error
		if !errors.As(err, &pe) {
			t.Errorf("Parse(%q) error = %v, want *Error", s, err)
		}
	}
}

func TestParseVersionWithLeadingVSaysToDropIt(t *testing.T) {
	_, err := Parse("github.com/a/b@v1.0.0", false)
	if err == nil {
		t.Fatal("no error")
	}
	if msg := err.Error(); !strings.Contains(msg, "'github.com/a/b@v1.0.0'") || !strings.Contains(msg, "'1.0.0'") {
		t.Errorf("message %q", msg)
	}
	want := "The source 'github.com/a/b@v1.0.0' is not valid: the ref 'v1.0.0' after '@' is not valid:" +
		" write the version without the leading 'v', as '1.0.0'."
	if err.Error() != want {
		t.Errorf("message %q, want %q", err.Error(), want)
	}
}

func TestParseAtInURLIsPartOfTheLocation(t *testing.T) {
	for _, s := range []string{
		"https://example.com/x.zip@tag:v1",
		"https://example.com/a%20b.zip",
		"https://[::1]/a.zip",
	} {
		r, err := Parse(s, false)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if r.Kind != KindURL || r.Target != s || r.GitRef != nil {
			t.Errorf("Parse(%q) = %+v", s, r)
		}
	}
}

func TestParseSpecWithGitRef(t *testing.T) {
	for _, tc := range []struct {
		s, target, variant string
		kind               GitRefKind
		name               string
	}{
		{"github.com/a/b@1.0.0", "github.com/a/b", "", GitRefVersion, "1.0.0"},
		{"github.com/a/b#lua@1.0.0-rc.1", "github.com/a/b", "lua", GitRefVersion, "1.0.0-rc.1"},
		{"github.com/a/b@branch:main", "github.com/a/b", "", GitRefBranch, "main"},
		{"github.com/a/b#lua@tag:v1.0.0", "github.com/a/b", "lua", GitRefTag, "v1.0.0"},
		{"github.com/a/b@branch:feat/a#b@c", "github.com/a/b", "", GitRefBranch, "feat/a#b@c"},
		{"github.com/a/b@rev:0123456789ABCDEF0123456789abcdef01234567", "github.com/a/b", "", GitRefRev,
			"0123456789abcdef0123456789abcdef01234567"},
	} {
		r, err := Parse(tc.s, false)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.s, err)
		}
		if r.Kind != KindSpec || r.Target != tc.target || r.Variant != tc.variant || r.GitRef == nil ||
			r.GitRef.Kind != tc.kind || r.GitRef.Name != tc.name {
			t.Errorf("Parse(%q) = %+v %+v", tc.s, r, r.GitRef)
		}
		again, err := Parse(r.String(), false)
		if err != nil || !again.Equal(r) {
			t.Errorf("round trip of %q: %+v, %v", tc.s, again, err)
		}
	}
}

func TestParseFileRefOnlyWhenAllowed(t *testing.T) {
	path := "/mods/a #1.zip"
	if runtime.GOOS == "windows" {
		path = `C:\mods\a #1.zip`
	}
	f, err := FromFile(path, "lua")
	if err != nil {
		t.Fatal(err)
	}
	text := f.String()
	r, err := Parse(text, true)
	if err != nil {
		t.Fatal(err)
	}
	if lp, _ := r.LocalPath(); r.Kind != KindFile || lp != path || r.Variant != "lua" {
		t.Errorf("Parse(%q) = %+v, local %q", text, r, lp)
	}
	if _, err := Parse(text, false); err == nil {
		t.Errorf("Parse(%q, false) succeeded", text)
	} else if !strings.Contains(err.Error(), "local paths are only allowed in package arguments on the"+
		" command line, not as dependencies") {
		t.Errorf("message %q", err)
	}
}

func TestFromFileDirectory(t *testing.T) {
	path := "/src/pkg"
	if runtime.GOOS == "windows" {
		path = `C:\src\pkg`
	}
	r, err := FromFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if lp, _ := r.LocalPath(); r.Kind != KindFile || lp != path {
		t.Errorf("FromFile(%q) = %+v, local %q", path, r, lp)
	}
	again, err := Parse(r.String(), true)
	if err != nil || !again.Equal(r) {
		t.Errorf("round trip: %+v, %v", again, err)
	}
}

func TestWithVariantReplacesVariant(t *testing.T) {
	r, _ := Parse("github.com/a/b#client", false)
	r, err := r.WithVariant("")
	if err != nil || r.String() != "github.com/a/b" {
		t.Errorf("WithVariant(\"\") = %q, %v", r, err)
	}
	if _, err := r.WithVariant("Bad"); err == nil {
		t.Error("WithVariant(\"Bad\") succeeded")
	}
}

func TestLocalPathOfNonFileRef(t *testing.T) {
	r, _ := Parse("github.com/a/b", false)
	if _, err := r.LocalPath(); err == nil {
		t.Error("LocalPath of a spec ref succeeded")
	}
}

func TestIsValidRefName(t *testing.T) {
	for name, want := range map[string]bool{
		"main": true, "feat/x#1": true, "release-2024": true, "build.zip": true, "v1.0.0": true, "a@b": true,
		"": false, "@": false, "/a": false, "a/": false, "a.": false, "a//b": false, "a..b": false, "a@{b": false,
		"a b": false, "a\tb": false, "a\x7fb": false, "a~b": false, "a^b": false, "a:b": false, "a?b": false,
		"a*b": false, "a[b": false, `a\b`: false, ".a": false, "a/.b": false, "a.lock": false, "a.lock/b": false,
	} {
		if got := IsValidRefName(name); got != want {
			t.Errorf("IsValidRefName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestGitRefErrors(t *testing.T) {
	for s, want := range map[string]string{
		"branch:":  `the branch name after "branch:" is missing`,
		"tag:a..b": "'a..b' is not a valid git tag name (see 'git check-ref-format')",
		"rev:abc":  "'abc' is not a full commit id: write all 40 hexadecimal digits, not an abbreviation",
		"V1.2.3":   "write the version without the leading 'v', as '1.2.3'",
		"1.0":      "a ref is a SemVer version such as 1.2.3 (without a leading 'v'), branch:<name>, tag:<name> or rev:<full commit id>",
	} {
		_, reason, ok := TryParseGitRef(s)
		if ok || reason != want {
			t.Errorf("TryParseGitRef(%q) = %v %q, want %q", s, ok, reason, want)
		}
	}
	if _, err := ParseGitRef("x"); err == nil || !strings.HasPrefix(err.Error(), "The ref 'x' of a remote repository is not valid: ") {
		t.Errorf("ParseGitRef error %v", err)
	}
	g, err := RevGitRef("0123456789ABCDEF0123456789ABCDEF01234567")
	if err != nil || g.String() != "rev:0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("RevGitRef = %v, %v", g, err)
	}
	if g, _, ok := TryParseGitRef("99999999999999999999999.0.0-rc.1+b.01"); !ok || g.Kind != GitRefVersion {
		t.Errorf("big version not accepted")
	}
}

func TestNames(t *testing.T) {
	if Fold("GitHub.com/ÄB") != "github.com/Äb" {
		t.Errorf("Fold = %q", Fold("GitHub.com/ÄB"))
	}
	if !NamesEqual("github.com/A/b", "GITHUB.COM/a/B") || NamesEqual("é", "É") {
		t.Error("NamesEqual")
	}
	if Identity("x.y/z", "") != "x.y/z" || Identity("x.y/z", "client") != "x.y/z#client" {
		t.Error("Identity")
	}
	for name, want := range map[string]bool{"a": true, "LeviLamina": true, "a.b_c-d": true, "1": true,
		"": false, ".a": false, "_a": false, "a b": false, "é": false} {
		if IsValidName(name) != want {
			t.Errorf("IsValidName(%q) != %v", name, want)
		}
	}
}

// TestDotnetGolden checks results recorded from lip main's C# (Ref.TryParse,
// GitRef.TryParse, Module.CheckPath and Ref.FromFile, run on .NET 10 on
// Windows) over a corpus aimed at the corners of URLs and file: URIs.
func TestDotnetGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/dotnet_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []map[string]any
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	str := func(m map[string]any, k string) string {
		s, _ := m[k].(string)
		return s
	}
	windows := runtime.GOOS == "windows"
	gitKinds := map[GitRefKind]string{GitRefVersion: "Version", GitRefBranch: "Branch", GitRefTag: "Tag", GitRefRev: "Rev"}
	for _, c := range cases {
		in, mode := str(c, "in"), str(c, "mode")
		ok, _ := c["ok"].(bool)
		got := map[string]any{}
		switch mode {
		case "ref", "reff":
			// A file: URI names a Windows path on Windows and a Unix path
			// elsewhere; the corpus was recorded on Windows (fileuri_test.go
			// covers the others).
			if mode == "reff" && !windows && hasFileScheme(in) {
				continue
			}
			r, reason, rok := TryParse(in, mode == "reff")
			got["ok"] = rok
			if rok {
				got["kind"], got["target"], got["variant"] = r.Kind.String(), r.Target, r.Variant
				if r.GitRef != nil {
					got["gitref"] = r.GitRef.String()
				}
				if r.Kind == KindFile {
					if !windows {
						delete(c, "local")
					} else if lp, err := r.LocalPath(); err != nil {
						got["local"] = "EXC:" + err.Error()
					} else {
						got["local"] = lp
					}
				}
			} else {
				got["err"] = reason
			}
		case "gitref":
			g, reason, gok := TryParseGitRef(in)
			got["ok"] = gok
			if gok {
				got["kind"], got["name"] = gitKinds[g.Kind], g.Name
			} else {
				got["err"] = reason
			}
		case "mod":
			err := CheckModulePath(in)
			got["ok"] = err == nil
			if err != nil {
				got["err"] = err.Error()
			}
		case "fromfile":
			if !windows {
				continue
			}
			r, err := FromFile(in, "")
			got["ok"] = err == nil
			if err != nil {
				got["exc"] = err.Error()
			} else {
				got["target"] = r.Target
				got["local"], _ = r.LocalPath()
			}
		}
		_ = ok
		for _, k := range []string{"ok", "kind", "target", "variant", "gitref", "local", "err", "name", "exc"} {
			want, have := c[k], got[k]
			if want == nil {
				want = ""
			}
			if have == nil {
				have = ""
			}
			if want != have {
				t.Errorf("%s %q: %s = %#v, want %#v", mode, in, k, have, want)
			}
		}
	}
}
