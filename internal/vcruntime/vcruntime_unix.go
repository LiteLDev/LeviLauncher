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
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

const vcRuntimeDownloadURL = "https://aka.ms/vc14/vc_redist.x64.exe"

func IsInstalled() bool {
	p, err := gameruntime.Default()
	return err == nil && IsInstalledIn(p)
}

func IsInstalledIn(p gameruntime.Profile) bool {
	for _, name := range []string{"vcruntime140.dll", "msvcp140.dll"} {
		if !p.HasSystemDLL(name) {
			return false
		}
		if p.BundledVCRuntimeDir() != "" {
			file, err := os.Open(filepath.Join(p.Prefix, "drive_c/windows/system32", name))
			if err != nil {
				return false
			}
			var marker [16]byte
			_, err = file.ReadAt(marker[:], 64)
			file.Close()
			if err != nil || string(marker[:]) == "Wine builtin DLL" {
				return false
			}
		}
	}
	return true
}

func Install(ctx context.Context, p gameruntime.Profile) error {
	if installed, err := installBundledRuntime(ctx, p); installed || err != nil {
		return err
	}
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

// MetalSharp Wine ships Microsoft's extracted x64 redistributable DLLs.
func installBundledRuntime(ctx context.Context, p gameruntime.Profile) (bool, error) {
	dir := p.BundledVCRuntimeDir()
	if dir == "" {
		return false, nil
	}
	for _, name := range []string{"vcruntime140.dll", "msvcp140.dll"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.Mode().IsRegular() {
			return false, nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".dll") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return false, err
		}
		target := filepath.Join(p.Prefix, "drive_c/windows/system32", entry.Name())
		current, err := os.ReadFile(target)
		if err == nil && (bytes.Equal(current, data) || !bytes.HasPrefix(current[min(64, len(current)):], []byte("Wine builtin DLL"))) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if err := fsx.WriteFileAtomic(target, data, 0644); err != nil {
			return false, err
		}
	}
	return IsInstalledIn(p), nil
}

func EnsureInteractive(ctx context.Context) {
	if !IsInstalled() {
		eventsink.From(ctx).Emit(EventMissing, "Configure the selected Wine environment's Windows VC runtime before launching the game")
	}
}

func EnsureStartupInteractive(context.Context) bool { return IsInstalled() }
