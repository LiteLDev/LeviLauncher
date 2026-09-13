package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportsEditorMode(t *testing.T) {
	for _, tc := range []struct {
		version, channel string
		want             bool
	}{
		{"1.21.49.99", "release", false}, {"1.21.50", "release", true},
		{"1.21.50.0", "Release", true}, {"1.21.50.00", "release", true},
		{"1.19.80.19", "preview", false}, {"1.19.80.20", "preview", true},
		{"1.19.80", "preview", false}, {"1.19.80.020", " Preview ", true},
		{"1.20.0.20", "release", false}, {"1.20.0.20", "preview", true},
		{"1.20.0.20", "beta", false}, {"1.21.50.20", "beta", true},
		{"1.21.119.0", "release", true}, {"1.21.120.0", "release", true},
		{"1.21.120.20", "preview", true}, {"1.21.120.21", "preview", true},
		{"26.20", "release", true}, {" 1.21.50 ", "", true},
		{"", "preview", false}, {"unknown", "release", false},
		{"1.21.50.0-preview", "preview", false}, {"1.21.50.0.1", "release", false},
		{"1.21.50.4294967296", "release", false}, {"1..50", "release", false},
	} {
		if got := SupportsEditorMode(tc.version, tc.channel); got != tc.want {
			t.Errorf("%q %q: got %v, want %v", tc.channel, tc.version, got, tc.want)
		}
	}
}

func TestEditorLaunchURIUsesPackageFormatAndChannel(t *testing.T) {
	for _, tc := range []struct{ packageType, channel, want string }{
		{PackageTypeUWP, "release", "minecraft:?Editor=true"},
		{PackageTypeUWP, "beta", "minecraft:?Editor=true"},
		{PackageTypeUWP, "preview", "minecraft-preview:?Editor=true"},
		{PackageTypeGDK, "release", "minecraft://creator/?Editor=true"},
		{PackageTypeGDK, "preview", "minecraft-preview://creator/?Editor=true"},
		{"", "", "minecraft://creator/?Editor=true"},
		{" UWP ", " Preview ", "minecraft-preview:?Editor=true"},
	} {
		if got := EditorLaunchURI(tc.packageType, tc.channel); got != tc.want {
			t.Errorf("%s %s: got %q, want %q", tc.packageType, tc.channel, got, tc.want)
		}
	}
}

func TestReadMetaGuardsEditorModeWithoutRewriting(t *testing.T) {
	for _, packageType := range []string{PackageTypeUWP, PackageTypeGDK} {
		for _, tc := range []struct {
			version, channel string
			want             bool
		}{
			{"1.21.49.99", "release", false}, {"1.21.50.0", "release", true},
			{"1.19.80.19", "preview", false}, {"1.19.80.20", "preview", true},
			{"", "preview", false}, {"invalid", "release", false},
		} {
			dir := t.TempDir()
			if err := WriteMeta(dir, VersionMeta{PackageType: packageType, GameVersion: tc.version, Type: tc.channel, EnableEditorMode: true}); err != nil {
				t.Fatal(err)
			}
			meta, err := ReadMeta(dir)
			if err != nil || meta.EnableEditorMode != tc.want {
				t.Fatalf("%s %s %s: %+v %v", packageType, tc.channel, tc.version, meta, err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, metaFileName))
			if err != nil || !strings.Contains(string(raw), `"enableEditorMode": true`) {
				t.Fatalf("reading metadata modified stored preference: %s %v", raw, err)
			}
		}
	}
}
