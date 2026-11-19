//go:build darwin

package gameinput

import (
	"bytes"
	"context"
	"fmt"
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
	win, err := p.WindowsPath(path)
	if err != nil {
		return err
	}
	if strings.ContainsAny(win, "\"\r\n") {
		return fmt.Errorf("invalid GameInput installer path")
	}
	win = strings.ReplaceAll(win, "%", "%%")
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var output bytes.Buffer
	if err := p.RunHook(ctx, filepath.Dir(path), `msiexec.exe /i "`+win+`" /passive /norestart /L*v "%TEMP%\LeviGameInputInstall.log"`, &output); err != nil {
		return fmt.Errorf("install GameInput: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}
