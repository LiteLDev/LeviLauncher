// Package engine implements lip 0.37.0's package operations
// (src/Lip.Core/Services/InstallService.cs, PublicApi/LipClient.cs,
// PackageRegistries/*) on top of the other lipcore packages:
//
//	Install(packages, dryRun, ignoreScripts, noDependencies)
//	Update(packages, dryRun, ignoreScripts)
//	Uninstall(packages, dryRun, ignoreScripts, noDependencies)
//	List() -> explicit, implicit "path#variant@version"
//	CacheClean()
//
// Sibling packages are reached through Hooks so the orchestration logic can
// be tested with fakes.
package engine

import (
	"context"
	"errors"
	"runtime"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lockfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/solver"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("engine: not implemented")

// Registry supplies package versions, manifests and asset contents
// (lip IPackageRegistry + SourceService).
type Registry interface {
	// Versions returns available versions of id (version text, oldest first).
	Versions(ctx context.Context, id manifest.PackageID) ([]string, error)
	// Manifest returns the manifest of spec (already migrated, parsed and with
	// tooth/version fixed up to match spec).
	Manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error)
	// OpenAsset opens one asset of spec ("self" = the git tree of the tag).
	OpenAsset(ctx context.Context, spec manifest.PackageSpec, asset manifest.Asset) (workspace.Source, error)
}

// PackageOps performs per-package file operations (workspace.Workspace).
type PackageOps interface {
	InstallPackage(ctx context.Context, variant *manifest.Variant, open workspace.SourceOpener) ([]string, error)
	UninstallPackage(ctx context.Context, variant *manifest.Variant, files []string) error
}

// Hooks connects the engine to sibling packages. DefaultHooks wires the real
// implementations; tests substitute fakes.
type Hooks struct {
	ParseVersion   func(s string) (solver.Version, error)
	ParseRange     func(s string) (solver.Constraint, error)
	ExactRange     func(v solver.Version) solver.Constraint
	ResolveVariant func(m *manifest.Manifest, label string) (*manifest.Variant, error)
	LoadState      func(dir string) (*lockfile.State, error)
	SaveState      func(dir string, s *lockfile.State) error
	Solve          func(ctx context.Context, reg solver.Registry, roots []solver.Requirement, logf solver.Logf) ([]solver.Selected, error)
	NewPackageOps  func(dir string, ignoreScripts bool, r report.Reporter) PackageOps
}

// DefaultHooks returns hooks backed by packages semver, manifest (with
// glob platform matching for rid), lockfile, solver and workspace.
func DefaultHooks(rid string) Hooks {
	panic("engine: not implemented")
}

// Engine operates on one workspace (instance directory).
type Engine struct {
	// Dir is the absolute workspace directory.
	Dir      string
	Registry Registry
	Reporter report.Reporter
	Hooks    Hooks
	// CacheClean is called by CacheClean (fetch.Cache.Clean in production).
	CleanCache func() error
}

// Install implements InstallService.InstallPackages / LipClient.Install.
// Each package string is a PackageSpec ("id@version") or a PackageId
// (latest version, prereleases included). Errors use lip's messages (e.g.
// "Package '<id>' is already explicitly installed") because callers match
// on them.
func (e *Engine) Install(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	return ErrNotImplemented
}

// Update implements InstallService.UpdatePackages.
func (e *Engine) Update(ctx context.Context, packages []string, dryRun, ignoreScripts bool) error {
	return ErrNotImplemented
}

// Uninstall implements InstallService.UninstallPackages; packages are ids.
func (e *Engine) Uninstall(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	return ErrNotImplemented
}

// List returns installed packages as "path#variant@version" (variant part
// omitted when empty), explicit and implicit, in lip's order (topological
// over stored dependencies starting from stored package order).
func (e *Engine) List(ctx context.Context) (explicit, implicit []string, err error) {
	return nil, nil, ErrNotImplemented
}

// CacheClean clears the download cache.
func (e *Engine) CacheClean(ctx context.Context) error {
	if e.CleanCache == nil {
		return nil
	}
	return e.CleanCache()
}

// CurrentRID returns the .NET runtime identifier for this process
// (RuntimeInformation.RuntimeIdentifier), e.g. "win-x64", "win-arm64",
// "linux-x64", "osx-arm64".
func CurrentRID() string {
	os := runtime.GOOS
	switch os {
	case "windows":
		os = "win"
	case "darwin":
		os = "osx"
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "x86"
	}
	return os + "-" + arch
}
