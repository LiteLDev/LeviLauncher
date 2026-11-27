//go:build darwin

package gameruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// GameLaunchOptions configures how a Windows game executable is started.
type GameLaunchOptions struct {
	WorkDir       string
	LogPath       string
	ExtraEnv      map[string]string
	HasWineBridge bool
}

func (p Profile) isSikarugirWrapper() (wrapper string, ok bool) {
	if p.App == "" || p.metalSharp.app != "" {
		return "", false
	}
	wrapper = filepath.Join(p.App, "Contents/MacOS/wineskinlauncher")
	wine := filepath.Join(p.App, "Contents/SharedSupport/wine/bin/wine")
	if !sameRuntimePath(p.Wine, wine) {
		return "", false
	}
	info, err := os.Stat(wrapper)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return "", false
	}
	return wrapper, true
}

// GameCommand builds a command for starting a Windows game executable.
// Wrapper- and LaunchServices-backed runtimes return detached=true because
// their launcher exits after handing off to the game process.
func (p Profile) GameCommand(ctx context.Context, exe string, args ...string) (cmd *exec.Cmd, detached bool, err error) {
	return p.GameCommandWithOptions(ctx, exe, GameLaunchOptions{}, args...)
}

// GameCommandWithOptions builds a game launch command with optional working
// directory, log redirection, and per-launch environment overrides.
func (p Profile) GameCommandWithOptions(ctx context.Context, exe string, opts GameLaunchOptions, args ...string) (cmd *exec.Cmd, detached bool, err error) {
	merged := mergeEnv(p.Environment(), opts.ExtraEnv)

	// Launch MetalSharp through our bundled LeviGameHost.app (when available) so
	// macOS RunningBoard activates Game Mode while injecting libLeviMetalFX.dylib
	// into MetalSharp's inner x86_64-unix/wine loader without modifying or re-signing
	// MetalSharp-Wine-x86_64.app. Fall back to opening MetalSharp directly if the
	// helper bundle is absent (e.g. during non-bundled development or unit tests).
	if p.metalSharp.app != "" {
		guest, err := p.WindowsPath(exe)
		if err != nil {
			return nil, false, err
		}
		if opts.WorkDir != "" {
			if guestDir, err := p.WindowsPath(opts.WorkDir); err == nil {
				merged["LEVI_WINE_WORKDIR"] = guestDir
			}
		}
		launchApp := p.metalSharp.app
		if hostApp, helperLib, wineUnix := p.nativeMetalGameHost(); hostApp != "" {
			launchApp = hostApp
			merged["LEVI_WINE_UNIX_BIN"] = wineUnix
			merged["LEVI_METAL_HELPER_LIB"] = helperLib
			merged["LEVI_METAL_OPAQUE"] = "1"
			if merged["D3DM_MTL4"] != "1" {
				for key, value := range p.D3DMetalFXLaunchEnv(opts.HasWineBridge) {
					if _, overridden := opts.ExtraEnv[key]; !overridden {
						merged[key] = value
					}
				}
			}
			applyNativeMetalPacing(merged, p)
			if p.CompatDebugLog {
				merged["LEVI_METALFX_DEBUG"] = "1"
			}
		}
		openArgs := []string{"-n", "-a", launchApp}
		if opts.LogPath != "" {
			openArgs = append(openArgs, "-o", opts.LogPath, "--stderr", opts.LogPath)
		}
		for _, entry := range formatEnv(merged) {
			openArgs = append(openArgs, "--env", entry)
		}
		openArgs = append(openArgs, "--args", guest)
		openArgs = append(openArgs, args...)
		return exec.CommandContext(ctx, "/usr/bin/open", openArgs...), true, nil
	}

	if wrapper, ok := p.isSikarugirWrapper(); ok {
		prefix := filepath.Join(p.App, "Contents/SharedSupport/prefix")
		if !sameRuntimePath(p.Prefix, prefix) {
			return nil, false, fmt.Errorf("Sikarugir requires the prefix belonging to the selected application: %s", prefix)
		}
		cmd = exec.CommandContext(ctx, wrapper, append([]string{exe}, args...)...)
		for _, entry := range formatEnv(merged) {
			key, _, _ := strings.Cut(entry, "=")
			// The SDK prepares its engine and prefix. A bare Wine loader
			// inherited here makes Wine 11 child processes exit immediately.
			if key == "WINELOADER" || key == "WINESERVER" || key == "WINEPREFIX" ||
				strings.HasPrefix(key, "WINEDLLPATH") || strings.HasPrefix(key, "DYLD_") {
				continue
			}
			cmd.Env = append(cmd.Env, entry)
		}
		return cmd, true, nil
	}

	guest, err := p.WindowsPath(exe)
	if err != nil {
		return nil, false, err
	}
	cmd = p.Command(ctx, guest, args...)
	cmd.Env = formatEnv(merged)
	return cmd, false, nil
}

func mergeEnv(base []string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key != "" && !strings.HasPrefix(key, "BASH_FUNC_") {
			merged[key] = value
		}
	}
	for key, value := range extra {
		if key != "" {
			merged[key] = value
		}
	}
	return merged
}

func formatEnv(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func applyNativeMetalPacing(merged map[string]string, p Profile) {
	switch p.VSync {
	case "on":
		merged["LEVI_METAL_VSYNC"] = "1"
	case "off":
		merged["LEVI_METAL_VSYNC"] = "0"
	default:
		delete(merged, "LEVI_METAL_VSYNC")
	}
	if p.MaxFPS > 0 {
		fps := strconv.Itoa(p.MaxFPS)
		merged["LEVI_METAL_MAX_FPS"] = fps
		// Let libLeviMetalFX.dylib be the single authoritative frame pacer to avoid
		// beat-frequency jitter between two competing sleep timers.
		delete(merged, "D3DM_MAX_FPS")
		delete(merged, "VKD3D_FRAME_RATE")
		delete(merged, "DXVK_FRAME_RATE")
		if cfg, ok := merged["MULLION_CONFIG"]; ok && cfg != "" {
			cfg = strings.ReplaceAll(cfg, "d3d11.preferredMaxFrameRate = "+fps, "d3d11.preferredMaxFrameRate = 0")
			cfg = strings.ReplaceAll(cfg, "d3d12.preferredMaxFrameRate = "+fps, "d3d12.preferredMaxFrameRate = 0")
			merged["MULLION_CONFIG"] = cfg
		}
	} else {
		delete(merged, "LEVI_METAL_MAX_FPS")
	}
}

func bundledGameHostAndMetalHelper() (string, string) {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "", ""
	}
	if resolved, evalErr := filepath.EvalSymlinks(exe); evalErr == nil && resolved != "" {
		exe = resolved
	}
	resources := filepath.Join(filepath.Dir(exe), "..", "Resources")
	hostApp := filepath.Clean(filepath.Join(resources, "LeviGameHost.app"))
	hostBin := filepath.Join(hostApp, "Contents", "MacOS", "LeviGameHost")
	helperLib := filepath.Clean(filepath.Join(resources, "libLeviMetalFX.dylib"))
	hostInfo, err := os.Stat(hostBin)
	if err != nil || !hostInfo.Mode().IsRegular() || hostInfo.Mode()&0111 == 0 {
		return "", ""
	}
	helperInfo, err := os.Stat(helperLib)
	if err != nil || !helperInfo.Mode().IsRegular() {
		return "", ""
	}
	return hostApp, helperLib
}

func (p Profile) nativeMetalGameHost() (app, helper, wine string) {
	if p.metalSharp.app == "" {
		return "", "", ""
	}
	wine = filepath.Join(p.metalSharp.app, "Contents/lib/wine/x86_64-unix/wine")
	info, err := os.Stat(wine)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return "", "", ""
	}
	app, helper = bundledGameHostAndMetalHelper()
	return app, helper, wine
}

func sameRuntimePath(a, b string) bool {
	a, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	b, err = filepath.EvalSymlinks(b)
	return err == nil && a == b
}
