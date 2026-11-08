package lipindex

import (
	"encoding/json"
	"testing"
)

// Catalogue documents shaped like levimc.org/mods/<id>/index.json.
const (
	fixtureHolo = `{
  "tooth": "github.com/MarmieQi/LHolo", "name": "LHolo", "description": "Schematic mod",
  "icon": "https://example.com/icon.png", "stars": 30, "updated": "2026-11-04T10:02:50Z",
  "tags": ["platform:levilamina", "type:mod"], "license": "GPL-3.0",
  "versions": [
    {"version": "26.51.10", "variants": [{"label": "client", "dependencies": {"github.com/LiteLDev/LeviLamina#client": "26.51.*"}}]},
    {"version": "26.60.0", "variants": [{"label": "client", "dependencies": {"github.com/LiteLDev/LeviLamina#client": "26.60.*"}}]},
    {"version": "26.61.0-beta.1", "variants": [{"label": "client", "dependencies": {"github.com/LiteLDev/LeviLamina#client": "26.61.*"}}]}
  ],
  "readmes": [{"file": "README.md", "lang": "en", "text": "# LHolo"}]
}`
	fixtureServerOnly = `{
  "tooth": "github.com/ShrBox/BackupHelper", "name": "BackupHelper", "stars": 3,
  "versions": [{"version": "1.0.0", "variants": [{"label": "", "dependencies": {"github.com/LiteLDev/LeviLamina": "1.x"}}]}]
}`
	fixtureLSE = `{
  "tooth": "github.com/LiteLDev/LegacyScriptEngine", "name": "LegacyScriptEngine", "stars": "120",
  "versions": [
    {"version": "0.9.0", "variants": [
      {"label": "client", "dependencies": {"github.com/LiteLDev/LegacyScriptEngine#client_lua": "0.9.0", "github.com/LiteLDev/LegacyScriptEngine#client_quickjs": "0.9.0"}},
      {"label": "client_lua", "dependencies": {"github.com/LiteLDev/LeviLamina#client": ">=26.60.0 <26.61.0"}},
      {"label": "client_quickjs", "dependencies": {"github.com/LiteLDev/LeviLamina#client": ">=26.60.0 <26.61.0"}}
    ]}
  ]
}`
	fixtureMoney = `{
  "tooth": "github.com/LiteLDev/LegacyMoney", "name": "LegacyMoney", "stars": 50, "updated": "2026-01-01T00:00:00Z",
  "description": "Economy for servers and clients", "tags": ["economy"],
  "versions": [
    {"version": "0.9.1", "variants": [{"label": "client", "dependencies": {"github.com/LiteLDev/LeviLamina#client": "26.51.*"}}]},
    {"version": "0.9.2", "variants": [{"label": "client", "dependencies": {"github.com/LiteLDev/LeviLamina#client": "26.60.*"}}]}
  ]
}`
	fixtureCrash = `{
  "tooth": "github.com/LiteLDev/CrashLogger", "name": "CrashLogger", "stars": 99,
  "versions": [{"version": "1.4.0", "variants": [{"label": "client", "dependencies": {}}]}]
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
