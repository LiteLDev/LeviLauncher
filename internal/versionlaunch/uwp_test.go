package versionlaunch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func TestUWPNativeArchitectureDoesNotBreakVanillaLaunch(t *testing.T) {
	fixture, err := os.ReadFile("../leviloader/LeviLauncher.dll")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		x64, console     bool
		mod              string
		prepare, wantErr bool
	}{
		{"x64", true, false, "", true, false},
		{"vanilla x86", false, false, "", false, false},
		{"console x86", false, true, "", false, true},
		{"native mod x86", false, false, "manifest.json", false, true},
		{"disabled mod x86", false, false, "manifest.json.close", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apppath.SetBaseRootOverride(t.TempDir())
			t.Cleanup(func() { apppath.SetBaseRootOverride("") })
			root, err := apppath.VersionsDir()
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "instance")
			if err := os.MkdirAll(filepath.Join(dir, "mods", "native"), 0755); err != nil {
				t.Fatal(err)
			}
			image := append([]byte(nil), fixture...)
			if !tc.x64 {
				coff := int(binary.LittleEndian.Uint32(image[60:64])) + 4
				binary.LittleEndian.PutUint16(image[coff:], 0x14c)
			}
			exe := filepath.Join(dir, "Minecraft.Windows.exe")
			if err := os.WriteFile(exe, image, 0644); err != nil {
				t.Fatal(err)
			}
			if tc.mod != "" {
				if err := os.WriteFile(filepath.Join(dir, "mods", "native", tc.mod), []byte(`{"type":"preload-native","entry":"test.dll"}`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			prepare, err := uwpNativePreparation(dir, "instance", exe, tc.console)
			if prepare != tc.prepare || (err != nil) != tc.wantErr {
				t.Fatalf("prepare=%v error=%v", prepare, err)
			}
		})
	}
}
