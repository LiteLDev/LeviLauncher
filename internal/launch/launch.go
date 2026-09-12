package launch

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
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

var (
	quitRequested   atomic.Bool
	userHidLauncher atomic.Bool

	// activeMonitors keys the running monitors by version directory. Force
	// launches skip the already-running check, so the same game can be asked
	// to start twice; a second monitor would apply the launch and exit
	// behaviors a second time.
	activeMonitors sync.Map
)

// QuitRequested reports whether the current shutdown was asked for through
// QuitLauncher. The window-close hook relies on this to tell an explicit quit
// apart from the user pressing the close button: if it hid the window on an
// explicit quit, the launcher would stay alive in the tray forever.
func QuitRequested() bool {
	return quitRequested.Load()
}

// QuitLauncher terminates the launcher regardless of the minimise-to-tray
// setting. Every "quit" affordance outside the window close button goes
// through here.
func QuitLauncher() {
	quitRequested.Store(true)
	application.Get().Quit()
}

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

func normalizeProcessPath(p string) string {
	s := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	s = strings.TrimPrefix(s, `\\?\`)
	s = strings.TrimPrefix(s, `\??\`)
	return s
}

func isGameRunning(versionDir string) bool {
	if versionDir == "" {
		return false
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	cleanVerDir := normalizeProcessPath(versionDir)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}

	for {
		if versions.IsMinecraftExecutable(windows.UTF16ToString(entry.ExeFile[:])) {
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					p := normalizeProcessPath(windows.UTF16ToString(buf[:size]))
					_ = windows.CloseHandle(h)
					if strings.HasPrefix(p, cleanVerDir+string(filepath.Separator)) {
						return true
					}
				} else {
					_ = windows.CloseHandle(h)
				}
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return false
}

func isGameWindowVisible() bool {
	return FindWindowByTitleExact("Minecraft") || FindWindowByTitleExact("Minecraft Preview") ||
		FindWindowByTitleExact("Minecraft: Windows 10 Edition") || FindWindowByTitleExact("Minecraft: Windows 10 Edition Beta")
}

func isProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(h, &exitCode); err != nil {
		return false
	}

	const stillActive = 259
	return exitCode == stillActive
}

func waitForGameProcess(ctx context.Context, versionDir string, launchPID uint32, timeout time.Duration) (bool, bool, bool) {
	if isGameRunning(versionDir) {
		return true, false, false
	}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, false, true
		case <-timer.C:
			return false, false, false
		case <-ticker.C:
			if isGameRunning(versionDir) {
				return true, false, false
			}
			if launchPID != 0 && !isProcessAlive(launchPID) {
				return false, true, false
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

func MonitorGameProcess(ctx context.Context, versionDir string, launchPID int) {
	if _, running := activeMonitors.LoadOrStore(versionDir, struct{}{}); running {
		return
	}
	defer activeMonitors.Delete(versionDir)

	var pid uint32
	if launchPID > 0 {
		pid = uint32(launchPID)
	}

	found, launchExited, canceled := waitForGameProcess(ctx, versionDir, pid, 120*time.Second)
	if canceled {
		return
	}

	if !found {
		if launchExited {
			application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_GAME")
		} else {
			application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_TIMEOUT")
		}
		discord.SetLauncherIdle()
		return
	}

	var visible bool
	found, visible, canceled = waitForGameWindow(ctx, versionDir, 60*time.Second)
	if canceled {
		return
	}

	if !found {
		application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_GAME")
		discord.SetLauncherIdle()
		return
	}

	application.Get().Event.Emit(EventMcLaunchDone, struct{}{})

	// waitForGameWindow reports found without visible when it times out: the
	// game process is alive but never put a window on screen. Getting the
	// launcher out of the way then would leave the user with neither a game
	// window nor the launcher that could tell them what went wrong.
	if visible {
		switch config.GetOnGameLaunch() {
		case config.OnGameLaunchClose:
			QuitLauncher()
			return
		case config.OnGameLaunchHide:
			if w := GetMainWindow(); w != nil {
				w.Hide()
			}
		case config.OnGameLaunchKeep:
			// leave the launcher where it is
		default: // config.OnGameLaunchMinimize
			if w := GetMainWindow(); w != nil {
				w.Minimise()
			}
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

				switch config.GetOnGameExit() {
				case config.OnGameExitClose:
					QuitLauncher()
				case config.OnGameExitKeep:
					// leave the launcher where it is
				default: // config.OnGameExitReopen
					if !userHidLauncher.Load() {
						RestoreLauncherWindow()
					}
				}
				return
			}
		}
	}
}

// GetMainWindow returns the launcher window, or nil while it does not exist
// yet: a monitor started from the single-instance pipe can run before the
// window is created.
func GetMainWindow() application.Window {
	w, ok := application.Get().Window.GetByName("main")
	if !ok {
		return nil
	}
	return w
}

// MarkLauncherHiddenByUser records that the user themselves sent the launcher
// to the tray. A finishing game must not drag it back out, which would override
// an explicit decision and cover whatever the user moved on to.
func MarkLauncherHiddenByUser() {
	userHidLauncher.Store(true)
}

// RestoreLauncherWindow brings the launcher back to the foreground, keeping
// whatever size state it had. UnMinimise is the only restore step: Restore and
// the raw SW_RESTORE both drop a maximised window back to its pre-maximised
// size, so a maximised launcher would shrink on every game exit and tray click.
func RestoreLauncherWindow() {
	userHidLauncher.Store(false)

	w := GetMainWindow()
	if w == nil {
		return
	}
	w.Show()
	w.UnMinimise()
	w.Focus()
}
