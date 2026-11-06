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
	"fmt"
	"io"
	"path/filepath"
	"runtime"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/glob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lockfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
	"github.com/liteldev/LeviLauncher/internal/lipcore/solver"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("engine: not implemented")

// Registry supplies package versions, manifests and asset contents
// (lip IPackageRegistry + SourceService).
type Registry interface {
	// Versions returns available versions of id (version text, oldest first).
	Versions(ctx context.Context, id manifest.PackageID) ([]string, error)
	// Manifest returns the manifest of spec as found in the package (already
	// migrated and parsed). Like lip's registries it is NOT fixed up: the
	// engine overwrites tooth/version with the requested spec (warning on a
	// real mismatch) when it installs the package, exactly like lip's
	// PackageInstaller.
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
	// ParseID parses a package id (manifest.ParsePackageID when nil).
	ParseID func(s string) (manifest.PackageID, error)
	// ParseSpec parses "id@version" without validating the version
	// (manifest.ParsePackageSpec when nil); the engine parses the version
	// strictly with ParseVersion.
	ParseSpec func(s string) (manifest.PackageSpec, error)
}

// DefaultHooks returns hooks backed by packages semver, manifest (with
// glob platform matching for rid), lockfile, solver and workspace.
func DefaultHooks(rid string) Hooks {
	return Hooks{
		ParseVersion: func(s string) (solver.Version, error) {
			v, err := semver.Parse(s)
			if err != nil {
				return nil, err
			}
			return semverVersion{v}, nil
		},
		ParseRange: func(s string) (solver.Constraint, error) {
			r, err := semver.ParseRange(s)
			if err != nil {
				return nil, err
			}
			return semverRange{r}, nil
		},
		ExactRange: func(v solver.Version) solver.Constraint {
			if sv, ok := v.(semverVersion); ok {
				return semverRange{semver.Exact(sv.v)}
			}
			return exactConstraint{v}
		},
		ResolveVariant: func(m *manifest.Manifest, label string) (*manifest.Variant, error) {
			return m.ResolveVariant(label, rid, glob.Match)
		},
		LoadState: lockfile.Load,
		SaveState: func(dir string, s *lockfile.State) error { return s.Save(dir) },
		Solve:     solver.Solve,
		NewPackageOps: func(dir string, ignoreScripts bool, r report.Reporter) PackageOps {
			return &workspace.Workspace{
				Dir:           dir,
				Reporter:      r,
				Glob:          compileGlob,
				IgnoreScripts: ignoreScripts,
			}
		},
		ParseID:   manifest.ParsePackageID,
		ParseSpec: manifest.ParsePackageSpec,
	}
}

func compileGlob(pattern string) (workspace.Matcher, error) {
	g, err := glob.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// semverVersion adapts semver.Version to solver.Version.
type semverVersion struct{ v semver.Version }

func (a semverVersion) Compare(o solver.Version) int {
	if b, ok := o.(semverVersion); ok {
		return a.v.Compare(b.v)
	}
	if b, err := semver.Parse(o.String()); err == nil {
		return a.v.Compare(b)
	}
	return 0
}

func (a semverVersion) String() string { return a.v.String() }

// semverRange adapts semver.Range to solver.Constraint.
type semverRange struct{ r semver.Range }

func (r semverRange) Contains(v solver.Version) bool {
	if sv, ok := v.(semverVersion); ok {
		return r.r.Contains(sv.v)
	}
	sv, err := semver.Parse(v.String())
	return err == nil && r.r.Contains(sv)
}

func (r semverRange) String() string { return r.r.String() }

// exactConstraint is a fallback exact range for foreign Version types.
type exactConstraint struct{ v solver.Version }

func (c exactConstraint) Contains(v solver.Version) bool { return c.v.Compare(v) == 0 }
func (c exactConstraint) String() string                 { return c.v.String() }

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

// New returns an engine for dir wired with DefaultHooks(CurrentRID()) and
// NewGitRegistry(githubProxy, r); CacheClean cleans the shared lip cache.
func New(dir, githubProxy string, r report.Reporter) (*Engine, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	reg := NewGitRegistry(githubProxy, r)
	e := &Engine{
		Dir:      abs,
		Registry: reg,
		Reporter: r,
		Hooks:    DefaultHooks(CurrentRID()),
	}
	if reg.Cache != nil {
		e.CleanCache = reg.Cache.Clean
	}
	return e, nil
}

func (e *Engine) rep() report.Reporter { return report.OrNop(e.Reporter) }

// engineBar sends the updates of one engine-level bar (resolve, task).
// Every value comes from counted work. Methods are no-ops on a nil bar (no
// bar is shown) and after done, so a deferred done backs up an explicit one.
type engineBar struct {
	rep    report.Reporter
	p      report.Progress
	closed bool
}

// newBar opens a bar with a random GUID id (like download bars) and reports
// its first update (Current 0).
func (e *Engine) newBar(stage report.Stage, unit report.Unit, message string, total int64) *engineBar {
	b := &engineBar{
		rep: e.rep(),
		p:   report.Progress{ID: newGUID(), Stage: stage, Message: message, Total: total, Unit: unit},
	}
	report.Report(b.rep, b.p)
	return b
}

// update reports cur as the amount done, working on item when not empty.
func (b *engineBar) update(cur int64, item string) {
	if b == nil || b.closed {
		return
	}
	b.p.Current = cur
	if item != "" {
		b.p.Item = item
	}
	report.Report(b.rep, b.p)
}

// step reports cur package steps done, now working on pkg.
func (b *engineBar) step(cur int64, pkg, message string) {
	if b == nil || b.closed {
		return
	}
	b.p.Package = pkg
	b.p.Message = message
	b.update(cur, pkg)
}

// done reports the last update of the bar with the amount done so far.
func (b *engineBar) done() {
	if b == nil || b.closed {
		return
	}
	b.closed = true
	b.p.Done = true
	report.Report(b.rep, b.p)
}

// doneAt is done with cur as the amount done (when it is more than the
// amount last reported), so a bar ended early still counts finished work.
func (b *engineBar) doneAt(cur int64) {
	if b == nil || b.closed {
		return
	}
	if cur > b.p.Current {
		b.p.Current = cur
	}
	b.done()
}

// openTaskBar opens the StageTask bar over n package steps, or returns nil
// (no bar) for a dry run, which applies nothing, or an empty plan.
func (e *Engine) openTaskBar(n int, dryRun bool) *engineBar {
	if dryRun || n == 0 {
		return nil
	}
	return e.newBar(report.StageTask, report.UnitPackages, "Applying package changes", int64(n))
}

// manifestCounter wraps the registry the solver reads, counting the
// distinct manifests read into the solve bar (whose total is unknown: the
// dependency graph is discovered while solving).
type manifestCounter struct {
	pkgRegistry
	bar  *engineBar
	seen map[manifest.PackageSpec]bool
}

func (c *manifestCounter) manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	m, err := c.pkgRegistry.manifest(ctx, spec)
	if err == nil && !c.seen[spec] {
		c.seen[spec] = true
		c.bar.update(int64(len(c.seen)), spec.String())
	}
	return m, err
}

// artifact is lip's PackageArtifact: a resolved spec plus its package
// source (the git tree of the tag).
type artifact struct {
	spec   manifest.PackageSpec
	source workspace.Source
}

var selfAsset = manifest.Asset{Type: manifest.AssetSelf}

// Install implements InstallService.InstallPackages / LipClient.Install.
// Each package string is a PackageSpec ("id@version") or a PackageId
// (latest version, prereleases included). Errors use lip's messages (e.g.
// "Cannot install package '<id>' because it is already explicitly
// installed.") because callers match on them.
func (e *Engine) Install(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	specs, flexible, err := e.parsePackageArgs(packages)
	if err != nil {
		return err
	}
	arts, err := e.resolveArtifacts(ctx, specs, flexible)
	if err != nil {
		return err
	}
	ops := e.Hooks.NewPackageOps(e.Dir, ignoreScripts, e.rep())

	if noDependencies {
		e.rep().Warning("--no-dependencies is enabled. This operation may result in a broken workspace.")
		bar := e.openTaskBar(len(arts), dryRun)
		done := 0
		defer func() { bar.doneAt(int64(done)) }()
		for i, a := range arts {
			bar.step(int64(i), a.spec.String(), "Installing "+a.spec.String())
			if err := e.installPackage(ctx, ops, a, dryRun, true); err != nil {
				return err
			}
			done = i + 1
		}
		return nil
	}

	explicit, err := e.installedPackages(scopeExplicit)
	if err != nil {
		return err
	}
	for _, a := range arts {
		if containsID(explicit, a.spec.ID) {
			return fmt.Errorf("Cannot install package '%s' because it is already explicitly installed.", a.spec.ID)
		}
	}

	reg := &compositeRegistry{e: e, regs: []pkgRegistry{artifactsRegistry{e: e, arts: arts}, e.baseRegistry()}}

	newExplicit := append(append([]manifest.PackageSpec{}, explicit...), artifactSpecs(arts)...)
	newInstalled, err := e.solve(ctx, reg, newExplicit)
	if err != nil {
		return err
	}
	installed, err := e.installedPackages(scopeAll)
	if err != nil {
		return err
	}
	if err := e.applySolution(ctx, ops, arts, installed, newInstalled, newExplicit, dryRun); err != nil {
		return err
	}
	return e.reconcileExplicitness(newInstalled, newExplicit, explicit, dryRun)
}

// Update implements InstallService.UpdatePackages.
func (e *Engine) Update(ctx context.Context, packages []string, dryRun, ignoreScripts bool) error {
	specs, flexible, err := e.parsePackageArgs(packages)
	if err != nil {
		return err
	}
	arts, err := e.resolveArtifacts(ctx, specs, flexible)
	if err != nil {
		return err
	}
	ops := e.Hooks.NewPackageOps(e.Dir, ignoreScripts, e.rep())

	installed, err := e.installedPackages(scopeAll)
	if err != nil {
		return err
	}
	for _, a := range arts {
		if !containsID(installed, a.spec.ID) {
			return fmt.Errorf("Cannot update package '%s' because it is not currently installed.", a.spec.ID)
		}
	}

	reg := &compositeRegistry{e: e, regs: []pkgRegistry{artifactsRegistry{e: e, arts: arts}, e.baseRegistry()}}

	explicit, err := e.installedPackages(scopeExplicit)
	if err != nil {
		return err
	}
	var newExplicit []manifest.PackageSpec
	for _, ep := range explicit {
		if !containsID(artifactSpecs(arts), ep.ID) {
			newExplicit = append(newExplicit, ep)
		}
	}
	newExplicit = append(newExplicit, artifactSpecs(arts)...)

	newInstalled, err := e.solve(ctx, reg, newExplicit)
	if err != nil {
		return err
	}
	if err := e.applySolution(ctx, ops, arts, installed, newInstalled, newExplicit, dryRun); err != nil {
		return err
	}
	return e.reconcileExplicitness(newInstalled, newExplicit, explicit, dryRun)
}

// Uninstall implements InstallService.UninstallPackages; packages are ids.
func (e *Engine) Uninstall(ctx context.Context, packages []string, dryRun, ignoreScripts, noDependencies bool) error {
	ops := e.Hooks.NewPackageOps(e.Dir, ignoreScripts, e.rep())

	if noDependencies {
		e.rep().Warning("--no-dependencies is enabled. This operation may result in a broken workspace.")
		bar := e.openTaskBar(len(packages), dryRun)
		done := 0
		defer func() { bar.doneAt(int64(done)) }()
		for i, p := range packages {
			id, err := e.parseID(p)
			if err != nil {
				return err
			}
			bar.step(int64(i), id.String(), "Uninstalling "+id.String())
			if err := e.uninstallPackage(ctx, ops, id, dryRun); err != nil {
				return err
			}
			done = i + 1
		}
		return nil
	}

	explicit, err := e.installedPackages(scopeExplicit)
	if err != nil {
		return err
	}
	ids := make([]manifest.PackageID, 0, len(packages))
	for _, p := range packages {
		id, err := e.parseID(p)
		if err != nil {
			return err
		}
		if !containsID(explicit, id) {
			return fmt.Errorf("Package '%s' is not explicitly installed and cannot be uninstalled without --no-dependencies.", id)
		}
		ids = append(ids, id)
	}

	var newExplicit []manifest.PackageSpec
	for _, p := range explicit {
		if !idIn(ids, p.ID) {
			newExplicit = append(newExplicit, p)
		}
	}
	newInstalled, err := e.solve(ctx, e.baseRegistry(), newExplicit)
	if err != nil {
		return err
	}
	installed, err := e.installedPackages(scopeAll)
	if err != nil {
		return err
	}
	removals := removedPackages(installed, newInstalled)
	if err := e.uninstallAll(ctx, ops, removals, dryRun); err != nil {
		return err
	}
	return e.reconcileExplicitness(newInstalled, newExplicit, explicit, dryRun)
}

// uninstallAll uninstalls packages in order under a StageTask bar.
func (e *Engine) uninstallAll(ctx context.Context, ops PackageOps, packages []manifest.PackageSpec, dryRun bool) error {
	bar := e.openTaskBar(len(packages), dryRun)
	done := 0
	defer func() { bar.doneAt(int64(done)) }()
	for i, p := range packages {
		bar.step(int64(i), p.String(), "Uninstalling "+p.String())
		if err := e.uninstallPackage(ctx, ops, p.ID, dryRun); err != nil {
			return err
		}
		done = i + 1
	}
	return nil
}

// removedPackages returns the installed packages missing from newInstalled
// in reverse (topological) order, the order they are uninstalled in.
func removedPackages(installed, newInstalled []manifest.PackageSpec) []manifest.PackageSpec {
	var out []manifest.PackageSpec
	for i := len(installed) - 1; i >= 0; i-- {
		if !containsSpec(newInstalled, installed[i]) {
			out = append(out, installed[i])
		}
	}
	return out
}

// List returns installed packages as "path#variant@version" (variant part
// omitted when empty), explicit and implicit, in lip's order (topological
// over stored dependencies starting from stored package order).
func (e *Engine) List(ctx context.Context) (explicit, implicit []string, err error) {
	ex, err := e.installedPackages(scopeExplicit)
	if err != nil {
		return nil, nil, err
	}
	im, err := e.installedPackages(scopeImplicit)
	if err != nil {
		return nil, nil, err
	}
	explicit = make([]string, 0, len(ex))
	for _, p := range ex {
		explicit = append(explicit, p.String())
	}
	implicit = make([]string, 0, len(im))
	for _, p := range im {
		implicit = append(implicit, p.String())
	}
	return explicit, implicit, nil
}

// CacheClean clears the download cache.
func (e *Engine) CacheClean(ctx context.Context) error {
	if e.CleanCache == nil {
		return nil
	}
	return e.CleanCache()
}

// applySolution uninstalls installed packages missing from the solution in
// reverse topological order, then installs new packages in topological
// order (InstallService.InstallPackages / UpdatePackages).
//
// Like lip, a dry run does not remove anything from the lock, so a dry-run
// version change fails at the install step with "Cannot install package ...
// because it is already installed with version ...". This is kept for parity.
//
// The StageTask bar counts both steps: removals, then installs.
func (e *Engine) applySolution(ctx context.Context, ops PackageOps, arts []artifact, installed, newInstalled, newExplicit []manifest.PackageSpec, dryRun bool) error {
	removals := removedPackages(installed, newInstalled)
	var installs []manifest.PackageSpec
	for _, np := range newInstalled {
		if !containsSpec(installed, np) {
			installs = append(installs, np)
		}
	}
	bar := e.openTaskBar(len(removals)+len(installs), dryRun)
	done := int64(0)
	defer func() { bar.doneAt(done) }()

	for _, p := range removals {
		bar.step(done, p.String(), "Uninstalling "+p.String())
		if err := e.uninstallPackage(ctx, ops, p.ID, dryRun); err != nil {
			return err
		}
		done++
	}
	for _, np := range installs {
		bar.step(done, np.String(), "Installing "+np.String())
		a, ok := findArtifact(arts, np)
		if !ok {
			src, err := e.Registry.OpenAsset(ctx, np, selfAsset)
			if err != nil {
				return err
			}
			a = artifact{spec: np, source: src}
		}
		if err := e.installPackage(ctx, ops, a, dryRun, containsSpec(newExplicit, np)); err != nil {
			return err
		}
		done++
	}
	return nil
}

// reconcileExplicitness ports InstallService.ReconcilePackageExplicitness.
func (e *Engine) reconcileExplicitness(installed, explicit, previousExplicit []manifest.PackageSpec, dryRun bool) error {
	for _, p := range installed {
		isExplicit := containsSpec(explicit, p)
		wasExplicit := containsSpec(previousExplicit, p)
		switch {
		case isExplicit && !wasExplicit:
			if dryRun {
				e.rep().Info(fmt.Sprintf("Dry run: would mark package '%s' version %s as explicitly installed.", p.ID, p.Version))
				continue
			}
			e.rep().Info(fmt.Sprintf("Marking package '%s' version %s as explicitly installed.", p.ID, p.Version))
			if err := e.updateExplicitness(p, true); err != nil {
				return err
			}
		case !isExplicit && wasExplicit:
			if dryRun {
				e.rep().Info(fmt.Sprintf("Dry run: would mark package '%s' version %s as not explicitly installed.", p.ID, p.Version))
				continue
			}
			e.rep().Info(fmt.Sprintf("Marking package '%s' version %s as not explicitly installed.", p.ID, p.Version))
			if err := e.updateExplicitness(p, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// installPackage ports PackageInstaller.InstallPackage.
func (e *Engine) installPackage(ctx context.Context, ops PackageOps, a artifact, dryRun, explicitInstall bool) error {
	installed, err := e.installedPackages(scopeAll)
	if err != nil {
		return err
	}
	for _, p := range installed {
		if p.ID == a.spec.ID {
			return fmt.Errorf("Cannot install package %s version %s because it is already installed with version %s", a.spec.ID, a.spec.Version, p.Version)
		}
	}
	if dryRun {
		e.rep().Info(fmt.Sprintf("Dry run: would install package %s version %s", a.spec.ID, a.spec.Version))
		return nil
	}
	e.rep().Info(fmt.Sprintf("Installing package %s version %s", a.spec.ID, a.spec.Version))

	raw, err := e.Registry.Manifest(ctx, a.spec)
	if err != nil {
		return err
	}
	m := e.fixupManifest(raw, a.spec)

	variant, err := e.Hooks.ResolveVariant(m, a.spec.ID.Variant)
	if err != nil {
		return err
	}

	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()
	open := func(ctx context.Context, asset manifest.Asset) (workspace.Source, error) {
		if asset.Type == manifest.AssetSelf {
			if a.source == nil {
				return nil, fmt.Errorf("package %s has no package source", a.spec)
			}
			return a.source, nil
		}
		src, err := e.Registry.OpenAsset(ctx, a.spec, asset)
		if err != nil {
			return nil, err
		}
		if c, ok := src.(io.Closer); ok {
			closers = append(closers, c)
		}
		return src, nil
	}
	files, err := ops.InstallPackage(workspace.WithPackage(ctx, a.spec.String()), variant, open)
	if err != nil {
		return err
	}
	return e.addInstalledPackage(a.spec, m, files, explicitInstall)
}

// fixupManifest returns a copy of m whose tooth/version are the requested
// ones, warning like lip when they really differ (not just in case or build
// metadata).
func (e *Engine) fixupManifest(m *manifest.Manifest, spec manifest.PackageSpec) *manifest.Manifest {
	out := *m
	if m.Tooth == spec.ID.Path && m.Version == spec.Version {
		return &out
	}
	if !equalFoldASCII(m.Tooth, spec.ID.Path) || !e.samePrecedence(m.Version, spec.Version) {
		e.rep().Warning(fmt.Sprintf("Manifest of package %s version %s declares '%s' version %s; using the requested package path and version.",
			spec.ID, spec.Version, m.Tooth, m.Version))
	}
	out.Tooth = spec.ID.Path
	out.Version = spec.Version
	return &out
}

func (e *Engine) samePrecedence(a, b string) bool {
	va, err := e.Hooks.ParseVersion(a)
	if err != nil {
		return false
	}
	vb, err := e.Hooks.ParseVersion(b)
	if err != nil {
		return false
	}
	return va.Compare(vb) == 0
}

// equalFoldASCII is StringComparison.OrdinalIgnoreCase for the ASCII-only
// package paths (module.CheckPath forbids non-ASCII).
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// uninstallPackage ports PackageInstaller.UninstallPackage.
func (e *Engine) uninstallPackage(ctx context.Context, ops PackageOps, id manifest.PackageID, dryRun bool) error {
	installed, err := e.installedPackages(scopeAll)
	if err != nil {
		return err
	}
	var existing manifest.PackageSpec
	found := false
	for _, p := range installed {
		if p.ID == id {
			existing, found = p, true
			break
		}
	}
	if !found {
		return fmt.Errorf("Cannot uninstall package %s because it is not installed", id)
	}
	if dryRun {
		e.rep().Info(fmt.Sprintf("Dry run: would uninstall package %s version %s", existing.ID, existing.Version))
		return nil
	}
	e.rep().Info(fmt.Sprintf("Uninstalling package %s version %s", existing.ID, existing.Version))

	m, err := e.installedPackageManifest(existing)
	if err != nil {
		return err
	}
	variant, err := e.Hooks.ResolveVariant(m, existing.ID.Variant)
	if err != nil {
		return err
	}
	files, err := e.installedPackageFiles(existing)
	if err != nil {
		return err
	}
	if err := ops.UninstallPackage(workspace.WithPackage(ctx, existing.String()), variant, files); err != nil {
		return err
	}
	return e.removeInstalledPackage(existing)
}

// parsePackageArgs ports LipClient.Install/Update argument parsing:
// PackageSpec first, then PackageId (latest version). Local archives and
// remote archive URLs are not supported.
func (e *Engine) parsePackageArgs(packages []string) (specs []manifest.PackageSpec, flexible []manifest.PackageID, err error) {
	for _, p := range packages {
		spec, specErr := e.parseSpec(p)
		if specErr == nil {
			specs = append(specs, spec)
			continue
		}
		id, idErr := e.parseID(p)
		if idErr == nil {
			flexible = append(flexible, id)
			continue
		}
		return nil, nil, &aggregateError{
			msg: fmt.Sprintf("Failed to parse package '%s'.", p),
			errs: []error{
				specErr,
				idErr,
				errors.New("local archive packages are not supported"),
				errors.New("remote archive URL packages are not supported"),
			},
		}
	}
	return specs, flexible, nil
}

func (e *Engine) parseID(s string) (manifest.PackageID, error) {
	if e.Hooks.ParseID != nil {
		return e.Hooks.ParseID(s)
	}
	return manifest.ParsePackageID(s)
}

// parseSpec parses "id@version" with a strictly parsed, normalized version.
func (e *Engine) parseSpec(s string) (manifest.PackageSpec, error) {
	parse := e.Hooks.ParseSpec
	if parse == nil {
		parse = manifest.ParsePackageSpec
	}
	spec, err := parse(s)
	if err != nil {
		return manifest.PackageSpec{}, err
	}
	v, err := e.Hooks.ParseVersion(spec.Version)
	if err != nil {
		return manifest.PackageSpec{}, err
	}
	spec.Version = v.String()
	return spec, nil
}

// resolveArtifacts ports InstallService.ResolvePackageArtifacts. It reports
// a StageResolve bar over the requested packages resolved.
func (e *Engine) resolveArtifacts(ctx context.Context, specs []manifest.PackageSpec, flexible []manifest.PackageID) ([]artifact, error) {
	var bar *engineBar
	if n := len(specs) + len(flexible); n > 0 {
		bar = e.newBar(report.StageResolve, report.UnitPackages, "Resolving packages", int64(n))
		defer bar.done()
	}
	var arts []artifact
	for _, spec := range specs {
		bar.update(int64(len(arts)), spec.String())
		src, err := e.Registry.OpenAsset(ctx, spec, selfAsset)
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact{spec: spec, source: src})
	}
	for _, id := range flexible {
		bar.update(int64(len(arts)), id.String())
		e.rep().Info(fmt.Sprintf("Getting available versions for package '%s'...", id))
		spec, err := e.latestVersion(ctx, id)
		if err != nil {
			return nil, err
		}
		src, err := e.Registry.OpenAsset(ctx, spec, selfAsset)
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact{spec: spec, source: src})
	}
	bar.update(int64(len(arts)), "")
	for i, a := range arts {
		for _, b := range arts[i+1:] {
			if a.spec.ID == b.spec.ID {
				return nil, fmt.Errorf("Multiple packages with ID '%s' were specified. Please ensure that each package ID is unique.", a.spec.ID)
			}
		}
	}
	return arts, nil
}

// latestVersion ports InstallService.GetLatestVersion (prereleases
// included) over the workspace + git registries.
func (e *Engine) latestVersion(ctx context.Context, id manifest.PackageID) (manifest.PackageSpec, error) {
	versions, err := e.baseRegistry().versions(ctx, id)
	if err != nil {
		return manifest.PackageSpec{}, err
	}
	if len(versions) == 0 {
		return manifest.PackageSpec{}, fmt.Errorf("Failed to find the latest version for package '%s'.", id)
	}
	return manifest.PackageSpec{ID: id, Version: versions[len(versions)-1].String()}, nil
}

// solve runs the dependency solver with exact requirements on roots. It
// reports an indeterminate StageResolve bar counting the distinct manifests
// read.
func (e *Engine) solve(ctx context.Context, reg pkgRegistry, roots []manifest.PackageSpec) ([]manifest.PackageSpec, error) {
	bar := e.newBar(report.StageResolve, report.UnitPackages, "Resolving dependencies", 0)
	defer bar.done()
	reg = &manifestCounter{pkgRegistry: reg, bar: bar, seen: map[manifest.PackageSpec]bool{}}

	reqs := make([]solver.Requirement, 0, len(roots))
	for _, r := range roots {
		v, err := e.Hooks.ParseVersion(r.Version)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, solver.Requirement{ID: r.ID.String(), Constraint: e.Hooks.ExactRange(v)})
	}
	logf := func(format string, args ...any) { e.rep().Info(fmt.Sprintf(format, args...)) }
	selected, err := e.Hooks.Solve(ctx, &solverAdapter{e: e, reg: reg}, reqs, logf)
	if err != nil {
		return nil, err
	}
	out := make([]manifest.PackageSpec, 0, len(selected))
	for _, s := range selected {
		id, err := e.parseID(s.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, manifest.PackageSpec{ID: id, Version: s.Version.String()})
	}
	return out, nil
}

func artifactSpecs(arts []artifact) []manifest.PackageSpec {
	out := make([]manifest.PackageSpec, len(arts))
	for i, a := range arts {
		out[i] = a.spec
	}
	return out
}

func findArtifact(arts []artifact, spec manifest.PackageSpec) (artifact, bool) {
	for _, a := range arts {
		if a.spec == spec {
			return a, true
		}
	}
	return artifact{}, false
}

func containsSpec(list []manifest.PackageSpec, s manifest.PackageSpec) bool {
	for _, p := range list {
		if p == s {
			return true
		}
	}
	return false
}

func containsID(list []manifest.PackageSpec, id manifest.PackageID) bool {
	for _, p := range list {
		if p.ID == id {
			return true
		}
	}
	return false
}

func idIn(ids []manifest.PackageID, id manifest.PackageID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// Size forwards workspace.Sizer to the wrapped archive source. The embedded
// fetch.AssetSource is an interface, so its dynamic type's Size method is not
// promoted; without this the placement bar could never count bytes for
// archive assets.
func (s archiveSource) Size(key string) (int64, bool) {
	if sz, ok := s.AssetSource.(workspace.Sizer); ok {
		return sz.Size(key)
	}
	return 0, false
}

var _ workspace.Sizer = archiveSource{}

// Close releases the registry's resources (retained git packs) when it has
// any. The engine must not be used afterwards.
func (e *Engine) Close() error {
	if c, ok := e.Registry.(io.Closer); ok {
		return c.Close()
	}
	return nil
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
