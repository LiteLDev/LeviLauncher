package gameruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func writeWineFixture(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func metalSharpFixture(t *testing.T) types.WineSettings {
	t.Helper()
	app := filepath.Join(t.TempDir(), "Wine with spaces.app")
	s := managedWineSettings(app)
	writeWineFixture(t, s.Executable, "#!/bin/sh\nexit 0\n", 0700)
	writeWineFixture(t, filepath.Join(app, "Contents/lib/wine/x86_64-unix/ntdll.so"), "", 0600)
	writeWineFixture(t, filepath.Join(app, "Contents/lib/dxmt/x86_64-unix/winemetal.so"), "", 0600)
	writeWineFixture(t, filepath.Join(app, metalSharpPayload, "vkd3d/vkd3d-proton/x86_64-windows/d3d12.dll"), "", 0600)
	for _, name := range []string{"system.reg", "user.reg", "drive_c/windows/system32/cmd.exe"} {
		writeWineFixture(t, filepath.Join(s.Prefix, name), "", 0600)
	}
	if err := os.MkdirAll(filepath.Join(s.Prefix, "dosdevices"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/", filepath.Join(s.Prefix, "dosdevices", "z:")); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMetalSharpExternalPrefixAndDirectLaunch(t *testing.T) {
	if err := CheckManagedWineRequirements(context.Background()); err != nil {
		t.Skip(err)
	}
	s := metalSharpFixture(t)
	p, err := profileFromSettings(types.WineSettings{App: s.App})
	if err != nil || p.Prefix != s.Prefix || p.Wine != s.Executable {
		t.Fatalf("MetalSharp profile = %+v, %v", p, err)
	}
	t.Setenv("WINEDLLPATH", "/legacy/wine")
	t.Setenv("DYLD_FALLBACK_LIBRARY_PATH", "/legacy/lib")
	t.Setenv("D3DMETAL_RUNTIME_DIR", "/legacy/gptk")
	t.Setenv("GRAPHICS_BACKEND", "wine")
	t.Setenv("MULLION_CONFIG", "host config")
	p.MaxFPS, p.EnableMetalFX, p.MetalFXScale, p.MetalHUD = 120, true, "balanced", true
	gameDir := t.TempDir()
	exe := filepath.Join(gameDir, "Minecraft.Windows.exe")
	logPath := filepath.Join(gameDir, "wine-launch.log")
	cmd, detached, err := p.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{
		WorkDir:  gameDir,
		LogPath:  logPath,
		ExtraEnv: map[string]string{"LEVI_WINE_INPUT_FIX": "1"},
	}, "-Editor", "true")
	guest := "Z:" + strings.ReplaceAll(exe, "/", `\`)
	guestDir := "Z:" + strings.ReplaceAll(gameDir, "/", `\`)
	if err != nil || !detached || cmd.Path != "/usr/bin/open" {
		t.Fatalf("expected LaunchServices launch for Game Mode, got %v, detached=%t, err=%v", cmd, detached, err)
	}
	joinedArgs := strings.Join(cmd.Args, "\n")
	for _, expected := range []string{
		"-n\n-a\n" + s.App,
		"-o\n" + logPath + "\n--stderr\n" + logPath,
		"--env\nWINEPREFIX=" + s.Prefix,
		"--env\nGRAPHICS_BACKEND=dxmt",
		"--env\nWINEMSYNC=1",
		"--env\nWINEDLLOVERRIDES=mscoree,mshtml=",
		"--env\nWINE_THREAD_QOS=1",
		"--env\nROSETTA_ADVERTISE_AVX=1",
		"--env\nMULLION_METALFX_SPATIAL_SWAPCHAIN=1",
		"--env\nMTL_HUD_ENABLED=1",
		"--env\nLEVI_WINE_INPUT_FIX=1",
		"--env\nLEVI_WINE_WORKDIR=" + guestDir,
		"d3d12.preferredMaxFrameRate = 120",
		"d3d12.metalSpatialUpscale = True",
		"--args\n" + guest + "\n-Editor\ntrue",
	} {
		if !strings.Contains(joinedArgs, expected) {
			t.Errorf("missing %q in open args", expected)
		}
	}
	for _, legacy := range []string{"WINEDLLPATH=", "DYLD_", "D3DMETAL_RUNTIME_DIR=", "host config"} {
		if strings.Contains(joinedArgs, legacy) {
			t.Errorf("inherited %s", legacy)
		}
	}
	// Verify D3DMetal lane passes MetalFX, FPS cap, and HUD stats.
	p.Graphics = "d3dmetal"
	d3dEnv := strings.Join(p.Environment(), "\n")
	for _, expected := range []string{
		"GRAPHICS_BACKEND=d3dmetal",
		"D3DM_ENABLE_METALFX=1",
		"D3DM_MAX_FPS=120",
		"D3DM_SHOW_HUD_STATS=1",
		"ROSETTA_ADVERTISE_AVX=1",
	} {
		if !strings.Contains(d3dEnv, expected) {
			t.Errorf("D3DMetal missing %s", expected)
		}
	}
	// Verify Mullion overrides mullion.conf's 60 FPS cap when MaxFPS is 0.
	p.Graphics, p.MaxFPS, p.EnableMetalFX, p.MetalFXScale = "auto", 0, false, "off"
	mullionEnv := strings.Join(p.Environment(), "\n")
	if !strings.Contains(mullionEnv, "d3d11.preferredMaxFrameRate = 0") || !strings.Contains(mullionEnv, "d3d12.preferredMaxFrameRate = 0") {
		t.Errorf("Mullion did not override default 60 FPS cap when MaxFPS=0: %s", mullionEnv)
	}
	// Verify MetalFX scale levels, ThreadQoS, Rosetta AVX disable, D3DMetal version, and compat debug logging.
	p.MetalFXScale = "performance"
	p.ThreadQoS = "aggressive"
	p.DisableRosettaAVX = true
	p.CompatDebugLog = true
	tunedEnv := strings.Join(p.Environment(), "\n")
	for _, expected := range []string{
		"WINE_THREAD_QOS=2",
		"d3d12.metalSpatialUpscaleFactor = 2.00",
		"XGR_TRACE=1",
		"MCWINRT_TRACE=1",
	} {
		if !strings.Contains(tunedEnv, expected) {
			t.Errorf("tuned env missing %s", expected)
		}
	}
	if strings.Contains(tunedEnv, "ROSETTA_ADVERTISE_AVX=") {
		t.Errorf("ROSETTA_ADVERTISE_AVX should be omitted when DisableRosettaAVX=true")
	}
	p.Graphics, p.D3DMetalVersion = "d3dmetal", "gptk3"
	if got := p.GraphicsBackend(); got != "D3DMetal (GPTK 3)" {
		t.Errorf("explicit GPTK 3 backend label = %q", got)
	}
	if !strings.Contains(strings.Join(p.Environment(), "\n"), "d3dmetal-gptk3") {
		t.Errorf("explicit GPTK 3 did not select d3dmetal-gptk3 runtime dir")
	}

	// Verify Metal 3 compatibility is enforced by default on Mullion and VKD3D.
	p.Graphics, p.EnableMetal4 = "mullion", false
	mullionM3Env := strings.Join(p.Environment(), "\n")
	for _, expected := range []string{"MULLION_D3D12_METAL4=0", "mullion.shaderMetalVersion = 320"} {
		if !strings.Contains(mullionM3Env, expected) {
			t.Errorf("Mullion Metal 3 default missing %s", expected)
		}
	}
	p.Graphics = "vkd3d"
	if !strings.Contains(strings.Join(p.Environment(), "\n"), "METALSHARP_METAL_VERSION=3") {
		t.Errorf("VKD3D Metal 3 default missing METALSHARP_METAL_VERSION=3")
	}

	// Verify auto defaults to D3DMetal when bundled D3DMetal is present, and enforces D3DM_MTL4=0 on GPTK 4 by default.
	d3dAutoDir, _ := p.metalSharp.d3dMetalFor("auto")
	writeWineFixture(t, filepath.Join(d3dAutoDir, "external/D3DMetal.framework/D3DMetal"), "", 0600)
	pAuto, err := profileFromSettings(types.WineSettings{App: s.App})
	if err != nil {
		t.Fatalf("profileFromSettings with bundled D3DMetal: %v", err)
	}
	if !strings.HasPrefix(pAuto.GraphicsBackend(), "D3DMetal (") {
		t.Errorf("auto with bundled D3DMetal selected %q, want D3DMetal", pAuto.GraphicsBackend())
	}
	autoEnv := strings.Join(pAuto.Environment(), "\n")
	if !strings.Contains(autoEnv, "GRAPHICS_BACKEND=d3dmetal") {
		t.Errorf("auto with bundled D3DMetal did not set GRAPHICS_BACKEND=d3dmetal: %s", autoEnv)
	}
	if strings.Contains(filepath.Base(d3dAutoDir), "gptk4") {
		if !strings.Contains(autoEnv, "D3DM_MTL4=0") {
			t.Errorf("GPTK 4 default did not enforce D3DM_MTL4=0: %s", autoEnv)
		}
		pAuto.EnableMetal4 = true
		if !strings.Contains(strings.Join(pAuto.Environment(), "\n"), "D3DM_MTL4=1") {
			t.Errorf("GPTK 4 with EnableMetal4=true did not set D3DM_MTL4=1")
		}
	}
	pAuto.MetalFXScale = "performance"
	if got := pAuto.EffectiveGraphics(); got != "d3dmetal" {
		t.Errorf("EffectiveGraphics = %q, want d3dmetal", got)
	}
	if got := pAuto.SpatialMetalFXFactor(); got != "2.00" {
		t.Errorf("SpatialMetalFXFactor = %q, want 2.00", got)
	}

	// Verify native Metal pacing helper applies VSync override and avoids double-throttling.
	pacingEnv := map[string]string{
		"D3DM_MAX_FPS":   "120",
		"MULLION_CONFIG": "d3d11.preferredMaxFrameRate = 120; d3d12.preferredMaxFrameRate = 120",
	}
	applyNativeMetalPacing(pacingEnv, Profile{VSync: "off", MaxFPS: 120})
	if pacingEnv["LEVI_METAL_VSYNC"] != "0" || pacingEnv["LEVI_METAL_MAX_FPS"] != "120" {
		t.Errorf("native pacing env = %+v", pacingEnv)
	}
	if _, exists := pacingEnv["D3DM_MAX_FPS"]; exists {
		t.Errorf("D3DM_MAX_FPS should be cleared when LEVI_METAL_MAX_FPS owns pacing")
	}
	if !strings.Contains(pacingEnv["MULLION_CONFIG"], "d3d12.preferredMaxFrameRate = 0") {
		t.Errorf("MULLION_CONFIG should uncap when LEVI_METAL_MAX_FPS owns pacing: %q", pacingEnv["MULLION_CONFIG"])
	}
	s.Prefix = filepath.Join(s.App, "prefix")
	for _, name := range []string{"system.reg", "user.reg", "drive_c/windows/system32/cmd.exe"} {
		writeWineFixture(t, filepath.Join(s.Prefix, name), "", 0600)
	}
	if _, err := profileFromSettings(s); err == nil {
		t.Fatal("accepted a prefix inside the signed bundle")
	}
}

func TestWineShaderCacheInfoAndClear(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	info, err := GetWineShaderCacheInfo()
	if err != nil || info.SizeBytes != 0 || info.FileCount != 0 {
		t.Fatalf("empty cache info = %+v, %v", info, err)
	}
	writeWineFixture(t, filepath.Join(wineShaderCacheRoot(), "mullion", "cache.bin"), "1234567890", 0600)
	info, err = GetWineShaderCacheInfo()
	if err != nil || info.SizeBytes != 10 || info.FileCount != 1 {
		t.Fatalf("populated cache info = %+v, %v", info, err)
	}
	if err := ClearWineShaderCache(); err != nil {
		t.Fatalf("ClearWineShaderCache: %v", err)
	}
	info, err = GetWineShaderCacheInfo()
	if err != nil || info.SizeBytes != 0 || info.FileCount != 0 {
		t.Fatalf("cleared cache info = %+v, %v", info, err)
	}
}

func TestLegacyWineInfoAndCleanup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("LOCALAPPDATA", root)
	oldDir := legacyManagedWineDir()
	oldApp := legacyManagedWineApp()
	writeWineFixture(t, filepath.Join(oldApp, "Contents/SharedSupport/wine/bin/wine"), "#!/bin/sh\n", 0700)
	writeWineFixture(t, filepath.Join(oldApp, "Contents/SharedSupport/prefix/drive_c/marker.txt"), "12345", 0600)
	if err := SaveSettings(types.WineSettings{App: oldApp}); err != nil {
		t.Fatal(err)
	}
	info, err := GetLegacyWineInfo()
	if err != nil || !info.Present || !info.Active || info.SizeBytes < 5 {
		t.Fatalf("active legacy info = %+v, %v", info, err)
	}
	if err := RemoveLegacyWineRuntime(); err == nil || !strings.Contains(err.Error(), "ERR_WINE_LEGACY_IN_USE") {
		t.Fatalf("expected ERR_WINE_LEGACY_IN_USE, got %v", err)
	}
	otherApp := filepath.Join(root, "Other.app")
	writeWineFixture(t, filepath.Join(otherApp, "Contents/SharedSupport/wine/bin/wine"), "#!/bin/sh\n", 0700)
	writeWineFixture(t, filepath.Join(otherApp, "Contents/SharedSupport/prefix/drive_c/marker.txt"), "", 0600)
	if err := SaveSettings(types.WineSettings{App: otherApp}); err != nil {
		t.Fatal(err)
	}
	info, err = GetLegacyWineInfo()
	if err != nil || !info.Present || info.Active {
		t.Fatalf("inactive legacy info = %+v, %v", info, err)
	}
	if err := RemoveLegacyWineRuntime(); err != nil {
		t.Fatalf("RemoveLegacyWineRuntime: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("legacy directory still exists: %v", err)
	}
}

func TestMetalSharpGraphicsValidation(t *testing.T) {
	if err := CheckManagedWineRequirements(context.Background()); err != nil {
		t.Skip(err)
	}
	s := metalSharpFixture(t)
	s.Graphics = "vkd3d"
	p, err := profileFromSettings(s)
	if err != nil || p.GraphicsBackend() != "VKD3D-Proton" {
		t.Fatalf("VKD3D: %v, %v", p, err)
	}
	s.Graphics = "d3dmetal"
	if _, err := profileFromSettings(s); !errors.Is(err, ErrD3DMetalUnavailable) {
		t.Fatalf("missing D3DMetal: %v", err)
	}
	s.Graphics = "auto"
	if err := os.Remove(filepath.Join(s.App, "Contents/lib/dxmt/x86_64-unix/winemetal.so")); err != nil {
		t.Fatal(err)
	}
	if _, err := profileFromSettings(s); !errors.Is(err, ErrGraphicsBackendUnavailable) {
		t.Fatalf("missing Mullion: %v", err)
	}
}

func TestMetalSharpWinebootRetriesPartialPrefix(t *testing.T) {
	s := metalSharpFixture(t)
	writeWineFixture(t, s.Executable, "#!/bin/sh\nexit 1\n", 0700)
	if err := initializeMetalSharp(context.Background(), s); err == nil {
		t.Fatal("partial prefix was treated as initialized")
	}
	writeWineFixture(t, s.Executable, "#!/bin/sh\ntest \"$1:$2\" = wineboot:-u\n", 0700)
	writeWineFixture(t, filepath.Join(filepath.Dir(s.Executable), "wineserver"), "#!/bin/sh\ntest \"$1\" = -w\n", 0700)
	if err := initializeMetalSharp(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	writeWineFixture(t, s.Executable, "#!/bin/sh\nexit 1\n", 0700)
	if err := initializeMetalSharp(context.Background(), s); err != nil {
		t.Fatalf("initialized prefix ran wineboot again: %v", err)
	}
}

func TestManagedUpgradeCopiesAccountCacheOnce(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	old := filepath.Join(apppath.LocalDir(), "Runtimes", "Sikarugir", "Levi Wine.app")
	previous := types.WineSettings{App: old, Prefix: filepath.Join(old, "Contents/SharedSupport/prefix")}
	path := "drive_c/users/qa/AppData/Roaming/xgr/auth.json"
	writeWineFixture(t, filepath.Join(previous.Prefix, path), `{"account":"old"}`, 0600)
	next := t.TempDir()
	dst := filepath.Join(next, path)
	if err := os.MkdirAll(filepath.Join(next, "drive_c/users/qa"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{"account":"old"}`, `{"account":"new"}`} {
		if err := migrateLegacyWineAccount(previous, next); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(dst)
		if err != nil || string(got) != want {
			t.Fatalf("account cache: %q, %v", got, err)
		}
		info, err := os.Stat(dst)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("account cache permissions: %v, %v", info, err)
		}
		writeWineFixture(t, dst, `{"account":"new"}`, 0600)
	}
}

func TestRemoveLegacyWineRequiresReadableSettings(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	legacyPrefix := filepath.Join(legacyManagedWineApp(), "Contents/SharedSupport/prefix")
	sentinel := filepath.Join(legacyPrefix, "user.reg")
	writeWineFixture(t, sentinel, "legacy prefix", 0600)

	for _, contents := range []string{"", "{"} {
		if contents != "" {
			writeWineFixture(t, settingsPath(), contents, 0600)
		}
		if err := RemoveLegacyWineRuntime(); err == nil || !strings.Contains(err.Error(), "ERR_WINE_LEGACY_SETTINGS") {
			t.Fatalf("RemoveLegacyWineRuntime with settings %q: %v", contents, err)
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAutoD3DMetalFallsBackToCompatibleGPTK(t *testing.T) {
	osVersion := macOSVersion
	t.Cleanup(func() { macOSVersion = osVersion })
	macOSVersion = func() [2]int { return [2]int{27, 0} }

	m := metalSharpRuntime{app: t.TempDir()}
	gptk3, _ := m.d3dMetalFor("gptk3")
	writeWineFixture(t, filepath.Join(gptk3, "external/D3DMetal.framework/D3DMetal"), "", 0600)
	if variants := strings.Join(m.availableD3DMetalVersions(), ","); variants != "auto,gptk3" {
		t.Fatalf("available selectors = %s", variants)
	}
	if got := m.effectiveGraphics("auto"); got != "d3dmetal" {
		t.Fatalf("automatic backend = %s", got)
	}
	if dir, _ := m.d3dMetalFor("auto"); dir != gptk3 {
		t.Fatalf("automatic toolkit = %s, want GPTK 3", dir)
	}
	p := Profile{metalSharp: m, Graphics: "auto", EnableMetal4: true, RetinaMode: true, MetalFXScale: "max"}
	if env := p.D3DMetalFXLaunchEnv(true); env["LEVI_WINE_RENDER_SCALE"] != "2.00" {
		t.Fatalf("GPTK 3 scaling = %v", env)
	}

	gptk4, _ := m.d3dMetalFor("gptk4")
	writeWineFixture(t, filepath.Join(gptk4, "external/D3DMetal.framework/D3DMetal"), "", 0600)
	if dir, _ := m.d3dMetalFor("auto"); dir != gptk4 {
		t.Fatalf("automatic toolkit = %s, want GPTK 4", dir)
	}
	if env := p.D3DMetalFXLaunchEnv(true); len(env) != 0 {
		t.Fatalf("Metal 4 must disable both custom scaling paths, got %v", env)
	}

	macOSVersion = func() [2]int { return [2]int{15, 3} }
	if variants := strings.Join(m.availableD3DMetalVersions(), ","); variants != "auto" {
		t.Fatalf("macOS 15.3 selectors = %s", variants)
	}
	if got := m.effectiveGraphics("auto"); got != "mullion" {
		t.Fatalf("macOS 15.3 automatic backend = %s", got)
	}
}

func TestStagedMetalSharpIsNotSelectedWithoutSettings(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("LEVI_WINE_APP", "")
	t.Setenv("LEVI_WINE_EXECUTABLE", "")
	staged := managedWineSettings(managedWineApp())
	writeWineFixture(t, staged.Executable, "#!/bin/sh\nexit 0\n", 0700)
	writeWineFixture(t, filepath.Join(staged.App, "Contents/lib/wine/x86_64-unix/ntdll.so"), "", 0600)
	if err := os.MkdirAll(filepath.Join(staged.App, metalSharpPayload), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staged.Prefix, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveSettings(types.WineSettings{})
	if err != nil || resolved.App == staged.App {
		t.Fatalf("staging changed the selected Wine runtime: %q, %v", resolved.App, err)
	}
	if _, err := profileFromSettings(staged); !errors.Is(err, ErrPrefixUninitialized) {
		t.Fatalf("incomplete prefix passed validation: %v", err)
	}
	if err := CheckManagedWineRequirements(context.Background()); err == nil {
		items, err := scanLocations(context.Background(), []string{filepath.Dir(staged.App)}, nil, t.TempDir(), types.WineSettings{})
		if err != nil || len(items) != 1 || items[0].Status != "needs_prefix" {
			t.Fatalf("discovery accepted incomplete prefix: %+v, %v", items, err)
		}
	}
}
