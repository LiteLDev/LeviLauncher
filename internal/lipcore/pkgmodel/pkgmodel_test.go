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
			{Platform: PlatformServer, Runtime: "win-x64"},
			{Platform: PlatformClient, Runtime: "win-x64", Assets: []Asset{{Type: AssetZip}, {Type: AssetMcmod}}},
		}},
		{Name: "lua", Entries: []Entry{{Platform: PlatformClient, Runtime: "linux-x64"}}},
	}}
	e := d.SelectEntry("", PlatformClient, "win-x64")
	if e == nil || e.Platform != PlatformClient || e.McmodAsset() != 1 {
		t.Errorf("SelectEntry = %+v", e)
	}
	if d.SelectEntry("lua", PlatformClient, "win-x64") != nil || d.SelectEntry("x", PlatformClient, "win-x64") != nil {
		t.Error("SelectEntry matched a wrong entry")
	}
	if (&Entry{}).McmodAsset() != -1 {
		t.Error("McmodAsset of no assets")
	}
	if _, ok := d.EntriesOf("lua"); !ok {
		t.Error("EntriesOf(lua)")
	}
}
