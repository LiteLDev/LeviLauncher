package pkgfile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Ported from tests/Lip.Core.Tests/Mcmod/Lock/LockStoreTests.cs (v0.38.0-beta.1).

var commit40 = strings.Repeat("c", 40)

const toothLockJSON = `{ "format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",` +
	` "packages": [] }`

func manifestJSON(id, version, variant, entry string) string {
	return `{"format":4, "id":"` + id + `","version":"` + version + `","variants":{"` + variant + `":[` + entry + `]} }`
}

const serverEntry = `{"platform":"server","runtime":"win-x64"}`

func manifest(t *testing.T, id, version, variant, entry string) *doc.Object {
	t.Helper()
	v, err := doc.ParseJSON([]byte(manifestJSON(id, version, variant, entry)), "x")
	if err != nil {
		t.Fatal(err)
	}
	return v.(*doc.Object)
}

func samplePackage(t *testing.T, id, source, commit, variant, entry string) pkgmodel.LockedPackage {
	return pkgmodel.LockedPackage{
		Manifest: manifest(t, id, "1.0.0", variant, entry),
		Variant:  variant,
		Explicit: true,
		Ref:      source,
		Commit:   commit,
		Files:    []string{"mods/" + id[strings.LastIndex(id, "/")+1:] + "/a.dll"},
	}
}

func gitPackage(t *testing.T, id string) pkgmodel.LockedPackage {
	return samplePackage(t, id, id, commit40, "", serverEntry)
}

func lockOf(packages ...pkgmodel.LockedPackage) *pkgmodel.LockFile {
	return &pkgmodel.LockFile{Format: 4, Platform: "server", Runtime: "win-x64", Packages: packages}
}

func lipPath(root string) string { return filepath.Join(root, ".lip", "workspace.yaml") }

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mustLoad(t *testing.T, root string) *pkgmodel.LockFile {
	t.Helper()
	lf, ok, err := LoadWorkspace(root)
	if err != nil || !ok {
		t.Fatalf("LoadWorkspace: %v %v", ok, err)
	}
	return lf
}

func TestLoadNoWorkspaceFileReturnsNothing(t *testing.T) {
	root := t.TempDir()
	lf, ok, err := LoadWorkspace(root)
	if lf != nil || ok || err != nil || exists(filepath.Join(root, ".lip")) {
		t.Fatalf("%v %v %v", lf, ok, err)
	}
	// Whatever other files the workspace has.
	for _, rel := range []string{".lip/lock.json", ".lip/workspace.json", "tooth_lock.json"} {
		writeFile(t, filepath.Join(root, rel), toothLockJSON)
		if _, ok, err := LoadWorkspace(root); ok || err != nil {
			t.Fatalf("%s: %v %v", rel, ok, err)
		}
		if readFile(t, filepath.Join(root, rel)) != toothLockJSON {
			t.Fatalf("%s changed", rel)
		}
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	root := t.TempDir()
	original := lockOf(
		samplePackage(t, "github.com/a/zip", "https://example.com/z.zip", "", "",
			`{"platform":"server","runtime":"win-x64","assets":[{"type":"self"},`+
				`{"type":"mcmod","urls":["https://example.com/m.zip"]}]}`),
		samplePackage(t, "github.com/a/git", "github.com/a/git#lua/client@1.0.0", commit40, "lua/client", serverEntry),
		samplePackage(t, "github.com/a/file", "file:///C:/abs/x.zip", "", "", serverEntry),
		samplePackage(t, "github.com/a/derived", "github.com/a/derived", commit40, "",
			`{"platform":"server","runtime":"win-x64","preserve":["**/config.json"],`+
				`"purge":["data","data/"],"hooks":{"post_install":["echo"]}}`))
	if err := SaveWorkspace(root, original); err != nil {
		t.Fatal(err)
	}
	loaded := mustLoad(t, root)
	if loaded.Platform != "server" || loaded.Runtime != "win-x64" || len(loaded.Packages) != 4 {
		t.Fatalf("%+v", loaded)
	}
	for i, want := range original.Packages {
		got := loaded.Packages[i]
		if !doc.Equal(want.Manifest, got.Manifest) || got.Variant != want.Variant || got.Explicit != want.Explicit ||
			got.Ref != want.Ref || got.Commit != want.Commit || !reflect.DeepEqual(got.Files, want.Files) {
			t.Errorf("package %d: %+v, want %+v", i, got, want)
		}
	}
	if d := describePackage(loaded.Packages[1].Manifest, loaded.Packages[1].Variant); d != "github.com/a/git#lua/client 1.0.0" {
		t.Errorf("%q", d)
	}
	// .lip is created hidden, and holds nothing but the workspace file.
	if hidden, err := txn.OS.Hidden(filepath.Join(root, ".lip")); err != nil || !hidden {
		t.Errorf(".lip hidden: %v %v", hidden, err)
	}
	ents, _ := os.ReadDir(filepath.Join(root, ".lip"))
	if len(ents) != 1 || ents[0].Name() != "workspace.yaml" {
		t.Errorf(".lip holds %v", ents)
	}
}

func TestSaveWritesExactlyTheMembersOfTheSchemaInOrder(t *testing.T) {
	root := t.TempDir()
	lf := lockOf(samplePackage(t, "github.com/a/zip", "https://example.com/z.zip", "", "", serverEntry),
		gitPackage(t, "github.com/LiteLDev/A"))
	if err := SaveWorkspace(root, lf); err != nil {
		t.Fatal(err)
	}
	want := "format: 4\nplatform: server\nruntime: win-x64\npackages:\n" +
		"- manifest:\n    format: 4\n    id: github.com/a/zip\n    version: 1.0.0\n    variants:\n      \"\":\n" +
		"      - platform: server\n        runtime: win-x64\n  variant: \"\"\n  explicit: true\n" +
		"  ref: https://example.com/z.zip\n  files:\n  - mods/zip/a.dll\n" +
		"- manifest:\n    format: 4\n    id: github.com/LiteLDev/A\n    version: 1.0.0\n    variants:\n      \"\":\n" +
		"      - platform: server\n        runtime: win-x64\n  variant: \"\"\n  explicit: true\n" +
		"  ref: github.com/LiteLDev/A\n  commit: " + commit40 + "\n  files:\n  - mods/A/a.dll\n"
	if got := readFile(t, lipPath(root)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSaveExistingLockReplacesItAsAWhole(t *testing.T) {
	root := t.TempDir()
	if err := SaveWorkspace(root, lockOf(gitPackage(t, "github.com/LiteLDev/A"), gitPackage(t, "github.com/LiteLDev/B"))); err != nil {
		t.Fatal(err)
	}
	if err := SaveWorkspace(root, lockOf(gitPackage(t, "github.com/LiteLDev/C"))); err != nil {
		t.Fatal(err)
	}
	lf := mustLoad(t, root)
	if len(lf.Packages) != 1 || stringMember(lf.Packages[0].Manifest, "id") != "github.com/LiteLDev/C" {
		t.Fatalf("%+v", lf.Packages)
	}
	ents, _ := os.ReadDir(root)
	if len(ents) != 1 {
		t.Fatalf("root holds %v", ents)
	}
}

func TestStageCommitAndDiscard(t *testing.T) {
	root := t.TempDir()
	other := lockOf()
	other.Runtime = "linux-x64"
	if err := SaveWorkspace(root, other); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, ".lip", "workspace.yaml.tmp")
	if err := StageWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if !exists(staged) || mustLoad(t, root).Runtime != "linux-x64" {
		t.Fatal("staging changed the workspace file")
	}
	if err := CommitStaged(root); err != nil {
		t.Fatal(err)
	}
	if exists(staged) || mustLoad(t, root).Runtime != "win-x64" {
		t.Fatal("CommitStaged did not move the staged file")
	}

	if err := SaveWorkspace(root, other); err != nil {
		t.Fatal(err)
	}
	if err := StageWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if err := DiscardStaged(root); err != nil {
		t.Fatal(err)
	}
	if err := CommitStaged(root); err != nil {
		t.Fatal(err)
	}
	if exists(staged) || mustLoad(t, root).Runtime != "linux-x64" {
		t.Fatal("DiscardStaged did not discard")
	}
	if err := DiscardStaged(root); err != nil {
		t.Fatal(err)
	}
}

func TestCommitStagedToothLock(t *testing.T) {
	// The first workspace file beside a tooth_lock.json §3 reads replaces it.
	root := t.TempDir()
	toothLock := filepath.Join(root, ToothLockFile)
	writeFile(t, toothLock, toothLockJSON)
	if err := StageWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if err := CommitStaged(root); err != nil {
		t.Fatal(err)
	}
	if exists(toothLock) {
		t.Fatal("tooth_lock.json kept")
	}
	mustLoad(t, root)

	// It is kept when there already is a workspace file.
	writeFile(t, toothLock, toothLockJSON)
	if err := SaveWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if readFile(t, toothLock) != toothLockJSON {
		t.Fatal("tooth_lock.json changed")
	}

	// And when §3 does not read it.
	for _, text := range []string{"not json", `{ "format_version": 2, "packages": [] }`,
		`{ "format_version": 3, "packages": [1] }`, `{ "format_version": 3 }`,
		`{ "format_version": 3, "packages": [{"manifest":{"format_version":4},"variant":"","locked":true,"files":[]}] }`,
		`{ "format_version": 3, "packages": [], "packages": [] }`} {
		root := t.TempDir()
		toothLock := filepath.Join(root, ToothLockFile)
		writeFile(t, toothLock, text)
		if err := SaveWorkspace(root, lockOf()); err != nil {
			t.Fatal(err)
		}
		if readFile(t, toothLock) != text {
			t.Fatalf("%s: tooth_lock.json changed", text)
		}
	}
	root = t.TempDir()
	toothLock = filepath.Join(root, ToothLockFile)
	writeFile(t, toothLock, `{"format_version":3,"packages":[{"manifest":{"format_version":2},"variant":"x",`+
		`"locked":false,"files":["a"]}]}`)
	if err := SaveWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if exists(toothLock) {
		t.Fatal("a tooth_lock.json with members was kept")
	}
}

func TestRecoverHooksCommitTheStagedFileAndDeleteToothLock(t *testing.T) {
	root := t.TempDir()
	toothLock := filepath.Join(root, ToothLockFile)
	writeFile(t, toothLock, toothLockJSON)
	if err := txn.EnsureLipDir(root); err != nil {
		t.Fatal(err)
	}
	tx, err := txn.Begin(root, txn.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.WriteFile("mods/a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	if err := StageWorkspace(root, lockOf()); err != nil {
		t.Fatal(err)
	}
	if err := tx.MarkCommitted(); err != nil {
		t.Fatal(err)
	}
	tx.Close() // killed before the staged file was moved into place
	res, err := txn.RecoverWith(root, RecoverHooks(root))
	if err != nil || res == nil || !res.Committed {
		t.Fatalf("%+v %v", res, err)
	}
	if exists(toothLock) || exists(filepath.Join(root, ".lip", "workspace.yaml.tmp")) {
		t.Fatal("recovery did not finish CommitStaged")
	}
	mustLoad(t, root)
}

func TestLoadHandWrittenWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, lipPath(root), `{
  "format": 4, "platform": "server", "runtime": "win-x64",
  "packages": [{
    "files": ["a.txt"], "ref": "github.com/a/b@branch:main", "commit": "`+commit40+`",
    "explicit": false, "variant": "",
    "manifest": `+manifestJSON("github.com/a/b", "1.2.3", "", serverEntry)+`
  }]
}`)
	p := mustLoad(t, root).Packages[0]
	if p.Ref != "github.com/a/b@branch:main" || p.Commit != commit40 || p.Explicit ||
		stringMember(p.Manifest, "version") != "1.2.3" {
		t.Fatalf("%+v", p)
	}
	writeFile(t, lipPath(root), "format: 4\nplatform: server\nruntime: win-x64\npackages:\n  - manifest:\n"+
		"      format: 4\n      id: github.com/a/b\n      version: 1.2.3\n      variants:\n        \"\":\n"+
		"          - platform: server\n            runtime: win-x64\n    variant: \"\"\n    explicit: true\n"+
		"    ref: github.com/a/b\n    commit: "+commit40+"\n    files: [a.txt, \"1.0\"]\n")
	p = mustLoad(t, root).Packages[0]
	if !reflect.DeepEqual(p.Files, []string{"a.txt", "1.0"}) || !p.Explicit {
		t.Fatalf("%+v", p)
	}
}

func TestLoadInvalidWorkspaceFiles(t *testing.T) {
	const invalid = "' is invalid, so lip cannot tell which packages are installed. If you edited it by hand," +
		" undo the change; if you have a copy of the file, restore it."
	const other = "' was written by a different version of lip, so this lip cannot tell which packages are installed."
	head := `"format": 4, "platform": "server", "runtime": "win-x64"`
	m := manifestJSON("github.com/a/b", "1.0.0", "", serverEntry)
	pkg := `"manifest": ` + m + `, "variant": "", "explicit": true, "ref": "github.com/a/b", "commit": "` +
		commit40 + `", "files": []`
	w := func(p string) string { return `{ ` + head + `, "packages": [{ ` + p + ` }] }` }
	for _, c := range []struct{ text, outer, cause string }{
		{`{ "format_version": 3, "packages": [] }`, invalid,
			`"format" is missing or not a number, but this version of lip reads only 4. If another version of lip` +
				` wrote the file, use that version.`},
		{`{ "format": 5, "packages": [] }`, other, `"format" is 5, but this version of lip reads only 4.`},
		{"format: '4'\nplatform: server\nruntime: win-x64\npackages: []\n", invalid, `"format" is missing`},
		{"", invalid, "The file holds no YAML document: it is empty or holds only comments."},
		{"[]", invalid, "The file is not a YAML mapping of keys to values."},
		{`{ ` + head + `, "packages": null }`, invalid, "The property or field 'packages' on type" +
			" 'Lip.Core.Mcmod.LockFile' doesn't allow setting null values. Consider updating its nullability" +
			" annotation. Path: $.packages | LineNumber: 0 | BytePositionInLine: 67."},
		{`{ ` + head + `, "packages": [null] }`, invalid, "packages[0] is null, but it must be an object."},
		{`{ "format": 4, "platform": 5, "runtime": "win-x64", "packages": [] }`, invalid, "The JSON value could" +
			" not be converted to System.String. Path: $.platform | LineNumber: 0 | BytePositionInLine: 24."},
		{`{ "format": 4, "packages": [] }`, invalid, "JSON deserialization for type 'Lip.Core.Mcmod.LockFile'" +
			" was missing required properties including: 'platform', 'runtime'."},
		{`{ "format": 4, "platform": "universal", "runtime": "win-x64", "packages": [] }`, invalid,
			`"platform" is "universal", but it must be "server" or "client".`},
		{`{ "format": 4, "platform": "server", "runtime": "", "packages": [] }`, invalid,
			`"runtime" is missing or empty, but it must be a runtime identifier such as "win-x64".`},
		{`{ "format": 4, "platform": "server", "runtime": "x", "packages": [1] }`, invalid, "The JSON value" +
			" could not be converted to Lip.Core.Mcmod.LockedPackage. Path: $.packages[0] | LineNumber: 0 |" +
			" BytePositionInLine: 59."},
		{w(strings.Replace(pkg, `"explicit": true,`, "", 1)), invalid, "JSON deserialization for type" +
			" 'Lip.Core.Mcmod.LockedPackage' was missing required properties including: 'explicit'."},
		{w(strings.Replace(pkg, `"ref"`, `"Ref"`, 1)), invalid, "including: 'ref'."},
		{w(strings.Replace(pkg, `"explicit": true`, `"explicit": "true"`, 1)), invalid, "The JSON value could" +
			" not be converted to System.Boolean. Path: $.packages[0].explicit | LineNumber: 0 |" +
			" BytePositionInLine: 219."},
		{w(strings.Replace(pkg, `"files": []`, `"files": [1]`, 1)), invalid, "Path: $.packages[0].files[0] |" +
			" LineNumber: 0 | BytePositionInLine: 303."},
		{w(strings.Replace(pkg, m, "[]", 1)), invalid,
			"packages[0].manifest is array, but it must be the package file of the package, a mapping."},
		{w(strings.Replace(pkg, `"1.0.0"`, `"1.0"`, 1)), invalid, "packages[0].manifest, the package file of" +
			" github.com/a/b 1.0, is not a valid package file."},
		{w(strings.Replace(pkg, `"variant": ""`, `"variant": "A"`, 1)), invalid,
			`packages[0].variant is "A", which is not a valid variant name.`},
		{w(strings.Replace(pkg, `"files": []`, `"files": ["a", null]`, 1)), invalid,
			"packages[0].files[1] is null, but it must be a workspace-relative path."},
		{w(strings.Replace(pkg, commit40, strings.ToUpper(commit40), 1)), invalid,
			`packages[0].commit is "` + strings.ToUpper(commit40) + `", but it must be 40 lowercase hexadecimal digits.`},
		{w(strings.Replace(pkg, `"`+commit40+`"`, "null", 1)), invalid, "packages[0].commit is null, but it must" +
			" be a string of 40 lowercase hexadecimal digits, or be left out."},
	} {
		root := t.TempDir()
		writeFile(t, lipPath(root), c.text)
		_, _, err := LoadWorkspace(root)
		var le *LockFormatError
		if !errors.As(err, &le) {
			t.Errorf("%s: %v", c.text, err)
			continue
		}
		if le.Message != "The workspace file '"+lipPath(root)+c.outer || le.OtherFormat != (c.outer == other) {
			t.Errorf("%s: %q", c.text, le.Message)
		}
		if le.Err == nil || !strings.Contains(le.Err.Error(), c.cause) {
			t.Errorf("%s: cause %v, want %q", c.text, le.Err, c.cause)
		}
		if readFile(t, lipPath(root)) != c.text {
			t.Errorf("%s: the file changed", c.text)
		}
	}
}

func TestStageInvalidLockWritesNothing(t *testing.T) {
	mutate := []func(*pkgmodel.LockFile){
		func(l *pkgmodel.LockFile) { l.Format = 3 },
		func(l *pkgmodel.LockFile) { l.Platform = "Server" },
		func(l *pkgmodel.LockFile) { l.Platform = "" },
		func(l *pkgmodel.LockFile) { l.Runtime = "" },
		func(l *pkgmodel.LockFile) { l.Packages[0].Manifest = nil },
		func(l *pkgmodel.LockFile) {
			l.Packages[0].Manifest = manifest(t, "github.com/a/b", "v1.0.0", "", serverEntry)
		},
		func(l *pkgmodel.LockFile) { l.Packages[0].Variant = "Client" },
		func(l *pkgmodel.LockFile) { l.Packages[0].Commit = "ABC" },
		func(l *pkgmodel.LockFile) { l.Packages[0].Commit = strings.ToUpper(commit40) },
	}
	for i, m := range mutate {
		root := t.TempDir()
		lf := lockOf(gitPackage(t, "github.com/LiteLDev/A"))
		m(lf)
		err := SaveWorkspace(root, lf)
		if !errors.Is(err, ErrInvalidWorkspace) || err.Error() != "lip was about to write an invalid workspace"+
			" file, and did not write it. This is a bug in lip. (Parameter 'lockFile')" {
			t.Errorf("%d: %v", i, err)
		}
		var le *LockFormatError
		if !errors.As(err, &le) {
			t.Errorf("%d: no cause", i)
		}
		if exists(filepath.Join(root, ".lip")) {
			t.Errorf("%d: .lip created", i)
		}
	}
}

func TestLoadMovedWorkspaceKeepsItsLock(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "a")
	if err := SaveWorkspace(root, lockOf(gitPackage(t, "github.com/LiteLDev/A"))); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(base, "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if lf := mustLoad(t, moved); stringMember(lf.Packages[0].Manifest, "id") != "github.com/LiteLDev/A" {
		t.Fatalf("%+v", lf)
	}
}
