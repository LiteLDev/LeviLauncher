package ref

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ports of the tests RefTests.cs and PackageArgumentsTests.cs gained with
// the file: URIs of RFC 8089 (lip a68921d, 93826f0, ddb96ca).

func TestFromFileSpecialCharactersPercentEncodesThemAndRoundTrips(t *testing.T) {
	// A '%' in a path is no escape (protocol/workspace.md §7.1, RFC 8089).
	for _, c := range []struct{ name, encoded string }{
		{"pkg%41dir", "pkg%2541dir"},
		{"a%20b", "a%2520b"},
		{"a #1", "a%20%231"},
		{"why?.zip", "why%3F.zip"},
		{"模组 é", "%E6%A8%A1%E7%BB%84%20%C3%A9"},
		{"a;b=c@d", "a;b=c@d"},
	} {
		path, uri := "/ws/"+c.name+"/x", "file:///ws/"+c.encoded+"/x"
		if onWindows {
			path, uri = `C:\ws\`+c.name+`\x`, "file:///C:/ws/"+c.encoded+"/x"
		}

		r, err := FromFile(path, "lua")
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(r.String(), true)
		if err != nil {
			t.Fatal(err)
		}

		local, _ := r.LocalPath()
		parsedLocal, _ := parsed.LocalPath()
		if r.Target != uri || local != path || !parsed.Equal(r) || parsedLocal != path {
			t.Errorf("%q: target %q, local %q, parsed %+v (%q)", c.name, r.Target, local, parsed, parsedLocal)
		}
	}
}

func TestParseFileURIDecodesEveryPercentEncodedOctetOnce(t *testing.T) {
	uri, want := "file:///ws/pkg%2541dir/%E6%A8%A1", "/ws/pkg%41dir/模"
	if onWindows {
		uri, want = "file:///c:/ws/pkg%2541dir/%E6%A8%A1", `c:\ws\pkg%41dir\模`
	}

	r, err := Parse(uri, true)

	if err != nil {
		t.Fatal(err)
	}
	if local, _ := r.LocalPath(); local != want || r.Target != uri {
		t.Fatalf("local %q, target %q", local, r.Target)
	}
}

func TestParseInvalidFileURIFails(t *testing.T) {
	for _, name := range []string{"a?x=1", "a%zz", "a%2Fb", "%FF", "a b", "[a]"} {
		uri := "file:///ws/" + name
		if onWindows {
			uri = "file:///C:/ws/" + name
		}
		if _, err := Parse(uri, true); err == nil {
			t.Errorf("%q parsed", uri)
		}
	}
}

func TestParseFileURIWithUserinfoOrPortFails(t *testing.T) {
	for _, uri := range []string{"file://user@server/share/x", "file://server:80/share/x"} {
		if _, err := Parse(uri, true); err == nil {
			t.Errorf("%q parsed", uri)
		}
	}
}

func TestFromFileUNCPathOnWindowsRoundTrips(t *testing.T) {
	if !onWindows {
		t.Skip("UNC paths are Windows paths.")
	}
	for _, c := range []struct{ path, uri string }{
		{`\\server\share\a%41 #`, "file://server/share/a%2541%20%23"},
		// A reg-name has neither '@' (userinfo) nor ':' (port) [RFC3986].
		{`\\server@SSL\share\x`, "file://server%40SSL/share/x"},
		{`\\localhost\share\x`, ""},
		{`\\LOCALHOST\c$\x`, ""},
	} {
		r, err := FromFile(c.path, "")
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(r.String(), true)
		if err != nil {
			t.Fatal(err)
		}
		local, _ := r.LocalPath()
		if (c.uri != "" && r.Target != c.uri) || local != c.path || !parsed.Equal(r) {
			t.Errorf("%q: target %q, local %q, parsed %+v", c.path, r.Target, local, parsed)
		}
	}
}

func TestFromFileRelativePathFails(t *testing.T) {
	for _, path := range []string{"relative/x", `relative\x`, ""} {
		if _, err := FromFile(path, ""); err == nil {
			t.Errorf("%q accepted", path)
		}
	}
}

func TestParseDirectoryWithPercentInNameNamesThatDirectory(t *testing.T) {
	// "%41" is part of the name, not an escape of "A".
	root := newWorkspace(t)
	path := filepath.Join(root, "pkg%41dir")
	for _, d := range []string{path, filepath.Join(root, "pkgAdir")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	r, err := ParsePackageArgument("./pkg%41dir", root)

	if err != nil {
		t.Fatal(err)
	}
	if local, _ := r.LocalPath(); local != path || !strings.HasSuffix(r.String(), "/pkg%2541dir") {
		t.Fatalf("local %q, ref %q", local, r.String())
	}
}
