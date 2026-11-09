package lipindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fixtures are for Windows x64, the launcher's platform; tests run on
// any platform.
func TestMain(m *testing.M) {
	TargetPlatform = "win-x64"
	os.Exit(m.Run())
}

// Catalogue documents shaped like levimc.org/api/v4/mods/<id>/index.json.
const (
	fixtureHolo = `{
  "id": "github.com/MarmieQi/LHolo", "name": "LHolo", "description": "Schematic mod", "owner": "MarmieQi",
  "logo": "https://example.com/icon.png", "stars": 30, "updated": "2026-11-04T10:02:50Z",
  "tags": ["platform:levilamina", "type:mod"], "license": "GPL-3.0",
  "side": ["client"], "platform": ["win-x64"], "gameVersions": ["1.26.60", "1.26.51"],
  "versions": [
    {"version": "26.61.0-beta.1", "prerelease": true, "variants": {"client": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": "26.61.*"}}]}},
    {"version": "26.60.0", "variants": {"client": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": "26.60.*"}}]}},
    {"version": "26.51.10", "variants": {"client": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": "26.51.*"}}]}}
  ],
  "readmes": [{"file": "README.md", "lang": "en", "text": "# LHolo"}]
}`
	fixtureServerOnly = `{
  "id": "github.com/ShrBox/BackupHelper", "name": "BackupHelper", "owner": "ShrBox", "stars": 3,
  "side": ["server"], "platform": ["win-x64"],
  "versions": [{"version": "1.0.0", "variants": {"": [{"side": "server", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina": "1.x"}}]}}]
}`
	fixtureLSE = `{
  "id": "github.com/LiteLDev/LegacyScriptEngine", "name": "LegacyScriptEngine", "owner": "LiteLDev", "stars": 120,
  "side": ["server", "client"], "platform": ["win-x64"],
  "versions": [
    {"version": "0.9.0", "variants": {
      "": [{"side": "server", "platform": "win-x64", "deps": {"github.com/LiteLDev/LegacyScriptEngine#lua": "0.9.0", "github.com/LiteLDev/LeviLamina": ">=26.60.0 <26.61.0"}}],
      "client": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LegacyScriptEngine#client_lua": "0.9.0", "github.com/LiteLDev/LegacyScriptEngine#client_quickjs": "0.9.0"}}],
      "client_lua": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": ">=26.60.0 <26.61.0"}}],
      "client_quickjs": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": ">=26.60.0 <26.61.0"}}],
      "lua": [{"side": "server", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina": ">=26.60.0 <26.61.0"}}]
    }}
  ]
}`
	fixtureMoney = `{
  "id": "github.com/LiteLDev/LegacyMoney", "name": "LegacyMoney", "owner": "LiteLDev", "stars": 50, "updated": "2026-01-01T00:00:00Z",
  "description": "Economy for servers and clients", "tags": ["economy"], "side": ["server", "client"], "platform": ["win-x64"],
  "gameVersions": ["1.26.60", "1.26.51"],
  "versions": [
    {"version": "0.9.2", "variants": {"": [{"side": "server", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina": "26.60.*"}}, {"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": "26.60.*"}}]}},
    {"version": "0.9.1", "variants": {"": [{"side": "client", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina#client": "26.51.*"}}]}}
  ]
}`
	fixtureCrash = `{
  "id": "github.com/LiteLDev/CrashLogger", "name": "CrashLogger", "owner": "LiteLDev", "stars": 99,
  "versions": [{"version": "1.4.0", "variants": {"client": [{"side": "client", "platform": "win-x64", "deps": {}}]}}]
}`
)

func detail(t *testing.T, doc string) *Detail {
	t.Helper()
	var d Detail
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		t.Fatal(err)
	}
	return &d
}

func fixtureIndex(t *testing.T) *Index {
	t.Helper()
	return Build([]*Detail{detail(t, fixtureMoney), detail(t, fixtureHolo), detail(t, fixtureServerOnly),
		detail(t, fixtureLSE), detail(t, fixtureCrash)})
}

// fixtureDB is a LeviLamina database for the fixtures.
func fixtureDB() LoaderDB {
	return NormalizeLoaderDB(map[string][]string{
		"1.26.51.01": {"26.51.0", "26.51.1"},
		"1.26.60.05": {"26.60.0", "26.60.4", "26.60.5-rc.1"},
	})
}

// The shared fixture (testdata/): /mods/index.json and four details of a
// local build of levimc.org/mods. The site's tests, the launcher's frontend
// (scripts/lipCatalog.test.mjs) and LeviLauncher-Android read the same bytes.

func readTestdata(t *testing.T, path string, out any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// sharedDetail is a detail of the shared fixture.
func sharedDetail(t *testing.T, id string) *Detail {
	t.Helper()
	var d Detail
	readTestdata(t, id+"/index.json", &d)
	return &d
}

// catalogEntry is a mod of the shared fixture's index.json.
type catalogEntry struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Owner        string            `json:"owner"`
	Tags         []string          `json:"tags"`
	Side         []string          `json:"side"`
	Platform     []string          `json:"platform"`
	GameVersions []string          `json:"gameVersions"`
	Deps         map[string]string `json:"deps"`
	Stars        float64           `json:"stars"`
	License      string            `json:"license"`
	Updated      string            `json:"updated"`
	Created      string            `json:"created"`
}

// sharedCatalog is every mod of the shared index as a Package with what
// the filters, the search and the sorts read (as the site has it, without
// the launcher's side and platform), in the index's order.
func sharedCatalog(t *testing.T) []*Package {
	t.Helper()
	var doc struct {
		Mods []catalogEntry `json:"mods"`
	}
	readTestdata(t, "index.json", &doc)
	out := make([]*Package, 0, len(doc.Mods))
	for i, m := range doc.Mods {
		license := LicenseOf(m.ID, m.License)
		out = append(out, &Package{Identifier: m.ID, Name: m.Name, Description: m.Description, Author: m.Owner, Owner: m.Owner,
			Tags: m.Tags, License: license, LicenseFamily: LicenseFamily(license), Stars: m.Stars, Updated: m.Updated,
			Created: m.Created, Side: m.Side, Platform: m.Platform, GameVersions: m.GameVersions, Deps: m.Deps, Order: i})
	}
	return out
}
