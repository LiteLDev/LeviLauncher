//go:build darwin

package gameinput

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

func IsInstalled() bool { return gameruntime.HasSystemDLL("GameInputRedist.dll") }
func runInstaller(ctx context.Context, path string) error {
	p, err := gameruntime.Default()
	if err != nil {
		return err
	}
	win, err := p.WindowsPath(path)
	if err != nil {
		return err
	}
	if strings.ContainsAny(win, "\"\r\n") {
		return fmt.Errorf("invalid GameInput installer path")
	}
	win = strings.ReplaceAll(win, "%", "%%")
	return gameruntime.RunHook(ctx, filepath.Dir(path), `msiexec.exe /i "`+win+`" /passive`, nil)
}
