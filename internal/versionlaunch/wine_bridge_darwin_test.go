package versionlaunch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLeviWineBridgeSkipsDisabledMod(t *testing.T) {
	instance := t.TempDir()
	var want string
	for _, fixture := range []struct {
		dir, manifest, entry string
	}{
		{"LeviClient", "manifest.json.close", "LeviWineBridge.dll"},
		{"OtherMod", "manifest.json", "Other.dll"},
		{"RenamedBridge", "manifest.json", "LeviWineBridge.dll"},
	} {
		dir := filepath.Join(instance, "mods", fixture.dir)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		dll := filepath.Join(dir, "LeviWineBridge.dll")
		if err := os.WriteFile(dll, nil, 0600); err != nil {
			t.Fatal(err)
		}
		manifest := []byte(`{"entry":"` + fixture.entry + `","type":"preload-native"}`)
		if err := os.WriteFile(filepath.Join(dir, fixture.manifest), manifest, 0600); err != nil {
			t.Fatal(err)
		}
		if fixture.dir == "RenamedBridge" {
			want = dll
		}
	}
	if got := findLeviWineBridge(instance); got != want {
		t.Fatalf("findLeviWineBridge = %q, want %q", got, want)
	}
}
