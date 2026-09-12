package uwpdownload

import (
	"strings"
	"testing"
)

const testID = "985D1EE4-0E9D-49DE-9A99-E208ADC08D0C"

func TestCatalogChannelsAndNumericOrder(t *testing.T) {
	input := `[["1.9.0.0","` + testID + `",0],["1.10.0.0","` + testID + `",1],["1.10.0.0","` + testID + `",2]]`
	got, err := ParseCatalog(strings.NewReader(input))
	if err != nil || len(got) != 3 {
		t.Fatalf("catalog: %v %v", got, err)
	}
	if got[0].Version != "1.10.0.0" || got[0].Type != "beta" || got[1].Type != "preview" || got[2].Type != "release" {
		t.Fatalf("channels/order: %+v", got)
	}
	for _, v := range got {
		if v.PackageType != "uwp" || v.UUID != strings.ToLower(testID) {
			t.Fatalf("metadata: %+v", v)
		}
	}
}

func TestRejectInvalidCatalogAndFilename(t *testing.T) {
	for _, input := range []string{`{}`, `[[1,2,3]]`, `[["1.2.3.4","bad",0]]`, `[["../x","` + testID + `",0]]`, `[["1.2.3.4","` + testID + `",3]]`, `[] {}`} {
		if _, err := ParseCatalog(strings.NewReader(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	for _, version := range []string{"../1.2.3.4", "1/2/3/4", "1.2.3.4:ads", ""} {
		if _, err := Filename(version, "release"); err == nil {
			t.Errorf("accepted filename %q", version)
		}
	}
	if _, err := Filename("1.2.3.4", "Release"); err == nil {
		t.Fatal("accepted unknown channel")
	}
}

func TestCacheChannelsDoNotCollide(t *testing.T) {
	seen := map[string]bool{}
	for _, channel := range []string{"release", "beta", "preview"} {
		name, err := Filename("1.20.0.0", channel)
		if err != nil || seen[name] || !strings.HasSuffix(name, ".appx") {
			t.Fatalf("cache filename: %q %v", name, err)
		}
		seen[name] = true
	}
}
