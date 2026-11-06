package lip

import (
	"context"
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
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
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
	return &engine.Engine{
		Dir:        workDir,
		Registry:   engine.NewGitRegistry(EffectiveGithubProxy(), r, engine.WithHTTPClient(engineHTTPClient)),
		Reporter:   r,
		Hooks:      engine.DefaultHooks(engine.CurrentRID()),
		CleanCache: fetch.DefaultCache().Clean,
	}
}

// engineHTTPClient carries the engine's git requests and downloads. Its
// transport follows the Windows system proxy, as lip's
// HttpClient.DefaultProxy did. It has no overall timeout (downloads use an
// idle timeout, git requests their own contexts) and, like Flurl, refuses
// https -> http redirects.
var engineHTTPClient = &http.Client{
	Transport:     httpx.DefaultClient().Transport,
	CheckRedirect: httpclient.CheckRedirect,
}

// dirLocks serializes writing operations per workspace so tooth_lock.json
// has one writer at a time. List does not take it: lip ran one process per
// call, so a List during an install read the last saved tooth_lock.json
// (lockfile.State.Save replaces the file atomically).
var dirLocks sync.Map // lock key -> chan struct{} (capacity 1)

// cacheMu lets package operations share the download cache while CacheClean
// waits for them and removes it exclusively.
var cacheMu sync.RWMutex

// cleanCache clears the download cache; replaced in tests.
var cleanCache = func() error { return fetch.DefaultCache().Clean() }

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
// calling goroutine into errors.
func callEngine(ctx context.Context, workDir string, r report.Reporter, fn func(context.Context, packageEngine) error) (err error) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
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
// ends with lip_task_finished. With exclusive, it holds workDir's lock for
// the whole operation.
func runPackageTask(ctx context.Context, method string, workDir string, packages []string, track, exclusive bool, fn func(context.Context, packageEngine) error) error {
	dir, err := resolveWorkDir(workDir)
	if err != nil {
		return err
	}
	ctx, cancel := normalizeCtx(ctx)
	defer cancel()

	var meta lipTaskMeta
	var reporter report.Reporter = logReporter{}
	if track {
		meta = newTaskMeta(method, dir, packages)
		reporter = taskReporter{meta: meta}
		emitTaskStarted(meta)
	}

	err = func() error {
		if !exclusive {
			return callEngine(ctx, dir, reporter, fn)
		}
		unlock, err := lockDir(ctx, dir)
		if err != nil {
			return err
		}
		defer unlock()
		return callEngine(ctx, dir, reporter, fn)
	}()

	if track {
		if err != nil {
			emitTaskFinished(meta, false, err.Error())
		} else {
			emitTaskFinished(meta, true, "")
		}
	}
	return err
}

// CacheClean clears lip's download cache.
func CacheClean(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("ERR_LIP_CACHE_CLEAN_FAILED: %w", err)
		}
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	err := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("lip cache clean panic: %v", p)
			}
		}()
		return cleanCache()
	}()
	if err != nil {
		return fmt.Errorf("ERR_LIP_CACHE_CLEAN_FAILED: %w", err)
	}
	return nil
}

// InstallPackages installs packages ("id" or "id@version") into workDir.
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

// UninstallPackages uninstalls package ids from workDir. With
// noDependencies, dependencies that become unused are kept.
func UninstallPackages(ctx context.Context, workDir string, packages []string, noDependencies bool) error {
	pkgs := normalizePackages(packages)
	return runPackageTask(ctx, "Uninstall", workDir, pkgs, true, true, func(ctx context.Context, e packageEngine) error {
		return e.Uninstall(ctx, pkgs, false, false, noDependencies)
	})
}

// PackageInstallState describes one installed package of a workspace.
type PackageInstallState struct {
	Installed         bool
	ExplicitInstalled bool
	InstalledVersion  string
}

type packageSpec struct {
	Path    string
	Variant string
	Version string
}

func (s packageSpec) id() string {
	if s.Variant == "" {
		return s.Path
	}
	return s.Path + "#" + s.Variant
}

// parsePackageSpec parses "path#variant@version" (variant and version
// optional).
func parsePackageSpec(raw string) (packageSpec, bool) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return packageSpec{}, false
	}

	version := ""
	if at := strings.LastIndex(spec, "@"); at > 0 {
		version = strings.TrimSpace(spec[at+1:])
		spec = strings.TrimSpace(spec[:at])
	}

	path := spec
	variant := ""
	if hash := strings.LastIndex(spec, "#"); hash > 0 {
		path = strings.TrimSpace(spec[:hash])
		variant = strings.TrimSpace(spec[hash+1:])
	}
	if path == "" {
		return packageSpec{}, false
	}
	return packageSpec{Path: path, Variant: variant, Version: version}, true
}

// buildPackageInstallStateIndex indexes List output by lowercased
// "path#variant".
func buildPackageInstallStateIndex(explicit, implicit []string) map[string]PackageInstallState {
	states := make(map[string]PackageInstallState)
	merge := func(specs []string, isExplicit bool) {
		for _, raw := range specs {
			spec, ok := parsePackageSpec(raw)
			if !ok {
				continue
			}
			key := strings.ToLower(spec.id())
			state := states[key]
			state.Installed = true
			if isExplicit {
				state.ExplicitInstalled = true
				if spec.Version != "" {
					state.InstalledVersion = spec.Version
				}
			} else if state.InstalledVersion == "" {
				state.InstalledVersion = spec.Version
			}
			states[key] = state
		}
	}
	merge(explicit, true)
	merge(implicit, false)
	return states
}

// ListPackageStates returns the installed packages of workDir keyed by
// lowercased "path#variant".
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

func trimPackageVersion(spec string) string {
	trimmed := strings.TrimSpace(spec)
	idx := strings.LastIndex(trimmed, "@")
	if idx <= 0 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:idx])
}

// GetPackageInstallState returns the state of packageRef ("path#variant",
// optionally with "@version") in workDir; zero when not installed.
func GetPackageInstallState(ctx context.Context, workDir string, packageRef string) (PackageInstallState, error) {
	target := strings.ToLower(trimPackageVersion(packageRef))
	if target == "" {
		return PackageInstallState{}, nil
	}
	states, err := ListPackageStates(ctx, workDir)
	if err != nil {
		return PackageInstallState{}, err
	}
	return states[target], nil
}

// IsPackageExplicitlyInstalled reports whether packageRef is installed
// explicitly (not only as a dependency) in workDir.
func IsPackageExplicitlyInstalled(ctx context.Context, workDir string, packageRef string) (bool, error) {
	state, err := GetPackageInstallState(ctx, workDir, packageRef)
	if err != nil {
		return false, err
	}
	return state.ExplicitInstalled, nil
}

// Names kept from the lipd daemon era; callers in mcservice use them.

func InstallPackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	return InstallPackages(ctx, workDir, packages)
}

func UpdatePackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	return UpdatePackages(ctx, workDir, packages)
}

func UninstallPackagesViaDaemon(ctx context.Context, workDir string, packages []string, noDependencies bool) error {
	return UninstallPackages(ctx, workDir, packages, noDependencies)
}

func ListPackageStatesViaDaemon(ctx context.Context, workDir string) (map[string]PackageInstallState, error) {
	return ListPackageStates(ctx, workDir)
}

func GetPackageInstallStateViaDaemon(ctx context.Context, workDir string, packageRef string) (PackageInstallState, error) {
	return GetPackageInstallState(ctx, workDir, packageRef)
}

func IsPackageExplicitlyInstalledViaDaemon(ctx context.Context, workDir string, packageRef string) (bool, error) {
	return IsPackageExplicitlyInstalled(ctx, workDir, packageRef)
}
