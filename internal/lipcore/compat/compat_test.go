package compat

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Ports of tests/Lip.Core.Tests/Mcmod/Compat/ToothCompatTests.cs
// (v0.38.0-beta.1).

const uuid3 = "289f771f-2c9a-4d73-9f3f-8492495a924d"

const multiPlatformV3 = `{
  "format_version": 3,
  "format_uuid": "` + uuid3 + `",
  "tooth": "github.com/user/repo",
  "version": "1.2.3",
  "info": { "name": "Repo", "description": "d", "tags": ["a"], "avatar_url": "" },
  "variants": [
    { "platform": "", "dependencies": { "github.com/user/common": ">=1.0.0" },
      "preserve_files": ["config/*.json"] },
    { "platform": "win-*", "dependencies": { "github.com/user/windows#client": "1.*" },
      "assets": [{ "type": "self", "placements": [
        { "type": "file", "src": "a.dll", "dest": "plugins/a/a.dll" } ] }],
      "scripts": { "post_install": ["echo win"] } },
    { "platform": "linux-x64", "dependencies": { "github.com/user/linux": "1.*" } },
    { "label": "client", "platform": "win-x64",
      "dependencies": { "github.com/user/clientdep": "^2.0.0" } },
    { "label": "extra", "platform": "linux-arm64" }
  ]
}`

func v3(variants string, extra ...string) string {
	s := `{"format_version": 3, "format_uuid": "` + uuid3 + `",
	  "tooth": "github.com/user/repo", "version": "1.0.0"`
	for _, e := range extra {
		s += ", " + e
	}
	return s + `, "variants": ` + variants + `}`
}

func derive(t *testing.T, tooth, platform, runtime string) *pkgmodel.Document {
	t.Helper()
	d, err := Derive([]byte(tooth), platform, runtime, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func variantNames(d *pkgmodel.Document) []string {
	var names []string
	for _, v := range d.Variants {
		names = append(names, v.Name)
	}
	return names
}

func single(t *testing.T, d *pkgmodel.Document, variant string) *pkgmodel.Entry {
	t.Helper()
	entries, ok := d.EntriesOf(variant)
	if !ok || len(entries) != 1 {
		t.Fatalf("variant %q: %d entries (present %v)", variant, len(entries), ok)
	}
	return &entries[0]
}

func depKeys(e *pkgmodel.Entry) []string {
	var keys []string
	for _, d := range e.Deps {
		keys = append(keys, d.Key)
	}
	return keys
}

func eq[T comparable](t *testing.T, what string, got, want []T) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func yamlOf(v any) string { return string(doc.MarshalYAML(v)) }

func TestDerivePlatformPatternMergesMatchingVariantsOfEachLabel(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "win-x64")
	if d.ID != "github.com/user/repo" || d.Version != "1.2.3" {
		t.Errorf("id, version = %s, %s", d.ID, d.Version)
	}
	eq(t, "variants", variantNames(d), []string{"", "client"})
	e := single(t, d, "")
	if !e.Derived || e.Platform != "server" || e.Runtime != "win-x64" {
		t.Errorf("entry = %+v", e)
	}
	eq(t, "deps", depKeys(e), []string{"github.com/user/common", "github.com/user/windows#client"})
	r := semver.MustParseVersionRange(e.Deps[1].Range)
	if !r.Contains(semver.MustParse("1.9.0")) || r.Contains(semver.MustParse("2.0.0")) {
		t.Errorf("range %s", e.Deps[1].Range)
	}
	eq(t, "preserve", e.Preserve, []string{"config/*.json"})
	eq(t, "post_install", e.Hooks.PostInstall, []string{"echo win"})
	if len(e.Assets) != 1 || e.Assets[0].Type != pkgmodel.AssetSelf ||
		!slices.Equal(e.Assets[0].Place, []pkgmodel.Placement{{Src: "a.dll", Dest: "plugins/a/a.dll"}}) {
		t.Errorf("assets = %+v", e.Assets)
	}
	if d.SelectEntry("", "server", "win-x64") != e {
		t.Error("SelectEntry does not select the entry")
	}
	eq(t, "client deps", depKeys(single(t, d, "client")), []string{"github.com/user/clientdep"})
}

func TestDeriveOtherRuntimeMatchingPatternUsesThatRuntime(t *testing.T) {
	for _, runtime := range []string{"win-arm64", "win-x86"} {
		d := derive(t, multiPlatformV3, "server", runtime)
		eq(t, "variants", variantNames(d), []string{""})
		e := single(t, d, "")
		if e.Runtime != runtime {
			t.Errorf("runtime = %s", e.Runtime)
		}
		eq(t, "deps", depKeys(e), []string{"github.com/user/common", "github.com/user/windows#client"})
		if d.SelectEntry("", "server", runtime) != e || d.SelectEntry("", "server", "win-x64") != nil {
			t.Error("SelectEntry")
		}
	}
}

func TestDeriveExactPlatformSelectsOnlyThatRuntime(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "linux-x64")
	eq(t, "variants", variantNames(d), []string{""})
	e := single(t, d, "")
	eq(t, "deps", depKeys(e), []string{"github.com/user/common", "github.com/user/linux"})
	if len(e.Assets) != 0 || len(e.Hooks.PostInstall) != 0 {
		t.Errorf("entry = %+v", e)
	}
}

func TestDeriveLabelMatchingOnlyAnyPlatformHasNoVariant(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "osx-arm64")
	eq(t, "deps", depKeys(single(t, d, "")), []string{"github.com/user/common"})
	eq(t, "variants", variantNames(d), []string{""})
}

func TestDeriveNoVariantMatchesRuntimeHasNoVariants(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64" }, { "label": "lua", "platform": "win-*" } ]`),
		"server", "linux-x64")
	if len(d.Variants) != 0 {
		t.Errorf("variants = %v", variantNames(d))
	}
	if v, _ := d.Raw.(*doc.Object).Get("variants"); v.(*doc.Object).Len() != 0 {
		t.Errorf("raw variants = %s", yamlOf(v))
	}
}

func TestDeriveClientPlatformVariantEmptyIsTheClientLabel(t *testing.T) {
	d := derive(t, multiPlatformV3, "client", "win-x64")
	e := single(t, d, "")
	if e.Platform != "client" || e.Runtime != "win-x64" {
		t.Errorf("entry = %+v", e)
	}
	eq(t, "deps", depKeys(e), []string{"github.com/user/clientdep"})
	eq(t, "client deps", depKeys(single(t, d, "client")), []string{"github.com/user/clientdep"})
	variants, _ := d.Raw.(*doc.Object).Get("variants")
	c, _ := variants.(*doc.Object).Get("client")
	empty, _ := variants.(*doc.Object).Get("")
	if yamlOf(c) != yamlOf(empty) {
		t.Errorf("client %s != %s", yamlOf(c), yamlOf(empty))
	}
	if d.SelectEntry("", "client", "win-x64") != e {
		t.Error("SelectEntry")
	}
}

func TestDeriveClientPlatformWithoutClientVariantForRuntimeHasNoVariantEmpty(t *testing.T) {
	if d := derive(t, multiPlatformV3, "client", "linux-x64"); len(d.Variants) != 0 {
		t.Errorf("variants = %v", variantNames(d))
	}
}

func TestDeriveClientPlatformWithoutClientLabelKeepsVariantEmpty(t *testing.T) {
	d := derive(t, v3(`[ { "label": "", "platform": "win-x64",
	  "dependencies": { "github.com/user/a": "1.0.0" } } ]`), "client", "win-x64")
	e := single(t, d, "")
	if e.Platform != "client" {
		t.Errorf("platform = %s", e.Platform)
	}
	eq(t, "deps", depKeys(e), []string{"github.com/user/a"})
}

func TestDeriveServerPlatformKeepsVariantEmptyAndClientLabel(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "win-x64")
	if slices.Equal(depKeys(single(t, d, "")), depKeys(single(t, d, "client"))) {
		t.Error("the variant \"\" is the client label on the server")
	}
	if single(t, d, "client").Platform != "server" {
		t.Error("platform")
	}
}

func TestDeriveDerivedDocumentIsFormat4WithEntriesForTheEnvironment(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "win-x64")
	want := `format: 4
id: github.com/user/repo
version: 1.2.3
info:
  name: Repo
  description: d
  tags:
  - a
variants:
  "":
  - platform: server
    runtime: win-x64
    deps:
      github.com/user/common: '>=1.0.0'
      github.com/user/windows#client: 1.*
    assets:
    - type: self
      place:
      - src: a.dll
        dest: plugins/a/a.dll
    preserve:
    - config/*.json
    hooks:
      post_install:
      - echo win
  client:
  - platform: server
    runtime: win-x64
    deps:
      github.com/user/clientdep: 2.*
`
	// The YAML lip v0.38 writes for it (the dependency ranges are written as
	// SemVersionRange.ToString() writes them: "^2.0.0" as "2.*").
	if got := yamlOf(d.Raw); got != want {
		t.Errorf("document:\n%s\nwant:\n%s", got, want)
	}
	e := single(t, d, "")
	eq(t, "entry keys", e.Raw.(*doc.Object).Keys(),
		[]string{"platform", "runtime", "deps", "assets", "preserve", "hooks"})
	if hooks, _ := e.Raw.(*doc.Object).Get("hooks"); string(doc.MarshalJSONCompact(hooks)) != `{"post_install":["echo win"]}` {
		t.Errorf("hooks = %s", doc.MarshalJSONCompact(hooks))
	}
}

func TestDeriveAvatarURLIsLogoOnlyWhenHTTPS(t *testing.T) {
	for avatar, logo := range map[string]string{
		"https://example.com/a.png": "https://example.com/a.png",
		"http://example.com/a.png":  "",
		"a.png":                     "",
		"":                          "",
	} {
		d := derive(t, v3(`[ { "platform": "win-x64" } ]`,
			`"info": { "name": "Repo", "avatar_url": "`+avatar+`" }`), "server", "win-x64")
		info, _ := d.Raw.(*doc.Object).Get("info")
		keys := info.(*doc.Object).Keys()
		want := []string{"name"}
		if logo != "" {
			want = append(want, "logo")
		}
		eq(t, "info keys", keys, want)
		if d.Info.Logo != logo {
			t.Errorf("logo = %q, want %q", d.Info.Logo, logo)
		}
	}
}

func TestDeriveNoAvatarURLHasNoLogo(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64" } ]`,
		`"info": { "name": "Repo", "description": "d", "tags": ["a"] }`), "server", "win-x64")
	info, _ := d.Raw.(*doc.Object).Get("info")
	eq(t, "info keys", info.(*doc.Object).Keys(), []string{"name", "description", "tags"})
}

func TestIsLogoChecksHTTPSPrefix(t *testing.T) {
	for in, want := range map[string]bool{
		"https://example.com/a.png": true, "https://": true, "http://example.com/a.png": false,
		"HTTPS://example.com/a.png": false, "a.png": false, "": false,
	} {
		if IsLogo(in) != want {
			t.Errorf("IsLogo(%q) = %v", in, !want)
		}
	}
}

func TestDeriveNoInfoHasNoInfo(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64" } ]`), "server", "win-x64")
	if d.Raw.(*doc.Object).Has("info") || d.Info != nil {
		t.Error("info present")
	}
}

func TestDeriveEmptyInfoMembersAreKept(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64" } ]`,
		`"info": { "name": "", "tags": [], "avatar_url": "" }`), "server", "win-x64")
	info, _ := d.Raw.(*doc.Object).Get("info")
	if got := string(doc.MarshalJSONCompact(info)); got != `{"name":"","tags":[]}` {
		t.Errorf("info = %s", got)
	}
}

func TestDeriveDerivedEntryHasPreserveAndPurge(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64",
	  "assets": [ { "type": "self", "placements": [
	    { "type": "file", "src": "a.dll", "dest": "plugins/a.dll" } ] } ],
	  "preserve_files": ["config.json"], "remove_files": ["logs"] } ]`), "server", "win-x64")
	e := single(t, d, "")
	raw := e.Raw.(*doc.Object)
	eq(t, "keys", raw.Keys(), []string{"platform", "runtime", "assets", "preserve", "purge"})
	eq(t, "preserve", e.Preserve, []string{"**/config.json"})
	eq(t, "purge", e.Purge, []string{"logs", "logs/"})
	assets, _ := raw.Get("assets")
	if got := string(doc.MarshalJSONCompact(assets)); got != `[{"type":"self","place":[{"src":"a.dll","dest":"plugins/a.dll"}]}]` {
		t.Errorf("assets = %s", got)
	}
}

func TestDeriveScriptsBecomeHooks(t *testing.T) {
	d := derive(t, v3(`[
	  { "scripts": { "pre_install": ["pre"], "install": ["install"], "uninstall": ["un"] } },
	  { "platform": "win-x64", "scripts": { "post_install": ["post"],
	    "pre_uninstall": ["preun"], "post_uninstall": ["postun"] } } ]`), "server", "win-x64")
	h := single(t, d, "").Hooks
	eq(t, "pre_install", h.PreInstall, []string{"pre"})
	eq(t, "post_install", h.PostInstall, []string{"install", "post"})
	eq(t, "pre_uninstall", h.PreUninstall, []string{"preun", "un"})
	eq(t, "post_uninstall", h.PostUninstall, []string{"postun"})
}

func TestDeriveMergedVariantsConcatenateAssetsAndFiles(t *testing.T) {
	d := derive(t, v3(`[
	  { "assets": [ { "type": "zip", "urls": ["https://example.com/a.zip"] } ],
	    "preserve_files": ["config.json"], "remove_files": ["logs"] },
	  { "platform": "win-*",
	    "assets": [ { "type": "self",
	      "placements": [ { "type": "dir", "src": "d", "dest": "plugins/d" } ] } ],
	    "preserve_files": ["plugins\\x\\*.json"], "remove_files": ["tmp/"] } ]`), "server", "win-x64")
	e := single(t, d, "")
	if len(e.Assets) != 2 || e.Assets[0].Type != pkgmodel.AssetZip || e.Assets[1].Type != pkgmodel.AssetSelf {
		t.Fatalf("assets = %+v", e.Assets)
	}
	eq(t, "urls", e.Assets[0].URLs, []string{"https://example.com/a.zip"})
	if len(e.Assets[0].Place) != 0 || len(e.Assets[1].URLs) != 0 {
		t.Errorf("assets = %+v", e.Assets)
	}
	eq(t, "place", e.Assets[1].Place, []pkgmodel.Placement{{Src: "d/", Dest: "plugins/d/"}})
	if e.McmodAsset() != -1 {
		t.Error("mcmod asset")
	}
	eq(t, "preserve", e.Preserve, []string{"**/config.json", "plugins/x/*.json"})
	eq(t, "purge", e.Purge, []string{"logs", "logs/", "tmp/"})
}

func TestDeriveAssetOfTypeWithoutFormat4EquivalentMakesTheVariantAbsent(t *testing.T) {
	for _, typ := range []string{"tar", "tgz", "uncompressed"} {
		tooth := v3(`[
		  { "assets": [ { "type": "zip", "urls": ["https://example.com/a.zip"] } ] },
		  { "label": "client", "assets": [ { "type": "zip", "urls": ["https://example.com/c.zip"] } ] },
		  { "label": "client", "platform": "win-*",
		    "assets": [ { "type": "` + typ + `", "urls": ["https://example.com/b"],
		      "placements": [ { "type": "file", "src": "", "dest": "b.exe" } ] } ] } ]`)
		eq(t, "server win", variantNames(derive(t, tooth, "server", "win-x64")), []string{""})
		if d := derive(t, tooth, "client", "win-x64"); len(d.Variants) != 0 {
			t.Errorf("client win variants = %v", variantNames(d))
		}
		eq(t, "client linux", variantNames(derive(t, tooth, "client", "linux-x64")), []string{"", "client"})
	}
}

func TestConvertPlacementFormat3PlacementBecomesMemberOfPlace(t *testing.T) {
	for _, c := range []struct{ typ, src, dest, wantSrc, wantDest string }{
		{"file", "a.dll", "plugins/a.dll", "a.dll", "plugins/a.dll"},
		{"file", `.\bin\a.dll`, `plugins\\a.dll`, "bin/a.dll", "plugins/a.dll"},
		{"file", "bin/*.dll", "plugins/a", "bin/*.dll", "plugins/a/"},
		{"file", "bin/a?.dll", "plugins/", "bin/a?.dll", "plugins/"},
		{"file", "bin/[ab].dll", "", "bin/[ab].dll", ""},
		{"dir", "bin", "plugins/a", "bin/", "plugins/a/"},
		{"dir", "./bin/", "plugins//a/", "bin/", "plugins/a/"},
		{"dir", "", "plugins/a", "", "plugins/a/"},
		{"dir", ".", ".", "", ""},
	} {
		typ := manifest.PlacementFile
		if c.typ == "dir" {
			typ = manifest.PlacementDir
		}
		src, dest := ConvertPlacement(manifest.Placement{Type: typ, Src: c.src, Dest: c.dest})
		if src != c.wantSrc || dest != c.wantDest {
			t.Errorf("ConvertPlacement(%s %q %q) = %q %q, want %q %q", c.typ, c.src, c.dest, src, dest, c.wantSrc, c.wantDest)
		}
		if !IsValidPlacement(src, dest) {
			t.Errorf("IsValidPlacement(%q, %q) = false", src, dest)
		}
	}
}

func TestDerivePlacementsAndGlobsInvalidInFormat4AreLeftOut(t *testing.T) {
	d := derive(t, v3(`[ { "platform": "win-x64",
	  "assets": [ { "type": "self", "placements": [
	    { "type": "file", "src": "", "dest": "b.exe" },
	    { "type": "file", "src": "a.dll", "dest": "." },
	    { "type": "file", "src": "c.dll", "dest": "plugins/c.dll" } ] } ],
	  "preserve_files": ["/abs", "a//b", "ok.json"],
	  "remove_files": ["/abs", "a//b", "logs/"] } ]`), "server", "win-x64")
	e := single(t, d, "")
	eq(t, "place", e.Assets[0].Place, []pkgmodel.Placement{{Src: "c.dll", Dest: "plugins/c.dll"}})
	eq(t, "preserve", e.Preserve, []string{"**/ok.json"})
	eq(t, "purge", e.Purge, []string{"logs/"})
}

func TestDeriveVersionIsTheVersionOfThePackage(t *testing.T) {
	tag := semver.MustParse("2.0.0")
	tagged, err := Derive([]byte(multiPlatformV3), "server", "win-x64", &tag)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := tagged.Raw.(*doc.Object).Get("version"); tagged.Version != "2.0.0" || v != "2.0.0" {
		t.Errorf("tagged version = %s, %v", tagged.Version, v)
	}
	if untagged := derive(t, multiPlatformV3, "server", "win-x64"); untagged.Version != "1.2.3" {
		t.Errorf("untagged version = %s", untagged.Version)
	}
}

func TestDeriveSameScriptInTwoMergedVariantsFails(t *testing.T) {
	tooth := v3(`[ { "scripts": { "install": ["a"] } },
	  { "platform": "win-x64", "scripts": { "install": ["b"] } } ]`)
	_, err := Derive([]byte(tooth), "server", "win-x64", nil)
	var fe *FormatError
	if !errors.As(err, &fe) || !strings.Contains(err.Error(), "labeled ''") {
		t.Errorf("err = %v", err)
	}
	eq(t, "post_install", single(t, derive(t, tooth, "server", "linux-x64"), "").Hooks.PostInstall, []string{"a"})
}

func TestDeriveSameDependencyInTwoMergedVariantsFails(t *testing.T) {
	_, err := Derive([]byte(v3(`[
	  { "label": "lua", "dependencies": { "github.com/user/a": "1.*" } },
	  { "label": "lua", "platform": "win-x64", "dependencies": { "github.com/user/a": "2.*" } } ]`)),
		"server", "win-x64", nil)
	var fe *FormatError
	if !errors.As(err, &fe) || !strings.Contains(err.Error(), "labeled 'lua'") {
		t.Errorf("err = %v", err)
	}
}

func TestDeriveV1IsReadAsFormat3First(t *testing.T) {
	d := derive(t, `{
	  "format_version": 1, "tooth": "github.com/LiteLScript-Dev/HelperLib", "version": "2.14.1",
	  "dependencies": { "github.com/user/dep": [[">=1.0.0"]] },
	  "placement": [{ "source": "src/*", "destination": "declaration/llse/*" }],
	  "commands": [ { "type": "install", "commands": ["echo install"], "GOOS": "windows", "GOARCH": "amd64" } ]
	}`, "server", "win-x64")
	if d.ID != "github.com/LiteLScript-Dev/HelperLib" {
		t.Errorf("id = %s", d.ID)
	}
	e := single(t, d, "")
	eq(t, "post_install", e.Hooks.PostInstall, []string{"echo install"})
	eq(t, "deps", depKeys(e), []string{"github.com/user/dep"})
}

func TestDeriveV2MapsAssetURLRemoveFilesAndScripts(t *testing.T) {
	d := derive(t, `{
	  "format_version": 2, "tooth": "github.com/LiteLDev/LeviLamina", "version": "1.1.0",
	  "info": { "name": "LeviLamina", "description": "", "author": "levimc", "tags": [] },
	  "asset_url": "https://example.com/v$(version)/levilamina.zip",
	  "dependencies": { "github.com/LiteLDev/bds": "1.21.60" },
	  "files": { "place": [{ "src": "LeviLamina/*", "dest": "plugins/LeviLamina/" }],
	             "remove": ["bedrock_server_mod.exe"] },
	  "platforms": [ { "goos": "windows", "goarch": "amd64",
	                   "commands": { "post_install": [".\\PeEditor.exe -mb"] } } ]
	}`, "server", "win-x64")
	if d.Version != "1.1.0" {
		t.Errorf("version = %s", d.Version)
	}
	e := single(t, d, "")
	eq(t, "deps", depKeys(e), []string{"github.com/LiteLDev/bds"})
	if !semver.MustParseVersionRange(e.Deps[0].Range).Contains(semver.MustParse("1.21.60")) {
		t.Errorf("range = %s", e.Deps[0].Range)
	}
	if len(e.Assets) != 1 || e.Assets[0].Type != pkgmodel.AssetZip {
		t.Fatalf("assets = %+v", e.Assets)
	}
	eq(t, "urls", e.Assets[0].URLs, []string{"https://example.com/v1.1.0/levilamina.zip"})
	eq(t, "purge", e.Purge, []string{"bedrock_server_mod.exe", "bedrock_server_mod.exe/"})
	eq(t, "post_install", e.Hooks.PostInstall, []string{`.\PeEditor.exe -mb`})
}

func TestDeriveV2PlatformPatternHasVariantForEveryRuntime(t *testing.T) {
	tooth := `{
	  "format_version": 2, "tooth": "github.com/test/tool", "version": "1.0.0",
	  "info": { "name": "", "description": "", "author": "", "tags": [] },
	  "platforms": [ { "goos": "windows", "commands": { "post_install": ["echo"] } } ]
	}`
	for runtime, windows := range map[string]bool{"win-x64": true, "linux-x64": false, "osx-arm64": false} {
		e := single(t, derive(t, tooth, "server", runtime), "")
		if e.Runtime != runtime {
			t.Errorf("runtime = %s", e.Runtime)
		}
		if (len(e.Hooks.PostInstall) == 1) != windows {
			t.Errorf("%s: post_install = %v", runtime, e.Hooks.PostInstall)
		}
	}
}

func TestPreserveFilesPatternWithoutSlashMatchesFileNameEverywhere(t *testing.T) {
	patterns := PreserveFiles([]string{"config.json", `plugins\x\*.json`, "mods/x/data/"})
	eq(t, "patterns", patterns, []string{"**/config.json", "plugins/x/*.json", "mods/x/data/"})
	g := TryParseGlob(patterns[0])
	if g == nil || !g.IsMatch("config.json") || !g.IsMatch("mods/x/config.json") {
		t.Error("**/config.json does not match")
	}
}

func TestRemoveFilesPatternWithoutTrailingSlashMatchesFilesAndDirectories(t *testing.T) {
	eq(t, "patterns", RemoveFiles([]string{"plugins/GMLIB", "logs/", `a\b`}),
		[]string{"plugins/GMLIB", "plugins/GMLIB/", "logs/", "a/b", "a/b/"})
}

func TestConvertScriptsV3HooksMapsInstallAndUninstall(t *testing.T) {
	h := ConvertScripts(manifest.Scripts{
		PreInstall: []string{"pre"}, Install: []string{"install"}, PostInstall: []string{"post"},
		PreUninstall: []string{"preun"}, Uninstall: []string{"un"}, PostUninstall: []string{"postun"},
	})
	eq(t, "pre_install", h.PreInstall, []string{"pre"})
	eq(t, "post_install", h.PostInstall, []string{"install", "post"})
	eq(t, "pre_uninstall", h.PreUninstall, []string{"preun", "un"})
	eq(t, "post_uninstall", h.PostUninstall, []string{"postun"})
}

func TestTryParseGlobInvalidPatternReturnsNil(t *testing.T) {
	for p, valid := range map[string]bool{"**/config.json": true, "logs/": true, "": false, "/abs": false, "a//b": false} {
		if (TryParseGlob(p) != nil) != valid {
			t.Errorf("TryParseGlob(%q) valid = %v", p, !valid)
		}
	}
}

// The derived document re-reads as the same entries.
func TestDeriveDerivedEntryReparses(t *testing.T) {
	d := derive(t, multiPlatformV3, "server", "win-x64")
	for _, v := range d.Variants {
		e, err := parseEntry(v.Entries[0].Raw.(*doc.Object))
		if err != nil {
			t.Fatal(err)
		}
		e.Derived = true
		if yamlOf(e.Raw) != yamlOf(v.Entries[0].Raw) || !slices.Equal(e.Preserve, v.Entries[0].Preserve) {
			t.Errorf("%s: reparsed entry differs", v.Name)
		}
	}
}

func TestDeriveRejectsUnknownPlatform(t *testing.T) {
	if _, err := Derive([]byte(multiPlatformV3), "desktop", "win-x64", nil); err == nil {
		t.Error("no error")
	}
}

func TestReadToothJSONFormatVersions(t *testing.T) {
	for _, in := range []string{`{"format_version": 4}`, `{}`, `[]`, `{"format_version": "3"}`, `not json`} {
		_, err := ParseToothJSON([]byte(in))
		var fe *FormatError
		if !errors.As(err, &fe) {
			t.Errorf("ParseToothJSON(%s) = %v, want a *FormatError", in, err)
		}
	}
}
