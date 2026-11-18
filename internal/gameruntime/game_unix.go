//go:build darwin

package gameruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GameCommand uses a bundled Sikarugir launcher's own loader infrastructure.
// Its wrapper can exit before the game starts, unlike a direct Wine process.
func (p Profile) GameCommand(ctx context.Context, exe string, args ...string) (cmd *exec.Cmd, detached bool, err error) {
	wrapper := filepath.Join(p.App, "Contents/MacOS/wineskinlauncher")
	wine := filepath.Join(p.App, "Contents/SharedSupport/wine/bin/wine")
	if p.App != "" && sameRuntimePath(p.Wine, wine) {
		if info, statErr := os.Stat(wrapper); statErr == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			prefix := filepath.Join(p.App, "Contents/SharedSupport/prefix")
			if !sameRuntimePath(p.Prefix, prefix) {
				return nil, false, fmt.Errorf("Sikarugir requires the prefix belonging to the selected application: %s", prefix)
			}
			cmd = exec.CommandContext(ctx, wrapper, append([]string{exe}, args...)...)
			for _, entry := range p.Environment() {
				key, _, _ := strings.Cut(entry, "=")
				// The SDK prepares its engine and prefix. A bare Wine loader
				// inherited here makes Wine 11 child processes exit immediately.
				if key == "WINELOADER" || key == "WINESERVER" || key == "WINEPREFIX" ||
					strings.HasPrefix(key, "WINEDLLPATH") || strings.HasPrefix(key, "DYLD_") {
					continue
				}
				cmd.Env = append(cmd.Env, entry)
			}
			return cmd, true, nil
		}
	}
	guest, err := p.WindowsPath(exe)
	if err != nil {
		return nil, false, err
	}
	return p.Command(ctx, guest, args...), false, nil
}

func sameRuntimePath(a, b string) bool {
	a, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	b, err = filepath.EvalSymlinks(b)
	return err == nil && a == b
}
