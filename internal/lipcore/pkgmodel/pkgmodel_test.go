package pkgmodel

import "testing"

func TestPlacementForm(t *testing.T) {
	for src, want := range map[string]PlacementForm{
		"":          PlacementDirectory,
		"a/":        PlacementDirectory,
		"a":         PlacementFile,
		"a/b.dll":   PlacementFile,
		"*.dll":     PlacementGlob,
		"a?/":       PlacementGlob,
		"[ab]/c":    PlacementGlob,
		"a/**/b":    PlacementGlob,
		"dir.v1/x/": PlacementDirectory,
	} {
		if got := (Placement{Src: src}).Form(); got != want {
			t.Errorf("Form(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestSelectEntry(t *testing.T) {
	d := &Document{Variants: []VariantEntries{
		{Name: "", Entries: []Entry{
			{Side: SideServer, Platform: "win-x64"},
			{Side: SideClient, Platform: "win-x64", Assets: []Asset{{Type: AssetZip}, {Type: AssetMcmod}}},
		}},
		{Name: "lua", Entries: []Entry{{Side: SideClient, Platform: "linux-x64"}}},
	}}
	e := d.SelectEntry("", SideClient, "win-x64")
	if e == nil || e.Side != SideClient || e.McmodAsset() != 1 {
		t.Errorf("SelectEntry = %+v", e)
	}
	if d.SelectEntry("lua", SideClient, "win-x64") != nil || d.SelectEntry("x", SideClient, "win-x64") != nil {
		t.Error("SelectEntry matched a wrong entry")
	}
	if (&Entry{}).McmodAsset() != -1 {
		t.Error("McmodAsset of no assets")
	}
	if _, ok := d.EntriesOf("lua"); !ok {
		t.Error("EntriesOf(lua)")
	}
}
