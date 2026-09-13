package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportsIsolation(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"1.19.70.1", false}, {"1.19.70.2", true}, {"1.19.70.02", true},
		{"1.19.70.3", true}, {"1.19.69.99", false}, {"1.19.70", false},
		{"1.20", true}, {"1.9.100.0", false}, {"1.21.100.0", true},
		{" 1.19.70.2 ", true}, {"", false}, {"unknown", false},
		{"1.19.70.2-preview", false}, {"1.19.70.2.0", false}, {"1.19.70.4294967296", false},
	}
	for _, tc := range cases {
		if got := SupportsIsolation(PackageTypeUWP, tc.version); got != tc.want {
			t.Errorf("UWP %q: got %v, want %v", tc.version, got, tc.want)
		}
		if !SupportsIsolation(PackageTypeGDK, tc.version) {
			t.Errorf("GDK isolation must remain available for %q", tc.version)
		}
	}
}

func TestReadMetaDisablesUnsupportedUWPIsolationWithoutRewriting(t *testing.T) {
	dir := t.TempDir()
	if err := WriteMeta(dir, VersionMeta{PackageType: PackageTypeUWP, GameVersion: "1.19.70.1", EnableIsolation: true}); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir)
	if err != nil || meta.EnableIsolation {
		t.Fatalf("unsupported isolation remains active: %+v %v", meta, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, metaFileName))
	if err != nil || !strings.Contains(string(raw), `"enableIsolation": true`) {
		t.Fatalf("reading metadata must not modify it: %s %v", raw, err)
	}
}
