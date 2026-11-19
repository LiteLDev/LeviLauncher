package gameruntime

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

//go:embed sikarugir/xgameruntime.dll
var xgameruntimeDLL []byte

//go:embed sikarugir/mcwinrt.dll
var mcwinrtDLL []byte

//go:embed sikarugir/winrt.reg
var winrtRegistry []byte

var sikarugirSetupMu sync.Mutex

// PrepareMinecraft fills the selected Sikarugir prefix's Minecraft-specific
// runtime gaps. Existing dependency binaries are retained.
func (p Profile) PrepareMinecraft(ctx context.Context) error {
	support := filepath.Join(p.App, "Contents/SharedSupport")
	wine := filepath.Join(support, "wine")
	if p.App == "" || !sameRuntimePath(p.Wine, filepath.Join(wine, "bin/wine")) ||
		!sameRuntimePath(p.Prefix, filepath.Join(support, "prefix")) {
		return nil
	}
	version, err := os.ReadFile(filepath.Join(wine, "version"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(version)), "wine sikarugir ") {
		return nil
	}

	sikarugirSetupMu.Lock()
	defer sikarugirSetupMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if err := configureMinecraftWrapper(ctx, filepath.Join(p.App, "Contents/Info.plist")); err != nil {
		return err
	}
	system := filepath.Join(p.Prefix, "drive_c/windows/system32")
	added := false
	for _, component := range []struct {
		name string
		data []byte
	}{{"xgameruntime.dll", xgameruntimeDLL}, {"mcwinrt.dll", mcwinrtDLL}} {
		target := filepath.Join(system, component.name)
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := fsx.WriteFileAtomic(target, component.data, 0644); err != nil {
			return err
		}
		added = true
	}

	marker := filepath.Join(p.Prefix, ".levilauncher-minecraft-compat")
	setupVersion := "1\n" + strings.TrimSpace(string(version))
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
	if strings.TrimSpace(string(version)) == "wine sikarugir 11.0 (revision 1)" {
		// This engine's GameInput service crashes at NtQueryWnfStateData.
		// Keep its installed DLL and suppress only the incompatible service.
		const service = `"HKLM\System\CurrentControlSet\Services\GameInputRedistService"`
		command += "\nif errorlevel 1 exit /b %errorlevel%\n" +
			"reg.exe query " + service + " /v Start >nul 2>&1\n" +
			"if errorlevel 1 exit /b 0\n" +
			"reg.exe add " + service + " /v Start /t REG_DWORD /d 4 /f"
	}
	var output bytes.Buffer
	if err := p.RunHook(ctx, temp, command, &output); err != nil {
		return fmt.Errorf("register Wine compatibility components: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return fsx.WriteFileAtomic(marker, []byte(setupVersion), 0600)
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
