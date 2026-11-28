package gameruntime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if err := initializeMetalSharp(context.Background(), s, false); err == nil {
		t.Fatal("partial prefix was treated as initialized")
	}
	writeWineFixture(t, s.Executable, "#!/bin/sh\ntest \"$1:$2\" = wineboot:-u\n", 0700)
	writeWineFixture(t, filepath.Join(filepath.Dir(s.Executable), "wineserver"), "#!/bin/sh\ntest \"$1\" = -w\n", 0700)
	if err := initializeMetalSharp(context.Background(), s, false); err != nil {
		t.Fatal(err)
	}
	writeWineFixture(t, s.Executable, "#!/bin/sh\nexit 1\n", 0700)
	if err := initializeMetalSharp(context.Background(), s, false); err != nil {
		t.Fatalf("initialized prefix ran wineboot again: %v", err)
	}
	// Force refresh (used on runtime version updates) must re-run wineboot -u.
	if err := initializeMetalSharp(context.Background(), s, true); err == nil {
		t.Fatal("force refresh skipped wineboot -u")
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

func TestAutoD3DMetalPrefersGPTK3ForRayTracing(t *testing.T) {
	t.Setenv("LEVI_GPTK4_RTX_FIX", "")
	osVersion := macOSVersion
	t.Cleanup(func() { macOSVersion = osVersion })
	macOSVersion = func() [2]int { return [2]int{27, 2} }

	m := metalSharpRuntime{app: t.TempDir()}
	gptk3, _ := m.d3dMetalFor("gptk3")
	gptk4, _ := m.d3dMetalFor("gptk4")
	writeWineFixture(t, filepath.Join(gptk3, "external/D3DMetal.framework/D3DMetal"), "", 0600)
	writeWineFixture(t, filepath.Join(gptk4, "external/D3DMetal.framework/D3DMetal"), "", 0600)

	// Without RTX, Auto selects GPTK 4 on macOS 27+.
	pNoRTX := Profile{
		metalSharp:      m,
		Graphics:        "d3dmetal",
		D3DMetalVersion: "auto",
		EnableMetal4:    true,
	}
	if got := pNoRTX.GraphicsBackend(); got != "D3DMetal (GPTK 4 beta 3)" {
		t.Fatalf("Auto without RTX backend = %q, want GPTK 4 beta 3", got)
	}
	if !pNoRTX.usesMetal4D3DMetal() {
		t.Fatalf("Auto without RTX should allow Metal 4")
	}

	// With RTX enabled, Auto selects GPTK 3 to avoid GPTK 4 beta 3's placed AS crash.
	pAutoRTX := Profile{
		metalSharp:       m,
		Graphics:         "d3dmetal",
		D3DMetalVersion:  "auto",
		EnableMetal4:     true,
		EnableRayTracing: true,
	}
	if got := pAutoRTX.GraphicsBackend(); got != "D3DMetal (GPTK 3)" {
		t.Fatalf("Auto with RTX backend = %q, want GPTK 3", got)
	}
	if pAutoRTX.usesMetal4D3DMetal() {
		t.Fatalf("Auto with RTX should not enable Metal 4 when routed to GPTK 3")
	}
	envAutoRTX := strings.Join(pAutoRTX.Environment(), "\n")
	if !strings.Contains(envAutoRTX, "D3DMETAL_RUNTIME_DIR="+gptk3) {
		t.Fatalf("Auto with RTX did not route D3DMETAL_RUNTIME_DIR to GPTK 3:\n%s", envAutoRTX)
	}
	if !strings.Contains(envAutoRTX, "D3DM_SUPPORT_DXR=1") {
		t.Fatalf("Auto with RTX missing D3DM_SUPPORT_DXR=1")
	}
	if strings.Contains(envAutoRTX, "D3DM_MTL4=") {
		t.Fatalf("Auto with RTX should not set D3DM_MTL4 when using GPTK 3:\n%s", envAutoRTX)
	}

	// Explicit GPTK 4 selection is still honored if the user overrides Auto.
	pExplicitGPTK4 := pAutoRTX
	pExplicitGPTK4.D3DMetalVersion = "gptk4"
	if got := pExplicitGPTK4.GraphicsBackend(); got != "D3DMetal (GPTK 4 beta 3)" {
		t.Fatalf("Explicit GPTK 4 with RTX backend = %q, want GPTK 4 beta 3", got)
	}
	envExplicit := strings.Join(pExplicitGPTK4.Environment(), "\n")
	if !strings.Contains(envExplicit, "D3DMETAL_RUNTIME_DIR="+gptk4) {
		t.Fatalf("Explicit GPTK 4 with RTX did not use GPTK 4 runtime dir:\n%s", envExplicit)
	}

	// When LeviWineBridge with the GPTK 4 RTX heap fix is installed, Auto + RTX upgrades to GPTK 4.
	prefix := t.TempDir()
	if err := os.MkdirAll(filepath.Join(prefix, "dosdevices"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/", filepath.Join(prefix, "dosdevices", "z:")); err != nil {
		t.Fatal(err)
	}
	pAutoRTX.Prefix = prefix
	exe := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")

	cmdNoFix, _, err := pAutoRTX.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{
		HasWineBridge:            true,
		HasWineBridgeGPTK4RTXFix: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(cmdNoFix.Args, "\n"), "D3DMETAL_RUNTIME_DIR="+gptk3) {
		t.Fatalf("Auto + RTX without GPTK4 RTX fix should stay on GPTK 3:\n%s", strings.Join(cmdNoFix.Args, "\n"))
	}

	cmdWithFix, _, err := pAutoRTX.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{
		HasWineBridge:            true,
		HasWineBridgeGPTK4RTXFix: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	joinedWithFix := strings.Join(cmdWithFix.Args, "\n")
	if !strings.Contains(joinedWithFix, "D3DMETAL_RUNTIME_DIR="+gptk4) {
		t.Fatalf("Auto + RTX with GPTK4 RTX fix should route to GPTK 4:\n%s", joinedWithFix)
	}
	if !strings.Contains(joinedWithFix, "D3DM_MTL4=1") {
		t.Fatalf("Auto + RTX with GPTK4 RTX fix and EnableMetal4=true should set D3DM_MTL4=1:\n%s", joinedWithFix)
	}

	pExplicitGPTK4.Prefix = prefix
	if _, _, err := pExplicitGPTK4.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{}); err == nil || !strings.Contains(err.Error(), "RTX heap fix") {
		t.Fatalf("explicit GPTK 4 + RTX without heap fix returned %v", err)
	}
	if _, _, err := pExplicitGPTK4.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{HasWineBridgeGPTK4RTXFix: true}); err != nil {
		t.Fatalf("explicit GPTK 4 + RTX with heap fix returned %v", err)
	}
	cmdDisabledFix, _, err := pAutoRTX.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{
		HasWineBridgeGPTK4RTXFix: true,
		ExtraEnv:                 map[string]string{"LEVI_GPTK4_RTX_FIX": "OFF"},
	})
	if err != nil || !strings.Contains(strings.Join(cmdDisabledFix.Args, "\n"), "D3DMETAL_RUNTIME_DIR="+gptk3) {
		t.Fatalf("Auto + RTX with disabled heap fix should use GPTK 3: cmd=%v err=%v", cmdDisabledFix, err)
	}
	if err := os.RemoveAll(gptk3); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pAutoRTX.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{}); err == nil || !strings.Contains(err.Error(), "RTX heap fix") {
		t.Fatalf("Auto + RTX with only GPTK 4 and no heap fix returned %v", err)
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

func TestMetalSharpThreeLaneRTXAndDLSS(t *testing.T) {
	if err := CheckManagedWineRequirements(context.Background()); err != nil {
		t.Skip(err)
	}
	s := metalSharpFixture(t)
	d3dDir, _ := (metalSharpRuntime{s.App}).d3dMetalFor("gptk3")
	writeWineFixture(t, filepath.Join(d3dDir, "external/D3DMetal.framework/D3DMetal"), "", 0600)

	// Poison host env to ensure stale cross-lane variables are cleared.
	t.Setenv("D3DM_SUPPORT_DXR", "1")
	t.Setenv("MULLION_ENABLE_NVEXT", "1")
	t.Setenv("METALSHARP_DLSS_SCALE", "dlaa")
	t.Setenv("MVK_CONFIG_ENABLE_EXPERIMENTAL_RAY_TRACING", "1")
	t.Setenv("LEVI_GPTK_DLSS_QUALITY", "ultra_performance")

	// 1. Mullion: RTX on + DLSS on (performance) suppresses spatial scaling and enables NVEXT + NVIDIA spoof.
	p, err := profileFromSettings(types.WineSettings{
		App:              s.App,
		Graphics:         "mullion",
		MetalFXScale:     "performance",
		EnableMetalFX:    true,
		EnableRayTracing: true,
		EnableDLSS:       true,
		DLSSQuality:      "performance",
	})
	if err != nil {
		t.Fatal(err)
	}
	mullionEnv := strings.Join(p.Environment(), "\n")
	for _, want := range []string{
		"GRAPHICS_BACKEND=dxmt",
		"MULLION_D3D12_RAYTRACING=1",
		"MULLION_ENABLE_NVEXT=1",
		"METALSHARP_DLSS_SCALE=performance",
		`dxgi.customVendorId = 10de; dxgi.customDeviceId = 2704; dxgi.customDeviceDesc = "NVIDIA GeForce RTX 4080"`,
		"d3d12.metalSpatialUpscale = False",
	} {
		if !strings.Contains(mullionEnv, want) {
			t.Errorf("Mullion RTX+DLSS missing %q in:\n%s", want, mullionEnv)
		}
	}
	for _, forbidden := range []string{
		"MULLION_METALFX_SPATIAL_SWAPCHAIN=",
		"D3DM_SUPPORT_DXR=",
		"MVK_CONFIG_ENABLE_EXPERIMENTAL_RAY_TRACING=",
		"LEVI_GPTK_DLSS_QUALITY=",
	} {
		if strings.Contains(mullionEnv, forbidden) {
			t.Errorf("Mullion RTX+DLSS leaked %q", forbidden)
		}
	}

	// Mullion RTX off + DLSS off explicitly disables both.
	p.EnableRayTracing, p.EnableDLSS, p.MetalFXScale, p.EnableMetalFX = false, false, "off", false
	mullionOff := strings.Join(p.Environment(), "\n")
	for _, want := range []string{"MULLION_D3D12_RAYTRACING=0", "MULLION_ENABLE_NVEXT=0"} {
		if !strings.Contains(mullionOff, want) {
			t.Errorf("Mullion off missing %q", want)
		}
	}
	if strings.Contains(mullionOff, "METALSHARP_DLSS_SCALE=") {
		t.Errorf("Mullion off leaked METALSHARP_DLSS_SCALE")
	}

	// 2. VKD3D-Proton: RTX on + DLSS on (balanced) sets MVK experimental RT, DXVK_ENABLE_NVAPI, VKD3D_METALFX_SPATIAL=0.
	p.Graphics, p.EnableRayTracing, p.EnableDLSS, p.DLSSQuality = "vkd3d", true, true, "balanced"
	p.MetalFXScale, p.EnableMetalFX = "performance", true
	vkd3dEnv := strings.Join(p.Environment(), "\n")
	for _, want := range []string{
		"GRAPHICS_BACKEND=vkd3d",
		"MVK_CONFIG_ENABLE_EXPERIMENTAL_RAY_TRACING=1",
		"DXVK_ENABLE_NVAPI=1",
		"VKD3D_METALFX_SPATIAL=0",
		"METALSHARP_DLSS_SCALE=balanced",
		`dxgi.customVendorId = 10de; dxgi.customDeviceId = 2704; dxgi.customDeviceDesc = "NVIDIA GeForce RTX 4080"`,
	} {
		if !strings.Contains(vkd3dEnv, want) {
			t.Errorf("VKD3D RTX+DLSS missing %q in:\n%s", want, vkd3dEnv)
		}
	}
	if strings.Contains(vkd3dEnv, "VKD3D_CONFIG=nodxr") || strings.Contains(vkd3dEnv, "metalSpatialUpscale = True") {
		t.Errorf("VKD3D RTX+DLSS had unexpected nodxr or spatial upscale:\n%s", vkd3dEnv)
	}

	// VKD3D RTX off + DLSS off sets nodxr.
	p.EnableRayTracing, p.EnableDLSS, p.MetalFXScale, p.EnableMetalFX = false, false, "off", false
	vkd3dOff := strings.Join(p.Environment(), "\n")
	if !strings.Contains(vkd3dOff, "VKD3D_CONFIG=nodxr") {
		t.Errorf("VKD3D off missing VKD3D_CONFIG=nodxr")
	}
	if strings.Contains(vkd3dOff, "MVK_CONFIG_ENABLE_EXPERIMENTAL_RAY_TRACING=") || strings.Contains(vkd3dOff, "DXVK_ENABLE_NVAPI=") {
		t.Errorf("VKD3D off leaked RT or NVAPI")
	}

	// 3. D3DMetal: RTX on + DLSS on + HasWineBridge sets DXR, MetalFX, NVIDIA spoof, and LEVI_GPTK_DLSS_QUALITY.
	p.Graphics, p.D3DMetalVersion = "d3dmetal", "gptk3"
	p.EnableRayTracing, p.EnableDLSS, p.DLSSQuality = true, true, "performance"
	p.MetalFXScale, p.EnableMetalFX = "performance", true
	d3dEnv := strings.Join(p.Environment(), "\n")
	for _, want := range []string{
		"GRAPHICS_BACKEND=d3dmetal",
		"D3DM_SUPPORT_DXR=1",
		"D3DM_ENABLE_METALFX=1",
		"D3DM_VENDOR_ID=0x10DE",
		"D3DM_DEVICE_ID=0x2704",
		"D3DM_DEVICE_DESCRIPTION=NVIDIA GeForce RTX 4080",
	} {
		if !strings.Contains(d3dEnv, want) {
			t.Errorf("D3DMetal RTX+DLSS missing %q in:\n%s", want, d3dEnv)
		}
	}
	if strings.Contains(d3dEnv, "METALSHARP_DLSS_SCALE=") {
		t.Errorf("D3DMetal should not set METALSHARP_DLSS_SCALE")
	}

	gameDir := t.TempDir()
	exe := filepath.Join(gameDir, "Minecraft.Windows.exe")
	cmdWithBridge, _, err := p.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{HasWineBridge: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(cmdWithBridge.Args, "\n"), "LEVI_GPTK_DLSS_QUALITY=performance") {
		t.Errorf("D3DMetal launch with bridge missing LEVI_GPTK_DLSS_QUALITY=performance")
	}
	cmdNoBridge, _, err := p.GameCommandWithOptions(context.Background(), exe, GameLaunchOptions{HasWineBridge: false})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(cmdNoBridge.Args, "\n"), "LEVI_GPTK_DLSS_QUALITY=") {
		t.Errorf("D3DMetal launch without bridge should not set LEVI_GPTK_DLSS_QUALITY")
	}

	// 4. PrepareGameDirectory creates .metalsharp/launch.env for D3DMetal when RTX or DLSS is enabled.
	if err := p.PrepareGameDirectory(exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, ".metalsharp", "launch.env")); err != nil {
		t.Fatalf("PrepareGameDirectory did not create .metalsharp/launch.env for DLSS/RTX: %v", err)
	}
}

func TestMetalSharpVersionDetection(t *testing.T) {
	app := t.TempDir()
	m := metalSharpRuntime{app: app}
	if got := m.version(); got != "" {
		t.Fatalf("empty app version = %q, want empty", got)
	}

	// Fallback to SOURCES.txt when Info.plist is absent.
	writeWineFixture(t, filepath.Join(app, metalSharpPayload, "SOURCES.txt"),
		"MetalSharp Wine 10.0 (x86_64)\nwine: abcdef\n", 0600)
	if got := m.version(); got != "10.0" {
		t.Fatalf("SOURCES.txt version = %q, want 10.0", got)
	}

	// Info.plist takes precedence over SOURCES.txt.
	writeWineFixture(t, filepath.Join(app, "Contents/Info.plist"), `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
  <key>CFBundleShortVersionString</key><string>10.5</string>
</dict></plist>`, 0600)
	if got := m.version(); got != "10.5" {
		t.Fatalf("Info.plist version = %q, want 10.5", got)
	}
}

func TestMetalSharpReleasesParsingChecksumAndFallback(t *testing.T) {
	sha105 := "afd91deb8e745a952f84bc525995bc2d14921a2a999c72fc1790a3cdfa215655"
	sha100 := "8c90c9c2fdb01e6d5f1a5ea40d3b18b388649ddc7238b9061fb4657b43ebf737"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			_, _ = w.Write([]byte(`[
				{"tag_name":"v10.5","name":"MetalSharp Wine 10.5","published_at":"2026-11-26T17:55:46Z","assets":[
					{"name":"MetalSharp-Wine-10.5-x86_64.zip","size":1582053390,"digest":"sha256:` + sha105 + `","browser_download_url":"https://example.com/10.5.zip"},
					{"name":"SHA256SUMS","size":295,"browser_download_url":"https://example.com/10.5.sums"}
				]},
				{"tag_name":"v10.0","name":"MetalSharp Wine 10.0","published_at":"2026-11-26T08:41:50Z","assets":[
					{"name":"MetalSharp-Wine-10.0-x86_64.zip","size":1557106370,"browser_download_url":"https://example.com/10.0.zip"},
					{"name":"SHA256SUMS","size":295,"browser_download_url":"http://` + r.Host + `/sums-10.0"}
				]},
				{"tag_name":"wine-mono-11.3.99","name":"Wine Mono","assets":[
					{"name":"SHA256SUMS","size":100,"browser_download_url":"https://example.com/mono.sums"}
				]}
			]`))
		case "/sums-10.0":
			_, _ = w.Write([]byte(sha100 + "  MetalSharp-Wine-10.0-x86_64.zip\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origURL := metalSharpReleasesURL
	origClient := metalSharpHTTPClient
	t.Cleanup(func() {
		metalSharpReleasesURL = origURL
		metalSharpHTTPClient = origClient
	})
	metalSharpReleasesURL = srv.URL + "/releases"
	metalSharpHTTPClient = srv.Client()

	releases, err := ListMetalSharpReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("filtered releases count = %d, want 2: %+v", len(releases), releases)
	}
	if releases[0].Tag != "v10.5" || !releases[0].IsLatest || releases[0].SHA256 != sha105 {
		t.Fatalf("release[0] = %+v", releases[0])
	}
	if releases[1].Tag != "v10.0" || releases[1].IsLatest {
		t.Fatalf("release[1] = %+v", releases[1])
	}

	asset, err := resolveMetalSharpAsset(context.Background(), "v10.0", types.WineDownloadSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if asset.sha != sha100 || asset.name != "MetalSharp-Wine-10.0-x86_64.zip" {
		t.Fatalf("resolved v10.0 asset = %+v", asset)
	}

	// Offline/error fallback must still return the pinned default release.
	metalSharpReleasesURL = srv.URL + "/missing"
	fallback, err := ListMetalSharpReleases(context.Background())
	if err != nil || len(fallback) != 1 || fallback[0].Tag != defaultMetalSharpTag || !fallback[0].IsLatest {
		t.Fatalf("fallback releases = %+v, %v", fallback, err)
	}
}

func TestMetalSharpAtomicReplaceAndRollback(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, metalSharpBundle)
	staged := filepath.Join(base, ".prepare-new", metalSharpBundle)
	writeWineFixture(t, filepath.Join(dest, "marker"), "old-10.0", 0600)
	writeWineFixture(t, filepath.Join(staged, "marker"), "new-10.5", 0600)

	backup, err := replaceAppAtomically(staged, dest)
	if err != nil || backup == "" {
		t.Fatalf("replaceAppAtomically = %q, %v", backup, err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "marker"))
	if err != nil || string(got) != "new-10.5" {
		t.Fatalf("replaced marker = %q, %v", got, err)
	}

	// Simulate post-swap failure and verify rollback restores the previous bundle.
	rollbackAppReplacement(dest, backup)
	got, err = os.ReadFile(filepath.Join(dest, "marker"))
	if err != nil || string(got) != "old-10.0" {
		t.Fatalf("rolled back marker = %q, %v", got, err)
	}

	// Verify stale archive cleanup removes older zip caches while keeping active zip.
	cacheDir := t.TempDir()
	writeWineFixture(t, filepath.Join(cacheDir, "MetalSharp-Wine-10.0-x86_64.zip"), "old", 0600)
	writeWineFixture(t, filepath.Join(cacheDir, "MetalSharp-Wine-10.5-x86_64.zip"), "new", 0600)
	writeWineFixture(t, filepath.Join(cacheDir, "unrelated.zip"), "keep", 0600)
	cleanupStaleMetalSharpArchives(cacheDir, "MetalSharp-Wine-10.5-x86_64.zip")
	if _, err := os.Stat(filepath.Join(cacheDir, "MetalSharp-Wine-10.0-x86_64.zip")); !os.IsNotExist(err) {
		t.Fatalf("stale archive was not removed")
	}
	for _, keep := range []string{"MetalSharp-Wine-10.5-x86_64.zip", "unrelated.zip"} {
		if _, err := os.Stat(filepath.Join(cacheDir, keep)); err != nil {
			t.Fatalf("expected %s to remain: %v", keep, err)
		}
	}
}

func TestManagedMetalSharpMultiVersionListSwitchAndRemove(t *testing.T) {
	if err := CheckManagedWineRequirements(context.Background()); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("LOCALAPPDATA", root)

	makeManagedApp := func(app, version string) {
		writeWineFixture(t, filepath.Join(app, metalSharpExecutable), "#!/bin/sh\necho 'wine-11.22'\n", 0700)
		writeWineFixture(t, filepath.Join(filepath.Dir(filepath.Join(app, metalSharpExecutable)), "wineserver"), "#!/bin/sh\nexit 0\n", 0700)
		writeWineFixture(t, filepath.Join(app, metalSharpPayload, "SOURCES.txt"), "MetalSharp Wine "+version+" (x86_64)\n", 0600)
		writeWineFixture(t, filepath.Join(app, "Contents/lib/wine/x86_64-unix/ntdll.so"), "ntdll", 0600)
		writeWineFixture(t, filepath.Join(app, "Contents/lib/dxmt/x86_64-unix/winemetal.so"), "mullion", 0600)
		writeWineFixture(t, filepath.Join(app, "Contents/Info.plist"),
			"<plist><dict><key>CFBundleShortVersionString</key><string>"+version+"</string></dict></plist>", 0600)
	}

	prefix := filepath.Join(managedWineRoot(), "prefix")
	for _, name := range []string{"system.reg", "user.reg", "drive_c/windows/system32/cmd.exe"} {
		writeWineFixture(t, filepath.Join(prefix, name), "prefix-data", 0600)
	}

	// Legacy root install (10.0) + versioned install (10.5) share the same prefix.
	app100 := managedWineApp()
	app105 := managedWineAppForVersion("10.5")
	makeManagedApp(app100, "10.0")
	makeManagedApp(app105, "10.5")

	if got := metalSharpPrefix(app105); got != prefix {
		t.Fatalf("metalSharpPrefix(v10.5) = %q, want %q", got, prefix)
	}

	// Activate v10.5 with custom RTX + DLSS settings.
	s := managedWineSettings(app105)
	s.EnableRayTracing = true
	s.EnableDLSS = true
	s.DLSSQuality = "balanced"
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	// List installed versions: newest first (10.5 active, 10.0 inactive).
	list, err := ListInstalledMetalSharpVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("installed versions len = %d, want 2: %+v", len(list), list)
	}
	if list[0].Version != "10.5" || !list[0].Active || list[0].SizeBytes == 0 {
		t.Fatalf("list[0] = %+v", list[0])
	}
	if list[1].Version != "10.0" || list[1].Active || list[1].SizeBytes == 0 {
		t.Fatalf("list[1] = %+v", list[1])
	}

	// Deleting the active version must be rejected.
	if err := RemoveManagedWineVersion(app105); err == nil || !strings.Contains(err.Error(), "ERR_WINE_VERSION_IN_USE") {
		t.Fatalf("expected ERR_WINE_VERSION_IN_USE, got %v", err)
	}
	// Deleting an external path outside Runtimes/MetalSharp must be rejected.
	if err := RemoveManagedWineVersion(filepath.Join(root, "outside.app")); err == nil || !strings.Contains(err.Error(), "ERR_WINE_VERSION_INVALID") {
		t.Fatalf("expected ERR_WINE_VERSION_INVALID, got %v", err)
	}

	// Switch to 10.0 and verify RTX/DLSS settings + prefix are preserved.
	switched, err := SwitchManagedWineVersion(context.Background(), app100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if switched.RuntimeVersion != "10.0" || !switched.EnableRayTracing || !switched.EnableDLSS || switched.DLSSQuality != "balanced" {
		t.Fatalf("switched settings lost preferences: %+v", switched)
	}

	// Now remove inactive v10.5 (plus its cached zip) and verify prefix remains untouched.
	cachedZip := filepath.Join(apppath.CacheDir(), "Wine", "MetalSharp-Wine-10.5-x86_64.zip")
	writeWineFixture(t, cachedZip, "zip", 0600)
	if err := RemoveManagedWineVersion(app105); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(app105)); !os.IsNotExist(err) {
		t.Fatalf("v10.5 directory still exists after removal: %v", err)
	}
	if _, err := os.Stat(cachedZip); !os.IsNotExist(err) {
		t.Fatalf("v10.5 cached zip still exists after removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "system.reg")); err != nil {
		t.Fatalf("shared prefix was damaged: %v", err)
	}
	listAfter, err := ListInstalledMetalSharpVersions()
	if err != nil || len(listAfter) != 1 || listAfter[0].Version != "10.0" || !listAfter[0].Active {
		t.Fatalf("list after removal = %+v, %v", listAfter, err)
	}
}
