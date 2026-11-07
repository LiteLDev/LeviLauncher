package ref

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ports of tests/Lip.Core.Tests/Mcmod/PackageArgumentsTests.cs
// (v0.38.0-beta.1), on the real file system.

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func newWorkspace(t *testing.T, files ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestParseArgumentRemoteRepository(t *testing.T) {
	ws := newWorkspace(t)
	for arg, want := range map[string]string{
		"github.com/LiteLDev/LeviLamina": "github.com/LiteLDev/LeviLamina",
		"github.com/a/b#client":          "github.com/a/b#client",
	} {
		r, err := ParsePackageArgument(arg, ws)
		if err != nil || r.Kind != KindSpec || r.GitRef != nil || r.String() != want {
			t.Errorf("ParsePackageArgument(%q) = %+v, %v", arg, r, err)
		}
	}
}

func TestParseArgumentGitRef(t *testing.T) {
	ws := newWorkspace(t)
	for _, tc := range []struct {
		arg     string
		kind    GitRefKind
		name    string
		variant string
	}{
		{"github.com/LiteLDev/LeviLamina@1.9.7", GitRefVersion, "1.9.7", ""},
		{"github.com/a/b#client@1.0.0-rc.1", GitRefVersion, "1.0.0-rc.1", "client"},
		{"github.com/LiteLDev/LeviLamina@branch:main", GitRefBranch, "main", ""},
		{"github.com/a/b#client@branch:feat/x#1", GitRefBranch, "feat/x#1", "client"},
		{"github.com/a/b@tag:release-2024", GitRefTag, "release-2024", ""},
		{"github.com/a/b@tag:build.zip", GitRefTag, "build.zip", ""},
		{"github.com/a/b@rev:" + testCommit, GitRefRev, testCommit, ""},
	} {
		r, err := ParsePackageArgument(tc.arg, ws)
		if err != nil || r.Kind != KindSpec || r.GitRef == nil || r.GitRef.Kind != tc.kind ||
			r.GitRef.Name != tc.name || r.Variant != tc.variant || r.String() != tc.arg {
			t.Errorf("ParsePackageArgument(%q) = %+v %+v, %v", tc.arg, r, r.GitRef, err)
		}
	}
}

func TestParseArgumentHTTPSURL(t *testing.T) {
	r, err := ParsePackageArgument("https://example.com/x.zip#lua", newWorkspace(t))
	if err != nil || r.Kind != KindURL || r.Target != "https://example.com/x.zip" || r.Variant != "lua" {
		t.Errorf("= %+v, %v", r, err)
	}
}

func TestParseArgumentRelativeLocalFile(t *testing.T) {
	ws := newWorkspace(t, "x.zip", "UP.ZIP", "sub/x.zip", "a#b.zip", "c#d")
	for _, tc := range []struct{ arg, file, variant string }{
		{"./x.zip", "x.zip", ""},
		{"x.zip", "x.zip", ""},
		{"UP.ZIP", "UP.ZIP", ""},
		{"sub/x.zip#lua", "sub/x.zip", "lua"},
		{"a#b.zip", "a#b.zip", ""},
		{"a#b.zip#lua", "a#b.zip", "lua"},
		{"./c#d#lua", "c#d", "lua"},
	} {
		r, err := ParsePackageArgument(tc.arg, ws)
		if err != nil {
			t.Fatalf("ParsePackageArgument(%q): %v", tc.arg, err)
		}
		lp, _ := r.LocalPath()
		if r.Kind != KindFile || lp != filepath.Join(ws, filepath.FromSlash(tc.file)) || r.Variant != tc.variant {
			t.Errorf("ParsePackageArgument(%q) = %+v, local %q", tc.arg, r, lp)
		}
	}
}

func TestParseArgumentExistingFileNotEndingInZip(t *testing.T) {
	ws := newWorkspace(t, "pkg.mcmod")
	r, err := ParsePackageArgument("pkg.mcmod", ws)
	lp, _ := r.LocalPath()
	if err != nil || r.Kind != KindFile || lp != filepath.Join(ws, "pkg.mcmod") {
		t.Errorf("= %+v (%q), %v", r, lp, err)
	}
}

func TestParseArgumentMissingMcmodFileIsNotALocalPath(t *testing.T) {
	// ".mcmod" is no local-path extension, so this is a remote repository.
	r, err := ParsePackageArgument("example.com/x.mcmod", newWorkspace(t))
	if err != nil || r.Kind != KindSpec {
		t.Errorf("= %+v, %v", r, err)
	}
}

func TestParseArgumentAbsoluteLocalFileOutsideWorkspace(t *testing.T) {
	ws := newWorkspace(t)
	path := filepath.Join(filepath.Dir(ws), "elsewhere", "mod.zip")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("zip"), 0o644)
	r, err := ParsePackageArgument(path, ws)
	lp, _ := r.LocalPath()
	if err != nil || lp != path || !strings.HasPrefix(r.String(), "file:///") {
		t.Errorf("= %+v (%q), %v", r, lp, err)
	}
}

func TestParseArgumentDirectory(t *testing.T) {
	ws := newWorkspace(t, "pkg/lip.yaml")
	for _, tc := range []struct{ arg, dir, variant string }{
		{"./pkg", "pkg", ""},
		{"./pkg/", "pkg", ""},
		{"pkg", "pkg", ""},
		{"pkg#lua", "pkg", "lua"},
		{".", "", ""},
	} {
		r, err := ParsePackageArgument(tc.arg, ws)
		if err != nil {
			t.Fatalf("ParsePackageArgument(%q): %v", tc.arg, err)
		}
		want := ws
		if tc.dir != "" {
			want = filepath.Join(ws, tc.dir)
		}
		lp, _ := r.LocalPath()
		if r.Kind != KindFile || lp != want || r.Variant != tc.variant {
			t.Errorf("ParsePackageArgument(%q) = %+v, local %q", tc.arg, r, lp)
		}
	}
}

func TestParseArgumentInvalid(t *testing.T) {
	ws := newWorkspace(t)
	for _, arg := range []string{
		"github.com/a/b@v1.0.0",
		"github.com/a/b@1.0",
		"github.com/a/b@branch:",
		"github.com/a/b@branch:a..b",
		"github.com/a/b@rev:abc1234",
		"github.com/a/b@commit:abc",
		"github.com/a/b#",
		"./missing.zip",
		"missing.zip",
		"missing.ZIP",
		"./missing",
		"https://example.com/x.zip#",
		"http://example.com/x.zip",
		"file:///c/x.zip",
		"ftp://example.com/x.zip",
	} {
		// The message names the argument and explains the accepted forms.
		_, err := ParsePackageArgument(arg, ws)
		if err == nil || !strings.Contains(err.Error(), "'"+arg+"'") || !strings.Contains(err.Error(), AcceptedForms) {
			t.Errorf("ParsePackageArgument(%q) error %v", arg, err)
		}
	}
}

func TestParseArgumentMessages(t *testing.T) {
	ws := newWorkspace(t)
	for arg, want := range map[string]string{
		"http://example.com/x.zip": "'http://example.com/x.zip' is not a valid package: lip downloads only over" +
			" https://, so use the https:// URL of the archive. " + AcceptedForms,
		"FILE:///x": "'FILE:///x' is not a valid package: a local package is written as its path (e.g." +
			" ./my-package.zip), not as a file: URI. " + AcceptedForms,
		"file:x": "'file:x' is not a valid package: a local package is written as its path (e.g." +
			" ./my-package.zip), not as a file: URI. " + AcceptedForms,
		"ftp://x/y": "'ftp://x/y' is not a valid package: lip cannot download from ftp:// URLs, only from" +
			" https:// URLs. " + AcceptedForms,
		"./missing.zip": "Cannot find the local package './missing.zip': " + filepath.Join(ws, "missing.zip") +
			" does not exist. Check the path (a relative path is relative to the workspace root " + ws + "). " +
			AcceptedForms,
	} {
		_, err := ParsePackageArgument(arg, ws)
		if err == nil || err.Error() != want {
			t.Errorf("ParsePackageArgument(%q) error\n %v\nwant\n %s", arg, err, want)
		}
	}
}
