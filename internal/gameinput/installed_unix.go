//go:build darwin

package gameinput

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

const redistDLL = "GameInputRedist.dll"

func IsInstalled() bool { return gameruntime.HasSystemDLL(redistDLL) }

func IsInstalledIn(p gameruntime.Profile) bool { return p.HasSystemDLL(redistDLL) }

// Install stages the bundled MSI and waits for it to finish in this prefix.
func Install(ctx context.Context, p gameruntime.Profile) error {
	dir, err := apppath.InstallersDir()
	if err != nil {
		return err
	}
	if len(msiBytes) == 0 {
		return fmt.Errorf("GameInput installer is missing from the launcher")
	}
	path := filepath.Join(dir, "GameInputRedist.msi")
	if err := fsx.WriteFileAtomic(path, msiBytes, 0600); err != nil {
		return err
	}
	if err := runInstallerIn(ctx, p, path); err != nil {
		return err
	}
	if !IsInstalledIn(p) {
		return fmt.Errorf("GameInput installer finished without %s", redistDLL)
	}
	return nil
}

func runInstaller(ctx context.Context, path string) error {
	p, err := gameruntime.Default()
	if err != nil {
		return err
	}
	return runInstallerIn(ctx, p, path)
}

func runInstallerIn(ctx context.Context, p gameruntime.Profile, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	dir := filepath.Dir(path)
	workaround := p.GameInputServiceWorkaround()
	if workaround != "" {
		temp, err := os.MkdirTemp("", "levi-gameinput-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		path, err = prepareWineInstaller(ctx, p, path, temp)
		if err != nil {
			return err
		}
		dir = temp
	}
	win, err := installerArg(p, path)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	command := `msiexec.exe /i ` + win + ` /passive /norestart /L*v "%TEMP%\LeviGameInputInstall.log"`
	if workaround != "" {
		const checkExit = "\nif errorlevel 1 exit /b %errorlevel%\n"
		command = wineMSIDatabase + " -i ServiceInstall.idt" + checkExit + command + checkExit + workaround
	}
	if err := p.RunHook(ctx, dir, command, &output); err != nil {
		return fmt.Errorf("install GameInput: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

const wineMSIDatabase = "msidb.exe -d GameInputRedist.msi -f ."

func prepareWineInstaller(ctx context.Context, p gameruntime.Profile, original, dir string) (string, error) {
	// Register the service as disabled on this Wine version. Keep the MSI's
	// configuration action, which places the DLL and writes device mappings.
	data, err := os.ReadFile(original)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "GameInputRedist.msi")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err := p.RunHook(ctx, dir, wineMSIDatabase+" -e ServiceInstall", &output); err != nil {
		return "", fmt.Errorf("export GameInput service table: %w: %s", err, strings.TrimSpace(output.String()))
	}
	if err := disableGameInputService(filepath.Join(dir, "ServiceInstall.idt")); err != nil {
		return "", err
	}
	return path, nil
}

func installerArg(p gameruntime.Profile, path string) (string, error) {
	win, err := p.WindowsPath(path)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(win, "\"\r\n") {
		return "", fmt.Errorf("invalid GameInput installer path")
	}
	return `"` + strings.ReplaceAll(win, "%", "%%") + `"`, nil
}
