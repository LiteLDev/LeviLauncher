package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/solver"
)

// pkgRegistry is lip's IPackageRegistry inside the engine; versions are
// parsed and sorted oldest first.
type pkgRegistry interface {
	versions(ctx context.Context, id manifest.PackageID) ([]solver.Version, error)
	manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error)
}

// baseRegistry is LipClient's top-level registry:
// Composite([[WorkspaceServicePackageRegistry], [GitRemotePackageRegistry]]).
func (e *Engine) baseRegistry() *compositeRegistry {
	return &compositeRegistry{e: e, regs: []pkgRegistry{workspaceRegistry{e: e}, remoteRegistry{e: e}}}
}

// parseSorted parses version texts, drops duplicates (by normalized text)
// and sorts them by precedence, oldest first.
func (e *Engine) parseSorted(texts []string) ([]solver.Version, error) {
	seen := map[string]bool{}
	out := make([]solver.Version, 0, len(texts))
	for _, t := range texts {
		v, err := e.Hooks.ParseVersion(t)
		if err != nil {
			return nil, err
		}
		if seen[v.String()] {
			continue
		}
		seen[v.String()] = true
		out = append(out, v)
	}
	sortVersions(out)
	return out, nil
}

func sortVersions(vs []solver.Version) {
	sort.SliceStable(vs, func(i, j int) bool { return vs[i].Compare(vs[j]) < 0 })
}

// compositeRegistry ports CompositePackageRegistry with one registry per
// group: versions are the union of every registry that succeeds (an error
// only when all fail); a manifest comes from the first registry that has it.
type compositeRegistry struct {
	e    *Engine
	regs []pkgRegistry
}

func (c *compositeRegistry) versions(ctx context.Context, id manifest.PackageID) ([]solver.Version, error) {
	var errs []error
	succeeded := 0
	seen := map[string]bool{}
	var out []solver.Version
	for _, r := range c.regs {
		vs, err := r.versions(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		succeeded++
		for _, v := range vs {
			if !seen[v.String()] {
				seen[v.String()] = true
				out = append(out, v)
			}
		}
	}
	if succeeded == 0 && len(errs) > 0 {
		return nil, &aggregateError{
			msg:  fmt.Sprintf("Failed to retrieve available versions for package %s from any registry.", id),
			errs: errs,
		}
	}
	sortVersions(out)
	return out, nil
}

func (c *compositeRegistry) manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	var errs []error
	for _, r := range c.regs {
		m, err := r.manifest(ctx, spec)
		if err == nil {
			return m, nil
		}
		errs = append(errs, err)
	}
	return nil, &aggregateError{
		msg:  fmt.Sprintf("Failed to retrieve package manifest for package %s from any registry.", spec),
		errs: errs,
	}
}

// artifactsRegistry ports ArtifactsPackageRegistry: the packages named on
// the command line.
type artifactsRegistry struct {
	e    *Engine
	arts []artifact
}

func (a artifactsRegistry) versions(ctx context.Context, id manifest.PackageID) ([]solver.Version, error) {
	var texts []string
	for _, art := range a.arts {
		if art.spec.ID == id {
			texts = append(texts, art.spec.Version)
		}
	}
	return a.e.parseSorted(texts)
}

func (a artifactsRegistry) manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	n := 0
	for _, art := range a.arts {
		if art.spec == spec {
			n++
		}
	}
	switch n {
	case 0:
		return nil, fmt.Errorf("Sequence contains no elements")
	case 1:
		// The artifact's tooth.json is the root tooth.json of its git tree,
		// which is exactly what the remote registry returns.
		return a.e.Registry.Manifest(ctx, spec)
	default:
		return nil, fmt.Errorf("Sequence contains more than one element")
	}
}

// workspaceRegistry ports WorkspaceServicePackageRegistry: the installed
// version of a package and its stored manifest.
type workspaceRegistry struct{ e *Engine }

func (w workspaceRegistry) versions(ctx context.Context, id manifest.PackageID) ([]solver.Version, error) {
	installed, err := w.e.installedPackages(scopeAll)
	if err != nil {
		return nil, err
	}
	var texts []string
	for _, p := range installed {
		if p.ID == id {
			texts = append(texts, p.Version)
		}
	}
	if len(texts) > 1 {
		return nil, fmt.Errorf("Sequence contains more than one matching element")
	}
	return w.e.parseSorted(texts)
}

func (w workspaceRegistry) manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	return w.e.installedPackageManifest(spec)
}

// remoteRegistry adapts Engine.Registry (the git registry).
type remoteRegistry struct{ e *Engine }

func (r remoteRegistry) versions(ctx context.Context, id manifest.PackageID) ([]solver.Version, error) {
	texts, err := r.e.Registry.Versions(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.e.parseSorted(texts)
}

func (r remoteRegistry) manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	return r.e.Registry.Manifest(ctx, spec)
}

// solverAdapter exposes a pkgRegistry as a solver.Registry
// (DependencySolver.GetDependencies resolves the variant for the current
// platform).
type solverAdapter struct {
	e    *Engine
	reg  pkgRegistry
	deps map[string][]solver.Requirement
}

func (s *solverAdapter) Versions(ctx context.Context, id string) ([]solver.Version, error) {
	pid, err := s.e.parseID(id)
	if err != nil {
		return nil, err
	}
	return s.reg.versions(ctx, pid)
}

// Dependencies is memoized per package spec: the solver asks again for
// every selected package when it builds its output.
func (s *solverAdapter) Dependencies(ctx context.Context, id string, v solver.Version) ([]solver.Requirement, error) {
	key := id + "@" + v.String()
	if reqs, ok := s.deps[key]; ok {
		return append([]solver.Requirement(nil), reqs...), nil
	}
	reqs, err := s.dependencies(ctx, id, v)
	if err != nil {
		return nil, err
	}
	if s.deps == nil {
		s.deps = map[string][]solver.Requirement{}
	}
	s.deps[key] = reqs
	return append([]solver.Requirement(nil), reqs...), nil
}

func (s *solverAdapter) dependencies(ctx context.Context, id string, v solver.Version) ([]solver.Requirement, error) {
	pid, err := s.e.parseID(id)
	if err != nil {
		return nil, err
	}
	spec := manifest.PackageSpec{ID: pid, Version: v.String()}
	m, err := s.reg.manifest(ctx, spec)
	if err != nil {
		return nil, err
	}
	variant, err := s.e.Hooks.ResolveVariant(m, pid.Variant)
	if err != nil {
		return nil, err
	}
	reqs := make([]solver.Requirement, 0, len(variant.Dependencies))
	for _, d := range variant.Dependencies {
		did, err := s.e.parseID(d.ID)
		if err != nil {
			return nil, err
		}
		c, err := s.e.Hooks.ParseRange(d.Range)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, fmt.Errorf("invalid version range '%s' for dependency '%s'", d.Range, d.ID)
		}
		reqs = append(reqs, solver.Requirement{ID: did.String(), Constraint: c})
	}
	return reqs, nil
}
