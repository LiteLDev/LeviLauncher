package lipindex

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParsePackage(t *testing.T) {
	p, ok := ParsePackage(detail(t, fixtureHolo))
	if !ok {
		t.Fatal("LHolo has a client variant")
	}
	if p.Identifier != "github.com/MarmieQi/LHolo" || p.Author != "MarmieQi" || p.Owner != "MarmieQi" || p.ProjectURL != "https://github.com/MarmieQi/LHolo" {
		t.Fatalf("identity = %q %q %q", p.Identifier, p.Author, p.ProjectURL)
	}
	if !reflect.DeepEqual(p.Versions, []string{"26.61.0-beta.1", "26.60.0", "26.51.10"}) {
		t.Fatalf("versions = %v", p.Versions)
	}
	if p.LatestVersion() != "26.60.0" {
		t.Fatalf("latest = %q, want the first stable", p.LatestVersion())
	}
	if !reflect.DeepEqual(p.LLRanges, []string{"26.61.*", "26.60.*", "26.51.*"}) {
		t.Fatalf("ranges = %v", p.LLRanges)
	}
	if p.Stars != 30 || p.AvatarURL != "https://example.com/icon.png" || p.License != "GPL-3.0" || p.LicenseFamily != LicenseCopyleft {
		t.Fatalf("info = %+v", p)
	}
	if !p.Files[0].Prerelease || p.Files[1].Prerelease {
		t.Fatalf("pre-releases = %+v", p.Files)
	}
	if !reflect.DeepEqual(p.Deps, map[string]string{"github.com/LiteLDev/LeviLamina": "26.61.*"}) {
		t.Fatalf("deps = %v", p.Deps)
	}
	v, ok := p.DefaultVariant()
	if !ok || v.PackageIdentifier != "github.com/MarmieQi/LHolo#client" {
		t.Fatalf("default variant = %+v", v)
	}
	if _, ok := ParsePackage(detail(t, fixtureServerOnly)); ok {
		t.Fatal("a package without a client entry is dropped")
	}
	TargetPlatform = "android-arm64"
	_, ok = ParsePackage(detail(t, fixtureHolo))
	TargetPlatform = "win-x64"
	if ok {
		t.Fatal("a package without an entry for the platform is dropped")
	}
	lse, ok := ParsePackage(detail(t, fixtureLSE))
	if !ok || lse.Stars != 120 {
		t.Fatalf("LSE = %+v %v", lse, ok)
	}
	var keys []string
	for _, v := range lse.Variants {
		keys = append(keys, v.Key)
	}
	if !reflect.DeepEqual(keys, []string{"client", "client_lua", "client_quickjs"}) {
		t.Fatalf("variant order = %v (only variants with a client entry)", keys)
	}
	if len(lse.Files) != 1 || len(lse.Files[0].LLRanges) != 0 || len(lse.Files[0].OtherDependencies) != 2 {
		t.Fatalf("client files = %+v", lse.Files)
	}
	// The default variant "" holds the client build of LegacyMoney.
	money, _ := ParsePackage(detail(t, fixtureMoney))
	if v, _ := money.DefaultVariant(); v.Key != "" || v.PackageIdentifier != "github.com/LiteLDev/LegacyMoney#" {
		t.Fatalf("money default = %+v", v)
	}
	if v, ok := money.FindVariant(""); !ok || !reflect.DeepEqual(v.Versions, []string{"0.9.2", "0.9.1"}) {
		t.Fatalf("money \"\" = %+v %v", v, ok)
	}
}

func TestVariantChoice(t *testing.T) {
	both := map[string][]Entry{
		"":       {{Side: SideServer, Platform: "win-x64"}, {Side: SideClient, Platform: "win-x64"}},
		"client": {{Side: SideClient, Platform: "win-x64"}},
		"zeta":   {{Side: SideClient, Platform: "win-x64"}},
		"alpha":  {{Side: SideClient, Platform: "linux-x64"}},
	}
	if got := QualifyingVariants(both, SideClient, "win-x64"); !reflect.DeepEqual(got, []string{"client", "", "zeta"}) {
		t.Fatalf("qualifying = %q", got)
	}
	if v, _ := PreferredVariant(both, SideServer, "win-x64"); v != "" {
		t.Fatalf("server = %q", v)
	}
	if v, _ := PreferredVariant(both, SideClient, "linux-x64"); v != "alpha" {
		t.Fatalf("linux = %q", v)
	}
	if _, ok := PreferredVariant(both, SideClient, "android-arm64"); ok {
		t.Fatal("no variant for android")
	}
	d := detail(t, fixtureHolo)
	if v, ok := ChooseVariant(d, "26.60.0", SideClient, "win-x64"); !ok || v != "client" {
		t.Fatalf("choose = %q %v", v, ok)
	}
	if _, ok := ChooseVariant(d, "", SideServer, "win-x64"); ok {
		t.Fatal("LHolo has no server variant")
	}
}

// The shared fixture's details: rule A2 as the site applies it (its
// latest.variant is the server's choice) and for the launcher.
func TestSharedDetails(t *testing.T) {
	arke := sharedDetail(t, "github.com/LiteLDev/Arke")
	if v, ok := ChooseVariant(arke, "", SideClient, "win-x64"); !ok || v != "client" {
		t.Fatalf("Arke client = %q %v", v, ok)
	}
	p, ok := ParsePackage(arke)
	if !ok || p.PreferredVariant != "client" || p.Owner != "LiteLDev" || p.LicenseFamily != LicenseNone || p.License != "" {
		t.Fatalf("Arke = %+v %v", p, ok)
	}
	if !reflect.DeepEqual(p.Deps, map[string]string{"github.com/LiteLDev/LeviLamina": "26.60.*"}) {
		t.Fatalf("Arke deps = %v", p.Deps)
	}
	if !reflect.DeepEqual(p.GameVersions, []string{"1.26.60"}) || !reflect.DeepEqual(p.Side, []string{"client"}) {
		t.Fatalf("Arke facets = %v %v", p.GameVersions, p.Side)
	}

	lse := sharedDetail(t, "github.com/LiteLDev/LegacyScriptEngine")
	if v, _ := ChooseVariant(lse, "", SideServer, ""); v != "" {
		t.Fatalf("LSE server = %q, the site's latest.variant is \"\"", v)
	}
	if got := QualifyingVariants(lse.Versions[0].Variants, SideClient, "win-x64"); !reflect.DeepEqual(got,
		[]string{"client", "client_lua", "client_nodejs", "client_python", "client_quickjs"}) {
		t.Fatalf("LSE client variants = %q", got)
	}
	if got := selfVariantAliases(lse, lse.ID); !reflect.DeepEqual(got, []string{"client_lua", "client_quickjs", "lua", "quickjs"}) {
		t.Fatalf("LSE aliases = %q", got)
	}
	lp, ok := ParsePackage(lse)
	if !ok || lp.PreferredVariant != "client" || lp.Versions[0] != "0.23.1" || !reflect.DeepEqual(lp.LLRanges[:1], []string{"26.60.*"}) {
		t.Fatalf("LSE = %+v %v", lp, ok)
	}
	if _, self := lp.Deps["github.com/LiteLDev/LegacyScriptEngine"]; self {
		t.Fatal("a package's deps leave itself out")
	}
	if lp.Deps["github.com/LiteLDev/LegacyMoney"] != "0.23.*" {
		t.Fatalf("LSE deps = %v", lp.Deps)
	}

	pland := sharedDetail(t, "github.com/IceBlcokMC/PLand")
	if _, ok := ParsePackage(pland); ok {
		t.Fatal("PLand runs on the server only")
	}
	if v, _ := ChooseVariant(pland, "", SideServer, "win-x64"); v != "" {
		t.Fatalf("PLand server = %q", v)
	}

	idx := Build([]*Detail{arke, lse, pland, sharedDetail(t, "github.com/zimuya4153/LSE_Waila")})
	if len(idx.Packages) != 2 || idx.Packages[0].Name != "Arke" || idx.Packages[1].Order != 1 {
		t.Fatalf("packages = %d", len(idx.Packages))
	}
	if len(idx.SelfVariants) != 1 || idx.SelfVariants[0].Identifier != "github.com/LiteLDev/LegacyScriptEngine" {
		t.Fatalf("self variants = %+v", idx.SelfVariants)
	}
}

// A detail of the catalogue before lip v0.40 ("tooth", "label") has no id
// and is not a package.
func TestOldDetailShape(t *testing.T) {
	var d Detail
	err := json.Unmarshal([]byte(`{"tooth":"github.com/a/b","versions":[{"version":"1.0.0","variants":{}}]}`), &d)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ParsePackage(&d); ok {
		t.Fatal("old detail accepted")
	}
}

func TestBuildIndexAndFind(t *testing.T) {
	idx := fixtureIndex(t)
	if len(idx.Packages) != 4 {
		t.Fatalf("packages = %d, want 4 (server-only dropped)", len(idx.Packages))
	}
	if idx.Packages[0].Name != "LegacyMoney" || idx.Packages[1].Name != "LHolo" || idx.Packages[2].Order != 2 {
		t.Fatal("packages keep the order of the details (the index's)")
	}
	if len(idx.SelfVariants) != 1 || !reflect.DeepEqual(idx.SelfVariants[0].Aliases, []string{"client_lua", "client_quickjs", "lua"}) {
		t.Fatalf("self variants = %+v", idx.SelfVariants)
	}
	for _, q := range []string{"github.com/LiteLDev/LegacyMoney", "github.com/LiteLDev/LegacyMoney#client",
		"GITHUB.COM/liteldev/legacymoney", "LiteLDev/LegacyMoney", "https://github.com/LiteLDev/LegacyMoney"} {
		if p, ok := idx.Find(q); !ok || p.Name != "LegacyMoney" {
			t.Errorf("Find(%q) = %v, %v", q, p, ok)
		}
	}
	if _, ok := idx.Find("nobody/nothing"); ok {
		t.Error("Find of an unknown package succeeded")
	}
}

func TestLookupKeyAndReference(t *testing.T) {
	cases := map[string]string{
		"LiteLDev/Foo":                          "github.com/liteldev/foo",
		"github.com/LiteLDev/Foo#client":        "github.com/liteldev/foo",
		"https://github.com/liteldev/foo@1.0.0": "github.com/liteldev/foo",
		"gitee.com/a/b":                         "gitee.com/a/b",
	}
	for in, want := range cases {
		if got := LookupKey(in); got != want {
			t.Errorf("LookupKey(%q) = %q, want %q", in, got, want)
		}
	}
	r, ok := ParseReference("LiteLDev/LegacyMoney#client@0.9.2")
	if !ok || r.ID != "github.com/LiteLDev/LegacyMoney" || r.Variant != "client" || !r.HasVariant || r.Version != "0.9.2" {
		t.Fatalf("ParseReference = %+v %v", r, ok)
	}
	r, ok = ParseReference("LiteLDev/LegacyMoney#")
	if !ok || r.Variant != "" || !r.HasVariant {
		t.Fatalf("ParseReference(#) = %+v %v", r, ok)
	}
	r, ok = ParseReference("https://github.com/LiteLDev/LegacyMoney/")
	if !ok || r.ID != "github.com/LiteLDev/LegacyMoney" || r.Version != "" || r.HasVariant {
		t.Fatalf("ParseReference(url) = %+v %v", r, ok)
	}
	for _, bad := range []string{"", "LegacyMoney", "a/b/c/d", "C:\\mods\\x"} {
		if _, ok := ParseReference(bad); ok {
			t.Errorf("ParseReference(%q) accepted", bad)
		}
	}
	if !IsLeviLamina("github.com/LiteLDev/LeviLamina#client") || !IsLeviLamina("liteldev/levilamina") || IsLeviLamina("LiteLDev/LegacyMoney") {
		t.Error("IsLeviLamina")
	}
	if u := DetailURL(DefaultBaseURL, "github.com/a b/c#client"); u != "https://levimc.org/api/v4/mods/github.com/a%20b/c/index.json" {
		t.Errorf("DetailURL = %q", u)
	}
	if u := OwnerURL("LiteLDev"); u != "https://github.com/LiteLDev" {
		t.Errorf("OwnerURL = %q", u)
	}
}
