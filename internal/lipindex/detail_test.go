package lipindex

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The deps a detail gives are the shared index's mods[].deps (the site's
// depsOf), and its description is the index's.
func TestSharedDetailsMatchIndex(t *testing.T) {
	var doc struct {
		Mods []catalogEntry `json:"mods"`
	}
	readTestdata(t, "index.json", &doc)
	listed := map[string]catalogEntry{}
	for _, m := range doc.Mods {
		listed[m.ID] = m
	}
	for _, id := range []string{"github.com/LiteLDev/Arke", "github.com/LiteLDev/LegacyScriptEngine",
		"github.com/IceBlcokMC/PLand", "github.com/zimuya4153/LSE_Waila"} {
		d := sharedDetail(t, id)
		p, ok := ParsePackage(d)
		if !ok {
			// Server-only mods are not the launcher's, but their deps are the same.
			p, ok = ParsePackageFor(d, SideServer, "win-x64")
		}
		if !ok {
			t.Fatalf("%s: no package", id)
		}
		m, ok := listed[id]
		if !ok {
			t.Fatalf("%s is not in the shared index", id)
		}
		if !reflect.DeepEqual(p.Deps, m.Deps) {
			t.Errorf("%s deps = %v, the index has %v", id, p.Deps, m.Deps)
		}
		if p.Description != m.Description {
			t.Errorf("%s description = %q, the index has %q", id, p.Description, m.Description)
		}
	}
}

const fixtureRenamedDeps = `{
  "id": "github.com/a/Self", "name": "Self", "owner": "a", "description": "  Self  ",
  "latest": {"variant": "", "version": "2.0.0", "install": "lip install github.com/a/Self@2.0.0"},
  "dependencies": {
    "github.com/old/name": {"id": "github.com/New/Name", "name": "Name", "owner": "New", "url": "https://levimc.org/mods/github.com/New/Name/"},
    "github.com/liteldev/levilamina": {"id": "github.com/LiteLDev/LeviLamina", "name": "LeviLamina", "owner": "LiteLDev", "url": "https://levimc.org/mods/github.com/LiteLDev/LeviLamina/"},
    "github.com/a/oldself": {"id": "github.com/a/Self", "name": "Self", "owner": "a", "url": "https://levimc.org/mods/github.com/a/Self/"},
    "github.com/x/unlisted": {"id": "github.com/x/unlisted", "name": "unlisted", "owner": "x"},
    "github.com/beta/only": {"id": "github.com/Beta/Only", "name": "Only", "owner": "Beta", "url": "https://levimc.org/mods/github.com/Beta/Only/"}
  },
  "versions": [
    {"version": "2.1.0-beta.1", "prerelease": true, "variants": {"zeta": [{"side": "client", "platform": "win-x64", "deps": {"github.com/Beta/Only": "1.*"}}]}},
    {"version": "2.0.0", "variants": {
      "zeta": [{"side": "client", "platform": "win-x64", "deps": {"github.com/liteldev/levilamina#client": "26.60.*", "github.com/Old/Name": "2.*"}}],
      "": [{"side": "server", "platform": "win-x64", "deps": {"github.com/LiteLDev/LeviLamina": "26.51.*", "github.com/X/Unlisted": "1.*", "github.com/a/OldSelf#lua": "2.0.0"}}]
    }}
  ]
}`

// A dependency is named as the catalogue names it; ranges join in the
// document's order; the version "latest" names is the one described.
func TestCatalogDeps(t *testing.T) {
	want := map[string]string{
		// "zeta" comes first in the document, though rule A2 puts "" first.
		"github.com/LiteLDev/LeviLamina": "26.60.* || 26.51.*",
		// a renamed repository, by its current name
		"github.com/New/Name": "2.*",
		// no page: as the key spells it
		"github.com/X/Unlisted": "1.*",
		// "github.com/a/OldSelf" is the package itself, under an old name
	}
	d := detail(t, fixtureRenamedDeps)
	p, ok := ParsePackage(d)
	if !ok {
		t.Fatal("Self has a client variant")
	}
	if !reflect.DeepEqual(p.Deps, want) {
		t.Fatalf("deps = %v, want %v", p.Deps, want)
	}
	if p.Description != "Self" || p.FullDescription != "Self" {
		t.Fatalf("description = %q / %q", p.Description, p.FullDescription)
	}

	// The cache keeps the document's order, which its own JSON loses.
	cache := Cache{Path: filepath.Join(t.TempDir(), "lip.json")}
	if err := cache.Save([]*Detail{d}, time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := cache.Load()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("cache = %v, %v", loaded, err)
	}
	if got := latestDeps(loaded[0]); !reflect.DeepEqual(got, want) {
		t.Fatalf("cached deps = %v, want %v", got, want)
	}

	// A detail built in code has no document order: variants in rule A2's
	// order, keys sorted.
	built := &Detail{ID: "github.com/a/Self", Versions: []DetailVersion{{Version: "1.0.0", Variants: map[string][]Entry{
		"zeta": {{Side: SideClient, Platform: "win-x64", Deps: map[string]string{"github.com/LiteLDev/LeviLamina#client": "26.60.*"}}},
		"":     {{Side: SideServer, Platform: "win-x64", Deps: map[string]string{"github.com/LiteLDev/LeviLamina": "26.51.*", "github.com/a/self#lua": "1.0.0"}}},
	}}}}
	if got := latestDeps(built); !reflect.DeepEqual(got, map[string]string{"github.com/LiteLDev/LeviLamina": "26.51.* || 26.60.*"}) {
		t.Fatalf("built deps = %v", got)
	}
	if got := latestDeps(&Detail{ID: "github.com/a/Empty"}); got == nil || len(got) != 0 {
		t.Fatalf("no versions = %v", got)
	}
}

// The site's truncate: 200 code points at most, the cut trimmed, "…" added.
func TestTruncateDescription(t *testing.T) {
	exact := strings.Repeat("字", 200)
	if got := truncateDescription("  " + exact + "\n"); got != exact {
		t.Fatalf("200 code points are kept, got %d", len([]rune(got)))
	}
	long := strings.Repeat("a", 197) + "  bcdef"
	if got := truncateDescription(long); got != strings.Repeat("a", 197)+"…" {
		t.Fatalf("cut = %q", got)
	}
	long = strings.Repeat("é", 250)
	if got := truncateDescription(long); got != strings.Repeat("é", 199)+"…" {
		t.Fatalf("cut = %q", got)
	}
	d := &Detail{Description: long, CatalogDescription: "From the index…"}
	if got := catalogDescription(d); got != "From the index…" {
		t.Fatalf("catalogue description = %q, want the index's", got)
	}
	d.CatalogDescription = ""
	if got := catalogDescription(d); got != strings.Repeat("é", 199)+"…" {
		t.Fatalf("catalogue description = %q, want the detail's cut", got)
	}
}
