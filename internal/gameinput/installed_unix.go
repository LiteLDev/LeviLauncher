//go:build darwin || linux

package gameinput

import (
	"context"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"os/exec"
)

func IsInstalled() bool { return gameruntime.HasSystemDLL("GameInputRedist.dll") }
func installerCommand(ctx context.Context, path string) (*exec.Cmd, error) {
	p, err := gameruntime.Default()
	if err != nil {
		return nil, err
	}
	win, err := p.WindowsPath(path)
	if err != nil {
		return nil, err
	}
	return p.Command(ctx, "msiexec.exe", "/i", win, "/passive"), nil
}
