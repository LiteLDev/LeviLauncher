//go:build darwin

// Package gameruntime executes Windows game components in the selected Wine prefix.
package gameruntime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Profile struct {
	App               string
	Wine              string
	Prefix            string
	Graphics          string
	D3DMetalVersion   string
	EnableMetal4      bool
	Sync              string
	ThreadQoS         string
	DisableRosettaAVX bool
	LeftCommandIsCtrl bool
	RetinaMode        bool
	MetalHUD          bool
	VSync             string
	MaxFPS            int
	EnableMetalFX     bool
	MetalFXScale      string
	CompatDebugLog    bool
	metalSharp        metalSharpRuntime
}

func Default() (Profile, error) {
	s, err := LoadSettings()
	if err != nil {
		return Profile{}, err
	}
	return profileFromSettings(s)
}

func (p Profile) WindowsPath(host string) (string, error) {
	absolute, err := filepath.Abs(host)
	if err != nil {
		return "", err
	}
	root := filepath.Join(p.Prefix, "drive_c")
	if relative, err := filepath.Rel(root, absolute); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return `C:\` + strings.ReplaceAll(relative, "/", `\`), nil
	}
	// Use only a verified drive mapping; do not assume every prefix exports Z:.
	mapped, err := filepath.EvalSymlinks(filepath.Join(p.Prefix, "dosdevices", "z:"))
	if err != nil || mapped != "/" {
		return "", errors.New("selected Wine prefix does not map the host root as Z:")
	}
	return `Z:` + strings.ReplaceAll(absolute, "/", `\`), nil
}
func (p Profile) Environment() []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == "__CFBundleIdentifier" || strings.HasPrefix(key, "CX_") || strings.HasPrefix(key, "WINE") || strings.HasPrefix(key, "DYLD_") || strings.HasPrefix(key, "DXVK_") || strings.HasPrefix(key, "DXMT_") {
			continue
		}
		values[key] = value
	}
	values["WINEPREFIX"] = p.Prefix
	if p.crossOverRoot() == "" {
		values["WINELOADER"] = p.Wine
		values["WINESERVER"] = filepath.Join(filepath.Dir(p.Wine), "wineserver")
	}
	values["WINEDEBUG"] = "-all"
	if p.metalSharp.app != "" {
		p.metalSharp.environment(p, values)
	} else if p.App != "" {
		support := filepath.Join(p.App, "Contents/SharedSupport")
		engine := filepath.Join(support, "wine")
		renderer := filepath.Join(support, "renderer/d3dmetal")
		frameworks := filepath.Join(p.App, "Contents/Frameworks")
		values["WINEDLLPATH"] = strings.Join([]string{filepath.Join(engine, "lib/wine/x86_64-windows"), filepath.Join(engine, "lib/wine/i386-windows"), filepath.Join(engine, "lib/wine/x86_64-unix")}, ":")
		values["WINEDLLPATH_D3DMETAL"] = filepath.Join(renderer, "wine")
		values["DYLD_FALLBACK_LIBRARY_PATH"] = strings.Join([]string{filepath.Join(engine, "lib"), frameworks, filepath.Join(renderer, "external"), filepath.Join(frameworks, "GStreamer.framework/Versions/1.0/lib"), "/usr/lib", "/usr/libexec", "/usr/lib/system"}, ":")
		values["CX_APPLEGPTK_LIBD3DSHARED_PATH"] = filepath.Join(renderer, "external/libd3dshared.dylib")
		values["D3DMETAL_FRAMEWORK_PATH"] = filepath.Join(renderer, "external/D3DMetal.framework/D3DMetal")
		values["WINEDLLOVERRIDES"] = "d3d11,d3d12,d3d12core,dxgi=n,b;mscoree,mshtml="
		values["WINEESYNC"] = "1"
		values["WINEMSYNC"] = "1"
		values["TERM"] = "dumb"
		values["XPC_SERVICE_NAME"] = "0"
	}
	if p.metalSharp.app == "" && p.Graphics == "wined3d" {
		delete(values, "WINEDLLPATH_D3DMETAL")
		delete(values, "CX_APPLEGPTK_LIBD3DSHARED_PATH")
		delete(values, "D3DMETAL_FRAMEWORK_PATH")
		values["WINEDLLOVERRIDES"] = "d3d11,d3d12,d3d12core,dxgi=b;mscoree,mshtml="
	}
	switch p.Sync {
	case "msync":
		values["WINEMSYNC"], values["WINEESYNC"], values["WINEFSYNC"] = "1", "0", "0"
	case "esync":
		values["WINEMSYNC"], values["WINEESYNC"], values["WINEFSYNC"] = "0", "1", "0"
	case "none":
		values["WINEMSYNC"], values["WINEESYNC"], values["WINEFSYNC"] = "0", "0", "0"
	}
	values["XGR_MODS"] = "0"
	delete(values, "XGR_TRACE")
	delete(values, "MCWINRT_TRACE")
	if p.CompatDebugLog {
		values["XGR_TRACE"] = "1"
		values["MCWINRT_TRACE"] = "1"
	}
	for _, key := range []string{"MICROSOFT_WINDOWSAPPRUNTIME_BOOTSTRAP_INITIALIZE_SHOWUI", "MICROSOFT_WINDOWSAPPRUNTIME_BOOTSTRAP_INITIALIZE_FAILFAST", "MICROSOFT_WINDOWSAPPRUNTIME_DEPLOYMENT_INITIALIZE_ONERRORSHOWUI"} {
		values[key] = "0"
	}
	if _, ok := values["NO_PROXY"]; !ok {
		values["NO_PROXY"] = "localhost,127.0.0.1,::1"
	}
	var keys []string
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
func (p Profile) Command(ctx context.Context, exe string, args ...string) *exec.Cmd {
	argv := append([]string{exe}, args...)
	if p.crossOverRoot() != "" {
		// Use the vendor wrapper so it loads the selected bottle's own
		// libraries, renderer and defaults instead of the free Wine layout.
		options := []string{"--bottle", p.Prefix, "--no-gui", "--no-update", "--debugmsg", "-all"}
		var overrides []string
		if p.Graphics != "" && p.Graphics != "auto" {
			overrides = append(overrides, "CX_GRAPHICS_BACKEND="+p.Graphics)
		}
		if p.Sync != "" && p.Sync != "auto" {
			msync, esync := "0", "0"
			if p.Sync == "msync" {
				msync = "1"
			}
			if p.Sync == "esync" {
				esync = "1"
			}
			overrides = append(overrides, "WINEMSYNC="+msync, "WINEESYNC="+esync, "WINEFSYNC=0")
		}
		if len(overrides) > 0 {
			options = append(options, "--env", strings.Join(overrides, " "))
		}
		argv = append(options, argv...)
	}
	cmd := exec.CommandContext(ctx, p.Wine, argv...)
	cmd.Env = p.Environment()
	return cmd
}

func (p Profile) crossOverRoot() string {
	real, _ := filepath.EvalSymlinks(p.Wine)
	for _, path := range []string{p.Wine, real} {
		root := filepath.Dir(filepath.Dir(path))
		parent := filepath.Base(filepath.Dir(path))
		if (parent != "bin" && parent != "CrossOver-Hosted Application") || filepath.Base(root) != "CrossOver" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "bin/cxbottle")); err == nil {
			return root
		}
	}
	return ""
}
func Command(ctx context.Context, exe string, args ...string) (*exec.Cmd, error) {
	p, err := Default()
	if err != nil {
		return nil, err
	}
	return p.Command(ctx, exe, args...), nil
}
func HasSystemDLL(name string) bool {
	p, err := Default()
	if err != nil {
		return false
	}
	return p.HasSystemDLL(name)
}

func (p Profile) HasSystemDLL(name string) bool {
	info, err := os.Stat(filepath.Join(p.Prefix, "drive_c/windows/system32", name))
	return err == nil && info.Mode().IsRegular()
}
