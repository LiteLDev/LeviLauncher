package lipindex

import (
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func ids(pkgs []*Package) []string {
	out := []string{}
	for _, p := range pkgs {
		out = append(out, p.Name)
	}
	return out
}

func TestSearch(t *testing.T) {
	idx := fixtureIndex(t)
	db := fixtureDB()
	// CrashLogger is hidden; default sort is stars, most first.
	got := ids(Search(idx.Packages, Query{Sort: SortStars, Descending: true}))
	if !reflect.DeepEqual(got, []string{"LegacyScriptEngine", "LegacyMoney", "LHolo"}) {
		t.Fatalf("all = %v", got)
	}
	// Game filter: needs a LeviLamina range satisfied for that game.
	got = ids(Search(idx.Packages, Query{GameVersion: "1.26.51.01", DB: db, Sort: SortName}))
	if !reflect.DeepEqual(got, []string{"LegacyMoney", "LHolo"}) {
		t.Fatalf("game 1.26.51 = %v", got)
	}
	got = ids(Search(idx.Packages, Query{LoaderVersion: "v26.60.4", DB: db, Sort: SortName}))
	if !reflect.DeepEqual(got, []string{"LegacyMoney", "LHolo"}) {
		t.Fatalf("loader 26.60.4 = %v", got)
	}
	// Without the database the version filters are off.
	if n := len(Search(idx.Packages, Query{GameVersion: "1.26.51.01"})); n != 3 {
		t.Fatalf("no db: %d results", n)
	}
	got = ids(Search(idx.Packages, Query{Text: "ECONOMY"}))
	if !reflect.DeepEqual(got, []string{"LegacyMoney"}) {
		t.Fatalf("keyword = %v", got)
	}
	got = ids(Search(idx.Packages, Query{Sort: SortUpdated, Descending: true}))
	if got[0] != "LHolo" {
		t.Fatalf("updated = %v", got)
	}
	if DefaultDescending(SortName) || !DefaultDescending(SortStars) {
		t.Fatal("name sorts ascending by default, others descending")
	}
}

func TestMapperAndListing(t *testing.T) {
	idx := fixtureIndex(t)
	m := NewMapper(idx)
	mods := []types.ModInfo{
		{Name: "LegacyMoney", Version: "0.9.1", Folder: "LegacyMoney"},
		{Name: "LegacyScriptEngine-lua", Version: "0.9.0", Folder: "client_lua"},
		{Name: "LSE QuickJs", Version: "0.9.0", Folder: "legacy-script-engine-client_quickjs"},
		{Name: "MyTool", Version: "1.0.0", Folder: "MyTool"},
		{Name: "LeviLamina", Version: "26.60.4", Folder: "LeviLamina"},
	}
	mapped := m.Map(mods)
	if st := mapped["LegacyMoney"]; st.Source != SourceUnique || st.Mapping != MappingName || st.TargetVersion != "0.9.2" || !st.CanUpdate {
		t.Fatalf("LegacyMoney = %+v", st)
	}
	if st := mapped["client_lua"]; st.Source != SourceUnique || st.Mapping != MappingSelfVariant || st.MatchedAlias != "client_lua" || st.CanUpdate {
		t.Fatalf("client_lua (exact alias) = %+v", st)
	}
	if st := mapped["legacy-script-engine-client_quickjs"]; st.Source != SourceUnique || st.MatchedAlias != "client_quickjs" {
		t.Fatalf("quickjs (fuzzy alias) = %+v", st)
	}
	if st := mapped["MyTool"]; st.Source != SourceNone {
		t.Fatalf("MyTool = %+v", st)
	}
	if st := mapped["LeviLamina"]; st.Source != SourceLeviLamina {
		t.Fatalf("LeviLamina = %+v", st)
	}
	cands := Candidates(mapped)
	if !reflect.DeepEqual(cands, []string{"github.com/LiteLDev/LegacyMoney", "github.com/LiteLDev/LegacyScriptEngine"}) {
		t.Fatalf("candidates = %v", cands)
	}
	visible := mods[:4]
	states := map[string]InstallState{
		LookupKey("github.com/LiteLDev/LegacyMoney"): {Installed: true, Explicit: true, InstalledVersion: "0.9.1"},
		// LSE is not installed through the package manager: its folders stay local.
		LookupKey("github.com/LiteLDev/LegacyScriptEngine"): {},
	}
	enabled := map[string]bool{"LegacyMoney": true, "client_lua": true}
	l := BuildListing(visible, mapped, states, enabled)
	if len(l.Groups) != 1 || l.Groups[0].PackageName != "LegacyMoney" || !l.Groups[0].CanUpdate || !l.Groups[0].AllEnabled {
		t.Fatalf("groups = %+v", l.Groups)
	}
	var names []string
	for _, it := range l.Items {
		names = append(names, it.Mod.Name)
	}
	if !reflect.DeepEqual(names, []string{"LegacyScriptEngine-lua", "LSE QuickJs", "MyTool"}) {
		t.Fatalf("items = %v", names)
	}
	// A failed query keeps self-variant groups (as the window does).
	states[LookupKey("github.com/LiteLDev/LegacyScriptEngine")] = InstallState{Error: "ERR_LIP_PACKAGE_QUERY_FAILED"}
	l = BuildListing(visible, mapped, states, enabled)
	if len(l.Groups) != 2 || l.Groups[1].AllEnabled || !l.Groups[1].AnyEnabled ||
		!reflect.DeepEqual(l.Groups[1].ChildLabels, []string{"client_lua", "client_quickjs"}) {
		t.Fatalf("groups with query error = %+v", l.Groups)
	}
	// Two packages with one name are ambiguous.
	dup := Build([]*Detail{detail(t, fixtureMoney), detail(t, `{"tooth":"github.com/Other/LegacyMoney","name":"LegacyMoney",
		"versions":[{"version":"1.0.0","variants":[{"label":"client"}]}]}`)})
	if st := NewMapper(dup).Map(mods[:1])["LegacyMoney"]; st.Source != SourceAmbiguous {
		t.Fatalf("ambiguous = %+v", st)
	}
	// Without a catalogue nothing maps.
	var none *Mapper
	if st := none.Map(mods[:1])["LegacyMoney"]; st.Source != SourceNone {
		t.Fatalf("nil mapper = %+v", st)
	}
}

func moneyVariant(t *testing.T) Variant {
	p, _ := ParsePackage(detail(t, fixtureMoney))
	v, _ := p.DefaultVariant()
	return v
}

func TestDecide(t *testing.T) {
	v := moneyVariant(t)
	db := fixtureDB()
	// No LeviLamina: the newest version for the game, with the newest
	// stable LeviLamina that fits.
	p := Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05", DB: db})
	if p.Blocker != "" || p.Version != "0.9.2" || !p.NeedsLoader || p.ReplacesLoader || p.LoaderVersion != "26.60.4" || p.Action != ActionInstall {
		t.Fatalf("fresh = %+v", p)
	}
	// An older game picks an older version.
	p = Decide(PlanInput{Variant: v, GameVersion: "1.26.51.01", DB: db, Loader: InstallState{Installed: true, InstalledVersion: "26.51.1"}})
	if p.Version != "0.9.1" || p.NeedsLoader || !p.LoaderCompatible {
		t.Fatalf("older game = %+v", p)
	}
	// Asking for a version the game cannot run.
	p = Decide(PlanInput{Variant: v, Version: "0.9.2", GameVersion: "1.26.51.01", DB: db})
	if p.Blocker != BlockGameUnsupported || !reflect.DeepEqual(p.SupportedGames, []string{"1.26.60.05"}) {
		t.Fatalf("incompatible = %+v", p)
	}
	// Replacing a LeviLamina that does not fit.
	p = Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05", DB: db,
		Loader: InstallState{Installed: true, InstalledVersion: "26.61.0"}, LoaderVersion: "26.60.0"})
	if !p.ReplacesLoader || p.LoaderVersion != "26.60.0" || !p.LoaderDowngrade {
		t.Fatalf("replace = %+v", p)
	}
	p = Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05", DB: db, LoaderVersion: "9.9.9"})
	if p.Blocker != BlockLoaderVersionInvalid {
		t.Fatalf("bad loader version = %+v", p)
	}
	// Action kinds.
	for _, c := range []struct {
		pkg  InstallState
		want string
	}{
		{InstallState{Installed: true, Explicit: true, InstalledVersion: "0.9.1"}, ActionUpgrade},
		{InstallState{Installed: true, Explicit: true, InstalledVersion: "1.0.0"}, ActionDowngrade},
		{InstallState{Installed: true, Explicit: true, InstalledVersion: "0.9.2"}, ActionReinstall},
		{InstallState{Installed: true, InstalledVersion: "0.9.2"}, ActionPromote},
		{InstallState{Installed: true}, ActionReinstall},
	} {
		p = Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05", DB: db, Pkg: c.pkg,
			Loader: InstallState{Installed: true, InstalledVersion: "26.60.4"}})
		if p.Action != c.want || p.NeedsLoader {
			t.Errorf("pkg %+v: action %s, want %s", c.pkg, p.Action, c.want)
		}
	}
	// Without the database a mod needing LeviLamina cannot be planned.
	if p = Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05"}); p.Blocker != BlockLoaderDBUnavailable {
		t.Fatalf("no db = %+v", p)
	}
	// A failed LeviLamina check blocks rather than reinstalling it.
	if p = Decide(PlanInput{Variant: v, GameVersion: "1.26.60.05", DB: db, Loader: InstallState{Error: "ERR_X"}}); p.Blocker != BlockLoaderCheckFailed {
		t.Fatalf("check failed = %+v", p)
	}
	if p = Decide(PlanInput{Variant: v, Version: "7.0.0", DB: db}); p.Blocker != BlockVersionNotFound {
		t.Fatalf("missing version = %+v", p)
	}
}
