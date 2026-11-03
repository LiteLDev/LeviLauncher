package launch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWindowNotificationPrecedesActivationCompletion(t *testing.T) {
	for _, activationErr := range []error{nil, errors.New("activation failed")} {
		t.Run(errorName(activationErr), func(t *testing.T) {
			notified := make(chan struct{})
			var completed, applied atomic.Bool
			m := observeGameLaunch(context.Background(), func(ctx context.Context) bool {
				_, visible, canceled := waitForWindowState(ctx, time.Minute, func() (bool, gameWindowState) {
					return true, gameWindowState{exists: true} // minimized/cloaked
				}, func() { close(notified) })
				return visible && !canceled
			}, func() bool { applied.Store(true); return true }, func(_ int, err error, actionApplied bool) {
				completed.Store(true)
				if err != activationErr || actionApplied {
					t.Errorf("activation result changed: err=%v applied=%t", err, actionApplied)
				}
			})
			t.Cleanup(func() { m.Finish(123, activationErr) })
			select {
			case <-notified:
			case <-time.After(time.Second):
				t.Fatal("prompt notification waited for activation or timeout")
			}
			if completed.Load() || applied.Load() {
				t.Fatal("window notification completed activation or hid the launcher")
			}
			m.Finish(123, activationErr)
			if !completed.Load() {
				t.Fatal("activation result was lost after notifying the prompt")
			}
		})
	}
}

func TestWindowNotificationDoesNotReplacePresentationWait(t *testing.T) {
	var calls, notifications int
	found, visible, canceled := waitForWindowState(context.Background(), 2*time.Second, func() (bool, gameWindowState) {
		calls++
		return true, gameWindowState{exists: true, presented: calls >= 3}
	}, func() { notifications++ })
	if !found || !visible || canceled || notifications != 1 || calls < 3 {
		t.Fatalf("result=(%t,%t,%t) notifications=%d probes=%d", found, visible, canceled, notifications, calls)
	}
}

func TestWindowAppearingOnFinalProbeStillNotifies(t *testing.T) {
	var probes, notifications int
	found, visible, canceled := waitForWindowState(context.Background(), time.Millisecond, func() (bool, gameWindowState) {
		probes++
		return true, gameWindowState{exists: probes > 1, presented: probes > 1}
	}, func() { notifications++ })
	if !found || !visible || canceled || notifications != 1 {
		t.Fatalf("final probe result=(%t,%t,%t) notifications=%d", found, visible, canceled, notifications)
	}
}

func TestNoWindowPreservesTimeoutAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name            string
		running, cancel bool
	}{
		{"process-without-window", true, false},
		{"process-exited", false, false},
		{"canceled", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			found, visible, canceled := waitForWindowState(ctx, time.Millisecond, func() (bool, gameWindowState) {
				return tc.running, gameWindowState{}
			}, func() { t.Error("notified without a game window") })
			if visible || canceled != tc.cancel || (!tc.cancel && found != tc.running) {
				t.Fatalf("result=(%t,%t,%t)", found, visible, canceled)
			}
		})
	}
}
