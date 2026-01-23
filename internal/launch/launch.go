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

func MonitorGameProcess(ctx context.Context, versionDir string) {
	const maxWait = 120
	var found bool
	for i := 0; i < maxWait; i++ {
		if isGameRunning(versionDir) {
			application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
			w := application.Get().Window.Current()
			if w != nil {
				w.Minimise()
			}
			found = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !found {
		application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
		discord.SetLauncherIdle()
		return
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
				w := application.Get().Window.Current()
				if w != nil {
					w.Restore()
				}
				return
			}
		}
	}
}
