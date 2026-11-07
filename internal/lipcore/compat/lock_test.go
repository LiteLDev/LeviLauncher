package compat

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// Ports of tests/Lip.Core.Tests/Mcmod/Compat/ToothLockMigrationTests.cs
// (v0.38.0-beta.1), for what this package does: reading tooth_lock.json and
// deriving the workspace file (the install/uninstall pipeline is the
// engine's).

const (
	lib   = "example.com/lib"
	app   = "example.com/app"
	other = "example.com/other"
)

// tooth is WorkspaceHarness.Tooth: a tooth.json of format 3 with a self
// asset placing file.dll at dest, for the label (default "") and "lua".
func tooth(id, version, dest, deps, label string) string {
	return `{"format_version": 3, "format_uuid": "` + uuid3 + `", "tooth": "` + id + `",
	  "version": "` + version + `", "variants": [
	    { "label": "` + label + `", "platform": "win-x64", "dependencies": ` + deps + `,
	      "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "file.dll", "dest": "` + dest + `" } ] } ] },
	    { "label": "lua", "platform": "win-x64",
	      "assets": [ { "type": "self", "placements": [ { "type": "file", "src": "file.dll", "dest": "` + dest + `" } ] } ] } ] }`
}

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	v, err := doc.ParseJSON([]byte(s), "test")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func member(t *testing.T, manifest string, variant string, locked bool, files ...string) ToothLockMember {
	return ToothLockMember{Manifest: mustJSON(t, manifest), Variant: variant, Locked: locked, Files: files}
}

func identities(lf *pkgmodel.LockFile) []string {
	var ids []string
	for _, p := range lf.Packages {
		ids = append(ids, p.Ref)
	}
	return ids
}

// app (explicitly installed, depending on lib) before lib, as lip v0.x may
// have listed them, with their files in the workspace.
func installedLibAndApp(t *testing.T, root string) *ToothLock {
	outside := "/elsewhere/x.dll"
	if runtime.GOOS == "windows" {
		outside = `C:\elsewhere\x.dll`
	}
	return &ToothLock{Members: []ToothLockMember{
		member(t, tooth(app, "1.0.0", "app/app.dll", `{ "`+lib+`": ">=1.0.0" }`, ""), "", true,
			filepath.Join(root, "app", "app.dll")),
		member(t, tooth(lib, "1.0.0", "lib/lib.dll", `{}`, ""), "", false,
			"lib/lib.dll", "../x.dll", outside, ""),
	}}
}

func commits(repo, tag string) (string, error) {
	return strings.ToUpper(strings.ReplaceAll(repo, ".", "")) + "@" + tag, nil
}

func TestDeriveWorkspaceInstallationOrderAndFiles(t *testing.T) {
	root := t.TempDir()
	lf, err := DeriveWorkspace(installedLibAndApp(t, root), root, "server", "win-x64", commits)
	if err != nil {
		t.Fatal(err)
	}
	if lf.Format != 4 || lf.Platform != "server" || lf.Runtime != "win-x64" {
		t.Errorf("lock file = %+v", lf)
	}
	eq(t, "packages", identities(lf), []string{lib, app})
	p := lf.Packages
	if p[0].Explicit || !p[1].Explicit || p[0].Commit != "examplecom/lib@v1.0.0" || p[1].Commit != "examplecom/app@v1.0.0" {
		t.Errorf("packages = %+v", p)
	}
	eq(t, "lib files", p[0].Files, []string{"lib/lib.dll"})
	eq(t, "app files", p[1].Files, []string{"app/app.dll"})
	m := p[0].Manifest.(*doc.Object)
	if f, _ := m.Get("format"); string(doc.MarshalJSONCompact(f)) != "4" {
		t.Errorf("format = %v", f)
	}
	if id, _ := m.Get("id"); id != lib {
		t.Errorf("id = %v", id)
	}
	if v, _ := m.Get("version"); v != "1.0.0" {
		t.Errorf("version = %v", v)
	}
	variants, _ := m.Get("variants")
	entries, _ := variants.(*doc.Object).Get("")
	e := entries.([]any)[0].(*doc.Object)
	if pl, _ := e.Get("platform"); pl != "server" {
		t.Errorf("platform = %v", pl)
	}
}

func TestDeriveWorkspaceWithoutCommitOfRecordsNoCommit(t *testing.T) {
	root := t.TempDir()
	lf, err := DeriveWorkspace(installedLibAndApp(t, root), root, "server", "win-x64", nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packages", identities(lf), []string{lib, app})
	for _, p := range lf.Packages {
		if p.Commit != "" {
			t.Errorf("commit = %q", p.Commit)
		}
	}
}

func TestDeriveWorkspaceCommitOfTagCannotBeObtainedFails(t *testing.T) {
	root := t.TempDir()
	tl := installedLibAndApp(t, root)
	tl.Members = append(tl.Members, member(t, tooth("example.com/gone", "1.0.0", "gone.dll", `{}`, ""), "", true, "gone.dll"))
	_, err := DeriveWorkspace(tl, root, "server", "win-x64", func(repo, tag string) (string, error) {
		if repo == "example.com/gone" {
			return "", errors.New("no such tag")
		}
		return "abc", nil
	})
	if err == nil || !strings.Contains(err.Error(), "tooth_lock.json") {
		t.Errorf("err = %v", err)
	}
	_, err = DeriveWorkspace(tl, root, "server", "win-x64", func(string, string) (string, error) { return "", nil })
	if err == nil {
		t.Error("an empty commit is no failure")
	}
}

func TestDeriveWorkspaceMemberWithoutVariantForEnvironmentIsLeftOut(t *testing.T) {
	root := t.TempDir()
	linux := strings.Replace(tooth("example.com/linux", "1.0.0", "linux.so", `{}`, ""), `"platform": "win-x64"`, `"platform": "linux-x64"`, 1)
	tl := &ToothLock{Members: []ToothLockMember{
		member(t, tooth(lib, "1.0.0", "lib/lib.dll", `{}`, ""), "", true, "lib/lib.dll"),
		member(t, tooth(lib, "1.0.0", "lib/lua.dll", `{}`, ""), "nope", true, "lib/lua.dll"),
		member(t, linux, "", true, "linux.so"),
		member(t, tooth(lib, "1.0.0", "x", `{}`, ""), "Bad", true),
		member(t, `{"format_version": 3}`, "", true),
	}}
	lf, err := DeriveWorkspace(tl, root, "server", "win-x64", nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packages", identities(lf), []string{lib})
}

func TestDeriveWorkspaceCycleOfDependenciesIsInAscendingOrderOfIdentities(t *testing.T) {
	var tl ToothLock
	for _, p := range [][2]string{{"example.com/c", "example.com/b"}, {"example.com/b", "example.com/a"}, {"example.com/a", "example.com/b"}} {
		tl.Members = append(tl.Members, member(t, tooth(p[0], "1.0.0", "x.dll", `{ "`+p[1]+`": "*" }`, ""), "", true))
	}
	lf, err := DeriveWorkspace(&tl, t.TempDir(), "server", "win-x64", nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packages", identities(lf), []string{"example.com/a", "example.com/b", "example.com/c"})
}

func TestDeriveWorkspaceVariantIsInRefAndIdentity(t *testing.T) {
	root := t.TempDir()
	tl := &ToothLock{Members: []ToothLockMember{
		member(t, tooth(lib, "1.0.0", "x.dll", `{}`, "lua"), "lua", false, filepath.Join(root, "x.dll")),
	}}
	lf, err := DeriveWorkspace(tl, root, "server", "win-x64", func(string, string) (string, error) {
		return strings.Repeat("A", 40), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Packages) != 1 {
		t.Fatalf("packages = %+v", lf.Packages)
	}
	p := lf.Packages[0]
	if p.Ref != lib+"#lua" || p.Variant != "lua" || p.Commit != strings.Repeat("a", 40) {
		t.Errorf("package = %+v", p)
	}
	eq(t, "files", p.Files, []string{"x.dll"})
}

// On the client platform, the variant "" of a tooth.json with a "client"
// label is that label, and a dependency on X#client orders after X#client.
func TestDeriveWorkspaceClientPlatform(t *testing.T) {
	client := `{"format_version": 3, "format_uuid": "` + uuid3 + `", "tooth": "example.com/ll",
	  "version": "1.0.0", "variants": [
	    { "platform": "win-x64", "dependencies": { "example.com/bds": "1.*" } },
	    { "label": "client", "platform": "win-x64" } ] }`
	tl := &ToothLock{Members: []ToothLockMember{
		member(t, tooth("example.com/a", "1.0.0", "a.dll", `{ "example.com/ll#client": "1.*" }`, ""), "", true),
		member(t, client, "client", true),
	}}
	lf, err := DeriveWorkspace(tl, t.TempDir(), "client", "win-x64", nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packages", identities(lf), []string{"example.com/ll#client", "example.com/a"})
	m := lf.Packages[0].Manifest.(*doc.Object)
	variants, _ := m.Get("variants")
	eq(t, "variants", variants.(*doc.Object).Keys(), []string{"", "client"})
}

func TestReadToothLockNotReadBySection3IsIgnored(t *testing.T) {
	for _, in := range []string{
		"not json",
		`{ "format_version": 2, "packages": [] }`,
		`{ "format_version": 3 }`,
		`{ "format_version": 3, "packages": [ { "variant": "", "locked": true, "files": [] } ] }`,
		`{ "format_version": 3, "packages": [ { "manifest": { "format_version": 4 }, "variant": "", "locked": true, "files": [] } ] }`,
		`{ "format_version": 3, "packages": [ { "manifest": { "format_version": 3 }, "variant": "", "locked": "yes", "files": [] } ] }`,
		`{ "format_version": 3, "packages": [ { "manifest": { "format_version": 3 }, "variant": "", "locked": true, "files": [1] } ] }`,
		`{ "format_version": 3, "packages": [ { "manifest": { "format_version": 3 }, "variant": 1, "locked": true, "files": [] } ] }`,
		`{ "format_version": 3, "packages": [ 1 ] }`,
		`{ "format_version": 3, "packages": [], "packages": [] }`,
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ToothLockFileName), []byte(in), 0o644); err != nil {
			t.Fatal(err)
		}
		if tl, ok := ReadToothLock(root); ok {
			t.Errorf("ReadToothLock(%s) = %+v", in, tl)
		}
	}
	if _, ok := ReadToothLock(t.TempDir()); ok {
		t.Error("a missing tooth_lock.json is read")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ToothLockFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadToothLock(dir); ok {
		t.Error("a directory tooth_lock.json is read")
	}
}

func TestReadToothLockReadsMembers(t *testing.T) {
	root := t.TempDir()
	in := `{ "format_version": 3.0, "format_uuid": "` + uuid3 + `", "packages": [
	  { "manifest": ` + tooth(lib, "1.0.0", "lib/lib.dll", `{}`, "") + `, "variant": "", "locked": true,
	    "files": ["lib/lib.dll"] } ] }`
	if err := os.WriteFile(filepath.Join(root, ToothLockFileName), []byte("\xef\xbb\xbf"+in), 0o644); err != nil {
		t.Fatal(err)
	}
	tl, ok := ReadToothLock(root)
	if !ok || len(tl.Members) != 1 || !tl.Members[0].Locked || !slices.Equal(tl.Members[0].Files, []string{"lib/lib.dll"}) {
		t.Fatalf("ReadToothLock = %+v, %v", tl, ok)
	}
	lf, err := DeriveWorkspace(tl, root, "server", "win-x64", nil)
	if err != nil || len(lf.Packages) != 1 {
		t.Fatalf("DeriveWorkspace = %+v, %v", lf, err)
	}
}

func TestRelativePath(t *testing.T) {
	root := t.TempDir()
	for in, want := range map[string]string{
		"a/b.dll":                         "a/b.dll",
		filepath.Join(root, "c", "d.txt"): "c/d.txt",
		"./e/../f.txt":                    "f.txt",
		"g/":                              "g/",
		"":                                "",
		".":                               "",
		"..":                              "",
		"../x":                            "",
		filepath.Dir(root):                "",
	} {
		got, ok := relativePath(root, in)
		if (want == "") == ok || got != want {
			t.Errorf("relativePath(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if runtime.GOOS == "windows" {
		upper := strings.ToUpper(root[:1]) + root[1:]
		if got, ok := relativePath(strings.ToLower(root[:1])+root[1:], filepath.Join(upper, "x.dll")); !ok || got != "x.dll" {
			t.Errorf("drive letter case: %q, %v", got, ok)
		}
	}
}
