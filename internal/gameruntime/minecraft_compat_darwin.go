package gameruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

//go:embed compat/xgameruntime.dll
var xgameruntimeDLL []byte

//go:embed compat/mcwinrt.dll
var mcwinrtDLL []byte

//go:embed compat/winrt.reg
var winrtRegistry []byte

var minecraftSetupMu sync.Mutex

const sikarugirGameInputServiceVersion = "wine sikarugir 11.0 (revision 1)"

const gameInputServiceCommands = `reg.exe query "HKLM\System\CurrentControlSet\Services\GameInputRedistService" /v Start >nul 2>&1` +
	"\nif errorlevel 1 exit /b 0\n" +
	`reg.exe add "HKLM\System\CurrentControlSet\Services\GameInputRedistService" /v Start /t REG_DWORD /d 4 /f` +
	"\nif errorlevel 1 exit /b %errorlevel%\n" +
	"sc.exe stop GameInputRedistService >nul 2>&1\nexit /b 0"

// GameInputServiceWorkaround keeps the MSI's input libraries without starting
// the service on the managed Wine runtime and the affected legacy build.
func (p Profile) GameInputServiceWorkaround() string {
	if p.metalSharp.app != "" || p.sikarugirVersion() == sikarugirGameInputServiceVersion {
		return gameInputServiceCommands
	}
	return ""
}

func (p Profile) sikarugirVersion() string {
	support := filepath.Join(p.App, "Contents/SharedSupport")
	wine := filepath.Join(support, "wine")
	if p.App == "" || !sameRuntimePath(p.Wine, filepath.Join(wine, "bin/wine")) ||
		!sameRuntimePath(p.Prefix, filepath.Join(support, "prefix")) {
		return ""
	}
	version, _ := os.ReadFile(filepath.Join(wine, "version"))
	return strings.TrimSpace(string(version))
}

// PrepareMinecraft installs and updates the launcher's compatibility shims.
// Third-party replacements already present in the prefix are preserved.
func (p Profile) PrepareMinecraft(ctx context.Context) error {
	version := p.sikarugirVersion()
	metalSharp := p.metalSharp.app != ""
	if !metalSharp && !strings.HasPrefix(version, "wine sikarugir ") {
		return nil
	}

	minecraftSetupMu.Lock()
	defer minecraftSetupMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if metalSharp {
		version = "metalsharp"
	} else if err := configureMinecraftWrapper(ctx, filepath.Join(p.App, "Contents/Info.plist")); err != nil {
		return err
	}
	system := filepath.Join(p.Prefix, "drive_c/windows/system32")
	added := false
	for _, component := range []struct {
		name       string
		data       []byte
		legacySHAs []string
	}{
		{"xgameruntime.dll", xgameruntimeDLL, []string{
			"1542228659aa1175696356c58fefd5d45537bec39fe4cd96a4323bd65862f10a",
			"bdbd8faa5154a3f90019d5d09bb5fc493a4264798e217d977077a4b95676e490",
		}},
		{"mcwinrt.dll", mcwinrtDLL, []string{
			"b96683a3a5ef509511645441145b4429e5afb65ca2ea7554d46ab9971b25d071",
		}},
	} {
		target := filepath.Join(system, component.name)
		current, err := os.ReadFile(target)
		if err == nil {
			if bytes.Equal(current, component.data) || !slices.Contains(component.legacySHAs, fmt.Sprintf("%x", sha256.Sum256(current))) {
				continue
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := fsx.WriteFileAtomic(target, component.data, 0644); err != nil {
			return err
		}
		added = true
	}

	marker := filepath.Join(p.Prefix, ".levilauncher-minecraft-compat")
	setupVersion := "1\n" + version
	serviceWorkaround := metalSharp || version == sikarugirGameInputServiceVersion
	if serviceWorkaround {
		setupVersion = "2\n" + version
	}
	if current, err := os.ReadFile(marker); err == nil && !added && string(current) == setupVersion {
		return nil
	}
	temp, err := os.MkdirTemp("", "levi-winrt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	registry := filepath.Join(temp, "winrt.reg")
	if err := os.WriteFile(registry, winrtRegistry, 0600); err != nil {
		return err
	}
	win, err := p.WindowsPath(registry)
	if err != nil {
		return err
	}
	if strings.ContainsAny(win, "\"\r\n") {
		return fmt.Errorf("invalid WinRT registry file path")
	}
	command := `reg.exe import "` + strings.ReplaceAll(win, "%", "%%") + `"`
	if serviceWorkaround {
		command += "\nif errorlevel 1 exit /b %errorlevel%\n" + gameInputServiceCommands
	}
	var output bytes.Buffer
	if err := p.RunHook(ctx, temp, command, &output); err != nil {
		return fmt.Errorf("register Wine compatibility components: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return fsx.WriteFileAtomic(marker, []byte(setupVersion), 0600)
}

// ApplyMacDriverSettings syncs macOS-only Wine Mac Driver key mapping preferences
// into the selected prefix. Launches stay zero-overhead when the setting is
// unchanged.
func (p Profile) ApplyMacDriverSettings(ctx context.Context) error {
	if p.Prefix == "" {
		return nil
	}
	desired := "0"
	if p.LeftCommandIsCtrl {
		desired = "1"
	}
	wineRetina := p.RetinaMode && !p.UsesNativeRetinaMetalFXUpscale()
	if p.metalSharp.app != "" || wineRetina {
		desired += fmt.Sprintf("/retina=%t", wineRetina)
	}
	marker := filepath.Join(p.Prefix, ".levilauncher-mac-input")
	current, err := os.ReadFile(marker)
	if err == nil && strings.TrimSpace(string(current)) == desired {
		return nil
	}
	if os.IsNotExist(err) && desired == "0" {
		return nil
	}

	minecraftSetupMu.Lock()
	defer minecraftSetupMu.Unlock()
	if current, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(current)) == desired {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := `reg.exe add "HKCU\Software\Wine\Mac Driver" /v LeftCommandIsCtrl /t REG_SZ /d N /f`
	if p.LeftCommandIsCtrl {
		cmd = `reg.exe add "HKCU\Software\Wine\Mac Driver" /v LeftCommandIsCtrl /t REG_SZ /d Y /f` +
			"\nif errorlevel 1 exit /b %errorlevel%\n" +
			`reg.exe add "HKCU\Software\Wine\Mac Driver" /v LeftOptionIsAlt /t REG_SZ /d Y /f`
	}
	if p.metalSharp.app != "" || wineRetina || strings.Contains(string(current), "/retina=") {
		value := "N"
		if wineRetina {
			value = "Y"
		}
		cmd += "\nif errorlevel 1 exit /b %errorlevel%\n" + `reg.exe add "HKCU\Software\Wine\Mac Driver" /v RetinaMode /t REG_SZ /d ` + value + " /f"
	}
	var output bytes.Buffer
	if err := p.RunHook(ctx, p.Prefix, cmd, &output); err != nil {
		return fmt.Errorf("apply macOS Wine input settings: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return fsx.WriteFileAtomic(marker, []byte(desired+"\n"), 0600)
}

func configureMinecraftWrapper(ctx context.Context, plist string) error {
	const field = "CLI Custom Commands"
	read := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", field, "raw", "-o", "-", plist)
	raw, err := read.Output()
	if err != nil {
		return fmt.Errorf("read Sikarugir launch configuration: %w", err)
	}
	current := strings.TrimSuffix(string(raw), "\n")
	updated := minecraftWrapperCommands(current)
	if updated == current {
		return nil
	}
	write := exec.CommandContext(ctx, "/usr/bin/plutil", "-replace", field, "-string", updated, plist)
	if data, err := write.CombinedOutput(); err != nil {
		return fmt.Errorf("write Sikarugir launch configuration: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

// minecraftWrapperCommands avoids a trailing semicolon because Sikarugir
// appends its own shell separator before invoking Wine.
func minecraftWrapperCommands(current string) string {
	var required []string
	for _, name := range []string{
		"XGR_MODS",
		"MICROSOFT_WINDOWSAPPRUNTIME_BOOTSTRAP_INITIALIZE_SHOWUI",
		"MICROSOFT_WINDOWSAPPRUNTIME_BOOTSTRAP_INITIALIZE_FAILFAST",
		"MICROSOFT_WINDOWSAPPRUNTIME_DEPLOYMENT_INITIALIZE_ONERRORSHOWUI",
	} {
		required = append(required, "export "+name+"=0")
	}
	// Repair the exact command written by the previous launcher version.
	if strings.TrimSpace(current) == strings.Join(required, "; ")+";" {
		return strings.Join(required, "; ")
	}
	var commands []string
	for _, command := range required {
		assignment := strings.TrimPrefix(command, "export ")
		name, _, _ := strings.Cut(assignment, "=")
		if !strings.Contains(current, name+"=") {
			commands = append(commands, command)
		}
	}
	if current != "" {
		commands = append(commands, current)
	}
	return strings.Join(commands, "; ")
}
