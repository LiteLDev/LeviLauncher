package launch

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Outcome is how a launch ended, as reported by the game monitor.
type Outcome struct {
	// Started is set once the launcher began starting the game
	// (mc.launch.start).
	Started bool
	// Up is set when the game window became visible (mc.launch.done).
	Up bool
	// Code is the failure code (ERR_LAUNCH_GAME, ERR_LAUNCH_TIMEOUT) when
	// the game did not come up.
	Code string
}

// ErrWaitTimeout is returned by a Watch wait function whose own timeout
// expired before the monitor reported an outcome.
var ErrWaitTimeout = errors.New("ERR_LAUNCH_TIMEOUT")

// Watch prepares a headless launch: it returns a context to pass to
// versionlaunch.Launcher.Launch and a function that blocks until the monitor
// reports the game window up or a failure. Every event is forwarded to next
// (which may be nil). The wait function returns ctx.Err() when ctx ends and
// ErrWaitTimeout when timeout (if > 0) expires first.
//
// Without a Wails application the monitor applies no window behaviour and
// stops once it has reported; use WaitGameExit or WaitPIDExit to follow the
// game until it exits.
func Watch(ctx context.Context, next eventsink.Sink) (context.Context, func(timeout time.Duration) (Outcome, error)) {
	var (
		mu  sync.Mutex
		out Outcome
	)
	done := make(chan struct{})
	var once sync.Once
	sink := eventsink.Func(func(name string, data ...any) {
		switch name {
		case EventMcLaunchStart:
			mu.Lock()
			out.Started = true
			mu.Unlock()
		case EventMcLaunchDone:
			mu.Lock()
			out.Up = true
			mu.Unlock()
			once.Do(func() { close(done) })
		case EventMcLaunchFailed:
			code := "ERR_LAUNCH_GAME"
			if len(data) > 0 {
				if s, ok := data[0].(string); ok && s != "" {
					code = s
				}
			}
			mu.Lock()
			out.Code = code
			mu.Unlock()
			once.Do(func() { close(done) })
		}
		if next != nil {
			next.Emit(name, data...)
		}
	})
	wait := func(timeout time.Duration) (Outcome, error) {
		var timer <-chan time.Time
		if timeout > 0 {
			t := time.NewTimer(timeout)
			defer t.Stop()
			timer = t.C
		}
		var err error
		select {
		case <-done:
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer:
			err = ErrWaitTimeout
		}
		mu.Lock()
		defer mu.Unlock()
		return out, err
	}
	return eventsink.With(ctx, sink), wait
}

// GameProcessIDs returns the PIDs of the Minecraft processes running from
// versionDir, sorted.
func GameProcessIDs(versionDir string) []uint32 {
	set := gameProcessIDs(utils.CanonicalWindowsPath(versionDir))
	out := make([]uint32, 0, len(set))
	for pid := range set {
		out = append(out, pid)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// GameWindowVisible reports whether one of pids shows a game window.
func GameWindowVisible(pids []uint32) bool {
	set := make(map[uint32]struct{}, len(pids))
	for _, pid := range pids {
		set[pid] = struct{}{}
	}
	return isGameWindowVisible(set)
}

// WaitGameExit blocks until no Minecraft process runs from versionDir, or
// ctx ends. It works for every launch type, including protocol and UWP
// activation where the launcher never owns the game process.
func WaitGameExit(ctx context.Context, versionDir string) error {
	dir := utils.CanonicalWindowsPath(versionDir)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if !isGameRunning(dir) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// WaitPIDExit blocks until process pid exits and returns its exit code, or
// returns ctx.Err() when ctx ends first.
func WaitPIDExit(ctx context.Context, pid uint32) (uint32, error) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	for {
		ev, err := windows.WaitForSingleObject(h, 250)
		if err != nil {
			return 0, err
		}
		if ev == windows.WAIT_OBJECT_0 {
			var code uint32
			if err := windows.GetExitCodeProcess(h, &code); err != nil {
				return 0, err
			}
			return code, nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}
