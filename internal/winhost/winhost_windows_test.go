package winhost

import (
	"sync/atomic"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDispatchRunsOnWindowThread(t *testing.T) {
	h, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if h.HWND() == 0 {
		t.Fatal("no window handle")
	}
	var tid uint32
	h.Dispatch(func() { tid = windows.GetCurrentThreadId() })
	if tid != h.tid {
		t.Fatalf("dispatched on thread %d, window thread is %d", tid, h.tid)
	}
	// Nested dispatch from the window thread must not deadlock.
	var nested atomic.Bool
	h.Dispatch(func() { h.Dispatch(func() { nested.Store(true) }) })
	if !nested.Load() {
		t.Fatal("nested dispatch did not run")
	}
	h.Close()
	if err := h.DispatchErr(func() {}); err != ErrClosed {
		t.Fatalf("dispatch after close = %v, want ErrClosed", err)
	}
}
