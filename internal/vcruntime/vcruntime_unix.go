//go:build darwin

package vcruntime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

const vcRuntimeDownloadURL = "https://aka.ms/vc14/vc_redist.x64.exe"

func IsInstalled() bool {
	p, err := gameruntime.Default()
	return err == nil && IsInstalledIn(p)
}

func IsInstalledIn(p gameruntime.Profile) bool {
	return p.HasSystemDLL("vcruntime140.dll") && p.HasSystemDLL("msvcp140.dll")
}

func Install(ctx context.Context, p gameruntime.Profile) error {
	dir, err := apppath.InstallersDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "vc_redist.x64.exe")
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := downloader.Download(ctx, vcRuntimeDownloadURL, path, downloader.SyncOptions{}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	win, err := p.WindowsPath(path)
	if err != nil {
		return err
	}
	if strings.ContainsAny(win, "\"\r\n") {
		return fmt.Errorf("invalid Visual C++ installer path")
	}
	var output bytes.Buffer
	command := `start "" /wait "` + strings.ReplaceAll(win, "%", "%%") + `" /install /quiet /norestart`
	if err := p.RunHook(ctx, dir, command, &output); err != nil {
		return fmt.Errorf("install Visual C++: %w: %s", err, strings.TrimSpace(output.String()))
	}
	if !IsInstalledIn(p) {
		return fmt.Errorf("Visual C++ installer finished without the required DLLs")
	}
	return nil
}

func EnsureInteractive(ctx context.Context) {
	if !IsInstalled() {
		eventsink.From(ctx).Emit(EventMissing, "Configure the selected Wine environment's Windows VC runtime before launching the game")
	}
}

func EnsureStartupInteractive(context.Context) bool { return IsInstalled() }
