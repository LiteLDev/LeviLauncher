package lip

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

type recordedEvent struct {
	name string
	data any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent{}, r.events...)
}

func recordEvents(t *testing.T) *eventRecorder {
	t.Helper()
	rec := &eventRecorder{}
	old := emitEvent
	emitEvent = func(event string, data any) {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		rec.events = append(rec.events, recordedEvent{name: event, data: data})
	}
	t.Cleanup(func() { emitEvent = old })
	return rec
}

type engineCall struct {
	method         string
	dir            string
	packages       []string
	dryRun         bool
	ignoreScripts  bool
	noDependencies bool
}

type fakeEngine struct {
	dir      string
	reporter report.Reporter
	shared   *fakeEngineState
}

type fakeEngineState struct {
	mu       sync.Mutex
	calls    []engineCall
	err      error
	explicit []string
	implicit []string
	// onCall runs inside every operation (after recording the call).
	onCall func(ctx context.Context, r report.Reporter) error
	closes int
}

func (f *fakeEngine) record(ctx context.Context, c engineCall) error {
	c.dir = f.dir
	f.shared.mu.Lock()
	f.shared.calls = append(f.shared.calls, c)
	onCall := f.shared.onCall
	err := f.shared.err
	f.shared.mu.Unlock()
	if onCall != nil {
		if hookErr := onCall(ctx, f.reporter); hookErr != nil {
			return hookErr
		}
	}
	return err
}

func (f *fakeEngine) Install(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	return f.record(ctx, engineCall{method: "Install", packages: packages, dryRun: dryRun, ignoreScripts: ignoreScripts, noDependencies: noDependencies})
}

func (f *fakeEngine) Update(ctx context.Context, packages []string, dryRun, ignoreScripts bool) error {
	return f.record(ctx, engineCall{method: "Update", packages: packages, dryRun: dryRun, ignoreScripts: ignoreScripts})
}

func (f *fakeEngine) Uninstall(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	return f.record(ctx, engineCall{method: "Uninstall", packages: packages, dryRun: dryRun, ignoreScripts: ignoreScripts, noDependencies: noDependencies})
}

func (f *fakeEngine) List(ctx context.Context) ([]string, []string, error) {
	if err := f.record(ctx, engineCall{method: "List"}); err != nil {
		return nil, nil, err
	}
	return f.shared.explicit, f.shared.implicit, nil
}

func (f *fakeEngine) CacheClean(ctx context.Context) error {
	return f.record(ctx, engineCall{method: "CacheClean"})
}

func (f *fakeEngine) Close() error {
	f.shared.mu.Lock()
	f.shared.closes++
	f.shared.mu.Unlock()
	return nil
}

func useFakeEngine(t *testing.T) *fakeEngineState {
	t.Helper()
	state := &fakeEngineState{}
	old := newEngine
	newEngine = func(workDir string, r report.Reporter) packageEngine {
		return &fakeEngine{dir: workDir, reporter: r, shared: state}
	}
	t.Cleanup(func() { newEngine = old })
	return state
}

var taskIDPattern = regexp.MustCompile(`^lip-\d+-\d+$`)

func TestInstallEmitsTaskEventsInOrder(t *testing.T) {
	rec := recordEvents(t)
	state := useFakeEngine(t)
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		r.Info("resolving")
		r.Success("step done")
		r.Warning("careful")
		r.Error("oops")
		r.Progress("dl-1", "Downloading https://example.com/a.zip", 42.5)
		return nil
	}

	dir := filepath.Join(t.TempDir(), "MyInstance")
	if err := InstallPackagesViaDaemon(context.Background(), dir, []string{" github.com/LiteLDev/LeviLamina#client@1.0.0 ", ""}); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if len(state.calls) != 1 {
		t.Fatalf("expected 1 engine call, got %+v", state.calls)
	}
	call := state.calls[0]
	if call.method != "Install" || call.dryRun || call.ignoreScripts || call.noDependencies {
		t.Fatalf("unexpected call: %+v", call)
	}
	if len(call.packages) != 1 || call.packages[0] != "github.com/LiteLDev/LeviLamina#client@1.0.0" {
		t.Fatalf("unexpected packages: %q", call.packages)
	}
	if !filepath.IsAbs(call.dir) || filepath.Base(call.dir) != "MyInstance" {
		t.Fatalf("unexpected engine dir: %q", call.dir)
	}

	events := rec.snapshot()
	wantNames := []string{EventLipTaskStarted, EventLipTaskLog, EventLipTaskLog, EventLipTaskLog, EventLipTaskLog, EventLipTaskProgress, EventLipTaskFinished}
	if len(events) != len(wantNames) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(wantNames), events)
	}
	for i, name := range wantNames {
		if events[i].name != name {
			t.Fatalf("event %d = %s, want %s", i, events[i].name, name)
		}
	}

	started := events[0].data.(LipTaskStartedEvent)
	if !taskIDPattern.MatchString(started.TaskID) {
		t.Fatalf("unexpected task id %q", started.TaskID)
	}
	if started.Method != "Install" || started.Target != "MyInstance" {
		t.Fatalf("unexpected started event: %+v", started)
	}
	if len(started.Packages) != 1 || started.Packages[0] != "github.com/LiteLDev/LeviLamina#client@1.0.0" {
		t.Fatalf("unexpected started packages: %q", started.Packages)
	}

	wantLogs := []struct{ level, message string }{
		{"info", "resolving"}, {"info", "step done"}, {"warning", "careful"}, {"error", "oops"},
	}
	for i, want := range wantLogs {
		got := events[1+i].data.(LipTaskLogEvent)
		if got.TaskID != started.TaskID || got.Method != "Install" || got.Target != "MyInstance" {
			t.Fatalf("log %d has wrong task fields: %+v", i, got)
		}
		if got.Level != want.level || got.Message != want.message {
			t.Fatalf("log %d = %s/%q, want %s/%q", i, got.Level, got.Message, want.level, want.message)
		}
	}

	progress := events[5].data.(LipTaskProgressEvent)
	if progress.TaskID != started.TaskID || progress.ProgressID != "dl-1" || progress.Percentage != 42.5 ||
		progress.Message != "Downloading https://example.com/a.zip" {
		t.Fatalf("unexpected progress event: %+v", progress)
	}

	finished := events[6].data.(LipTaskFinishedEvent)
	if finished.TaskID != started.TaskID || !finished.Success || finished.Error != "" || finished.DurationMs < 0 {
		t.Fatalf("unexpected finished event: %+v", finished)
	}
}

func TestOperationMethodsAndArguments(t *testing.T) {
	rec := recordEvents(t)
	state := useFakeEngine(t)
	dir := t.TempDir()

	if err := UpdatePackagesViaDaemon(context.Background(), dir, []string{"a@1.0.0"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if err := UninstallPackagesViaDaemon(context.Background(), dir, []string{"a"}, true); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if err := UninstallPackagesViaDaemon(context.Background(), dir, []string{"b"}, false); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	if len(state.calls) != 3 {
		t.Fatalf("unexpected calls: %+v", state.calls)
	}
	if c := state.calls[0]; c.method != "Update" || c.dryRun || c.ignoreScripts {
		t.Fatalf("unexpected update call: %+v", c)
	}
	if c := state.calls[1]; c.method != "Uninstall" || c.dryRun || c.ignoreScripts || !c.noDependencies {
		t.Fatalf("noDependencies must reach the noDependencies argument: %+v", c)
	}
	if c := state.calls[2]; c.method != "Uninstall" || c.dryRun || c.ignoreScripts || c.noDependencies {
		t.Fatalf("unexpected uninstall call: %+v", c)
	}

	var methods []string
	for _, ev := range rec.snapshot() {
		if started, ok := ev.data.(LipTaskStartedEvent); ok {
			methods = append(methods, started.Method)
		}
	}
	if fmt.Sprint(methods) != "[Update Uninstall Uninstall]" {
		t.Fatalf("unexpected started methods: %v", methods)
	}
}

func TestInstallErrorIsReturnedAndReported(t *testing.T) {
	rec := recordEvents(t)
	state := useFakeEngine(t)
	state.err = errors.New("Package 'github.com/LiteLDev/LeviLamina#client' is already explicitly installed")

	err := InstallPackagesViaDaemon(context.Background(), t.TempDir(), []string{"github.com/LiteLDev/LeviLamina#client@1.0.0"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != state.err.Error() {
		t.Fatalf("engine message must be preserved, got %q", err.Error())
	}
	if code := ErrorCode(err); code != "ERR_LIP_INSTALL_FAILED" {
		t.Fatalf("unexpected code %q", code)
	}

	events := rec.snapshot()
	last := events[len(events)-1]
	finished, ok := last.data.(LipTaskFinishedEvent)
	if !ok || last.name != EventLipTaskFinished {
		t.Fatalf("last event is %s", last.name)
	}
	if finished.Success || finished.Error != state.err.Error() {
		t.Fatalf("unexpected finished event: %+v", finished)
	}
}

func TestEnginePanicBecomesError(t *testing.T) {
	rec := recordEvents(t)
	old := newEngine
	newEngine = func(string, report.Reporter) packageEngine { panic("engine: not implemented") }
	t.Cleanup(func() { newEngine = old })

	err := InstallPackagesViaDaemon(context.Background(), t.TempDir(), []string{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
	events := rec.snapshot()
	if len(events) != 2 || events[1].name != EventLipTaskFinished || events[1].data.(LipTaskFinishedEvent).Success {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestEmptyWorkDirRejected(t *testing.T) {
	rec := recordEvents(t)
	state := useFakeEngine(t)
	if err := InstallPackagesViaDaemon(context.Background(), "  ", []string{"x"}); err == nil {
		t.Fatal("expected error for empty work dir")
	}
	if _, err := ListPackageStatesViaDaemon(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty work dir")
	}
	if len(state.calls) != 0 || len(rec.snapshot()) != 0 {
		t.Fatalf("nothing should run: calls=%+v events=%+v", state.calls, rec.snapshot())
	}
}

func TestListPackageStates(t *testing.T) {
	rec := recordEvents(t)
	state := useFakeEngine(t)
	state.explicit = []string{"github.com/LiteLDev/LeviLamina#client@1.2.3"}
	state.implicit = []string{"github.com/LiteLDev/CrashLogger#client@1.3.3", "github.com/LiteLDev/PeEditor@3.9.2"}
	dir := t.TempDir()

	states, err := ListPackageStatesViaDaemon(context.Background(), dir)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(rec.snapshot()) != 0 {
		t.Fatalf("list must not emit task events: %+v", rec.snapshot())
	}
	ll := states["github.com/liteldev/levilamina#client"]
	if !ll.Installed || !ll.ExplicitInstalled || ll.InstalledVersion != "1.2.3" {
		t.Fatalf("unexpected LeviLamina state: %+v", ll)
	}
	dep := states["github.com/liteldev/crashlogger#client"]
	if !dep.Installed || dep.ExplicitInstalled || dep.InstalledVersion != "1.3.3" {
		t.Fatalf("unexpected dependency state: %+v", dep)
	}
	if pe, ok := states["github.com/liteldev/peeditor"]; !ok || pe.InstalledVersion != "3.9.2" {
		t.Fatalf("expected variant-less dependency, got %+v", states)
	}

	got, err := GetPackageInstallStateViaDaemon(context.Background(), dir, "github.com/LiteLDev/LeviLamina#client@9.9.9")
	if err != nil || got != ll {
		t.Fatalf("GetPackageInstallState = %+v, %v", got, err)
	}
	explicit, err := IsPackageExplicitlyInstalledViaDaemon(context.Background(), dir, "github.com/LiteLDev/CrashLogger#client")
	if err != nil || explicit {
		t.Fatalf("CrashLogger explicit = %v, %v", explicit, err)
	}
	missing, err := GetPackageInstallStateViaDaemon(context.Background(), dir, "github.com/x/y")
	if err != nil || missing.Installed {
		t.Fatalf("missing = %+v, %v", missing, err)
	}

	state.err = errors.New("boom")
	if _, err := ListPackageStatesViaDaemon(context.Background(), dir); err == nil {
		t.Fatal("expected list error")
	}
}

func TestBuildPackageInstallStateIndexPrefersExplicitVersion(t *testing.T) {
	states := buildPackageInstallStateIndex(
		[]string{"a/b#c@2.0.0"},
		[]string{"A/B#c@1.0.0", ""},
	)
	if len(states) != 1 {
		t.Fatalf("unexpected states: %+v", states)
	}
	if s := states["a/b#c"]; !s.ExplicitInstalled || s.InstalledVersion != "2.0.0" {
		t.Fatalf("unexpected state: %+v", s)
	}
}

func TestOperationsSerializedPerWorkDir(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	var active, maxActive int
	var mu sync.Mutex
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return nil
	}

	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = InstallPackagesViaDaemon(context.Background(), dir, []string{"x"})
		}()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Fatalf("operations on one dir overlapped: max %d", maxActive)
	}
}

func TestLockWaitHonorsContext(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		entered <- struct{}{}
		<-release
		return nil
	}
	dir := t.TempDir()
	done := make(chan error, 1)
	go func() { done <- InstallPackagesViaDaemon(context.Background(), dir, []string{"x"}) }()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := UpdatePackagesViaDaemon(ctx, dir, []string{"x"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first install failed: %v", err)
	}
}

// List only reads tooth_lock.json; as with lipd (one process per call) it
// must not wait for an install running on the same instance.
func TestListDoesNotWaitForRunningInstall(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	state.explicit = []string{"github.com/LiteLDev/LeviLamina#client@1.2.3"}
	release := make(chan struct{})
	entered := make(chan struct{})
	var first sync.Once
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		blocked := false
		first.Do(func() { blocked = true })
		if blocked {
			close(entered)
			<-release
		}
		return nil
	}
	dir := t.TempDir()
	done := make(chan error, 1)
	go func() { done <- InstallPackagesViaDaemon(context.Background(), dir, []string{"x"}) }()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	states, err := ListPackageStatesViaDaemon(ctx, dir)
	close(release)
	if err != nil {
		t.Fatalf("List during install: %v", err)
	}
	if !states["github.com/liteldev/levilamina#client"].Installed {
		t.Fatalf("unexpected states: %+v", states)
	}
	if err := <-done; err != nil {
		t.Fatalf("install failed: %v", err)
	}
}

// The real engine reaches GitHub through httpx's blocking system-proxy
// transport, which waits for PAC/WPAD like lip's HttpClient.DefaultProxy
// (the shared UI transport would go direct when the lookup is slow), and
// refuses https -> http redirects like Flurl.
func TestNewEngineUsesSystemProxyClient(t *testing.T) {
	setProxyConfig(t, "", false)
	e, ok := newEngine(t.TempDir(), nil).(*engine.Engine)
	if !ok {
		t.Fatal("newEngine did not return *engine.Engine")
	}
	reg, ok := e.Registry.(*engine.GitRegistry)
	if !ok {
		t.Fatalf("Registry = %T", e.Registry)
	}
	git, ok := reg.Git.(*gitproto.Client)
	if !ok {
		t.Fatalf("Git = %T", reg.Git)
	}
	if engineHTTPClient.Transport == nil || engineHTTPClient.Transport == httpx.DefaultClient().Transport {
		t.Fatal("engine client must use httpx's blocking proxy transport, not the shared UI transport")
	}
	want := engineHTTPClient.Transport
	for name, c := range map[string]*http.Client{"git": git.HTTP, "downloader": reg.Downloader.HTTP} {
		if c == nil || c.Transport != want {
			t.Errorf("%s client does not use the engine transport", name)
			continue
		}
		if c.Timeout != 0 {
			t.Errorf("%s client has a whole-request timeout %v", name, c.Timeout)
		}
		next, _ := http.NewRequest(http.MethodGet, "http://example.com/b", nil)
		prev, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
		if c.CheckRedirect == nil || c.CheckRedirect(next, []*http.Request{prev}) != http.ErrUseLastResponse {
			t.Errorf("%s client follows https -> http redirects", name)
		}
	}
}

func TestNormalizeCtxAddsDefaultTimeout(t *testing.T) {
	ctx, cancel := normalizeCtx(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > defaultTaskTimeout || time.Until(deadline) < defaultTaskTimeout-time.Minute {
		t.Fatalf("unexpected deadline %v (ok=%v)", deadline, ok)
	}

	parent, parentCancel := context.WithTimeout(context.Background(), time.Second)
	defer parentCancel()
	got, c := normalizeCtx(parent)
	defer c()
	if got != parent {
		t.Fatal("context with deadline must be kept")
	}
}

func TestCacheClean(t *testing.T) {
	var calls int
	var cleanErr error
	old := cleanCache
	cleanCache = func() error {
		calls++
		return cleanErr
	}
	t.Cleanup(func() { cleanCache = old })

	if err := CacheClean(context.Background()); err != nil {
		t.Fatalf("cache clean failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 clean, got %d", calls)
	}

	cleanErr = errors.New("locked")
	err := CacheClean(context.Background())
	if code := ErrorCode(err); code != "ERR_LIP_CACHE_CLEAN_FAILED" {
		t.Fatalf("unexpected code %q (%v)", code, err)
	}
}

func TestCacheCleanWaitsForRunningTasks(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		entered <- struct{}{}
		<-release
		return nil
	}
	cleaned := make(chan struct{}, 1)
	old := cleanCache
	cleanCache = func() error {
		cleaned <- struct{}{}
		return nil
	}
	t.Cleanup(func() { cleanCache = old })

	installDone := make(chan error, 1)
	go func() { installDone <- InstallPackagesViaDaemon(context.Background(), t.TempDir(), []string{"x"}) }()
	<-entered
	cleanDone := make(chan error, 1)
	go func() { cleanDone <- CacheClean(context.Background()) }()

	select {
	case <-cleaned:
		t.Fatal("cache cleaned while a task was running")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-installDone; err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if err := <-cleanDone; err != nil {
		t.Fatalf("cache clean failed: %v", err)
	}
}

// blockInstallThenClean starts an install that holds the cache until
// release is closed, then a CacheClean that waits behind it. It returns once
// the clean is pending; cleanup releases both.
func blockInstallThenClean(t *testing.T, state *fakeEngineState, cleanCtx context.Context) (release chan struct{}, cleanDone chan error) {
	t.Helper()
	release = make(chan struct{})
	entered := make(chan struct{})
	var first sync.Once
	state.onCall = func(ctx context.Context, r report.Reporter) error {
		blocked := false
		first.Do(func() { blocked = true })
		if blocked {
			close(entered)
			<-release
		}
		return nil
	}
	old := cleanCache
	cleanCache = func() error { return nil }

	installDone := make(chan error, 1)
	go func() { installDone <- InstallPackagesViaDaemon(context.Background(), t.TempDir(), []string{"x"}) }()
	<-entered
	cleanDone = make(chan error, 1)
	go func() { cleanDone <- CacheClean(cleanCtx) }()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(time.Millisecond) {
		cacheLock.mu.Lock()
		pending := cacheLock.writersWaiting > 0
		cacheLock.mu.Unlock()
		if pending || len(cleanDone) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("CacheClean never started waiting")
		}
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		if err := <-installDone; err != nil {
			t.Errorf("install failed: %v", err)
		}
		<-cleanDone
		cleanCache = old
	})
	return release, cleanDone
}

// List never touches the download cache, so a CacheClean queued behind a
// running install must not hold it back.
func TestListDoesNotWaitForPendingCacheClean(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	blockInstallThenClean(t, state, context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := ListPackageStatesViaDaemon(ctx, t.TempDir())
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("List: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("List blocked behind a pending CacheClean")
	}
}

// A writing operation queued behind a pending CacheClean gives up at its
// deadline.
func TestCacheWaitHonorsContext(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	blockInstallThenClean(t, state, context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- UpdatePackagesViaDaemon(ctx, t.TempDir(), []string{"x"}) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Update ignored its context while waiting for the cache")
	}
}

// CacheClean gives up at its deadline while an install holds the cache, and
// operations held back by it then proceed.
func TestCacheCleanHonorsContext(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	cleanCtx, cancelClean := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelClean()
	_, cleanDone := blockInstallThenClean(t, state, cleanCtx)

	select {
	case err := <-cleanDone:
		cleanDone <- err // for the cleanup
		if code := ErrorCode(err); code != "ERR_LIP_CACHE_CLEAN_FAILED" || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected error %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CacheClean ignored its context")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := UpdatePackagesViaDaemon(ctx, t.TempDir(), []string{"x"}); err != nil {
		t.Fatalf("Update after abandoned clean: %v", err)
	}
}

func TestNormalizeTaskTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Inst A")
	if got := normalizeTaskTarget(dir); got != "Inst A" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeTaskTarget(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

// Every operation closes its engine, releasing retained git packs.
func TestOperationsCloseEngine(t *testing.T) {
	recordEvents(t)
	state := useFakeEngine(t)
	dir := t.TempDir()
	ctx := context.Background()
	if err := InstallPackagesViaDaemon(ctx, dir, []string{"example.com/a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ListPackageStatesViaDaemon(ctx, dir); err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closes != 2 {
		t.Fatalf("engine closed %d times, want 2", state.closes)
	}
}
