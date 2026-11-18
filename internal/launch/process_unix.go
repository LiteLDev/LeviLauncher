//go:build darwin

package launch

import (
	"context"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"path/filepath"
	"syscall"
	"time"
)

func gameProcessIDs(dir string) map[uint32]struct{} {
	result := map[uint32]struct{}{}
	expected := utils.CanonicalWindowsPath(filepath.Join(dir, "Minecraft.Windows.exe"))
	for _, p := range gameruntime.Processes() {
		if utils.CanonicalWindowsPath(p.Executable) == expected {
			result[uint32(p.PID)] = struct{}{}
		}
	}
	return result
}
func isProcessAlive(pid uint32) bool { return syscall.Kill(int(pid), 0) == nil }
func isGameWindowVisible(pids map[uint32]struct{}) bool {
	for pid := range pids {
		if gameruntime.WindowVisible(int(pid)) {
			return true
		}
	}
	return false
}
func WaitPIDExit(ctx context.Context, pid uint32) (uint32, error) {
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	for isProcessAlive(pid) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
	return 0, nil
}
