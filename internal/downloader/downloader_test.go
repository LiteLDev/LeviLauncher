package downloader

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartVerifiedPublishesOnlyAfterFullResumedFilePasses(t *testing.T) {
	const payload = "complete Minecraft archive"
	const prefixLength = 8
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", prefixLength) {
			t.Errorf("resume range = %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", prefixLength, len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(payload[prefixLength:]))
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "game.appx")
	if err := os.WriteFile(dest+".download", []byte(payload[:prefixLength]), 0o644); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finishVerification := func() { releaseOnce.Do(func() { close(release) }) }
	defer finishVerification()
	manager, events := newVerifiedTestManager(Options{Resume: true})
	manager.StartVerified(context.Background(), server.URL, dest, func(filename string) error {
		if filename != dest+".download" {
			t.Errorf("verified %q instead of staging file", filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil || string(data) != payload {
			t.Errorf("incomplete verification input: %q %v", data, err)
		}
		close(entered)
		<-release
		return nil
	})
	waitForSignal(t, entered, "verification start")
	assertMissingDownloadFile(t, dest)
	if done, failures := events.snapshot(); done != 0 || len(failures) != 0 {
		t.Fatalf("published before verification: %d %v", done, failures)
	}
	finishVerification()
	waitForTaskRemoval(t, manager, dest)
	waitForSignal(t, events.completed, "verified completion event")
	if done, failures := events.snapshot(); done != 1 || len(failures) != 0 {
		t.Fatalf("verified completion: %d %v", done, failures)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != payload {
		t.Fatalf("published archive: %q %v", data, err)
	}
	assertMissingDownloadFile(t, dest+".download")
}

func TestStartVerifiedFailureNeverPublishesAndRemovesUntrustedPartial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("corrupt archive")) }))
	defer server.Close()
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%v", existing), func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "game.appx")
			if existing {
				if err := os.WriteFile(dest, []byte("previous verified archive"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			manager, events := newVerifiedTestManager(Options{Resume: true})
			var calls atomic.Int32
			manager.StartVerified(context.Background(), server.URL, dest, func(filename string) error {
				calls.Add(1)
				if filename != dest+".download" {
					t.Errorf("wrong verification input %q", filename)
				}
				return errors.New("ERR_UWP_INTEGRITY")
			})
			waitForTaskRemoval(t, manager, dest)
			if calls.Load() != 1 {
				t.Fatalf("verification calls = %d", calls.Load())
			}
			if done, failures := events.snapshot(); done != 0 || len(failures) != 1 || failures[0] != "ERR_UWP_INTEGRITY" {
				t.Fatalf("integrity failure events: done=%d errors=%v", done, failures)
			}
			assertMissingDownloadFile(t, dest+".download")
			if existing {
				data, err := os.ReadFile(dest)
				if err != nil || string(data) != "previous verified archive" {
					t.Fatalf("existing archive changed: %q %v", data, err)
				}
			} else {
				assertMissingDownloadFile(t, dest)
			}
		})
	}
}

func TestStartVerifiedCancellationDuringVerificationNeverPublishes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("archive")) }))
	defer server.Close()
	for _, parentContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("parentContext=%v", parentContext), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dest := filepath.Join(t.TempDir(), "game.appx")
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			finishVerification := func() { releaseOnce.Do(func() { close(release) }) }
			defer finishVerification()
			manager, events := newVerifiedTestManager(Options{RemoveOnCancel: true})
			manager.StartVerified(ctx, server.URL, dest, func(string) error {
				close(entered)
				<-release
				return nil
			})
			waitForSignal(t, entered, "verification start")
			if parentContext {
				cancel()
			} else {
				manager.CancelTask(dest)
			}
			finishVerification()
			waitForTaskRemoval(t, manager, dest)
			if done, failures := events.snapshot(); done != 0 || len(failures) != 0 {
				t.Errorf("cancelled verification events: done=%d errors=%v", done, failures)
			}
			assertMissingDownloadFile(t, dest)
			assertMissingDownloadFile(t, dest+".download")
		})
	}
}

func TestVerificationPauseResumeRechecksCompleteFileWithoutHTTPRequest(t *testing.T) {
	const payload = "verified Minecraft archive"
	expected := sha256.Sum256([]byte(payload))
	for _, action := range []string{"resume", "tamper", "cancel_task", "cancel_all"} {
		t.Run(action, func(t *testing.T) {
			tampered := action == "tamper"
			var requests, verifications atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) != 1 || r.Header.Get("Range") != "" {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				_, _ = w.Write([]byte(payload))
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "game.appx")
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			finishVerification := func() { releaseOnce.Do(func() { close(release) }) }
			defer finishVerification()
			manager, events := newVerifiedTestManager(Options{Resume: true, RemoveOnCancel: true})
			manager.StartVerified(context.Background(), server.URL, dest, func(filename string) error {
				data, err := os.ReadFile(filename)
				if err != nil {
					return err
				}
				actual := sha256.Sum256(data)
				if verifications.Add(1) == 1 {
					close(entered)
					<-release
				}
				if actual != expected {
					return errors.New("ERR_UWP_INTEGRITY")
				}
				return nil
			})
			waitForSignal(t, entered, "first verification")
			manager.PauseTask(dest)
			finishVerification()
			waitForTaskState(t, manager, dest, false, true)
			assertMissingDownloadFile(t, dest)
			data, err := os.ReadFile(dest + ".download")
			if err != nil || string(data) != payload {
				t.Fatalf("paused file lost: %q %v", data, err)
			}
			if done, failures := events.snapshot(); done != 0 || len(failures) != 0 {
				t.Fatalf("pause emitted completion/error: %d %v", done, failures)
			}
			if action == "cancel_task" || action == "cancel_all" {
				if action == "cancel_task" {
					manager.CancelTask(dest)
				} else {
					manager.Cancel()
				}
				waitForTaskRemoval(t, manager, dest)
				assertMissingDownloadFile(t, dest)
				assertMissingDownloadFile(t, dest+".download")
				if done, failures := events.snapshot(); done != 0 || len(failures) != 0 {
					t.Fatalf("paused cancellation: %d %v", done, failures)
				}
				return
			}
			if tampered {
				data[len(data)-1] ^= 1
				if err := os.WriteFile(dest+".download", data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			manager.ResumeTask(dest)
			waitForTaskRemoval(t, manager, dest)
			if requests.Load() != 1 || verifications.Load() != 2 {
				t.Fatalf("resume must reverify without HTTP: requests=%d verifications=%d", requests.Load(), verifications.Load())
			}
			if !tampered {
				waitForSignal(t, events.completed, "resumed completion event")
			}
			done, failures := events.snapshot()
			if tampered {
				if done != 0 || len(failures) != 1 || failures[0] != "ERR_UWP_INTEGRITY" {
					t.Fatalf("tampered resume: %d %v", done, failures)
				}
				assertMissingDownloadFile(t, dest)
			} else {
				if done != 1 || len(failures) != 0 {
					t.Fatalf("valid resume: %d %v", done, failures)
				}
				data, err := os.ReadFile(dest)
				if err != nil || string(data) != payload {
					t.Fatalf("published resumed archive: %q %v", data, err)
				}
			}
			assertMissingDownloadFile(t, dest+".download")
		})
	}
}

func TestMD5RetriesRetainOriginalSuccessAndFailureBehavior(t *testing.T) {
	const payload = "expected package"
	expected := fmt.Sprintf("%x", md5.Sum([]byte(payload)))
	for _, successAttempt := range []int32{2, 4} {
		t.Run(fmt.Sprintf("successAttempt=%d", successAttempt), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := requests.Add(1)
				if r.Header.Get("Range") != "" {
					t.Errorf("MD5 retry reused corrupt partial: %q", r.Header.Get("Range"))
				}
				if attempt >= successAttempt {
					_, _ = w.Write([]byte(payload))
				} else {
					_, _ = w.Write([]byte("corrupt package"))
				}
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "game.msixvc")
			manager, events := newVerifiedTestManager(Options{Resume: true})
			manager.Start(context.Background(), server.URL, dest, expected)
			waitForTaskRemoval(t, manager, dest)
			if successAttempt == 2 {
				waitForSignal(t, events.completed, "MD5 retry completion event")
			}
			done, failures := events.snapshot()
			if successAttempt == 2 {
				if requests.Load() != 2 || done != 1 || len(failures) != 1 {
					t.Fatalf("MD5 retry success: requests=%d done=%d errors=%v", requests.Load(), done, failures)
				}
				data, err := os.ReadFile(dest)
				if err != nil || string(data) != payload {
					t.Fatalf("MD5 verified data: %q %v", data, err)
				}
			} else {
				if requests.Load() != 3 || done != 0 || len(failures) != 3 || failures[2] != "ERR_MD5_MISMATCH" {
					t.Fatalf("MD5 retry limit: requests=%d done=%d errors=%v", requests.Load(), done, failures)
				}
				assertMissingDownloadFile(t, dest)
			}
			assertMissingDownloadFile(t, dest+".download")
		})
	}
}

func TestMD5PauseResumeRechecksChangedFile(t *testing.T) {
	const payload = "expected package"
	expected := fmt.Sprintf("%x", md5.Sum([]byte(payload)))
	var requests, checks atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Range") != "" {
			t.Errorf("MD5 refetch should start fresh: %q", r.Header.Get("Range"))
		}
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "game.msixvc")
	manager, events := newVerifiedTestManager(Options{Resume: true})
	manager.events.Status = "status"
	manager.events.StatusFactory = func(status, _ string) any { return status }
	record := manager.emitEvent
	manager.emitEvent = func(name string, data ...any) bool {
		if name == "status" && data[0] == "verifying" && checks.Add(1) == 1 {
			manager.PauseTask(dest)
		}
		return record(name, data...)
	}
	manager.Start(context.Background(), server.URL, dest, expected)
	waitForTaskState(t, manager, dest, false, true)
	assertMissingDownloadFile(t, dest)
	if err := os.WriteFile(dest+".download", []byte("modified package"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager.ResumeTask(dest)
	waitForTaskRemoval(t, manager, dest)
	waitForSignal(t, events.completed, "MD5 resumed completion event")
	if requests.Load() != 2 || checks.Load() != 3 {
		t.Fatalf("MD5 not repeated before refetch: requests=%d checks=%d", requests.Load(), checks.Load())
	}
	if done, failures := events.snapshot(); done != 1 || len(failures) != 1 {
		t.Fatalf("MD5 resumed result: %d %v", done, failures)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != payload {
		t.Fatalf("paused modification published: %q %v", data, err)
	}
}

func TestCancellationAfterPublicationCannotContradictCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("verified package"))
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "game.appx")
	manager, events := newVerifiedTestManager(Options{RemoveOnCancel: true})
	manager.events.Progress = "progress"
	manager.events.Status = "status"
	manager.events.StatusFactory = func(status, _ string) any { return status }
	record := manager.emitEvent
	var lateCancellation, cancelledEvents atomic.Int32
	manager.emitEvent = func(name string, data ...any) bool {
		if name == "progress" {
			if _, err := os.Stat(dest); err == nil {
				lateCancellation.Add(1)
				manager.CancelTask(dest)
			}
		}
		if name == "status" && data[0] == "cancelled" {
			cancelledEvents.Add(1)
		}
		return record(name, data...)
	}
	manager.StartVerified(context.Background(), server.URL, dest, func(string) error { return nil })
	waitForSignal(t, events.completed, "committed completion")
	if lateCancellation.Load() != 1 || cancelledEvents.Load() != 0 {
		t.Fatalf("late cancellation contradicted committed file: attempts=%d cancelled=%d", lateCancellation.Load(), cancelledEvents.Load())
	}
	if done, failures := events.snapshot(); done != 1 || len(failures) != 0 {
		t.Fatalf("commit events: done=%d errors=%v", done, failures)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "verified package" {
		t.Fatalf("committed file: %q %v", data, err)
	}
}

type verifiedTestEvents struct {
	mu           sync.Mutex
	done         int
	failures     []string
	completed    chan struct{}
	completeOnce sync.Once
}

func (events *verifiedTestEvents) snapshot() (int, []string) {
	events.mu.Lock()
	defer events.mu.Unlock()
	return events.done, append([]string(nil), events.failures...)
}

func newVerifiedTestManager(options Options) (*Manager, *verifiedTestEvents) {
	events := &verifiedTestEvents{completed: make(chan struct{})}
	manager := newTestManager(Events{Done: "done", Error: "error"}, options)
	manager.emitEvent = func(name string, data ...any) bool {
		events.mu.Lock()
		defer events.mu.Unlock()
		if name == "done" {
			events.done++
			events.completeOnce.Do(func() { close(events.completed) })
		}
		if name == "error" {
			events.failures = append(events.failures, data[0].(string))
		}
		return true
	}
	return manager, events
}

func assertMissingDownloadFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file %q unexpectedly published/retained: %v", path, err)
	}
}

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
