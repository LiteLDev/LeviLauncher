package launch

import (
	"context"
	"encoding/csv"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/registry"
)

const (
	EventMcLaunchStart         = "mc.launch.start"
	EventMcLaunchDone          = "mc.launch.done"
	EventMcLaunchFailed        = "mc.launch.failed"
	EventGamingServicesMissing = "gamingservices.missing"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procFindWindowW     = user32.NewProc("FindWindowW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

func FindWindowByTitleExact(title string) bool {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
	if hwnd == 0 {
		return false
	}

	isVisible, _, _ := procIsWindowVisible.Call(hwnd)
	return isVisible != 0
}

func EnsureGamingServicesInstalled(ctx context.Context) bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err != nil {
		application.Get().Event.Emit(EventGamingServicesMissing, struct{}{})
		return false
	}
	return true
}

func isGameRunning(versionDir string) bool {
	if versionDir == "" {
		return false
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_Process -Filter \"Name='Minecraft.Windows.exe'\" | Select-Object ExecutablePath | ConvertTo-Csv -NoTypeInformation")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	reader := csv.NewReader(strings.NewReader(string(out)))
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return false
	}
	idx := -1
	for i, col := range records[0] {
		if strings.EqualFold(col, "ExecutablePath") {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	cleanVerDir := strings.ToLower(filepath.Clean(versionDir))
	for _, row := range records[1:] {
		if len(row) <= idx {
			continue
		}
		path := strings.ToLower(filepath.Clean(row[idx]))
		if strings.HasPrefix(path, cleanVerDir) {
			return true
		}
	}
	return false
}

func isGameWindowVisible() bool {
	return FindWindowByTitleExact("Minecraft") || FindWindowByTitleExact("Minecraft Preview")
}

func waitForGameProcess(ctx context.Context, versionDir string, timeout time.Duration) (bool, bool) {
	if isGameRunning(versionDir) {
		return true, false
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, true
		case <-timer.C:
			return false, false
		case <-ticker.C:
			if isGameRunning(versionDir) {
				return true, false
			}
		}
	}
}

func waitForGameWindow(ctx context.Context, versionDir string, timeout time.Duration) (bool, bool, bool) {
	if isGameWindowVisible() {
		return true, true, false
	}
	if !isGameRunning(versionDir) {
		return false, false, false
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	lastRunningCheck := time.Now()
	for {
		select {
		case <-ctx.Done():
			return false, false, true
		case <-timer.C:
			return true, false, false
		case <-ticker.C:
			if isGameWindowVisible() {
				return true, true, false
			}
			if time.Since(lastRunningCheck) >= 2*time.Second {
				lastRunningCheck = time.Now()
				if !isGameRunning(versionDir) {
					return false, false, false
				}
			}
		}
	}
}

func MonitorGameProcess(ctx context.Context, versionDir string) {
	found, canceled := waitForGameProcess(ctx, versionDir, 120*time.Second)
	if canceled {
		return
	}

	if !found {
		application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
		discord.SetLauncherIdle()
		return
	}

	found, visible, canceled := waitForGameWindow(ctx, versionDir, 60*time.Second)
	if canceled {
		return
	}

	application.Get().Event.Emit(EventMcLaunchDone, struct{}{})

	if !found {
		discord.SetLauncherIdle()
		return
	}

	if visible {
		w := application.Get().Window.Current()
		if w != nil {
			w.Minimise()
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !isGameRunning(versionDir) {
				discord.SetLauncherIdle()
				// 似乎会出现Bug，后续修复
				//w := application.Get().Window.Current()
				//if w != nil {
					//w.Restore()
				//}
				return
			}
		}
	}
}
