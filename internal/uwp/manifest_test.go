package uwp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoricalManifestVersionsMatchCatalog(t *testing.T) {
	for _, tc := range []struct{ catalog, packaged, updateID string }{
		{"0.14.0.1", "0.140.1.0", "b17c2923-587a-442b-9bc6-76a13c02978c"},
		{"0.15.0.0", "0.150.0.0", "28354370-eb00-498c-b6cb-1b2dbbdd8042"},
		{"1.0.0.16", "1.0.16.0", "4234690b-30b9-46a4-a5f9-811953a151d5"},
	} {
		t.Run(tc.catalog, func(t *testing.T) {
			m := Manifest{Identity: Identity{Version: tc.packaged}}
			if got := m.GameVersion(); got != tc.catalog {
				t.Fatalf("update %s: package %s produced %s, want %s", tc.updateID, tc.packaged, got, tc.catalog)
			}
		})
	}
}

func TestInstallHistoricalUWPExecutable(t *testing.T) {
	manifest := strings.NewReplacer("1.21.9301.0", "0.140.1.0", "Minecraft.Windows.exe", "Minecraft.Win10.DX11.exe").Replace(testManifest("neutral"))
	archive := writeArchive(t, zipBytes(t, map[string]string{"AppxManifest.xml": manifest, "Minecraft.Win10.DX11.exe": "historical executable fixture"}))
	m, err := Install(context.Background(), archive, filepath.Join(t.TempDir(), "historical"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.GameVersion() != "0.14.0.1" || m.Applications[0].Executable != "Minecraft.Win10.DX11.exe" {
		t.Fatalf("unexpected historical instance: %+v", m)
	}
}
