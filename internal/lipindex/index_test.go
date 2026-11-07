package lipindex

import (
	"reflect"
	"testing"
)

func TestParsePackage(t *testing.T) {
	p, ok := ParsePackage(detail(t, fixtureHolo))
	if !ok {
		t.Fatal("LHolo has a client variant")
	}
	if p.Identifier != "github.com/MarmieQi/LHolo" || p.Author != "MarmieQi" || p.ProjectURL != "https://github.com/MarmieQi/LHolo" {
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
	if p.Stars != 30 || p.AvatarURL != "https://example.com/icon.png" || p.License != "GPL-3.0" {
		t.Fatalf("info = %+v", p)
	}
	v, ok := p.DefaultVariant()
	if !ok || v.PackageIdentifier != "github.com/MarmieQi/LHolo#client" {
		t.Fatalf("default variant = %+v", v)
	}
	if _, ok := ParsePackage(detail(t, fixtureServerOnly)); ok {
		t.Fatal("a package without a client variant is dropped")
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
		t.Fatalf("variant order = %v", keys)
	}
	if len(lse.Files) != 1 || len(lse.Files[0].LLRanges) != 0 || len(lse.Files[0].OtherDependencies) != 2 {
		t.Fatalf("client files = %+v", lse.Files)
	}
}

func TestBuildIndexAndFind(t *testing.T) {
	idx := fixtureIndex(t)
	if len(idx.Packages) != 4 {
		t.Fatalf("packages = %d, want 4 (server-only dropped)", len(idx.Packages))
	}
	if len(idx.SelfVariants) != 1 || !reflect.DeepEqual(idx.SelfVariants[0].Aliases, []string{"client_lua", "client_quickjs"}) {
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
	if !ok || r.ID != "github.com/LiteLDev/LegacyMoney" || r.Variant != "client" || r.Version != "0.9.2" {
		t.Fatalf("ParseReference = %+v %v", r, ok)
	}
	r, ok = ParseReference("https://github.com/LiteLDev/LegacyMoney/")
	if !ok || r.ID != "github.com/LiteLDev/LegacyMoney" || r.Version != "" {
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
	if u := DetailURL("https://levimc.org/mods/", "github.com/a b/c#client"); u != "https://levimc.org/mods/github.com/a%20b/c/index.json" {
		t.Errorf("DetailURL = %q", u)
	}
}
