package launch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWindowActionPrecedesActivationResult(t *testing.T) {
	for _, activationErr := range []error{nil, errors.New("activation failed after showing a window")} {
		t.Run(errorName(activationErr), func(t *testing.T) {
			visible := make(chan struct{})
			applied := make(chan struct{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			completed := 0
			m := observeGameLaunch(ctx, func(ctx context.Context) bool {
				select {
				case <-visible:
					return true
				case <-ctx.Done():
					return false
				}
			}, func() bool {
				close(applied)
				return true
			}, func(pid int, err error, actionApplied bool) {
				completed++
				if pid != 123 || err != activationErr || !actionApplied {
					t.Errorf("lost activation outcome: pid=%d err=%v actionApplied=%t", pid, err, actionApplied)
				}
			})
			t.Cleanup(func() { m.Finish(123, activationErr) })
			// Windows has shown the game, but activation has not returned yet.
			close(visible)
			select {
			case <-applied:
			case <-time.After(3 * time.Second):
				t.Fatal("window action waited for activation to return")
			}
			if completed != 0 {
				t.Fatal("reported success/failure before activation returned")
			}
			m.Finish(123, activationErr)
			m.Finish(456, errors.New("duplicate completion"))
			if completed != 1 {
				t.Fatalf("completion ran %d times", completed)
			}
		})
	}
}

func errorName(err error) string {
	if err == nil {
		return "success"
	}
	return "failure"
}

func TestActivationCompletionCancelsAndJoinsWindowObserver(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	var applied atomic.Bool
	activationErr := errors.New("activation failed without a window")
	m := observeGameLaunch(context.Background(), func(ctx context.Context) bool {
		close(started)
		<-ctx.Done()
		close(stopped)
		// Simulate window detection racing with the activation error.
		return true
	}, func() bool {
		applied.Store(true)
		return true
	}, func(_ int, err error, actionApplied bool) {
		select {
		case <-stopped:
		default:
			t.Error("completion did not join the window observer")
		}
		if err != activationErr || actionApplied {
			t.Errorf("unexpected result: err=%v applied=%t", err, actionApplied)
		}
	})
	<-started
	m.Finish(0, activationErr)
	if applied.Load() {
		t.Fatal("canceled observer applied a late window action")
	}
}

func TestGameLaunchMonitorReservationAndRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	first := BeginGameLaunch(ctx, dir)
	if first == nil {
		t.Fatal("first activation did not reserve the monitor")
	}
	err := errors.New("activation failed")
	t.Cleanup(func() { first.Finish(0, err) })
	if duplicate := BeginGameLaunch(ctx, dir); duplicate != nil {
		duplicate.Finish(0, err)
		t.Fatal("duplicate activation created another observer")
	}
	first.Finish(0, err)
	if _, exists := activeMonitors.Load(utils.CanonicalWindowsPath(dir)); exists {
		t.Fatal("failed activation kept its monitor reservation")
	}
	retry := BeginGameLaunch(ctx, dir)
	if retry == nil {
		t.Fatal("failed activation prevented retry")
	}
	retry.Finish(0, err)
	var absent *GameLaunchMonitor
	absent.Finish(0, err)
}

type launchTestWindow struct {
	application.Window
	visible, minimized bool
	hides, minimizes   int
}

func (w *launchTestWindow) IsVisible() bool   { return w.visible }
func (w *launchTestWindow) IsMinimised() bool { return w.minimized }
func (w *launchTestWindow) Hide() application.Window {
	w.hides++
	w.visible = false
	return w
}
func (w *launchTestWindow) Minimise() application.Window {
	w.minimizes++
	w.minimized = true
	return w
}

func TestEarlyWindowActionRespectsLaunchSettingsAndExistingState(t *testing.T) {
	for _, tc := range []struct {
		name, behavior              string
		visible, minimized, applied bool
		hides, minimizes            int
	}{
		{"minimize", config.OnGameLaunchMinimize, true, false, true, 0, 1},
		{"hide", config.OnGameLaunchHide, true, false, true, 1, 0},
		{"keep", config.OnGameLaunchKeep, true, false, false, 0, 0},
		{"defer-close", config.OnGameLaunchClose, true, false, false, 0, 0},
		{"already-hidden", config.OnGameLaunchMinimize, false, false, false, 0, 0},
		{"already-minimized", config.OnGameLaunchMinimize, true, true, false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &launchTestWindow{visible: tc.visible, minimized: tc.minimized}
			applied := applyLaunchWindowBehavior(w, tc.behavior)
			if applied != tc.applied || w.hides != tc.hides || w.minimizes != tc.minimizes {
				t.Errorf("applied=%t hides=%d minimizes=%d", applied, w.hides, w.minimizes)
			}
		})
	}
	if applyLaunchWindowBehavior(nil, config.OnGameLaunchMinimize) {
		t.Error("nonexistent launcher window reported an applied action")
	}
}
