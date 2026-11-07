package launch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
)

func TestWatchReportsOutcome(t *testing.T) {
	var forwarded []string
	next := eventsink.Func(func(name string, _ ...any) { forwarded = append(forwarded, name) })

	ctx, wait := Watch(context.Background(), next)
	sink := eventsink.From(ctx)
	sink.Emit(EventMcLaunchStart, struct{}{})
	sink.Emit(EventMcLaunchDone, struct{}{})
	out, err := wait(time.Second)
	if err != nil || !out.Started || !out.Up || out.Code != "" {
		t.Fatalf("done: %+v %v", out, err)
	}
	if len(forwarded) != 2 {
		t.Fatalf("forwarded %q", forwarded)
	}

	ctx, wait = Watch(context.Background(), nil)
	eventsink.From(ctx).Emit(EventMcLaunchFailed, "ERR_LAUNCH_TIMEOUT")
	if out, err = wait(0); err != nil || out.Up || out.Code != "ERR_LAUNCH_TIMEOUT" {
		t.Fatalf("failed: %+v %v", out, err)
	}

	_, wait = Watch(context.Background(), nil)
	if _, err = wait(10 * time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("timeout: %v", err)
	}

	parent, cancel := context.WithCancel(context.Background())
	_, wait = Watch(parent, nil)
	cancel()
	if _, err = wait(0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestMonitorWithoutApplicationOnlyReports(t *testing.T) {
	// No game runs from this folder: the headless monitor must report a
	// failure through the context's sink without touching any window.
	ctx, wait := Watch(context.Background(), nil)
	dir := t.TempDir()
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitorWithTimeouts(ctx, dir, 0, false, 50*time.Millisecond, 50*time.Millisecond)
	}()
	out, err := wait(5 * time.Second)
	if err != nil || out.Code != "ERR_LAUNCH_TIMEOUT" {
		t.Fatalf("outcome %+v %v", out, err)
	}
	<-done
}

func TestMonitorTimeoutsFromContext(t *testing.T) {
	if p, w := timeoutsFrom(context.Background()); p != defaultProcessTimeout || w != defaultWindowTimeout {
		t.Fatalf("defaults = %v %v", p, w)
	}
	ctx := WithMonitorTimeouts(context.Background(), 8*time.Minute, 0)
	if p, w := timeoutsFrom(ctx); p != 8*time.Minute || w != defaultWindowTimeout {
		t.Fatalf("custom = %v %v", p, w)
	}
}
