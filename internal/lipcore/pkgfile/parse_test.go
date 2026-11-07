package pkgfile

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// Ported from tests/Lip.Core.Tests/Mcmod/Parsing/PackageFileParserTests.cs
// (v0.38.0-beta.1). The documents are written as JSON, which is YAML too.

const validDoc = `{"format":4,"id":"example.com/owner/tool","version":"1.2.3",` +
	`"variants":{"":[{"platform":"server","runtime":"win-x64"}]}}`

func withEntry(entry string) string {
	return `{"format":4,"id":"example.com/owner/tool","version":"1.2.3","variants":{"":[` + entry + `]}}`
}

func withMembers(members string) string {
	return withEntry(`{"platform":"server","runtime":"win-x64",` + members + `}`)
}

// setRoot replaces (or with v == "" removes) a member of validDoc.
func setRoot(key, v string) string {
	members := []struct{ k, v string }{{"format", "4"}, {"id", `"example.com/owner/tool"`},
		{"version", `"1.2.3"`}, {"variants", `{"":[{"platform":"server","runtime":"win-x64"}]}`}}
	var parts []string
	found := false
	for _, m := range members {
		if m.k == key {
			found = true
			if v == "" {
				continue
			}
			m.v = v
		}
		parts = append(parts, `"`+m.k+`":`+m.v)
	}
	if !found {
		parts = append(parts, `"`+key+`":`+v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func mustParse(t *testing.T, text string) *pkgmodel.Document {
	t.Helper()
	d, err := ParseYAML([]byte(text))
	if err != nil {
		t.Fatalf("ParseYAML(%s): %v", text, err)
	}
	return d
}

func singleEntry(t *testing.T, text string) pkgmodel.Entry {
	t.Helper()
	d := mustParse(t, text)
	entries, ok := d.EntriesOf("")
	if !ok || len(entries) != 1 {
		t.Fatalf("want a single entry of variant \"\", got %+v", d.Variants)
	}
	return entries[0]
}

// assertInvalid checks that the document is rejected with a message naming
// path, and returns the error.
func assertInvalid(t *testing.T, text, path string) *FormatError {
	t.Helper()
	_, err := ParseYAML([]byte(text))
	var fe *FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("ParseYAML(%s) = %v, want a *FormatError", text, err)
	}
	if !strings.Contains(fe.Message, "at "+path) {
		t.Fatalf("ParseYAML(%s): %q does not name %s", text, fe.Message, path)
	}
	return fe
}

func TestParseMinimalDocumentReadsMembers(t *testing.T) {
	d := mustParse(t, validDoc)
	if d.ID != "example.com/owner/tool" || d.Version != "1.2.3" || d.Format != 4 || d.Info != nil {
		t.Fatalf("got %+v", d)
	}
	if len(d.Variants) != 1 || d.Variants[0].Name != "" {
		t.Fatalf("variants %+v", d.Variants)
	}
	e := d.Variants[0].Entries[0]
	if e.Platform != "server" || e.Runtime != "win-x64" || len(e.Deps) != 0 || len(e.Assets) != 0 ||
		len(e.Preserve) != 0 || len(e.Purge) != 0 || e.Derived || e.McmodAsset() != -1 {
		t.Fatalf("entry %+v", e)
	}
	if e.Deps == nil || e.Assets == nil || e.Hooks.PreInstall == nil {
		t.Fatal("absent members must read as empty, not nil")
	}
	raw, ok := e.Raw.(*doc.Object)
	if !ok {
		t.Fatalf("entry Raw is %T", e.Raw)
	}
	if v, _ := raw.Get("platform"); v != "server" {
		t.Fatalf("entry Raw platform %v", v)
	}
	if _, ok := d.Raw.(*doc.Object); !ok {
		t.Fatalf("document Raw is %T", d.Raw)
	}
}

func TestParseBlockYAMLReadsMembers(t *testing.T) {
	d := mustParse(t, "# lip.yaml\nformat: 4\nid: example.com/owner/tool\nversion: 1.0.0\ninfo:\n"+
		"  name: 'true'\n  tags: ['null', \"~\"]\nvariants:\n  \"\":\n    - platform: server\n"+
		"      runtime: win-x64\n      deps:\n        example.com/a: ^1.0.0\n      hooks:\n"+
		"        post_install:\n          - echo yes\n")
	if d.Version != "1.0.0" || d.Info.Name != "true" || !reflect.DeepEqual(d.Info.Tags, []string{"null", "~"}) {
		t.Fatalf("got %+v %+v", d, d.Info)
	}
	e := d.Variants[0].Entries[0]
	if !reflect.DeepEqual(e.Deps, []pkgmodel.Dep{{Key: "example.com/a", Range: "^1.0.0"}}) ||
		!reflect.DeepEqual(e.Hooks.PostInstall, []string{"echo yes"}) {
		t.Fatalf("entry %+v", e)
	}
}

func TestParsePlainScalarOfOtherTypeThrows(t *testing.T) {
	base := "format: 4\nid: example.com/owner/tool\nversion: 1.2.3\nvariants: { \"\": [ { platform: server, runtime: win-x64 } ] }"
	for _, c := range []struct{ line, path string }{
		{"version: 1.0", "$.version"}, {"version: true", "$.version"}, {"version: null", "$.version"},
		{"version:", "$.version"}, {"format: \"4\"", "$.format"}, {"format: 4.5", "$.format"},
	} {
		key := c.line[:strings.Index(c.line, ":")]
		var lines []string
		for _, l := range strings.Split(base, "\n") {
			if strings.HasPrefix(l, key+":") {
				l = c.line
			}
			lines = append(lines, l)
		}
		assertInvalid(t, strings.Join(lines, "\n"), c.path)
	}
}

func TestParseFullDocumentReadsEveryMember(t *testing.T) {
	text := withMembers(`"deps":{"example.com/a":"^1.0.0","example.com/b#lua":">=2.0.0 <3.0.0"},` +
		`"assets":[{"type":"self","place":[{"src":"bin/","dest":"tools/x/"}]},` +
		`{"type":"zip","urls":["https://a.example/x.zip","https://b.example/x.zip"],` +
		`"place":[{"src":"x.dll","dest":"plugins/x.dll"},{"src":"lib/*.dll","dest":""}]},` +
		`{"type":"mcmod","urls":["https://a.example/x.mcmod"]}],` +
		`"preserve":["config/**","data/"],"purge":["logs/"],` +
		`"hooks":{"pre_install":["a"],"post_install":["b","c"],"pre_uninstall":["d"],"post_uninstall":["e"]}`)
	text = strings.Replace(text, `"format":4`, `"format":4,"info":{"name":"Tool","description":"d","tags":["x"],`+
		`"logo":"https://a.example/a.png"}`, 1)
	d := mustParse(t, text)
	if !reflect.DeepEqual(*d.Info, pkgmodel.Info{Name: "Tool", Description: "d", Tags: []string{"x"},
		Logo: "https://a.example/a.png"}) {
		t.Fatalf("info %+v", d.Info)
	}
	e := d.Variants[0].Entries[0]
	wantDeps := []pkgmodel.Dep{{Key: "example.com/a", Range: "^1.0.0"}, {Key: "example.com/b#lua", Range: ">=2.0.0 <3.0.0"}}
	if !reflect.DeepEqual(e.Deps, wantDeps) {
		t.Fatalf("deps %+v", e.Deps)
	}
	wantAssets := []pkgmodel.Asset{
		{Type: pkgmodel.AssetSelf, URLs: []string{}, Place: []pkgmodel.Placement{{Src: "bin/", Dest: "tools/x/"}}},
		{Type: pkgmodel.AssetZip, URLs: []string{"https://a.example/x.zip", "https://b.example/x.zip"},
			Place: []pkgmodel.Placement{{Src: "x.dll", Dest: "plugins/x.dll"}, {Src: "lib/*.dll", Dest: ""}}},
		{Type: pkgmodel.AssetMcmod, URLs: []string{"https://a.example/x.mcmod"}, Place: []pkgmodel.Placement{}},
	}
	if !reflect.DeepEqual(e.Assets, wantAssets) {
		t.Fatalf("assets %+v", e.Assets)
	}
	if e.McmodAsset() != 2 {
		t.Fatalf("McmodAsset = %d", e.McmodAsset())
	}
	if !reflect.DeepEqual(e.Preserve, []string{"config/**", "data/"}) || !reflect.DeepEqual(e.Purge, []string{"logs/"}) {
		t.Fatalf("globs %v %v", e.Preserve, e.Purge)
	}
	wantHooks := pkgmodel.Hooks{PreInstall: []string{"a"}, PostInstall: []string{"b", "c"},
		PreUninstall: []string{"d"}, PostUninstall: []string{"e"}}
	if !reflect.DeepEqual(e.Hooks, wantHooks) {
		t.Fatalf("hooks %+v", e.Hooks)
	}
}

func TestParseRootRules(t *testing.T) {
	for _, key := range []string{"format", "id", "version", "variants"} {
		fe := assertInvalid(t, setRoot(key, ""), "$."+key)
		if !strings.HasSuffix(fe.Message, "is required.") && key != "format" {
			t.Errorf("%s: %q", key, fe.Message)
		}
	}
	for _, v := range []string{"3", "4.5", `"4"`, "null", "-1", "[]"} {
		fe := assertInvalid(t, setRoot("format", v), "$.format")
		if fe.Message != "Invalid package file at $.format: must be 4." || fe.Newer {
			t.Errorf("format %s: %+v", v, fe)
		}
	}
	for _, v := range []string{"4.0", "4e0", "40e-1"} {
		mustParse(t, setRoot("format", v))
	}
	_, err := ParseYAML([]byte(setRoot("format", "5")))
	var fe *FormatError
	if !errors.As(err, &fe) || !fe.Newer || fe.Message != `The package file has "format": 5, a format this lip`+
		` does not know (it knows format 4). The package needs a newer lip: update lip.` {
		t.Fatalf("format 5: %v", err)
	}
	// The keys of the unreleased tooth.json of format_version 4 are not read.
	assertInvalid(t, `{"format_version":4,"tooth":"example.com/owner/tool","version":"1.2.3",`+
		`"variants":{"":[{"platform":"server","runtime":"win-x64"}]}}`, "$.format")
	for _, text := range []string{"[]", `"lip"`, "null"} {
		fe := assertInvalid(t, text, "$")
		if !strings.Contains(fe.Message, "at $:") {
			t.Errorf("%s: %q", text, fe.Message)
		}
	}
}

func TestParseID(t *testing.T) {
	for _, id := range []string{"github.com/LiteLDev/LeviLamina", "example.com/a/v2", "example.com/a.b-c_d~e"} {
		if d := mustParse(t, setRoot("id", `"`+id+`"`)); d.ID != id {
			t.Errorf("id %q read as %q", id, d.ID)
		}
	}
	for _, id := range []string{"", "tool", "/example.com/a", "example.com/a/", "example.com//a", "example.com/a b",
		"example.com/../a", "https://example.com/a"} {
		fe := assertInvalid(t, setRoot("id", `"`+id+`"`), "$.id")
		if !strings.Contains(fe.Message, "'"+id+"' is not a valid Go module path") || fe.Err == nil {
			t.Errorf("id %q: %q", id, fe.Message)
		}
	}
	fe := assertInvalid(t, setRoot("id", "1"), "$.id")
	if fe.Message != "Invalid package file at $.id: must be a string, not a number." {
		t.Errorf("%q", fe.Message)
	}
}

func TestParseVersion(t *testing.T) {
	for _, v := range []string{"0.0.0", "1.0.0-rc.1", "1.0.0-rc.1+build.5"} {
		mustParse(t, setRoot("version", `"`+v+`"`))
	}
	for _, v := range []string{"v1.0.0", "1.0", "01.0.0", "1.0.0-01", " 1.0.0", ""} {
		fe := assertInvalid(t, setRoot("version", `"`+v+`"`), "$.version")
		if fe.Message != "Invalid package file at $.version: '"+v+"' is not a version." {
			t.Errorf("%q", fe.Message)
		}
	}
}

func TestParseVariants(t *testing.T) {
	for _, v := range []string{"lua", "a_1", "0", "engine/lua"} {
		d := mustParse(t, setRoot("variants", `{"":[{"platform":"server","runtime":"win-x64"}],"`+v+
			`":[{"platform":"client","runtime":"win-x64"}]}`))
		if len(d.Variants) != 2 || d.Variants[1].Name != v || d.Variants[1].Entries[0].Platform != "client" {
			t.Errorf("variant %q: %+v", v, d.Variants)
		}
	}
	for _, v := range []string{"Lua", "a-b", "a.b", "a/b/c", "/a", "a/", " "} {
		fe := assertInvalid(t, setRoot("variants", `{"`+v+`":[{"platform":"server","runtime":"win-x64"}]}`),
			`$.variants["`+v+`"]`)
		if !strings.HasSuffix(fe.Message, ": '"+v+"' is not a variant.") {
			t.Errorf("%q", fe.Message)
		}
	}
	if d := mustParse(t, setRoot("variants", "{}")); len(d.Variants) != 0 {
		t.Errorf("no variants: %+v", d.Variants)
	}
	for v, msg := range map[string]string{"[]": "must not be empty", "{}": "must be an array, not an object",
		"null": "must be an array, not null"} {
		fe := assertInvalid(t, setRoot("variants", `{"":`+v+`}`), `$.variants[""]`)
		if fe.Message != `Invalid package file at $.variants[""]: `+msg+"." {
			t.Errorf("%q", fe.Message)
		}
	}
	for _, v := range []string{"[]", `""`} {
		assertInvalid(t, setRoot("variants", v), "$.variants")
	}
}

func TestParsePlatformAndRuntime(t *testing.T) {
	for _, p := range []string{`""`, `"Server"`, `"any"`, `"win-x64"`, "null"} {
		fe := assertInvalid(t, withEntry(`{"platform":`+p+`,"runtime":"win-x64"}`), `$.variants[""][0].platform`)
		if !strings.HasSuffix(fe.Message, `platform: must be "server" or "client".`) {
			t.Errorf("%q", fe.Message)
		}
	}
	for _, key := range []string{"platform", "runtime"} {
		other := map[string]string{"platform": `"runtime":"win-x64"`, "runtime": `"platform":"server"`}[key]
		assertInvalid(t, withEntry(`{`+other+`}`), `$.variants[""][0].`+key)
	}
	for _, r := range []string{"win-x64", "linux-musl-arm64", "osx.13-arm64", "any"} {
		if e := singleEntry(t, withEntry(`{"platform":"server","runtime":"`+r+`"}`)); e.Runtime != r {
			t.Errorf("runtime %q", e.Runtime)
		}
	}
	for _, r := range []string{`""`, `"win-*"`, `"win x64"`, `"win--x64"`, `"-win"`, "1"} {
		assertInvalid(t, withEntry(`{"platform":"server","runtime":`+r+`}`), `$.variants[""][0].runtime`)
	}
	// The assets are read before the platform.
	fe := assertInvalid(t, withEntry(`{"platform":"bad","assets":[{"type":"bad"}]}`), `$.variants[""][0].assets[0].type`)
	_ = fe
}

func TestParseDuplicateEntries(t *testing.T) {
	fe := assertInvalid(t, setRoot("variants", `{"":[{"platform":"server","runtime":"win-x64"},`+
		`{"platform":"client","runtime":"win-x64"},{"platform":"server","runtime":"win-x64","hooks":{}}]}`),
		`$.variants[""][2]`)
	if fe.Message != `Invalid package file at $.variants[""][2]: has the platform and the runtime of $.variants[""][0].` {
		t.Errorf("%q", fe.Message)
	}
	d := mustParse(t, setRoot("variants", `{"":[{"platform":"server","runtime":"win-x64"},`+
		`{"platform":"client","runtime":"win-x64"},{"platform":"server","runtime":"linux-x64"}],`+
		`"lua":[{"platform":"server","runtime":"win-x64"}]}`))
	entries, _ := d.EntriesOf("")
	if got := d.SelectEntry("", "client", "win-x64"); got != &entries[1] {
		t.Errorf("SelectEntry = %+v", got)
	}
	if d.SelectEntry("", "client", "linux-x64") != nil || d.SelectEntry("js", "server", "win-x64") != nil {
		t.Error("SelectEntry found an entry that does not exist")
	}
}

func TestParseDependencies(t *testing.T) {
	for _, key := range []string{"example.com/a", "example.com/a/v2#lua", "example.com/a#engine/lua"} {
		e := singleEntry(t, withMembers(`"deps":{"`+key+`":"*"}`))
		if !reflect.DeepEqual(e.Deps, []pkgmodel.Dep{{Key: key, Range: "*"}}) {
			t.Errorf("deps %+v", e.Deps)
		}
	}
	for key, suffix := range map[string]string{
		"example.com/a#":            " (the variant after '#' is missing; ",
		"example.com/a#Lua":         " (the variant 'Lua' is not valid; ",
		"example.com/a#a/b/c":       " (the variant 'a/b/c' is not valid; ",
		"example.com/a@1.0.0":       "",
		"example.com/a@branch:main": "",
		"example.com/a#lua@1.0.0":   "",
		"https://example.com/a.zip": "",
		"":                          " (",
	} {
		fe := assertInvalid(t, withMembers(`"deps":{"`+key+`":"*"}`), `$.variants[""][0].deps["`+key+`"]`)
		want := `: '` + key + `' is not <package id> or <package id>#<variant>` + suffix
		if !strings.Contains(fe.Message, want) || (suffix == "" && !strings.HasSuffix(fe.Message, "#<variant>.")) {
			t.Errorf("key %q: %q", key, fe.Message)
		}
	}
	for _, r := range []string{"*", "1.2.3", "^1.2.0", "~1.2.0", "1.*", ">=1.0.0 <2.0.0", "1.0.0 || 3.0.0"} {
		if e := singleEntry(t, withMembers(`"deps":{"example.com/a":"`+r+`"}`)); e.Deps[0].Range != r {
			t.Errorf("range %q", e.Deps[0].Range)
		}
	}
	for _, r := range []string{`"abc"`, `">="`, `"1.0.0 -"`, "1", `[">=1.0.0"]`, "null"} {
		fe := assertInvalid(t, withMembers(`"deps":{"example.com/a":`+r+`}`), `$.variants[""][0].deps["example.com/a"]`)
		if strings.HasPrefix(r, `"`) && (fe.Err == nil || !strings.HasSuffix(fe.Message, "' is not a version range.")) {
			t.Errorf("range %s: %q", r, fe.Message)
		}
	}
	assertInvalid(t, withMembers(`"deps":["example.com/a"]`), `$.variants[""][0].deps`)
}

func TestParseAssets(t *testing.T) {
	for _, c := range []struct{ asset, member, reason string }{
		{`{"type":"tar","urls":["https://a.example/x.tar"]}`, ".type", `must be "self", "zip" or "mcmod"`},
		{`{"type":"Zip","urls":["https://a.example/x.zip"]}`, ".type", `must be "self", "zip" or "mcmod"`},
		{`{"urls":["https://a.example/x.zip"]}`, ".type", "is required"},
		{`{"type":"zip"}`, ".urls", "is required"},
		{`{"type":"zip","urls":[]}`, ".urls", "must not be empty"},
		{`{"type":"zip","urls":"https://a.example/x.zip"}`, ".urls", "must be an array, not a string"},
		{`{"type":"zip","urls":[1]}`, ".urls[0]", "must be a string, not a number"},
		{`{"type":"mcmod"}`, ".urls", "is required"},
		{`{"type":"mcmod","urls":[]}`, ".urls", "must not be empty"},
		{`{"type":"mcmod","urls":["https://a.example/x.mcmod"],"place":[]}`, ".place",
			`is not allowed for an asset of type "mcmod"`},
		{`{"type":"self","place":{}}`, ".place", "must be an array, not an object"},
		{`"self"`, "", "must be an object, not a string"},
	} {
		fe := assertInvalid(t, withMembers(`"assets":[`+c.asset+`]`), `$.variants[""][0].assets[0]`+c.member)
		want := `Invalid package file at $.variants[""][0].assets[0]` + c.member + ": " + c.reason + "."
		if fe.Message != want {
			t.Errorf("%s: %q, want %q", c.asset, fe.Message, want)
		}
	}
	// URLs are not checked to be https.
	e := singleEntry(t, withMembers(`"assets":[{"type":"zip","urls":["http://x","not a url",""]}]`))
	if len(e.Assets[0].URLs) != 3 {
		t.Errorf("urls %v", e.Assets[0].URLs)
	}
	// Unknown members such as "placements" and the urls of a self asset are ignored.
	for _, a := range []string{`{"type":"self","urls":["https://a.example/x.zip"]}`,
		`{"type":"mcmod","urls":["https://a.example/x.mcmod"],"placements":[{"src":"a.dll","dest":"a.dll"}]}`,
		`{"type":"self","placements":{}}`, `{"type":"zip","urls":["https://a.example/x.zip"],"placements":1}`} {
		e := singleEntry(t, withMembers(`"assets":[`+a+`]`))
		if len(e.Assets[0].Place) != 0 || (e.Assets[0].Type == pkgmodel.AssetSelf && len(e.Assets[0].URLs) != 0) {
			t.Errorf("%s: %+v", a, e.Assets[0])
		}
	}
	e = singleEntry(t, withMembers(`"assets":[{"type":"self","place":[{"src":"a.dll","dest":"plugins/a.dll"}],`+
		`"placements":[{"src":"b.dll","dest":"plugins/b.dll"}]}]`))
	if !reflect.DeepEqual(e.Assets[0].Place, []pkgmodel.Placement{{Src: "a.dll", Dest: "plugins/a.dll"}}) {
		t.Errorf("place %+v", e.Assets[0].Place)
	}
	e = singleEntry(t, withMembers(`"preserve_files":["/not a glob"],"remove_files":[1]`))
	if len(e.Preserve) != 0 || len(e.Purge) != 0 {
		t.Error("old glob keys must be ignored")
	}
	fe := assertInvalid(t, withMembers(`"assets":[{"type":"mcmod","urls":["https://a/a.mcmod"]},`+
		`{"type":"zip","urls":["https://a/b.zip"]},{"type":"mcmod","urls":["https://a/c.mcmod"]}]`), `$.variants[""][0].assets`)
	if fe.Message != `Invalid package file at $.variants[""][0].assets: has more than one asset of type "mcmod".` {
		t.Errorf("%q", fe.Message)
	}
	d := mustParse(t, setRoot("variants", `{"":[{"platform":"server","runtime":"win-x64","assets":[`+
		`{"type":"mcmod","urls":["https://a/s.mcmod"]}]},{"platform":"client","runtime":"win-x64","assets":[`+
		`{"type":"mcmod","urls":["https://a/c.mcmod"]}]}]}`))
	for _, e := range d.Variants[0].Entries {
		if e.McmodAsset() != 0 {
			t.Errorf("McmodAsset %d", e.McmodAsset())
		}
	}
}

func TestParsePlacements(t *testing.T) {
	for _, c := range []struct {
		src, dest string
		form      pkgmodel.PlacementForm
	}{
		{"a.dll", "plugins/a.dll", pkgmodel.PlacementFile}, {"bin/a.dll", "a.dll", pkgmodel.PlacementFile},
		{"", "", pkgmodel.PlacementDirectory}, {"", "tools/x/", pkgmodel.PlacementDirectory},
		{"bin/", "", pkgmodel.PlacementDirectory}, {"bin/", "tools/x/", pkgmodel.PlacementDirectory},
		{"*.dll", "", pkgmodel.PlacementGlob}, {"bin/**/*.dll", "plugins/", pkgmodel.PlacementGlob},
		{"bin/a?.dll", "plugins/", pkgmodel.PlacementGlob}, {"bin/[ab].dll", "plugins/", pkgmodel.PlacementGlob},
		{"bin/*/", "plugins/", pkgmodel.PlacementGlob}, {"a.dll", "C/a.dll", pkgmodel.PlacementFile},
		{"...", "a..b", pkgmodel.PlacementFile},
	} {
		e := singleEntry(t, withMembers(`"assets":[{"type":"self","place":[{"src":"`+c.src+`","dest":"`+c.dest+`"}]}]`))
		p := e.Assets[0].Place[0]
		if p != (pkgmodel.Placement{Src: c.src, Dest: c.dest}) || p.Form() != c.form {
			t.Errorf("%+v: %+v %v", c, p, p.Form())
		}
		if !IsValidPlacement(c.src, c.dest) {
			t.Errorf("IsValidPlacement(%q, %q) = false", c.src, c.dest)
		}
	}
	for _, c := range []struct{ src, dest, member, reason string }{
		{"a.dll", "plugins/", "dest", "'plugins/' must name a file for a file src"},
		{"a.dll", "", "dest", "'' must name a file for a file src"},
		{"bin/", "tools/x", "dest", `'tools/x' must be "" or end with '/' for a directory src`},
		{"", "tools", "dest", `'tools' must be "" or end with '/' for a directory src`},
		{"*.dll", "plugins/a.dll", "dest", `'plugins/a.dll' must be "" or end with '/' for a glob src`},
		{"bin/[^a]", "plugins/", "src", "'bin/[^a]' is not a glob"},
		{"/a.dll", "a.dll", "src", "'/a.dll' must not start with '/'"},
		{"C:/a.dll", "a.dll", "src", "'C:/a.dll' must not start with a drive letter"},
		{"c:a.dll", "a.dll", "src", "'c:a.dll' must not start with a drive letter"},
		{"bin//a.dll", "a.dll", "src", "'bin//a.dll' must not contain '//'"},
		{`bin\\a.dll`, "a.dll", "src", `'bin\a.dll' must not contain '\'`},
		{"../a.dll", "a.dll", "src", "'../a.dll' must not contain a '..' segment"},
		{"bin/../a.dll", "a.dll", "src", "'bin/../a.dll' must not contain a '..' segment"},
		{"..", "a/", "src", "'..' must not contain a '..' segment"},
		{"a.dll", "/a.dll", "dest", "'/a.dll' must not start with '/'"},
		{"a.dll", "D:/a.dll", "dest", "'D:/a.dll' must not start with a drive letter"},
		{"a.dll", "plugins//a.dll", "dest", "'plugins//a.dll' must not contain '//'"},
		{"a.dll", `plugins\\a.dll`, "dest", `'plugins\a.dll' must not contain '\'`},
		{"a.dll", "../a.dll", "dest", "'../a.dll' must not contain a '..' segment"},
		{"a.dll", "plugins/../../a.dll", "dest", "'plugins/../../a.dll' must not contain a '..' segment"},
	} {
		fe := assertInvalid(t, withMembers(`"assets":[{"type":"zip","urls":["https://a.example/x.zip"],`+
			`"place":[{"src":"`+c.src+`","dest":"`+c.dest+`"}]}]`), `$.variants[""][0].assets[0].place[0].`+c.member)
		if !strings.HasSuffix(fe.Message, "."+c.member+": "+c.reason+".") {
			t.Errorf("%+v: %q", c, fe.Message)
		}
	}
	if IsValidPlacement("a.dll", "plugins/") || IsValidPlacement("/a", "a") {
		t.Error("IsValidPlacement accepted an invalid placement")
	}
	for _, c := range []struct{ placement, member string }{
		{`{"dest":"a"}`, ".src"}, {`{"src":"a"}`, ".dest"}, {`{"src":1,"dest":"a"}`, ".src"}, {`["a","a"]`, ""},
	} {
		assertInvalid(t, withMembers(`"assets":[{"type":"self","place":[`+c.placement+`]}]`),
			`$.variants[""][0].assets[0].place[0]`+c.member)
	}
}

func TestParseGlobs(t *testing.T) {
	for _, key := range []string{"preserve", "purge"} {
		globs := []string{"config.json", "**/*.json", "data/", "plugins/[a-z]?/*", ".hidden"}
		e := singleEntry(t, withMembers(`"`+key+`":["config.json","**/*.json","data/","plugins/[a-z]?/*",".hidden"]`))
		got := map[string][]string{"preserve": e.Preserve, "purge": e.Purge}[key]
		if !reflect.DeepEqual(got, globs) {
			t.Errorf("%s: %v", key, got)
		}
	}
	for _, c := range []struct{ key, glob string }{
		{"preserve", `""`}, {"preserve", `"/config.json"`}, {"preserve", `"config//a.json"`},
		{"preserve", `"config\\a.json"`}, {"preserve", "1"}, {"purge", `""`}, {"purge", `"/logs/"`},
		{"purge", `"logs//"`}, {"purge", `"logs\\"`},
	} {
		fe := assertInvalid(t, withMembers(`"`+c.key+`":[`+c.glob+`]`), `$.variants[""][0].`+c.key+`[0]`)
		if c.glob != "1" && (fe.Err == nil || !strings.HasSuffix(fe.Message, "' is not a glob.")) {
			t.Errorf("%+v: %q", c, fe.Message)
		}
	}
}

func TestParseHooksAndInfo(t *testing.T) {
	for _, c := range []struct{ hooks, member string }{
		{`{"pre_install":"a"}`, ".pre_install"}, {`{"post_install":[1]}`, ".post_install[0]"},
		{`{"pre_uninstall":{}}`, ".pre_uninstall"}, {`{"post_uninstall":[null]}`, ".post_uninstall[0]"},
		{`["a"]`, ""},
	} {
		assertInvalid(t, withMembers(`"hooks":`+c.hooks), `$.variants[""][0].hooks`+c.member)
	}
	withInfo := func(info string) string { return setRoot("info", info) }
	for _, c := range []struct{ info, member string }{
		{`{"name":1}`, ".name"}, {`{"description":[]}`, ".description"}, {`{"tags":"a"}`, ".tags"},
		{`{"tags":[1]}`, ".tags[0]"}, {`{"logo":null}`, ".logo"}, {`"Tool"`, ""},
	} {
		assertInvalid(t, withInfo(c.info), "$.info"+c.member)
	}
	for _, logo := range []string{"https://a.example/logo.png", "https://"} {
		if d := mustParse(t, withInfo(`{"logo":"`+logo+`"}`)); d.Info.Logo != logo {
			t.Errorf("logo %q", d.Info.Logo)
		}
	}
	for _, logo := range []string{`"http://a.example/logo.png"`, `"logo.png"`, `"HTTPS://a.example/logo.png"`, `""`,
		"1", `["https://a.example/logo.png"]`} {
		fe := assertInvalid(t, withInfo(`{"logo":`+logo+`}`), "$.info.logo")
		if strings.HasPrefix(logo, `"`) && !strings.HasSuffix(fe.Message, `does not start with "https://".`) {
			t.Errorf("%q", fe.Message)
		}
	}
	mustParse(t, withInfo(`{"avatar_url":"logo.png"}`))
}

func TestParseUnknownKeysAreIgnored(t *testing.T) {
	text := withMembers(`"x-entry":{"anything":[]},"assets":[{"type":"self","x":1,` +
		`"place":[{"src":"a.dll","dest":"b.dll","type":"dir"}]},` +
		`{"type":"mcmod","urls":["https://a.example/a.mcmod"],"sha256":"?"}],"hooks":{"pre_pack":["x"],"install":5}`)
	text = strings.Replace(text, `"format":4`, `"format":4,"info":{"name":"n","author":5,"tags":[]},"x-root":[1,2],`+
		`"deps":{"not a key":5},"format_uuid":5,"tooth":5`, 1)
	e := singleEntry(t, text)
	if e.Assets[0].Place[0] != (pkgmodel.Placement{Src: "a.dll", Dest: "b.dll"}) || e.McmodAsset() != 1 ||
		len(e.Hooks.PreInstall) != 0 || !e.Raw.(*doc.Object).Has("x-entry") {
		t.Errorf("entry %+v", e)
	}
}

func TestParseYAMLErrors(t *testing.T) {
	for _, c := range []struct{ text, inner string }{
		{validDoc[:len(validDoc)-1], "not valid YAML"},
		{validDoc + "\n---\n" + validDoc, "more than one YAML document"},
		{validDoc[:len(validDoc)-1] + `,"version":"1.2.3"}`, "appears twice"},
		{strings.Replace(withMembers(`"hooks":{}`), `"hooks":{}`, `"hooks":{},"hooks":{}`, 1), "appears twice"},
		{validDoc[:len(validDoc)-1] + ",\"x\":\"\xc3\x28\"}", "UTF-8"},
		{strings.Replace(validDoc, `"1.2.3"`, `"\uD800"`, 1), ""},
	} {
		_, err := ParseYAML([]byte(c.text))
		var fe *FormatError
		if !errors.As(err, &fe) || fe.Message != "The lip.yaml is not a valid YAML file of the kind lip reads." {
			t.Errorf("%q: %v", c.text, err)
			continue
		}
		var ye *doc.YAMLError
		if !errors.As(err, &ye) || !strings.Contains(fe.Err.Error(), c.inner) || strings.Contains(fe.Err.Error(), "lip.yaml") {
			t.Errorf("%q: inner %v", c.text, fe.Err)
		}
	}
	mustParse(t, "# comment\n"+validDoc)
	mustParse(t, "\xef\xbb\xbf"+validDoc)
}

func TestFormat(t *testing.T) {
	for text, want := range map[string]int64{`{"format":4}`: 4, `{"format":3.0}`: 3, `{"format":1}`: 1,
		`{"format":2.5}`: -1, `{"format":"3"}`: -1, `{"format_version":4}`: -1, `{}`: -1, `[4]`: -1} {
		v, err := doc.ParseJSON([]byte(text), "x")
		if err != nil {
			t.Fatal(err)
		}
		got, ok := Format(v)
		if !ok {
			got = -1
		}
		if got != want {
			t.Errorf("Format(%s) = %d, want %d", text, got, want)
		}
	}
}

func TestParseEntry(t *testing.T) {
	valid, _ := doc.ParseJSON([]byte(`{"platform":"client","runtime":"linux-x64",`+
		`"assets":[{"type":"zip","urls":["https://a.example/x.zip"]}]}`), "x")
	e, err := ParseEntry(valid)
	if err != nil || e.Platform != "client" || e.Runtime != "linux-x64" || e.Derived || len(e.Assets) != 1 ||
		e.Assets[0].Type != pkgmodel.AssetZip {
		t.Fatalf("%+v %v", e, err)
	}
	invalid, _ := doc.ParseJSON([]byte(`{"platform":"client","runtime":"linux-x64",`+
		`"assets":[{"type":"tar","urls":["https://a.example/x.tar"]}]}`), "x")
	if _, err := ParseEntry(invalid); err == nil || !strings.HasPrefix(err.Error(), "Invalid package file at $.assets[0].type:") {
		t.Fatalf("%v", err)
	}
}

func TestIsRuntime(t *testing.T) {
	for v, want := range map[string]bool{"win-x64": true, "linux-musl-arm64": true, "osx.13-arm64": true,
		"win_x64": true, "": false, "win-*": false, "win-": false, "win x64": false, "win-x64\n": false} {
		if IsRuntime(v) != want {
			t.Errorf("IsRuntime(%q) != %v", v, want)
		}
	}
}
