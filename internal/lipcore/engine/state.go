package engine

import (
	"fmt"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/lockfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// This file ports lip's WorkspaceService: every operation reloads
// tooth_lock.json and every mutation saves it immediately.

type packageScope int

const (
	scopeAll packageScope = iota
	scopeExplicit
	scopeImplicit
)

// loadState reads tooth_lock.json (WorkspaceService.LoadWorkspaceState).
//
// Deliberate deviation from lip: lip prints a load error, continues with an
// empty state and its next save then wipes the user's lock file. Here any
// error from LoadState (a missing file is not an error) is returned, so
// Install/Update/Uninstall abort without saving and List reports it.
func (e *Engine) loadState() (*lockfile.State, error) {
	s, err := e.Hooks.LoadState(e.Dir)
	if err != nil {
		return nil, fmt.Errorf("Failed to load workspace state from 'tooth_lock.json': %w", err)
	}
	if s == nil {
		return lockfile.New(), nil
	}
	if s.Packages == nil {
		s.Packages = []lockfile.Package{}
	}
	return s, nil
}

func (e *Engine) saveState(s *lockfile.State) error {
	return e.Hooks.SaveState(e.Dir, s)
}

func packageSpecOf(p lockfile.Package) manifest.PackageSpec {
	return p.Spec()
}

func findStatePackage(s *lockfile.State, spec manifest.PackageSpec) int {
	for i, p := range s.Packages {
		if packageSpecOf(p) == spec {
			return i
		}
	}
	return -1
}

// installedPackages ports WorkspaceService.GetInstalledPackages: packages of
// the scope, topologically sorted over their stored dependencies (edges only
// within the scope), starting from the stored order.
func (e *Engine) installedPackages(scope packageScope) ([]manifest.PackageSpec, error) {
	s, err := e.loadState()
	if err != nil {
		return nil, err
	}
	var nodes []depNode
	for _, p := range s.Packages {
		switch scope {
		case scopeExplicit:
			if !p.Locked {
				continue
			}
		case scopeImplicit:
			if p.Locked {
				continue
			}
		}
		spec := packageSpecOf(p)
		m := p.Manifest
		variant, err := e.Hooks.ResolveVariant(&m, spec.ID.Variant)
		if err != nil {
			return nil, err
		}
		deps := make([]manifest.PackageID, 0, len(variant.Dependencies))
		for _, d := range variant.Dependencies {
			id, err := e.parseID(d.ID)
			if err != nil {
				return nil, err
			}
			deps = append(deps, id)
		}
		nodes = append(nodes, depNode{spec: spec, deps: deps})
	}
	return topologicalSort(nodes)
}

type depNode struct {
	spec manifest.PackageSpec
	deps []manifest.PackageID
}

// topologicalSort ports IDependencySolver.TopologicalSort.
func topologicalSort(nodes []depNode) ([]manifest.PackageSpec, error) {
	byID := make(map[manifest.PackageID]int, len(nodes))
	for i, n := range nodes {
		if _, dup := byID[n.spec.ID]; dup {
			return nil, fmt.Errorf("An item with the same key has already been added. Key: %s", n.spec.ID)
		}
		byID[n.spec.ID] = i
	}
	visited := map[manifest.PackageID]bool{}
	onStack := map[manifest.PackageID]bool{}
	sorted := make([]manifest.PackageSpec, 0, len(nodes))
	var visit func(n depNode) error
	visit = func(n depNode) error {
		if visited[n.spec.ID] {
			return nil
		}
		if onStack[n.spec.ID] {
			return fmt.Errorf("Circular dependency detected involving package '%s'", n.spec.ID)
		}
		onStack[n.spec.ID] = true
		for _, d := range n.deps {
			if i, ok := byID[d]; ok {
				if err := visit(nodes[i]); err != nil {
					return err
				}
			}
		}
		delete(onStack, n.spec.ID)
		visited[n.spec.ID] = true
		sorted = append(sorted, n.spec)
		return nil
	}
	for _, n := range nodes {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return sorted, nil
}

// addInstalledPackage ports WorkspaceService.AddInstalledPackage.
func (e *Engine) addInstalledPackage(spec manifest.PackageSpec, m *manifest.Manifest, files []string, explicit bool) error {
	if spec.ID.Path != m.Tooth || spec.Version != m.Version {
		return fmt.Errorf("Package spec '%s' does not match manifest path '%s' and version '%s'. (Parameter 'packageSpec')", spec, m.Tooth, m.Version)
	}
	if files == nil {
		files = []string{}
	}
	s, err := e.loadState()
	if err != nil {
		return err
	}
	if findStatePackage(s, spec) >= 0 {
		return fmt.Errorf("Cannot add package '%s' because it is already installed.", spec)
	}
	s.Packages = append(s.Packages, lockfile.Package{
		Files:    files,
		Locked:   explicit,
		Manifest: *m,
		Variant:  spec.ID.Variant,
	})
	return e.saveState(s)
}

func (e *Engine) installedPackageManifest(spec manifest.PackageSpec) (*manifest.Manifest, error) {
	s, err := e.loadState()
	if err != nil {
		return nil, err
	}
	i := findStatePackage(s, spec)
	if i < 0 {
		return nil, fmt.Errorf("Cannot get manifest of package '%s' because it is not installed.", spec)
	}
	m := s.Packages[i].Manifest
	return &m, nil
}

func (e *Engine) installedPackageFiles(spec manifest.PackageSpec) ([]string, error) {
	s, err := e.loadState()
	if err != nil {
		return nil, err
	}
	i := findStatePackage(s, spec)
	if i < 0 {
		return nil, fmt.Errorf("Cannot get files of package '%s' because it is not installed.", spec)
	}
	return append([]string(nil), s.Packages[i].Files...), nil
}

func (e *Engine) removeInstalledPackage(spec manifest.PackageSpec) error {
	s, err := e.loadState()
	if err != nil {
		return err
	}
	if findStatePackage(s, spec) < 0 {
		return fmt.Errorf("Cannot remove package '%s' because it is not installed.", spec)
	}
	kept := make([]lockfile.Package, 0, len(s.Packages))
	for _, p := range s.Packages {
		if packageSpecOf(p) != spec {
			kept = append(kept, p)
		}
	}
	s.Packages = kept
	return e.saveState(s)
}

func (e *Engine) updateExplicitness(spec manifest.PackageSpec, explicit bool) error {
	s, err := e.loadState()
	if err != nil {
		return err
	}
	if findStatePackage(s, spec) < 0 {
		return fmt.Errorf("Cannot update explicitness of package '%s' because it is not installed.", spec)
	}
	for i := range s.Packages {
		if packageSpecOf(s.Packages[i]) == spec {
			s.Packages[i].Locked = explicit
		}
	}
	return e.saveState(s)
}

// aggregateError formats like .NET AggregateException.Message:
// "<msg> (<inner 1>) (<inner 2>)".
type aggregateError struct {
	msg  string
	errs []error
}

func (a *aggregateError) Error() string {
	var b strings.Builder
	b.WriteString(a.msg)
	for _, err := range a.errs {
		b.WriteString(" (")
		b.WriteString(err.Error())
		b.WriteString(")")
	}
	return b.String()
}

func (a *aggregateError) Unwrap() []error { return a.errs }

// wrappedError is .NET's Exception(message, inner): Error() is only the
// message (as lip reports it); the cause stays reachable via errors.Unwrap.
type wrappedError struct {
	msg   string
	cause error
}

func (w *wrappedError) Error() string { return w.msg }
func (w *wrappedError) Unwrap() error { return w.cause }
