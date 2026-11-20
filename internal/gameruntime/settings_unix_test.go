//go:build darwin

package gameruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func fixtureSettings(t *testing.T) types.WineSettings {
	t.Helper()
	root := t.TempDir()
	wine := filepath.Join(root, "engine with spaces", "wine")
	if err := os.MkdirAll(filepath.Dir(wine), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wine, []byte("#!/bin/sh\nprintf 'wine-test 1.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(root, "中文 prefix")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	return types.WineSettings{Executable: wine, Prefix: prefix, Graphics: "wined3d", Sync: "none"}
}

func TestWineSettingsPersistAndApplyToNextLaunch(t *testing.T) {
	s := fixtureSettings(t)
	path := filepath.Join(t.TempDir(), "wine.json")
	if err := saveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings(path)
	if err != nil || loaded != s {
		t.Fatalf("round trip: %+v %v", loaded, err)
	}
	p, err := profileFromSettings(loaded)
	if err != nil {
		t.Fatal(err)
	}
	env := strings.Join(p.Environment(), "\n")
	for _, value := range []string{"WINEPREFIX=" + s.Prefix, "WINELOADER=" + s.Executable, "WINEMSYNC=0", "WINEESYNC=0", "WINEFSYNC=0", "WINEDLLOVERRIDES=d3d11,d3d12,d3d12core,dxgi=b"} {
		if !strings.Contains(env, value) {
			t.Errorf("missing %s", value)
		}
	}
	version, err := CheckSettings(context.Background(), loaded)
	if err != nil || version != "wine-test 1.0" {
		t.Fatalf("version: %q %v", version, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("settings permissions: %v %v", info, err)
	}
}

func TestInvalidWineSettingsKeepLastSavedConfiguration(t *testing.T) {
	s := fixtureSettings(t)
	path := filepath.Join(t.TempDir(), "wine.json")
	if err := saveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []types.WineSettings{
		{Executable: s.Executable, Prefix: "/missing/prefix"},
		{Executable: "/missing/wine", Prefix: s.Prefix},
		{Executable: s.Executable, Prefix: s.Prefix, Graphics: "unknown"},
		{Executable: s.Executable, Prefix: s.Prefix, Sync: "unknown"},
		{Executable: s.Executable, Prefix: s.Prefix, Graphics: "d3dmetal"},
	} {
		if err := saveSettings(path, invalid); err == nil {
			t.Errorf("accepted invalid configuration: %+v", invalid)
		}
		loaded, err := loadSettings(path)
		if err != nil || loaded != s {
			t.Fatalf("invalid save overwrote settings: %+v %v", loaded, err)
		}
	}
}

func TestWineSettingsDoNotResetSavedFieldsFromEnvironment(t *testing.T) {
	s := fixtureSettings(t)
	t.Setenv("LEVI_WINE_APP", "/old/app")
	t.Setenv("LEVI_WINE_EXECUTABLE", "/old/wine")
	t.Setenv("LEVI_WINEPREFIX", "/old/prefix")
	got, err := resolveSettings(s)
	if err != nil || got != s {
		t.Fatalf("environment replaced saved configuration: %+v %v", got, err)
	}
}

func TestWineSyncModesAreMutuallyExclusive(t *testing.T) {
	for mode, enabled := range map[string]string{"msync": "WINEMSYNC", "esync": "WINEESYNC", "none": ""} {
		p := Profile{App: "/free/Wine.app", Wine: "/free/wine", Prefix: "/free/prefix", Sync: mode}
		env := strings.Join(p.Environment(), "\n")
		for _, key := range []string{"WINEMSYNC", "WINEESYNC", "WINEFSYNC"} {
			want := "0"
			if key == enabled {
				want = "1"
			}
			if !strings.Contains(env, key+"="+want) {
				t.Errorf("%s: incorrect %s", mode, key)
			}
		}
	}
}
