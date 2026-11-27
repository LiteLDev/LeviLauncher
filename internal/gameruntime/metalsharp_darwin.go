package gameruntime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
)

const (
	metalSharpBundle     = "MetalSharp-Wine-x86_64.app"
	metalSharpExecutable = "Contents/MacOS/wine"
	metalSharpPayload    = "Contents/share/metalsharp"
)

var (
	ErrUnsupportedArch            = errors.New("ERR_WINE_UNSUPPORTED_ARCH")
	ErrUnsupportedOS              = errors.New("ERR_WINE_UNSUPPORTED_OS")
	ErrGraphicsBackendUnavailable = errors.New("ERR_WINE_GRAPHICS_BACKEND_UNAVAILABLE")
)

type metalSharpRuntime struct{ app string }

func managedWineApp() string {
	return filepath.Join(apppath.LocalDir(), "Runtimes", "MetalSharp", metalSharpBundle)
}

func metalSharpPrefix(app string) string {
	sibling := filepath.Join(filepath.Dir(app), "prefix")
	cleanDir := filepath.Clean(filepath.Dir(app))
	if cleanDir == "/Applications" || strings.HasPrefix(cleanDir, "/Applications/") {
		if info, err := os.Stat(sibling); err != nil || !info.IsDir() {
			return filepath.Join(filepath.Dir(managedWineApp()), "prefix")
		}
	}
	return sibling
}

func isMetalSharpApp(app string) bool {
	if app == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(app, metalSharpPayload))
	if err != nil || !info.IsDir() {
		return false
	}
	// The ARM64 distribution has the same payload folder.
	_, err = os.Stat(filepath.Join(app, "Contents/lib/wine/x86_64-unix/ntdll.so"))
	return err == nil
}

func appWineExecutable(app string) string {
	if isMetalSharpApp(app) {
		return filepath.Join(app, metalSharpExecutable)
	}
	return filepath.Join(app, "Contents/SharedSupport/wine/bin/wine")
}

func metalSharpForSettings(s types.WineSettings) metalSharpRuntime {
	if isMetalSharpApp(s.App) {
		return metalSharpRuntime{s.App}
	}
	if s.App == "" && s.Executable != "" {
		real, err := filepath.EvalSymlinks(s.Executable)
		if err == nil {
			app := filepath.Dir(filepath.Dir(filepath.Dir(real)))
			if real == filepath.Join(app, metalSharpExecutable) && isMetalSharpApp(app) {
				return metalSharpRuntime{app}
			}
		}
	}
	return metalSharpRuntime{}
}

var macOSVersion = sync.OnceValue(func() [2]int {
	data, _ := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
	var version [2]int
	for i, part := range strings.Split(strings.TrimSpace(string(data)), ".") {
		if i >= len(version) {
			break
		}
		version[i], _ = strconv.Atoi(part)
	}
	return version
})

var appleSilicon = sync.OnceValue(func() bool {
	// hw.optional.arm64 also identifies the host from a translated launcher.
	data, _ := exec.Command("/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
	return strings.TrimSpace(string(data)) == "1"
})

func CheckManagedWineRequirements(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !appleSilicon() {
		return ErrUnsupportedArch
	}
	if macOSVersion()[0] < 15 {
		return ErrUnsupportedOS
	}
	return nil
}

func (m metalSharpRuntime) d3dMetalFor(version string) (string, string) {
	switch version {
	case "gptk3":
		return filepath.Join(m.app, metalSharpPayload, "runtime", "d3dmetal-gptk3"), "GPTK 3"
	case "gptk4":
		return filepath.Join(m.app, metalSharpPayload, "runtime", "d3dmetal-gptk4-beta3"), "GPTK 4 beta 3"
	default:
		name, label := "d3dmetal-gptk3", "GPTK 3"
		if macOSVersion()[0] >= 27 {
			name, label = "d3dmetal-gptk4-beta3", "GPTK 4 beta 3"
		}
		return filepath.Join(m.app, metalSharpPayload, "runtime", name), label
	}
}

func (m metalSharpRuntime) availableD3DMetalVersions() []string {
	if m.app == "" {
		return nil
	}
	versions := []string{"auto"}
	if macOSVersion()[0] >= 27 {
		dir4, _ := m.d3dMetalFor("gptk4")
		if _, err := os.Stat(filepath.Join(dir4, "external/D3DMetal.framework")); err == nil {
			versions = append(versions, "gptk4")
		}
	}
	dir3, _ := m.d3dMetalFor("gptk3")
	if _, err := os.Stat(filepath.Join(dir3, "external/D3DMetal.framework")); err == nil {
		versions = append(versions, "gptk3")
	}
	return versions
}

func metalFXFactor(scale string, enabled bool, retina bool) string {
	switch scale {
	case "quality":
		return "1.33"
	case "balanced":
		return "1.50"
	case "performance":
		return "2.00"
	case "ultra_performance":
		return "2.50"
	case "extreme":
		return "3.00"
	case "max":
		return "4.00"
	case "off":
		return ""
	default:
		if enabled {
			if retina {
				return "2.00"
			}
			return "1.50"
		}
		return ""
	}
}

func threadQoSValue(mode string) string {
	switch mode {
	case "default":
		return "0"
	case "aggressive":
		return "2"
	default:
		return "1"
	}
}

func (m metalSharpRuntime) hasD3DMetal(version string) bool {
	if m.app == "" {
		return false
	}
	osVer := macOSVersion()
	if version == "gptk4" && osVer[0] < 27 {
		return false
	}
	if (version == "gptk3" || osVer[0] < 27) && osVer[0] == 15 && osVer[1] < 4 {
		return false
	}
	dir, _ := m.d3dMetalFor(version)
	_, err := os.Stat(filepath.Join(dir, "external/D3DMetal.framework"))
	return err == nil
}

func (m metalSharpRuntime) effectiveGraphics(graphics string) string {
	switch graphics {
	case "", "auto":
		if m.hasD3DMetal("auto") {
			return "d3dmetal"
		}
		return "mullion"
	default:
		return graphics
	}
}

func (m metalSharpRuntime) validate(p Profile) error {
	if err := CheckManagedWineRequirements(context.Background()); err != nil {
		return err
	}
	if err := checkExternalPrefix(m.app, p.Prefix); err != nil {
		return err
	}
	var required string
	switch m.effectiveGraphics(p.Graphics) {
	case "mullion":
		required = filepath.Join(m.app, "Contents/lib/dxmt/x86_64-unix/winemetal.so")
	case "vkd3d":
		required = filepath.Join(m.app, metalSharpPayload, "vkd3d/vkd3d-proton/x86_64-windows/d3d12.dll")
	case "d3dmetal":
		version := macOSVersion()
		if p.D3DMetalVersion == "gptk4" && version[0] < 27 {
			return fmt.Errorf("%w: GPTK 4 requires macOS 27 or later", ErrD3DMetalUnavailable)
		}
		if (p.D3DMetalVersion == "gptk3" || version[0] < 27) && version[0] == 15 && version[1] < 4 {
			return fmt.Errorf("%w: GPTK 3 requires macOS 15.4 or later", ErrD3DMetalUnavailable)
		}
		dir, _ := m.d3dMetalFor(p.D3DMetalVersion)
		if _, err := os.Stat(filepath.Join(dir, "external/D3DMetal.framework")); err != nil {
			return ErrD3DMetalUnavailable
		}
	case "wined3d":
	default:
		return errors.New("unknown Wine graphics backend")
	}
	if required != "" {
		if _, err := os.Stat(required); err != nil {
			return fmt.Errorf("%w: %s", ErrGraphicsBackendUnavailable, p.Graphics)
		}
	}
	return nil
}

func checkExternalPrefix(app, prefix string) error {
	app, err := filepath.EvalSymlinks(app)
	if err != nil {
		return err
	}
	prefix, err = filepath.EvalSymlinks(prefix)
	if err != nil {
		return err
	}
	if relative, err := filepath.Rel(app, prefix); err == nil && filepath.IsLocal(relative) {
		return errors.New("MetalSharp Wine requires a prefix outside its app bundle")
	}
	return nil
}

func (p Profile) GraphicsBackend() string {
	if p.metalSharp.app == "" {
		return p.Graphics
	}
	switch p.metalSharp.effectiveGraphics(p.Graphics) {
	case "mullion":
		return "Mullion"
	case "d3dmetal":
		_, kit := p.metalSharp.d3dMetalFor(p.D3DMetalVersion)
		return "D3DMetal (" + kit + ")"
	case "vkd3d":
		return "VKD3D-Proton"
	default:
		return "WineD3D"
	}
}

// EffectiveGraphics returns the resolved backend ID ("d3dmetal", "mullion", "vkd3d", "wined3d").
func (p Profile) EffectiveGraphics() string {
	if p.metalSharp.app == "" {
		return p.Graphics
	}
	return p.metalSharp.effectiveGraphics(p.Graphics)
}

// SpatialMetalFXFactor returns the configured MetalFX spatial scaling factor
// (e.g. "1.33", "1.50", "2.00"), or "" when disabled.
func (p Profile) SpatialMetalFXFactor() string {
	return metalFXFactor(p.MetalFXScale, p.EnableMetalFX, p.RetinaMode)
}

// UsesNativeRetinaMetalFXUpscale reports whether D3DMetal should keep Wine's
// Win32 surface at 1x logical points and let libLeviMetalFX.dylib upgrade the
// CAMetalLayer to native Retina backing pixels via MTLFXSpatialScaler.
// This covers RetinaMode=false with MetalFX enabled AND any >= 2.0x preset
// (2.0x / 2.5x / 3.0x / 4.0x), avoiding half-size startup windows and halving
// Win32 coordinate scaling.
func (p Profile) UsesNativeRetinaMetalFXUpscale() bool {
	if p.EffectiveGraphics() != "d3dmetal" {
		return false
	}
	factor := p.SpatialMetalFXFactor()
	if factor == "" {
		return false
	}
	if !p.RetinaMode {
		return true
	}
	parsed, err := strconv.ParseFloat(factor, 64)
	return err == nil && parsed >= 1.99
}

// D3DMetalFXLaunchEnv returns the extra environment variables for native
// Apple MTLFXSpatialScaler under D3DMetal.
func (p Profile) D3DMetalFXLaunchEnv(hasBridge bool) map[string]string {
	if p.EffectiveGraphics() != "d3dmetal" {
		return nil
	}
	factor := p.SpatialMetalFXFactor()
	if factor == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(factor, 64)
	if err != nil || parsed <= 1.01 {
		return nil
	}
	env := map[string]string{}
	if p.UsesNativeRetinaMetalFXUpscale() {
		env["LEVI_METALFX_RETINA_UPSCALE"] = "1"
		if parsed > 2.01 && hasBridge {
			env["LEVI_METALFX_SPATIAL_FACTOR"] = factor
			env["LEVI_WINE_RENDER_SCALE"] = fmt.Sprintf("%.2f", parsed/2.0)
		}
		return env
	}
	if hasBridge {
		env["LEVI_WINE_RENDER_SCALE"] = factor
		env["LEVI_METALFX_SPATIAL_FACTOR"] = factor
	}
	return env
}

func (m metalSharpRuntime) environment(p Profile, values map[string]string) {
	for key := range values {
		if strings.HasPrefix(key, "D3DM") || strings.HasPrefix(key, "MULLION_") ||
			strings.HasPrefix(key, "VKD3D_") || strings.HasPrefix(key, "WFDXCOMPAT_") ||
			strings.HasPrefix(key, "ROSETTA_") ||
			strings.HasPrefix(key, "LEVI_METAL") || strings.HasPrefix(key, "LEVI_WINE_RENDER_SCALE") ||
			key == "LEVI_WINE_UNIX_BIN" || key == "LEVI_GAME_ICON_PATH" || key == "LEVI_GAME_TITLE" ||
			key == "METALSHARP_METAL_VERSION" ||
			key == "OPENGL_BACKEND" || key == "MTL_HUD_ENABLED" || key == "MTL_SHADER_CACHE_DIR" {
			delete(values, key)
		}
	}
	values["WINEDLLOVERRIDES"] = "mscoree,mshtml="
	values["WINEMSYNC"], values["WINEESYNC"], values["WINEFSYNC"] = "1", "0", "0"
	values["WINE_THREAD_QOS"] = threadQoSValue(p.ThreadQoS)
	if !p.DisableRosettaAVX {
		values["ROSETTA_ADVERTISE_AVX"] = "1"
	}
	values["GRAPHICS_BACKEND"] = "dxmt"
	if p.MetalHUD {
		values["MTL_HUD_ENABLED"] = "1"
	}
	fps := strconv.Itoa(p.MaxFPS)
	fxFactor := metalFXFactor(p.MetalFXScale, p.EnableMetalFX, p.RetinaMode)
	useMetal3 := !p.EnableMetal4 || macOSVersion()[0] < 27
	lane := "mullion"
	switch m.effectiveGraphics(p.Graphics) {
	case "d3dmetal":
		lane = "d3dmetal"
		values["GRAPHICS_BACKEND"] = "d3dmetal"
		d3dDir, _ := m.d3dMetalFor(p.D3DMetalVersion)
		values["D3DMETAL_RUNTIME_DIR"] = d3dDir
		if strings.Contains(filepath.Base(d3dDir), "gptk4") {
			if useMetal3 {
				values["D3DM_MTL4"] = "0"
			} else {
				values["D3DM_MTL4"] = "1"
			}
		}
		if fxFactor != "" {
			values["D3DM_ENABLE_METALFX"] = "1"
		}
		if p.MaxFPS > 0 {
			values["D3DM_MAX_FPS"] = fps
		}
		if p.MetalHUD {
			values["D3DM_SHOW_HUD_STATS"] = "1"
		}
	case "vkd3d":
		lane = "vkd3d"
		values["GRAPHICS_BACKEND"] = "vkd3d"
		if useMetal3 {
			values["METALSHARP_METAL_VERSION"] = "3"
		}
		if p.MaxFPS > 0 {
			values["VKD3D_FRAME_RATE"] = fps
			values["DXVK_FRAME_RATE"] = fps
		}
		if fxFactor != "" {
			values["DXVK_CONFIG"] = fmt.Sprintf(
				"d3d11.metalSpatialUpscale = True; d3d12.metalSpatialUpscale = True; d3d11.metalSpatialUpscaleFactor = %s; d3d12.metalSpatialUpscaleFactor = %s",
				fxFactor, fxFactor,
			)
		}
	case "wined3d":
		lane = "wined3d"
		values["GRAPHICS_BACKEND"] = "wine"
	default:
		var options []string
		if useMetal3 {
			values["MULLION_D3D12_METAL4"] = "0"
			options = append(options, "mullion.shaderMetalVersion = 320")
		} else {
			values["MULLION_D3D12_METAL4"] = "1"
		}
		// Override mullion.conf's bundled 60 FPS cap when MaxFPS is 0 (unlimited)
		// and apply the explicit cap for both D3D11 and D3D12 when > 0.
		options = append(options, "d3d11.preferredMaxFrameRate = "+fps, "d3d12.preferredMaxFrameRate = "+fps)
		if fxFactor != "" {
			values["MULLION_METALFX_SPATIAL_SWAPCHAIN"] = "1"
			options = append(options,
				"d3d11.metalSpatialUpscale = True",
				"d3d12.metalSpatialUpscale = True",
				"d3d11.metalSpatialUpscaleFactor = "+fxFactor,
				"d3d12.metalSpatialUpscaleFactor = "+fxFactor,
			)
		} else {
			options = append(options,
				"d3d11.metalSpatialUpscale = False",
				"d3d12.metalSpatialUpscale = False",
				"d3d11.metalSpatialUpscaleFactor = 1.00",
				"d3d12.metalSpatialUpscaleFactor = 1.00",
			)
		}
		values["MULLION_CONFIG"] = strings.Join(options, "; ")
	}
	if cacheDir := apppath.CacheDir(); cacheDir != "" {
		shaderCache := filepath.Join(cacheDir, "Wine", "Shaders", lane)
		if os.MkdirAll(shaderCache, 0700) == nil {
			values["MTL_SHADER_CACHE_DIR"] = shaderCache
			switch lane {
			case "mullion":
				values["MULLION_SHADER_CACHE_PATH"] = shaderCache
				values["DXMT_SHADER_CACHE_PATH"] = shaderCache
				values["DXMT_PIPELINE_CACHE_PATH"] = shaderCache
			case "vkd3d":
				values["VKD3D_SHADER_CACHE_PATH"] = shaderCache
				values["DXVK_STATE_CACHE_PATH"] = shaderCache
			}
		}
	}
}

// PrepareGameDirectory enables MetalSharp's display-size adjustment for
// MetalFX. The upstream loader checks for launch.env beside the executable.
func (p Profile) PrepareGameDirectory(exe string) error {
	effective := p.metalSharp.effectiveGraphics(p.Graphics)
	if p.metalSharp.app == "" || metalFXFactor(p.MetalFXScale, p.EnableMetalFX, p.RetinaMode) == "" ||
		(effective != "mullion" && effective != "vkd3d") {
		return nil
	}
	dir := filepath.Join(filepath.Dir(exe), ".metalsharp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	marker, err := os.OpenFile(filepath.Join(dir, "launch.env"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return marker.Close()
}

func (p Profile) BundledVCRuntimeDir() string {
	if p.metalSharp.app == "" {
		return ""
	}
	return filepath.Join(p.metalSharp.app, metalSharpPayload, "redist/vcrun2022-x64")
}

func wineShaderCacheRoot() string {
	return filepath.Join(apppath.CacheDir(), "Wine", "Shaders")
}

func GetWineShaderCacheInfo() (types.WineShaderCacheInfo, error) {
	root := wineShaderCacheRoot()
	info := types.WineShaderCacheInfo{Path: root}
	stat, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	if !stat.IsDir() {
		return info, nil
	}
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if fi, err := entry.Info(); err == nil {
			info.SizeBytes += fi.Size()
			info.FileCount++
		}
		return nil
	})
	return info, nil
}

func ClearWineShaderCache() error {
	root := wineShaderCacheRoot()
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return os.RemoveAll(root)
}

func KillWineProcesses(ctx context.Context) error {
	p, err := Default()
	if err != nil {
		return err
	}
	killCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	server := filepath.Join(filepath.Dir(p.Wine), "wineserver")
	if info, statErr := os.Stat(server); statErr == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
		cmd := exec.CommandContext(killCtx, server, "-k")
		cmd.Env = p.Environment()
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			// wineserver -k exits non-zero when no server is running for this prefix.
			msg := strings.ToLower(strings.TrimSpace(string(out)))
			if msg != "" && !strings.Contains(msg, "no wineserver") && !strings.Contains(msg, "no such file") && !strings.Contains(msg, "not found") {
				return fmt.Errorf("stop wineserver: %w: %s", runErr, strings.TrimSpace(string(out)))
			}
		}
		waitCtx, waitCancel := context.WithTimeout(ctx, 3*time.Second)
		defer waitCancel()
		waitCmd := exec.CommandContext(waitCtx, server, "-w")
		waitCmd.Env = p.Environment()
		_ = waitCmd.Run()
		return nil
	}
	return ErrExecutableMissing
}
