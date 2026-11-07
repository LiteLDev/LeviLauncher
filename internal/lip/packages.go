package lip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

const defaultTaskTimeout = 5 * time.Minute

// packageEngine is the subset of *engine.Engine the adapter uses.
type packageEngine interface {
	Install(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error
	Update(ctx context.Context, packages []string, dryRun, ignoreScripts bool) error
	Uninstall(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error
	List(ctx context.Context) (explicit, implicit []string, err error)
	CacheClean(ctx context.Context) error
}

var _ packageEngine = (*engine.Engine)(nil)

// newEngine builds the engine for an absolute workspace directory (empty
// for directory-independent calls such as CacheClean); replaced in tests.
var newEngine = func(workDir string, r report.Reporter) packageEngine {
	return engine.New(workDir, engine.Options{GithubProxy: EffectiveGithubProxy(), HTTPClient: engineHTTPClient}, r)
}

// engineHTTPClient carries the engine's git requests and downloads. Its
// transport follows the Windows system proxy and, like lip's
// HttpClient.DefaultProxy, waits for PAC/WPAD resolution instead of
// falling back to a direct connection. It has no overall timeout (downloads
// use an idle timeout, git requests their own contexts) and, like Flurl,
// refuses https -> http redirects.
var engineHTTPClient = &http.Client{
	Transport:     httpx.NewBlockingProxyTransport(),
	CheckRedirect: httpclient.CheckRedirect,
}

// dirLocks serializes writing operations per workspace so the workspace
// (.lip/workspace.yaml) has one writer at a time. List does not take it: lip
// runs one process per call, so a List during an install reads the last
// saved workspace file, which lip replaces atomically.
var dirLocks sync.Map // lock key -> chan struct{} (capacity 1)

// cacheLock lets package operations share the download cache while
// CacheClean waits for them and removes it exclusively. List does not take
// it: it never touches the cache.
var cacheLock ctxRWLock

// ctxRWLock is a writer-preferring read/write lock whose waits honour a
// context. Like sync.RWMutex, a waiting writer holds back new readers so
// CacheClean is not starved by back-to-back installs.
type ctxRWLock struct {
	mu             sync.Mutex
	readers        int
	writer         bool
	writersWaiting int
	changed        chan struct{} // closed and dropped on every state change
}

// waitCh returns the channel closed on the next state change; l.mu is held.
func (l *ctxRWLock) waitCh() chan struct{} {
	if l.changed == nil {
		l.changed = make(chan struct{})
	}
	return l.changed
}

// notify wakes all waiters; l.mu is held.
func (l *ctxRWLock) notify() {
	if l.changed != nil {
		close(l.changed)
		l.changed = nil
	}
}

func (l *ctxRWLock) RLock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	for l.writer || l.writersWaiting > 0 {
		ch := l.waitCh()
		l.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
		l.mu.Lock()
	}
	l.readers++
	l.mu.Unlock()
	return nil
}

func (l *ctxRWLock) RUnlock() {
	l.mu.Lock()
	l.readers--
	if l.readers == 0 {
		l.notify()
	}
	l.mu.Unlock()
}

func (l *ctxRWLock) Lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	l.writersWaiting++
	for l.writer || l.readers > 0 {
		ch := l.waitCh()
		l.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			l.mu.Lock()
			l.writersWaiting--
			// Readers held back by this writer may proceed now.
			l.notify()
			l.mu.Unlock()
			return ctx.Err()
		}
		l.mu.Lock()
	}
	l.writersWaiting--
	l.writer = true
	l.mu.Unlock()
	return nil
}

func (l *ctxRWLock) Unlock() {
	l.mu.Lock()
	l.writer = false
	l.notify()
	l.mu.Unlock()
}

func dirLockKey(dir string) string {
	// Resolve junctions/symlinks so aliases of one instance share a lock.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(dir)
	}
	return dir
}

func lockDir(ctx context.Context, dir string) (func(), error) {
	value, _ := dirLocks.LoadOrStore(dirLockKey(dir), make(chan struct{}, 1))
	sem := value.(chan struct{})
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func normalizeCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), defaultTaskTimeout)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTaskTimeout)
}

func resolveWorkDir(workDir string) (string, error) {
	if strings.TrimSpace(workDir) == "" {
		return "", fmt.Errorf("empty work directory")
	}
	abs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(workDir)))
	if err != nil {
		return "", fmt.Errorf("resolve work directory: %w", err)
	}
	return abs, nil
}

// callEngine builds an engine and runs fn, turning panics raised on the
// calling goroutine into errors. With usesCache, it holds cacheLock (shared)
// for the call.
func callEngine(ctx context.Context, workDir string, r report.Reporter, usesCache bool, fn func(context.Context, packageEngine) error) (err error) {
	if usesCache {
		if err := cacheLock.RLock(ctx); err != nil {
			return err
		}
		defer cacheLock.RUnlock()
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("lip engine panic: %v", p)
		}
	}()
	eng := newEngine(workDir, r)
	if c, ok := eng.(io.Closer); ok {
		defer c.Close()
	}
	return fn(ctx, eng)
}

// runPackageTask runs one operation on workDir. With track, it emits
// lip_task_started, forwards reports as lip_task_log/lip_task_progress and
// ends with lip_task_finished. With exclusive (a writing operation), it
// holds workDir's lock and a shared cacheLock for the whole operation.
func runPackageTask(ctx context.Context, method string, workDir string, packages []string, track, exclusive bool, fn func(context.Context, packageEngine) error) error {
	dir, err := resolveWorkDir(workDir)
	if err != nil {
		return err
	}
	ctx, cancel := normalizeCtx(ctx)
	defer cancel()

	var meta lipTaskMeta
	var tracker *taskReporter
	var reporter report.Reporter = logReporter{}
	if track {
		meta = newTaskMeta(method, dir, packages)
		tracker = newTaskReporter(meta)
		reporter = tracker
		emitTaskStarted(meta)
	}

	err = func() error {
		if !exclusive {
			return callEngine(ctx, dir, reporter, false, fn)
		}
		unlock, err := lockDir(ctx, dir)
		if err != nil {
			return err
		}
		defer unlock()
		return callEngine(ctx, dir, reporter, true, fn)
	}()

	if track {
		finishTask(meta, tracker, err)
	}
	return err
}

// finishTask ends the bars r still has open, then sends lip_task_finished.
func finishTask(meta lipTaskMeta, r *taskReporter, err error) {
	r.close()
	if err != nil {
		emitTaskFinished(meta, false, formatEngineError(err))
	} else {
		emitTaskFinished(meta, true, "")
	}
}

// CacheClean clears lip's download cache once running package operations
// have finished. It is a tracked task with Method "CacheClean", an empty
// Target (the cache is shared by all instances) and no packages; it starts
// before waiting for those operations, so the wait shows as a running task.
func CacheClean(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	meta := newTaskMeta("CacheClean", "", nil)
	tracker := newTaskReporter(meta)
	emitTaskStarted(meta)
	err := func() error {
		if err := cacheLock.Lock(ctx); err != nil {
			return err
		}
		defer cacheLock.Unlock()
		return callEngine(ctx, "", tracker, false, func(ctx context.Context, e packageEngine) error {
			return e.CacheClean(ctx)
		})
	}()
	if err != nil {
		err = fmt.Errorf("ERR_LIP_CACHE_CLEAN_FAILED: %w", err)
	}
	finishTask(meta, tracker, err)
	return err
}

// InstallPackages installs packages ("<id>[#<variant>][@<version>]") into
// workDir.
func InstallPackages(ctx context.Context, workDir string, packages []string) error {
	pkgs := normalizePackages(packages)
	return runPackageTask(ctx, "Install", workDir, pkgs, true, true, func(ctx context.Context, e packageEngine) error {
		return e.Install(ctx, pkgs, false, false, false)
	})
}

// UpdatePackages updates packages in workDir.
func UpdatePackages(ctx context.Context, workDir string, packages []string) error {
	pkgs := normalizePackages(packages)
	return runPackageTask(ctx, "Update", workDir, pkgs, true, true, func(ctx context.Context, e packageEngine) error {
		return e.Update(ctx, pkgs, false, false)
	})
}

// UninstallPackages uninstalls packages (identities or sources) from workDir. With
// noDependencies, dependencies that become unused are kept.
func UninstallPackages(ctx context.Context, workDir string, packages []string, noDependencies bool) error {
	pkgs := normalizePackages(packages)
	return runPackageTask(ctx, "Uninstall", workDir, pkgs, true, true, func(ctx context.Context, e packageEngine) error {
		return e.Uninstall(ctx, pkgs, false, false, noDependencies)
	})
}

// PackageInstallState describes the installed package of a workspace with
// one package id.
type PackageInstallState struct {
	// Identity is the identity the workspace records (<id>[#<variant>]),
	// e.g. "github.com/LiteLDev/LeviLamina#client" for a workspace migrated
	// from tooth_lock.json. Update and uninstall the package by it.
	Identity string
	// ID is the package id as recorded, Variant the variant ("" for none).
	ID                string
	Variant           string
	Installed         bool
	ExplicitInstalled bool
	InstalledVersion  string
}

// PackageSpec is a package argument "<id>[#<variant>][@<version>]" or an
// entry of List ("<identity>@<version>").
type PackageSpec struct {
	ID      string
	Variant string
	// Version is the part after "@" (a version, or another ref of lip 0.38
	// such as "branch:main"); "" when there is none.
	Version string
}

// Identity is the identity of s: its id, followed by "#" and the variant
// when there is one.
func (s PackageSpec) Identity() string {
	return ref.Identity(s.ID, s.Variant)
}

// ParsePackageSpec parses "<id>[#<variant>][@<version>]" as lip 0.38 parses
// a remote repository source: everything after the first "@" is the ref,
// and a "#" before it starts the variant.
func ParsePackageSpec(raw string) (PackageSpec, bool) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return PackageSpec{}, false
	}
	if r, _, ok := ref.TryParse(spec, false); ok && r.Kind == ref.KindSpec {
		out := PackageSpec{ID: r.Target, Variant: r.Variant}
		if r.GitRef != nil {
			out.Version = r.GitRef.String()
		}
		return out, true
	}
	// List entries come from the engine; split them leniently so a version
	// or variant lip would not accept as an argument still yields a state.
	head, version := spec, ""
	if at := strings.IndexByte(spec, '@'); at >= 0 {
		head, version = spec[:at], strings.TrimSpace(spec[at+1:])
	}
	id, variant := head, ""
	if hash := strings.IndexByte(head, '#'); hash >= 0 {
		id, variant = head[:hash], strings.TrimSpace(head[hash+1:])
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return PackageSpec{}, false
	}
	return PackageSpec{ID: id, Variant: variant, Version: version}, true
}

// PackageKey is the key of a package id in ListPackageStates: the id folded
// ASCII case-insensitively, as lip compares package ids.
func PackageKey(id string) string {
	return ref.Fold(strings.TrimSpace(id))
}

// buildPackageInstallStateIndex indexes List output by package id
// (PackageKey). When several variants of one id are installed, the
// explicitly installed one is kept, otherwise the first.
func buildPackageInstallStateIndex(explicit, implicit []string) map[string]PackageInstallState {
	states := make(map[string]PackageInstallState)
	merge := func(specs []string, isExplicit bool) {
		for _, raw := range specs {
			spec, ok := ParsePackageSpec(raw)
			if !ok {
				continue
			}
			key := PackageKey(spec.ID)
			if _, exists := states[key]; exists {
				// Explicit entries are merged first, so the first one wins.
				continue
			}
			states[key] = PackageInstallState{
				Identity:          spec.Identity(),
				ID:                spec.ID,
				Variant:           spec.Variant,
				Installed:         true,
				ExplicitInstalled: isExplicit,
				InstalledVersion:  spec.Version,
			}
		}
	}
	merge(explicit, true)
	merge(implicit, false)
	return states
}

// ListPackageStates returns the installed packages of workDir keyed by
// package id (PackageKey).
func ListPackageStates(ctx context.Context, workDir string) (map[string]PackageInstallState, error) {
	var explicit, implicit []string
	err := runPackageTask(ctx, "List", workDir, nil, false, false, func(ctx context.Context, e packageEngine) error {
		var err error
		explicit, implicit, err = e.List(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return buildPackageInstallStateIndex(explicit, implicit), nil
}

// GetPackageInstallState returns the state of the package with the id of
// packageRef ("<id>[#<variant>][@<version>]"; variant and version are
// ignored) in workDir; zero when no variant of it is installed.
func GetPackageInstallState(ctx context.Context, workDir string, packageRef string) (PackageInstallState, error) {
	spec, ok := ParsePackageSpec(packageRef)
	if !ok {
		return PackageInstallState{}, nil
	}
	states, err := ListPackageStates(ctx, workDir)
	if err != nil {
		return PackageInstallState{}, err
	}
	return states[PackageKey(spec.ID)], nil
}

// IsPackageExplicitlyInstalled reports whether a package with the id of
// packageRef is installed explicitly (not only as a dependency) in workDir.
func IsPackageExplicitlyInstalled(ctx context.Context, workDir string, packageRef string) (bool, error) {
	state, err := GetPackageInstallState(ctx, workDir, packageRef)
	if err != nil {
		return false, err
	}
	return state.ExplicitInstalled, nil
}

// isAlreadyExplicitlyInstalled reports whether err is lip's refusal to
// install a package that is already explicitly installed.
func isAlreadyExplicitlyInstalled(err error) bool {
	return errors.Is(err, engine.ErrAlreadyExplicitlyInstalled)
}

// IsAlreadyExplicitlyInstalled reports whether err is lip's refusal to
// install a package that is already explicitly installed (lip 0.38 asks to
// run update instead).
func IsAlreadyExplicitlyInstalled(err error) bool {
	return isAlreadyExplicitlyInstalled(err)
}

// formatEngineError is the full message of an engine error, as lip prints
// it.
func formatEngineError(err error) string {
	if err == nil {
		return ""
	}
	return engine.FormatError(err)
}

// ErrorMessage is the full message of an error of a package operation, as
// lip prints it.
func ErrorMessage(err error) string {
	return formatEngineError(err)
}
