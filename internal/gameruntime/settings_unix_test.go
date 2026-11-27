//go:build darwin

package gameruntime

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
	return types.WineSettings{
		Executable:      wine,
		Prefix:          prefix,
		Graphics:        "wined3d",
		D3DMetalVersion: "auto",
		Sync:            "none",
		ThreadQoS:       "balanced",
		VSync:            "auto",
		MetalFXScale:     "off",
		MetalFXSharpness: "auto",
	}
}

func TestWineSettingsPersistAndApplyToNextLaunch(t *testing.T) {
	s := fixtureSettings(t)
	path := filepath.Join(t.TempDir(), "wine.json")
	if err := saveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings(path)
	if err != nil || !reflect.DeepEqual(loaded, s) {
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
		if err != nil || !reflect.DeepEqual(loaded, s) {
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
	if err != nil || !reflect.DeepEqual(got, s) {
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

func TestRetinaMetalFXDefaultScalePolicy(t *testing.T) {
	base := fixtureSettings(t)
	base.Graphics = "auto"
	base.MetalFXScale = ""
	base.EnableMetalFX = true

	base.RetinaMode = true
	gotRetina, err := resolveSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if gotRetina.MetalFXScale != "performance" || !gotRetina.EnableMetalFX {
		t.Fatalf("retina + legacy EnableMetalFX = %q (enabled=%t), want performance", gotRetina.MetalFXScale, gotRetina.EnableMetalFX)
	}

	base.RetinaMode = false
	gotStandard, err := resolveSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if gotStandard.MetalFXScale != "balanced" || !gotStandard.EnableMetalFX {
		t.Fatalf("non-retina + legacy EnableMetalFX = %q (enabled=%t), want balanced", gotStandard.MetalFXScale, gotStandard.EnableMetalFX)
	}

	if got := metalFXFactor("", true, true); got != "2.00" {
		t.Fatalf("metalFXFactor(\"\", true, true) = %q, want 2.00", got)
	}
	if got := metalFXFactor("", true, false); got != "1.50" {
		t.Fatalf("metalFXFactor(\"\", true, false) = %q, want 1.50", got)
	}
	for preset, want := range map[string]string{
		"quality":           "1.33",
		"balanced":          "1.50",
		"performance":       "2.00",
		"ultra_performance": "2.50",
		"extreme":           "3.00",
		"max":               "4.00",
	} {
		if got := metalFXFactor(preset, true, true); got != want {
			t.Fatalf("metalFXFactor(%q) = %q, want %q", preset, got, want)
		}
		p := Profile{Graphics: "d3dmetal", MetalFXScale: preset}
		env := p.D3DMetalFXLaunchEnv(true)
		if env["LEVI_METALFX_SPATIAL_FACTOR"] != want || env["LEVI_WINE_RENDER_SCALE"] != want ||
			env["LEVI_METALFX_RETINA_UPSCALE"] != "" {
			t.Errorf("standard-resolution %s scaling = %v, want paired %s scaling", preset, env, want)
		}
		if env := p.D3DMetalFXLaunchEnv(false); len(env) != 0 {
			t.Errorf("standard-resolution %s without the bridge = %v", preset, env)
		}
	}

	p2x := Profile{Graphics: "d3dmetal", RetinaMode: true, EnableMetalFX: true, MetalFXScale: "performance"}
	if env := p2x.D3DMetalFXLaunchEnv(true); env["LEVI_METALFX_RETINA_UPSCALE"] != "1" || env["LEVI_WINE_RENDER_SCALE"] != "" {
		t.Fatalf("2.0x D3DMetalFXLaunchEnv = %v, want pure retina upscale without Win32 scaling", env)
	}
	p4x := Profile{Graphics: "d3dmetal", RetinaMode: true, EnableMetalFX: true, MetalFXScale: "max"}
	if env := p4x.D3DMetalFXLaunchEnv(true); env["LEVI_METALFX_RETINA_UPSCALE"] != "1" || env["LEVI_METALFX_SPATIAL_FACTOR"] != "4.00" || env["LEVI_WINE_RENDER_SCALE"] != "2.00" {
		t.Fatalf("4.0x D3DMetalFXLaunchEnv = %v, want retina upscale + 4.00 spatial + 2.00 Win32 scale", env)
	}
	pQualityRetina := Profile{Graphics: "d3dmetal", RetinaMode: true, EnableMetalFX: true, MetalFXScale: "quality"}
	if env := pQualityRetina.D3DMetalFXLaunchEnv(true); env["LEVI_METALFX_RETINA_UPSCALE"] != "" || env["LEVI_METALFX_SPATIAL_FACTOR"] != "1.33" || env["LEVI_WINE_RENDER_SCALE"] != "1.33" {
		t.Fatalf("1.33x Retina D3DMetalFXLaunchEnv = %v, want 1.33 paired scaling", env)
	}
}

func TestMetalFXSharpeningPolicy(t *testing.T) {
	base := fixtureSettings(t)
	resolved, err := resolveSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.MetalFXSharpness != "auto" {
		t.Fatalf("default MetalFXSharpness = %q, want auto", resolved.MetalFXSharpness)
	}

	invalid := base
	invalid.MetalFXSharpness = "99"
	if _, err := profileFromSettings(invalid); err == nil {
		t.Fatalf("expected invalid MetalFXSharpness to fail validation")
	}

	autoCases := []struct {
		scale string
		want  string
	}{
		{"off", ""},
		{"quality", ""},
		{"balanced", ""},
		{"performance", "0.15"},
		{"ultra_performance", "0.20"},
		{"extreme", "0.25"},
		{"max", "0.25"},
	}
	for _, tc := range autoCases {
		if got := metalFXSharpnessValue("auto", tc.scale, tc.scale != "off", false); got != tc.want {
			t.Errorf("auto sharpness for scale %q = %q, want %q", tc.scale, got, tc.want)
		}
	}

	for level, want := range map[string]string{
		"0":  "",
		"10": "0.10",
		"15": "0.15",
		"25": "0.25",
		"35": "0.35",
	} {
		if got := metalFXSharpnessValue(level, "balanced", true, false); got != want {
			t.Errorf("explicit sharpness %q = %q, want %q", level, got, want)
		}
		if got := metalFXSharpnessValue(level, "off", false, false); got != "" {
			t.Errorf("sharpness %q with MetalFX off = %q, want empty", level, got)
		}
	}

	pD3D := Profile{Graphics: "d3dmetal", RetinaMode: true, MetalFXScale: "performance", MetalFXSharpness: "auto"}
	if env := pD3D.D3DMetalFXLaunchEnv(false); env["LEVI_METALFX_SHARPNESS"] != "0.15" || env["LEVI_METALFX_SHARPEN_ONLY"] != "" {
		t.Fatalf("D3DMetal 2x auto sharpen env = %v", env)
	}
	if env := pD3D.MetalFXSharpenLaunchEnv(); len(env) != 0 {
		t.Fatalf("D3DMetal should not use sharpen-only env, got %v", env)
	}

	pD3DOff := Profile{Graphics: "d3dmetal", RetinaMode: true, MetalFXScale: "performance", MetalFXSharpness: "0"}
	if env := pD3DOff.D3DMetalFXLaunchEnv(false); env["LEVI_METALFX_SHARPNESS"] != "" {
		t.Fatalf("D3DMetal sharpness=0 should omit LEVI_METALFX_SHARPNESS, got %v", env)
	}

	pMullion := Profile{Graphics: "mullion", MetalFXScale: "performance", MetalFXSharpness: "25"}
	if env := pMullion.MetalFXSharpenLaunchEnv(); env["LEVI_METALFX_SHARPNESS"] != "0.25" || env["LEVI_METALFX_SHARPEN_ONLY"] != "1" {
		t.Fatalf("Mullion sharpen-only env = %v", env)
	}
	pVKD3D := Profile{Graphics: "vkd3d", MetalFXScale: "balanced", MetalFXSharpness: "auto"}
	if env := pVKD3D.MetalFXSharpenLaunchEnv(); len(env) != 0 {
		t.Fatalf("VKD3D 1.5x auto sharpness should be inactive, got %v", env)
	}
	pVKD3DExplicit := Profile{Graphics: "vkd3d", MetalFXScale: "balanced", MetalFXSharpness: "10"}
	if env := pVKD3DExplicit.MetalFXSharpenLaunchEnv(); env["LEVI_METALFX_SHARPNESS"] != "0.10" || env["LEVI_METALFX_SHARPEN_ONLY"] != "1" {
		t.Fatalf("VKD3D explicit sharpen-only env = %v", env)
	}
}
