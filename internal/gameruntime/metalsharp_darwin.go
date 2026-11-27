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
	metalSharpBundle      = "MetalSharp-Wine-x86_64.app"
	metalSharpExecutable  = "Contents/MacOS/wine"
	metalSharpPayload     = "Contents/share/metalsharp"
	nvidiaSpoofVendorID   = "0x10DE"
	nvidiaSpoofDeviceID   = "0x2704"
	nvidiaSpoofDeviceDesc = "NVIDIA GeForce RTX 4080"
	dxgiSpoofConfig       = `dxgi.customVendorId = 10de; dxgi.customDeviceId = 2704; dxgi.customDeviceDesc = "NVIDIA GeForce RTX 4080"`
)

var (
	ErrUnsupportedArch            = errors.New("ERR_WINE_UNSUPPORTED_ARCH")
	ErrUnsupportedOS              = errors.New("ERR_WINE_UNSUPPORTED_OS")
	ErrGraphicsBackendUnavailable = errors.New("ERR_WINE_GRAPHICS_BACKEND_UNAVAILABLE")
)

type metalSharpRuntime struct{ app string }

func managedWineRoot() string {
	return filepath.Join(apppath.LocalDir(), "Runtimes", "MetalSharp")
}

func managedWineVersionsDir() string {
	return filepath.Join(managedWineRoot(), "versions")
}

func managedWineApp() string {
	return filepath.Join(managedWineRoot(), metalSharpBundle)
}

func managedWineAppForVersion(version string) string {
	v := normalizeMetalSharpVersion(version)
	if v == "" {
		return managedWineApp()
	}
	return filepath.Join(managedWineVersionsDir(), "v"+v, metalSharpBundle)
}

func isManagedMetalSharpApp(app string) bool {
	if app == "" {
		return false
	}
	cleanApp := filepath.Clean(app)
	root := filepath.Clean(managedWineRoot())
	if cleanApp == filepath.Join(root, metalSharpBundle) {
		return true
	}
	versionsDir := filepath.Join(root, "versions")
	rel, err := filepath.Rel(versionsDir, cleanApp)
	if err != nil || !filepath.IsLocal(rel) {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) == 2 && parts[0] != "" && !strings.HasPrefix(parts[0], ".") && parts[1] == metalSharpBundle
}

func metalSharpPrefix(app string) string {
	if isManagedMetalSharpApp(app) {
		return filepath.Join(managedWineRoot(), "prefix")
	}
	sibling := filepath.Join(filepath.Dir(app), "prefix")
	cleanDir := filepath.Clean(filepath.Dir(app))
	if cleanDir == "/Applications" || strings.HasPrefix(cleanDir, "/Applications/") {
		if info, err := os.Stat(sibling); err != nil || !info.IsDir() {
			return filepath.Join(managedWineRoot(), "prefix")
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
	return m.d3dMetalForSelection(m.selectedD3DMetalVersion(version))
}

func (m metalSharpRuntime) d3dMetalForProfile(p Profile) (string, string) {
	return m.d3dMetalForSelection(m.selectedD3DMetalVersionFor(p.D3DMetalVersion, p.EnableRayTracing))
}

func (m metalSharpRuntime) d3dMetalForSelection(selected string) (string, string) {
	if selected == "gptk4" {
		return filepath.Join(m.app, metalSharpPayload, "runtime", "d3dmetal-gptk4-beta3"), "GPTK 4 beta 3"
	}
	return filepath.Join(m.app, metalSharpPayload, "runtime", "d3dmetal-gptk3"), "GPTK 3"
}

func (m metalSharpRuntime) selectedD3DMetalVersion(version string) string {
	return m.selectedD3DMetalVersionFor(version, false)
}

func (m metalSharpRuntime) selectedD3DMetalVersionFor(version string, rayTracing bool) string {
	if version == "gptk3" || version == "gptk4" {
		return version
	}
	// Apple's D3DMetal GPTK 4 beta 3 maps D3D12_HEAP_TYPE_CUSTOM heaps to
	// MTLStorageModeShared while hardcoding MTLResourceStorageModePrivate when
	// creating placed acceleration-structure GPU header buffers, which aborts
	// Minecraft Bedrock RTX on world load. GPTK 3 allocates acceleration
	// structures on its own private internal allocator and runs RTX cleanly.
	if rayTracing && m.hasD3DMetal("gptk3") {
		return "gptk3"
	}
	if versions := m.availableD3DMetalVersions(); len(versions) > 1 {
		return versions[1]
	}
	return "gptk3"
}

func (m metalSharpRuntime) version() string {
	if m.app == "" {
		return ""
	}
	if v := readMetalSharpInfoPlistVersion(filepath.Join(m.app, "Contents/Info.plist")); v != "" {
		return v
	}
	return readMetalSharpSourcesVersion(filepath.Join(m.app, metalSharpPayload, "SOURCES.txt"))
}

func normalizeMetalSharpVersion(raw string) string {
	return strings.TrimLeft(strings.TrimSpace(raw), "vV")
}

func readMetalSharpInfoPlistVersion(plistPath string) string {
	data, err := os.ReadFile(plistPath)
	if err != nil || len(data) == 0 {
		return ""
	}
	if strings.HasPrefix(string(data), "bplist") {
		out, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-", plistPath).Output()
		if err != nil {
			return ""
		}
		return normalizeMetalSharpVersion(string(out))
	}
	text := string(data)
	const keyTag = "<key>CFBundleShortVersionString</key>"
	idx := strings.Index(text, keyTag)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(keyTag):]
	start := strings.Index(rest, "<string>")
	if start < 0 {
		return ""
	}
	rest = rest[start+len("<string>"):]
	end := strings.Index(rest, "</string>")
	if end < 0 {
		return ""
	}
	return normalizeMetalSharpVersion(rest[:end])
}

func readMetalSharpSourcesVersion(sourcesPath string) string {
	data, err := os.ReadFile(sourcesPath)
	if err != nil {
		return ""
	}
	firstLine, _, _ := strings.Cut(string(data), "\n")
	firstLine = strings.TrimSpace(firstLine)
	const prefix = "MetalSharp Wine "
	if len(firstLine) <= len(prefix) || !strings.EqualFold(firstLine[:len(prefix)], prefix) {
		return ""
	}
	fields := strings.Fields(firstLine[len(prefix):])
	if len(fields) == 0 {
		return ""
	}
	return normalizeMetalSharpVersion(fields[0])
}

func (m metalSharpRuntime) availableD3DMetalVersions() []string {
	if m.app == "" {
		return nil
	}
	versions := []string{"auto"}
	for _, version := range []string{"gptk4", "gptk3"} {
		if m.hasD3DMetal(version) {
			versions = append(versions, version)
		}
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

func explicitMetalFXSharpness(sharpness string) string {
	switch sharpness {
	case "10":
		return "0.10"
	case "15":
		return "0.15"
	case "25":
		return "0.25"
	case "35":
		return "0.35"
	default:
		return ""
	}
}

func metalFXSharpnessValue(sharpness, scale string, enabled, retina bool) string {
	factor := metalFXFactor(scale, enabled, retina)
	if factor == "" {
		return ""
	}
	if explicit := explicitMetalFXSharpness(sharpness); explicit != "" {
		return explicit
	}
	switch sharpness {
	case "0":
		return ""
	case "", "auto":
		switch factor {
		case "2.00":
			return "0.15"
		case "2.50":
			return "0.20"
		case "3.00", "4.00":
			return "0.25"
		default:
			return ""
		}
	default:
		return ""
	}
}

func dlssMetalFXSharpnessValue(sharpness, quality string) string {
	if explicit := explicitMetalFXSharpness(sharpness); explicit != "" {
		return explicit
	}
	switch sharpness {
	case "0":
		return ""
	case "", "auto":
		switch quality {
		case "ultra_performance":
			return "0.25"
		case "performance", "balanced":
			return "0.15"
		case "dlaa":
			return ""
		default:
			// quality / auto (game default = Quality 67%)
			return "0.10"
		}
	default:
		return ""
	}
}

func dlssScaleEnvValue(quality string) string {
	switch quality {
	case "ultra_performance", "performance", "balanced", "quality", "dlaa":
		return quality
	default:
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
	if version == "gptk3" && osVer[0] == 15 && osVer[1] < 4 {
		return false
	}
	dir, _ := m.d3dMetalFor(version)
	_, err := os.Stat(filepath.Join(dir, "external/D3DMetal.framework"))
	return err == nil
}

func (m metalSharpRuntime) effectiveGraphics(graphics string) string {
	switch graphics {
	case "", "auto":
		if len(m.availableD3DMetalVersions()) > 1 {
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
		selected := m.selectedD3DMetalVersionFor(p.D3DMetalVersion, p.EnableRayTracing)
		version := macOSVersion()
		if selected == "gptk4" && version[0] < 27 {
			return fmt.Errorf("%w: GPTK 4 requires macOS 27 or later", ErrD3DMetalUnavailable)
		}
		if selected == "gptk3" && version[0] == 15 && version[1] < 4 {
			return fmt.Errorf("%w: GPTK 3 requires macOS 15.4 or later", ErrD3DMetalUnavailable)
		}
		if !m.hasD3DMetal(selected) {
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
		_, kit := p.metalSharp.d3dMetalForProfile(p)
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

// EffectiveRayTracing reports whether DXR ray tracing is active on a supported
// MetalSharp graphics lane.
func (p Profile) EffectiveRayTracing() bool {
	if !p.EnableRayTracing {
		return false
	}
	switch p.EffectiveGraphics() {
	case "d3dmetal", "mullion", "vkd3d":
		return true
	default:
		return false
	}
}

// EffectiveDLSS reports whether DLSS (routed to MetalFX Temporal via NGX) is
// active on a supported MetalSharp graphics lane. Minecraft Bedrock only uses
// NGX/DLSS while Ray Tracing is enabled, so Temporal upscaling is inactive when
// EnableRayTracing is false.
func (p Profile) EffectiveDLSS() bool {
	if !p.EnableRayTracing || !p.EnableDLSS {
		return false
	}
	switch p.EffectiveGraphics() {
	case "d3dmetal", "mullion", "vkd3d":
		return true
	default:
		return false
	}
}

// SpatialMetalFXFactor returns the configured MetalFX spatial scaling factor
// (e.g. "1.33", "1.50", "2.00"), or "" when disabled or superseded by DLSS.
func (p Profile) SpatialMetalFXFactor() string {
	if p.EffectiveDLSS() {
		return ""
	}
	return metalFXFactor(p.MetalFXScale, p.EnableMetalFX, p.RetinaMode)
}

func (p Profile) usesMetal4D3DMetal() bool {
	return p.EnableMetal4 && macOSVersion()[0] >= 27 &&
		p.metalSharp.selectedD3DMetalVersionFor(p.D3DMetalVersion, p.EnableRayTracing) == "gptk4"
}

var displayRetinaOversampleRatio = mainDisplayRetinaOversampleRatio

// D3DMetalDLSSLaunchEnv returns the LeviWineBridge environment variable used to
// override Apple GPTK's default DLSS quality preset under D3DMetal.
func (p Profile) D3DMetalDLSSLaunchEnv(hasBridge bool) map[string]string {
	if p.EffectiveGraphics() != "d3dmetal" || !p.EffectiveDLSS() || !hasBridge {
		return nil
	}
	switch p.DLSSQuality {
	case "ultra_performance", "performance", "balanced", "dlaa":
		return map[string]string{
			"LEVI_GPTK_DLSS_QUALITY": p.DLSSQuality,
		}
	default:
		return nil
	}
}

// DLSSPanelLimitLaunchEnv caps Retina DLSS output (and therefore internal RTX
// ray-tracing and MetalFX Temporal resolution) to the display's physical panel
// pixels via LeviWineBridge when macOS uses a non-integer scaled Retina mode.
func (p Profile) DLSSPanelLimitLaunchEnv(hasBridge bool) map[string]string {
	if !p.EffectiveDLSS() || !p.RetinaMode || !p.MetalFXPanelLimit || !hasBridge {
		return nil
	}
	ratio := displayRetinaOversampleRatio()
	if ratio <= 1.01 || ratio > 4.0 {
		return nil
	}
	return map[string]string{
		"LEVI_WINE_RENDER_SCALE": fmt.Sprintf("%.4f", ratio),
	}
}

// D3DMetalFXLaunchEnv returns the extra environment variables for native
// Apple MTLFXSpatialScaler and optional CAS sharpening under D3DMetal.
func (p Profile) D3DMetalFXLaunchEnv(hasBridge bool) map[string]string {
	if p.EffectiveGraphics() != "d3dmetal" || p.usesMetal4D3DMetal() || p.EffectiveDLSS() {
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
	sharp := metalFXSharpnessValue(p.MetalFXSharpness, p.MetalFXScale, p.EnableMetalFX, p.RetinaMode)
	env := map[string]string{}
	if p.RetinaMode && parsed >= 2.0 {
		// Without a bridge, keep the logical-pixel to Retina path at 2×.
		env["LEVI_METALFX_RETINA_UPSCALE"] = "1"
		if parsed > 2.0 && hasBridge {
			env["LEVI_METALFX_SPATIAL_FACTOR"] = factor
			env["LEVI_WINE_RENDER_SCALE"] = fmt.Sprintf("%.2f", parsed/2.0)
		}
	} else if hasBridge {
		env["LEVI_WINE_RENDER_SCALE"] = factor
		env["LEVI_METALFX_SPATIAL_FACTOR"] = factor
	} else {
		return env
	}
	if sharp != "" {
		env["LEVI_METALFX_SHARPNESS"] = sharp
	}
	if p.RetinaMode && p.MetalFXPanelLimit {
		env["LEVI_METALFX_PANEL_LIMIT"] = "1"
	}
	return env
}

// MetalFXSharpenLaunchEnv returns the extra environment variables for post-upscale
// CAS sharpening on renderers that run upscaling internally (Mullion, VKD3D-Proton,
// and D3DMetal when DLSS handles upscaling).
func (p Profile) MetalFXSharpenLaunchEnv() map[string]string {
	effective := p.EffectiveGraphics()
	if p.EffectiveDLSS() {
		if effective != "mullion" && effective != "vkd3d" && effective != "d3dmetal" {
			return nil
		}
		if effective == "d3dmetal" && p.usesMetal4D3DMetal() {
			return nil
		}
		sharp := dlssMetalFXSharpnessValue(p.MetalFXSharpness, p.DLSSQuality)
		if sharp == "" {
			return nil
		}
		return map[string]string{
			"LEVI_METALFX_SHARPNESS":    sharp,
			"LEVI_METALFX_SHARPEN_ONLY": "1",
		}
	}
	if effective != "mullion" && effective != "vkd3d" {
		return nil
	}
	sharp := metalFXSharpnessValue(p.MetalFXSharpness, p.MetalFXScale, p.EnableMetalFX, p.RetinaMode)
	if sharp == "" {
		return nil
	}
	return map[string]string{
		"LEVI_METALFX_SHARPNESS":    sharp,
		"LEVI_METALFX_SHARPEN_ONLY": "1",
	}
}

func (m metalSharpRuntime) environment(p Profile, values map[string]string) {
	for key := range values {
		if strings.HasPrefix(key, "D3DM") || strings.HasPrefix(key, "MULLION_") ||
			strings.HasPrefix(key, "VKD3D_") || strings.HasPrefix(key, "WFDXCOMPAT_") ||
			strings.HasPrefix(key, "ROSETTA_") ||
			strings.HasPrefix(key, "LEVI_METAL") || strings.HasPrefix(key, "LEVI_WINE_RENDER_SCALE") ||
			strings.HasPrefix(key, "LEVI_GPTK_DLSS_") || strings.HasPrefix(key, "METALSHARP_") ||
			strings.HasPrefix(key, "MVK_CONFIG_") ||
			key == "LEVI_WINE_UNIX_BIN" || key == "LEVI_GAME_ICON_PATH" || key == "LEVI_GAME_TITLE" ||
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
	dlssActive := p.EffectiveDLSS()
	fxFactor := p.SpatialMetalFXFactor()
	useMetal3 := !p.EnableMetal4 || macOSVersion()[0] < 27
	lane := "mullion"
	switch m.effectiveGraphics(p.Graphics) {
	case "d3dmetal":
		lane = "d3dmetal"
		values["GRAPHICS_BACKEND"] = "d3dmetal"
		d3dDir, _ := m.d3dMetalForProfile(p)
		values["D3DMETAL_RUNTIME_DIR"] = d3dDir
		if m.selectedD3DMetalVersionFor(p.D3DMetalVersion, p.EnableRayTracing) == "gptk4" {
			if p.usesMetal4D3DMetal() {
				values["D3DM_MTL4"] = "1"
			} else {
				values["D3DM_MTL4"] = "0"
			}
		}
		if p.EnableRayTracing {
			values["D3DM_SUPPORT_DXR"] = "1"
		}
		if fxFactor != "" || dlssActive {
			values["D3DM_ENABLE_METALFX"] = "1"
		}
		if dlssActive {
			values["D3DM_VENDOR_ID"] = nvidiaSpoofVendorID
			values["D3DM_DEVICE_ID"] = nvidiaSpoofDeviceID
			values["D3DM_DEVICE_DESCRIPTION"] = nvidiaSpoofDeviceDesc
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
		if p.EnableRayTracing {
			values["MVK_CONFIG_ENABLE_EXPERIMENTAL_RAY_TRACING"] = "1"
		} else {
			values["VKD3D_CONFIG"] = "nodxr"
		}
		if p.MaxFPS > 0 {
			values["VKD3D_FRAME_RATE"] = fps
			values["DXVK_FRAME_RATE"] = fps
		}
		var dxvkOptions []string
		if dlssActive {
			values["DXVK_ENABLE_NVAPI"] = "1"
			values["VKD3D_METALFX_SPATIAL"] = "0"
			dxvkOptions = append(dxvkOptions, dxgiSpoofConfig)
			if scale := dlssScaleEnvValue(p.DLSSQuality); scale != "" {
				values["METALSHARP_DLSS_SCALE"] = scale
			}
		} else if fxFactor != "" {
			dxvkOptions = append(dxvkOptions, fmt.Sprintf(
				"d3d11.metalSpatialUpscale = True; d3d12.metalSpatialUpscale = True; d3d11.metalSpatialUpscaleFactor = %s; d3d12.metalSpatialUpscaleFactor = %s",
				fxFactor, fxFactor,
			))
		}
		if len(dxvkOptions) > 0 {
			values["DXVK_CONFIG"] = strings.Join(dxvkOptions, "; ")
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
		if p.EnableRayTracing {
			values["MULLION_D3D12_RAYTRACING"] = "1"
		} else {
			values["MULLION_D3D12_RAYTRACING"] = "0"
		}
		// Override mullion.conf's bundled 60 FPS cap when MaxFPS is 0 (unlimited)
		// and apply the explicit cap for both D3D11 and D3D12 when > 0.
		options = append(options, "d3d11.preferredMaxFrameRate = "+fps, "d3d12.preferredMaxFrameRate = "+fps)
		if dlssActive {
			values["MULLION_ENABLE_NVEXT"] = "1"
			options = append(options, dxgiSpoofConfig)
			if scale := dlssScaleEnvValue(p.DLSSQuality); scale != "" {
				values["METALSHARP_DLSS_SCALE"] = scale
			}
		} else {
			values["MULLION_ENABLE_NVEXT"] = "0"
		}
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

// PrepareGameDirectory enables MetalSharp's launched-game detection (used for
// MetalFX display-size adjustment, NGX/DLSS discovery, and GPU spoofing).
// The upstream loader checks for .metalsharp/launch.env beside the executable.
func (p Profile) PrepareGameDirectory(exe string) error {
	if p.metalSharp.app == "" {
		return nil
	}
	effective := p.metalSharp.effectiveGraphics(p.Graphics)
	needsLaunchEnv := p.EffectiveDLSS() || p.EffectiveRayTracing() ||
		(p.SpatialMetalFXFactor() != "" && (effective == "mullion" || effective == "vkd3d"))
	if !needsLaunchEnv {
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
