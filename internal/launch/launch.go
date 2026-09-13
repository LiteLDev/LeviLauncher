package launch

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
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

func EnsureGamingServicesInstalled(ctx context.Context) bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err != nil {
		application.Get().Event.Emit(EventGamingServicesMissing, struct{}{})
		return false
	}
	return true
}

func isGameRunning(versionDir string) bool {
	return len(gameProcessIDs(versionDir)) != 0
}

func gameProcessIDs(versionDir string) map[uint32]struct{} {
	cleanVerDir := utils.CanonicalWindowsPath(versionDir)
	if cleanVerDir == "" {
		return nil
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}

	pids := make(map[uint32]struct{})
	for {
		if versions.IsMinecraftExecutable(windows.UTF16ToString(entry.ExeFile[:])) {
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					p := utils.NormalizeWindowsPath(windows.UTF16ToString(buf[:size]))
					_ = windows.CloseHandle(h)
					if strings.HasPrefix(p, cleanVerDir+string(filepath.Separator)) {
						pids[entry.ProcessID] = struct{}{}
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
	return pids
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
	if ctx.Err() != nil {
		return false, false, true
	}
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
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return false, false, true
		}
		pids := gameProcessIDs(versionDir)
		if len(pids) == 0 {
			return false, false, false
		}
		if isGameWindowVisible(pids) {
			return true, true, false
		}
		select {
		case <-ctx.Done():
			return false, false, true
		case <-timer.C:
			return isGameRunning(versionDir), false, false
		case <-ticker.C:
		}
	}
}

func MonitorGameProcess(ctx context.Context, versionDir string, launchPID int) {
	// Different linked folder names can refer to the same running game.
	versionDir = utils.CanonicalWindowsPath(versionDir)
	if _, running := activeMonitors.LoadOrStore(versionDir, struct{}{}); running {
		return
	}
	monitorGameProcess(ctx, versionDir, launchPID, false)
}

// The caller owns the activeMonitors entry, including during UWP activation.
func monitorGameProcess(ctx context.Context, versionDir string, launchPID int, launchBehaviorApplied bool) {
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
		if launchBehaviorApplied && !userHidLauncher.Load() {
			RestoreLauncherWindow()
		}
		if launchExited {
			application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_GAME")
		} else {
			application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_TIMEOUT")
		}
		discord.SetLauncherIdle()
		return
	}

	// An early observer already saw this instance's window. The user may
	// have minimized the game since then; do not wait for it to reappear.
	visible := launchBehaviorApplied
	if !visible {
		found, visible, canceled = waitForGameWindow(ctx, versionDir, 60*time.Second)
	}
	if canceled {
		return
	}

	if !found {
		if launchBehaviorApplied && !userHidLauncher.Load() {
			RestoreLauncherWindow()
		}
		application.Get().Event.Emit(EventMcLaunchFailed, "ERR_LAUNCH_GAME")
		discord.SetLauncherIdle()
		return
	}

	application.Get().Event.Emit(EventMcLaunchDone, struct{}{})

	// waitForGameWindow reports found without visible when it times out: the
	// game process is alive but never put a window on screen. Getting the
	// launcher out of the way then would leave the user with neither a game
	// window nor the launcher that could tell them what went wrong.
	if visible && !launchBehaviorApplied {
		if config.GetOnGameLaunch() == config.OnGameLaunchClose {
			QuitLauncher()
			return
		}
		applyReversibleLaunchBehavior()
	}

	// Presence updates may involve IPC. Keep them after the window action and
	// on this monitor so a slow update cannot postpone minimizing the launcher
	// or race with this instance's failure/exit presence update.
	meta, _ := versions.ReadMeta(versionDir)
	discord.SetPlayingVersion(strings.TrimSpace(meta.GameVersion))

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
