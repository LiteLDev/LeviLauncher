package lipindex

import (
	"reflect"
	"testing"
)

// The catalogue logic against the shared fixture's index (111 mods). The
// expected values are the site's (levimc.org mods/src/lib/{catalog-query,
// search}.ts) on these bytes, and the frontend's lipCatalog.test.mjs
// checks the same ones.

func packageIDs(list []*Package) []string {
	out := []string{}
	for _, p := range list {
		out = append(out, p.Identifier)
	}
	return out
}

func TestSharedCatalogOptions(t *testing.T) {
	pkgs := sharedCatalog(t)
	if len(pkgs) != 111 {
		t.Fatalf("mods = %d", len(pkgs))
	}
	o := Options(pkgs)
	if got := o.Tags[:20]; !reflect.DeepEqual(got, []string{
		"library", "utility", "management", "game-mechanics", "adventure", "visual", "economy", "teleport",
		"anticheat", "optimization", "technology", "protocol", "gmlib", "lse", "pland", "backpack", "building",
		"camera-editor", "carry", "collection",
	}) {
		t.Fatalf("tags = %q", got)
	}
	if contains(o.Tags, "bugfix") {
		t.Fatal("a featured tag no mod has is not offered")
	}
	if !reflect.DeepEqual(o.License, []LicenseOption{
		{LicensePermissive, []string{"MIT", "CC0-1.0", "Apache-2.0", "BSD-3-Clause", "Unlicense"}},
		{LicenseWeakCopyleft, []string{"LGPL-3.0"}},
		{LicenseCopyleft, []string{"GPL-3.0", "AGPL-3.0"}},
		{LicenseNone, nil},
	}) {
		t.Fatalf("license = %+v", o.License)
	}
	if got := o.Deps[:3]; !reflect.DeepEqual(got, []string{"github.com/LiteLDev/LeviLamina",
		"github.com/GroupMountain/GMLIB-Release", "github.com/GroupMountain/ModAPI-Release"}) {
		t.Fatalf("deps = %q", got)
	}
	if got := o.Authors[:3]; !reflect.DeepEqual(got, []string{"MiracleForest", "GroupMountain", "LiteLDev"}) {
		t.Fatalf("authors = %q", got)
	}
	if len(o.GameVersions) != 26 || !reflect.DeepEqual(o.GameVersions[:3], []string{"1.26.60", "1.26.51", "1.26.40"}) {
		t.Fatalf("game versions = %q", o.GameVersions)
	}
}

func TestSharedCatalogCounts(t *testing.T) {
	r := Run(sharedCatalog(t), Query{})
	if len(r.Matched) != 111 {
		t.Fatalf("matched = %d", len(r.Matched))
	}
	if got := packageIDs(r.Matched[:3]); !reflect.DeepEqual(got, []string{"github.com/LiteLDev/Arke",
		"github.com/LiteLDev/LegacyScriptEngine", "github.com/IceBlcokMC/PLand"}) {
		t.Fatalf("relevance without a search = %q, want the index order", got)
	}
	want := map[string]int{"library": 8, "utility": 10, "management": 2, "game-mechanics": 6, "adventure": 2, "visual": 5,
		"economy": 2, "teleport": 2, "anticheat": 1, "optimization": 2, "technology": 2, "protocol": 1, "gmlib": 6, "lse": 5, "pland": 2}
	for k, n := range want {
		if r.Counts.Tags[k] != n {
			t.Errorf("tags[%s] = %d, want %d", k, r.Counts.Tags[k], n)
		}
	}
	if !reflect.DeepEqual(r.Counts.License, map[string]int{
		"permissive": 38, "weak-copyleft": 20, "copyleft": 41, "other": 0, "none": 12,
		"MIT": 19, "CC0-1.0": 12, "Apache-2.0": 5, "BSD-3-Clause": 1, "Unlicense": 1, "LGPL-3.0": 20, "GPL-3.0": 31, "AGPL-3.0": 10,
	}) {
		t.Fatalf("license counts = %v", r.Counts.License)
	}
}

func TestSharedCatalogFilters(t *testing.T) {
	pkgs := sharedCatalog(t)
	run := func(q Query) []string { return packageIDs(Run(pkgs, q).Matched) }
	if got := run(Query{Tags: Facet{Inc: []string{"economy"}}}); !reflect.DeepEqual(got,
		[]string{"github.com/LiteLDev/LegacyMoney", "github.com/carefly/RLXMoney"}) {
		t.Fatalf("economy = %q", got)
	}
	if n := len(run(Query{Tags: Facet{Exc: []string{"library"}}})); n != 103 {
		t.Fatalf("-library = %d", n)
	}
	for q, n := range map[string]int{"permissive": 38, "GPL-3.0": 31, "none": 12} {
		if got := len(run(Query{License: []string{q}})); got != n {
			t.Errorf("license %s = %d, want %d", q, got, n)
		}
	}
	if n := len(run(Query{License: []string{"permissive", "LGPL-3.0"}})); n != 58 {
		t.Fatalf("license values are OR: %d", n)
	}
	if got := run(Query{Deps: []string{"github.com/liteldev/legacyscriptengine"}}); !reflect.DeepEqual(got,
		[]string{"github.com/copper-lamp/LitematicaBE", "github.com/superx101/CALL"}) {
		t.Fatalf("dep LSE = %q", got)
	}
	if n := len(run(Query{Authors: []string{"liteldev"}})); n != 7 {
		t.Fatalf("author = %d", n)
	}
	if n := len(run(Query{GameVersions: []string{"1.21.80"}})); n != 26 {
		t.Fatalf("game version = %d", n)
	}
	// The launcher's side and platform: the client on Windows x64.
	client := 0
	for _, p := range pkgs {
		if contains(p.Side, TargetSide) && contains(p.Platform, "win-x64") {
			client++
		}
	}
	if client != 62 {
		t.Fatalf("client win-x64 = %d", client)
	}
}

func TestSharedCatalogSearch(t *testing.T) {
	pkgs := sharedCatalog(t)
	run := func(text string) []string { return packageIDs(Run(pkgs, Query{Text: text}).Matched) }
	if got := run("land"); !reflect.DeepEqual(got, []string{"github.com/IceBlcokMC/PLand",
		"github.com/IceBlcokMC/PLand-LegacyRemoteCallApi", "github.com/lwenk/PlandProjectileFix"}) {
		t.Fatalf("land = %q", got)
	}
	if got := SearchCatalog(pkgs, "land"); !reflect.DeepEqual(got, map[string]int{"github.com/IceBlcokMC/PLand": 14,
		"github.com/IceBlcokMC/PLand-LegacyRemoteCallApi": 8, "github.com/lwenk/PlandProjectileFix": 5}) {
		t.Fatalf("land scores = %v", got)
	}
	if got := run("领地"); !reflect.DeepEqual(got, []string{"github.com/IceBlcokMC/PLand"}) {
		t.Fatalf("领地 = %q", got)
	}
	teleport := []string{"github.com/engsr6982/TeleportSystem", "github.com/MiracleForest/Bedstones"}
	if got := run("teleport"); !reflect.DeepEqual(got, teleport) {
		t.Fatalf("teleport = %q", got)
	}
	if got := run("传送"); !reflect.DeepEqual(got, teleport) {
		t.Fatalf("传送 = %q (featured labels in every language)", got)
	}
	if got := run("lse waila"); !reflect.DeepEqual(got, []string{"github.com/zimuya4153/LSE_Waila"}) {
		t.Fatalf("every word is required: %q", got)
	}
	if SearchCatalog(pkgs, "  ") != nil {
		t.Fatal("an empty search scores nothing")
	}
}

func TestSharedCatalogSorts(t *testing.T) {
	pkgs := sharedCatalog(t)
	first := func(sort string) []string { return packageIDs(Run(pkgs, Query{Sort: sort}).Matched[:3]) }
	for sort, want := range map[string][]string{
		SortStars:   {"github.com/LiteLDev/LegacyScriptEngine", "github.com/GroupMountain/GMLIB-Release", "github.com/LiteLDev/LeviOptimize"},
		SortUpdated: {"github.com/Ieckseyr/HologramLib", "github.com/neverforward/Insight", "github.com/MRUIAW/UtilityRenderDistanceExpansion"},
		SortNewest:  {"github.com/LiteLDev/Arke", "github.com/ch1-k-mc/BetterF3-Bedrock", "github.com/MRUIAW/UtilityRenderDistanceExpansion"},
	} {
		if got := first(sort); !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %q, want %q", sort, got, want)
		}
	}
}

func TestTagsAndLicenses(t *testing.T) {
	if NormalizeTag(" Category:Utility ") != "utility" {
		t.Fatal("NormalizeTag")
	}
	if got := VisibleTags([]string{"platform:levilamina", "type:mod", "LeviLamina", "client", "Shader", "shader", "lse", "pland:library"}); !reflect.DeepEqual(got, []string{"shader", "lse", "pland:library"}) {
		t.Fatalf("visible = %q", got)
	}
	if TagKey("money") != "economy" || TagKey("pland:library") != "library" || TagKey("type:mod") != "" {
		t.Fatal("TagKey")
	}
	if got := TagFacet([]string{"tp", "-teleport", "money", "mod"}); !reflect.DeepEqual(got, Facet{Inc: []string{"economy"}, Exc: []string{"teleport"}}) {
		t.Fatalf("TagFacet = %+v", got)
	}
	f := FeaturedTagsOf([]string{"tp", "waystones", "money", "custom"})
	if len(f) != 2 || f[0].Tag.ID != "economy" || f[1].Tag.ID != "teleport" || !reflect.DeepEqual(f[1].Raw, []string{"tp", "waystones"}) {
		t.Fatalf("featured = %+v", f)
	}
	if got := PlainTagsOf([]string{"tp", "custom", "type:mod"}); !reflect.DeepEqual(got, []string{"custom"}) {
		t.Fatalf("plain = %q", got)
	}
	for spdx, fam := range map[string]string{"mit": LicensePermissive, "BSD-2-Clause": LicensePermissive, "LGPL-2.1": LicenseWeakCopyleft,
		"AGPL-3.0": LicenseCopyleft, "NOASSERTION": LicenseOther, "": LicenseNone} {
		if got := LicenseFamily(spdx); got != fam {
			t.Errorf("LicenseFamily(%q) = %s, want %s", spdx, got, fam)
		}
	}
	if LicenseOf("github.com/Copper-Lamp/BSChat", "NOASSERTION") != "AGPL-3.0" || LicenseOf("github.com/Ieckseyr/HologramLib", "") != "MIT" {
		t.Fatal("license overrides")
	}
	for in, want := range map[string]string{"1.21.50.07": "1.21.50", "1.26.051.1": "1.26.51", "latest": "latest"} {
		if got := CatalogGameVersion(in); got != want {
			t.Errorf("CatalogGameVersion(%q) = %q", in, got)
		}
	}
	games := []string{"1.26.60", "1.26.51", "1.21.132"}
	for in, want := range map[string]string{"latest": "1.26.60", "1.26.51": "1.26.51", "1.26.51.01": "1.26.51", "1.26": "1.26.60", "1.21": "1.21.132"} {
		if got, ok := ResolveGameVersion(games, in); !ok || got != want {
			t.Errorf("ResolveGameVersion(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := ResolveGameVersion(games, "1.20"); ok {
		t.Error("ResolveGameVersion(1.20) matched")
	}
}
