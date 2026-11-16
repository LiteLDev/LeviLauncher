package gameinput

import (
	"context"
	winreg "golang.org/x/sys/windows/registry"
	"os/exec"
)

func IsInstalled() bool {
	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInputRedist`, winreg.READ); err == nil {
		return true
	}
	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInput`, winreg.READ); err == nil {
		return true
	}
	return false
}

func installerCommand(ctx context.Context, path string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "msiexec", "/i", path, "/passive"), nil
}
