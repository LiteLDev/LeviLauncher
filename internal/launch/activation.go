package launch

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// GameLaunchMonitor observes windows while a synchronous activation call is
// still in progress. The caller must call Finish when activation returns.
type GameLaunchMonitor struct {
	once          sync.Once
	stopObserving func() bool
	complete      func(int, error, bool)
}

// BeginGameLaunch must be called after registration and native preparation,
// immediately before activation. Only reversible window actions happen early;
// success events and quitting the launcher wait for activation to succeed.
func BeginGameLaunch(ctx context.Context, versionDir string) *GameLaunchMonitor {
	versionDir = utils.CanonicalWindowsPath(versionDir)
	if _, running := activeMonitors.LoadOrStore(versionDir, struct{}{}); running {
		return nil
	}
	started := time.Now()
	return observeGameLaunch(ctx, func(ctx context.Context) bool {
		found, _, canceled := waitForGameProcess(ctx, versionDir, 0, 120*time.Second)
		if !found || canceled {
			return false
		}
		_, visible, canceled := waitForGameWindow(ctx, versionDir, 60*time.Second)
		if visible && !canceled {
			log.Printf("Game window detected during activation after %s", time.Since(started).Round(time.Millisecond))
		}
		return visible && !canceled
	}, applyReversibleLaunchBehavior, func(pid int, err error, applied bool) {
		log.Printf("Game activation returned after %s (pid=%d, window action applied=%t, error=%v)", time.Since(started).Round(time.Millisecond), pid, applied, err)
		if err != nil || ctx.Err() != nil {
			defer activeMonitors.Delete(versionDir)
			if applied && ctx.Err() == nil && !userHidLauncher.Load() {
				RestoreLauncherWindow()
			}
			return
		}
		// Transfer the same monitor reservation; do not apply the window action
		// again if the user has restored the launcher during activation.
		go monitorGameProcess(ctx, versionDir, pid, applied)
	})
}

func observeGameLaunch(ctx context.Context, waitVisible func(context.Context) bool, apply func() bool, complete func(int, error, bool)) *GameLaunchMonitor {
	watchCtx, cancel := context.WithCancel(ctx)
	done := make(chan bool, 1)
	go func() {
		applied := false
		if waitVisible(watchCtx) && watchCtx.Err() == nil {
			applied = apply()
		}
		done <- applied
	}()
	return &GameLaunchMonitor{
		stopObserving: func() bool {
			cancel()
			return <-done
		},
		complete: complete,
	}
}

// Finish stops and joins the early observer before reporting the outcome, so
// a failed activation cannot race with a late minimize or leave an observer.
func (m *GameLaunchMonitor) Finish(pid int, err error) {
	if m == nil {
		return
	}
	m.once.Do(func() { m.complete(pid, err, m.stopObserving()) })
}

func applyReversibleLaunchBehavior() bool {
	if userHidLauncher.Load() {
		return false
	}
	return applyLaunchWindowBehavior(GetMainWindow(), config.GetOnGameLaunch())
}

func applyLaunchWindowBehavior(w application.Window, behavior string) bool {
	if behavior == config.OnGameLaunchClose || behavior == config.OnGameLaunchKeep {
		return false
	}
	if w == nil || !w.IsVisible() || w.IsMinimised() {
		return false
	}
	if behavior == config.OnGameLaunchHide {
		w.Hide()
	} else {
		w.Minimise()
	}
	return true
}
