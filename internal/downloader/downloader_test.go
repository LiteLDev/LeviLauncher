package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartSameDestinationStartsOneTransfer(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	var startOnce sync.Once
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseResponse) })
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			startOnce.Do(func() { close(requestStarted) })
			<-releaseResponse
			_, _ = writer.Write([]byte("downloaded"))
		},
	))
	defer server.Close()
	defer release()

	dest := filepath.Join(t.TempDir(), "artifact.bin")
	done := make(chan struct{})
	var doneOnce sync.Once
	manager := newTestManager(Events{Done: "done"}, Options{})
	manager.emitEvent = func(name string, _ ...any) bool {
		if name == "done" {
			doneOnce.Do(func() { close(done) })
		}
		return true
	}

	manager.Start(context.Background(), server.URL, dest, "")
	manager.Start(context.Background(), server.URL, dest, "")
	waitForSignal(t, requestStarted, "request start")

	if got := requestCount.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	release()
	waitForSignal(t, done, "download completion")

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "downloaded" {
		t.Fatalf("destination = %q, want downloaded", data)
	}
}

func TestConcurrentResumeTaskStartsOneTransfer(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	var startOnce sync.Once
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseResponse) })
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			startOnce.Do(func() { close(requestStarted) })
			<-releaseResponse
			_, _ = writer.Write([]byte("resumed"))
		},
	))
	defer server.Close()
	defer release()

	dest := filepath.Join(t.TempDir(), "artifact.bin")
	manager := newTestManager(Events{}, Options{Resume: true})
	manager.tasks[dest] = &state{
		ctx:    context.Background(),
		url:    server.URL,
		dest:   dest,
		paused: true,
	}

	var callers sync.WaitGroup
	for range 20 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			manager.ResumeTask(dest)
		}()
	}
	callers.Wait()
	waitForSignal(t, requestStarted, "resumed request start")

	if got := requestCount.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	release()
	waitForTaskRemoval(t, manager, dest)
}

func TestPauseTaskRetainsPartialTaskWithoutCancellationError(t *testing.T) {
	t.Parallel()

	requestStarted := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, request *http.Request) {
			startOnce.Do(func() { close(requestStarted) })
			<-request.Context().Done()
		},
	))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "artifact.bin")
	var eventMu sync.Mutex
	var statuses []string
	var errors []string
	manager := newTestManager(
		Events{
			Status: "status",
			Error:  "error",
			StatusFactory: func(status string, _ string) any {
				return status
			},
			ErrorFactory: func(message string, _ string) any {
				return message
			},
		},
		Options{Resume: true, RemoveOnCancel: true},
	)
	manager.emitEvent = func(name string, data ...any) bool {
		eventMu.Lock()
		defer eventMu.Unlock()
		if name == "status" {
			statuses = append(statuses, data[0].(string))
		}
		if name == "error" {
			errors = append(errors, data[0].(string))
		}
		return true
	}

	manager.Start(context.Background(), server.URL, dest, "")
	waitForSignal(t, requestStarted, "request start")
	manager.PauseTask(dest)
	waitForTaskState(t, manager, dest, false, true)

	eventMu.Lock()
	defer eventMu.Unlock()
	if len(errors) != 0 {
		t.Fatalf("pause emitted errors: %v", errors)
	}
	for _, status := range statuses {
		if status == "cancelled" {
			t.Fatalf("pause emitted cancelled status: %v", statuses)
		}
	}
}

func newTestManager(events Events, options Options) *Manager {
	manager := NewManager(events, options)
	manager.emitEvent = func(string, ...any) bool { return true }
	return manager
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForTaskRemoval(t *testing.T, manager *Manager, dest string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		manager.mu.Lock()
		_, exists := manager.tasks[dest]
		manager.mu.Unlock()
		if !exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for task %q removal", dest)
}

func waitForTaskState(
	t *testing.T,
	manager *Manager,
	dest string,
	running bool,
	paused bool,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		manager.mu.Lock()
		task := manager.tasks[dest]
		matches := task != nil &&
			task.running == running &&
			task.paused == paused
		manager.mu.Unlock()
		if matches {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf(
		"timed out waiting for task %q state running=%t paused=%t",
		dest,
		running,
		paused,
	)
}
