//go:build darwin || linux

// Package gameruntime executes Windows game components in the selected Wine prefix.
package gameruntime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Profile struct {
	App    string
	Wine   string
	Prefix string
}

func Default() (Profile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Profile{}, err
	}
	p := Profile{}
	if runtime.GOOS == "darwin" {
		p.App = os.Getenv("LEVI_WINE_APP")
		if p.App == "" {
			p.App = filepath.Join(home, "Applications", "Minecraft Wine", "Minecraft Wine.app")
		}
		p.Wine = filepath.Join(p.App, "Contents/SharedSupport/wine/bin/wine")
		p.Prefix = filepath.Join(p.App, "Contents/SharedSupport/prefix")
	} else {
		p.Wine = os.Getenv("LEVI_WINE_EXECUTABLE")
		if p.Wine == "" {
			p.Wine, err = exec.LookPath("wine")
			if err != nil {
				return p, errors.New("select a Wine executable before launching Windows components")
			}
		}
		data := os.Getenv("XDG_DATA_HOME")
		if data == "" {
			data = filepath.Join(home, ".local/share")
		}
		p.Prefix = filepath.Join(data, "levilauncher/wineprefix")
	}
	if value := os.Getenv("LEVI_WINEPREFIX"); value != "" {
		p.Prefix = value
	}
	if _, err = os.Stat(p.Wine); err != nil {
		return p, errors.New("free Wine runtime is missing; select the installed Minecraft Wine.app")
	}
	if _, err = os.Stat(filepath.Join(p.Prefix, "drive_c")); err != nil {
		return p, errors.New("selected Wine prefix is not initialized")
	}
	return p, nil
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
	values["WINELOADER"] = p.Wine
	values["WINESERVER"] = filepath.Join(filepath.Dir(p.Wine), "wineserver")
	values["WINEDEBUG"] = "-all"
	if p.App != "" {
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
	values["XGR_MODS"] = "0"
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
	cmd := exec.CommandContext(ctx, p.Wine, append([]string{exe}, args...)...)
	cmd.Env = p.Environment()
	return cmd
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
	info, err := os.Stat(filepath.Join(p.Prefix, "drive_c/windows/system32", name))
	return err == nil && info.Mode().IsRegular()
}
