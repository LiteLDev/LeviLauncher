// Package solver ports lip 0.37.0's backtracking dependency solver
// (src/Lip.Core/Services/DependencySolver.cs, IDependencySolver.TopologicalSort).
//
// It is written against small interfaces so it does not depend on the
// concrete semver/manifest packages; package engine adapts them.
package solver

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("solver: not implemented")

// Version is an opaque comparable version.
type Version interface {
	// Compare returns -1, 0 or 1 by precedence.
	Compare(other Version) int
	String() string
}

// Constraint is a version range.
type Constraint interface {
	Contains(v Version) bool
	String() string
}

// Requirement is "package id must satisfy constraint" (lip PackageReqt).
type Requirement struct {
	// ID is a package id string ("path" or "path#variant"); ids are compared
	// exactly ("x#client" and "x" are different packages).
	ID         string
	Constraint Constraint
}

func (r Requirement) String() string { return r.ID + "@" + r.Constraint.String() }

// Registry supplies candidates (lip IPackageRegistry + manifest variant deps).
type Registry interface {
	// Versions returns available versions of id, oldest first.
	Versions(ctx context.Context, id string) ([]Version, error)
	// Dependencies returns the requirements of id@v for the current platform
	// (resolved variant dependencies), in manifest order.
	Dependencies(ctx context.Context, id string, v Version) ([]Requirement, error)
}

// Selected is one package in the solution.
type Selected struct {
	ID      string
	Version Version
	// Requirements are the dependencies of this package (used for ordering).
	Requirements []Requirement
}

// Logf, if set, receives the same informational messages lip prints
// (e.g. "Resolving dependencies for: ...").
type Logf func(format string, args ...any)

// Solve finds one version per package satisfying all requirements, exactly
// as lip does: candidate sets per id; backtracking picks the unresolved id
// with the fewest candidates and tries the newest version first; selecting
// a version propagates its dependencies (an already-selected dependency must
// satisfy the range, otherwise the candidate set is intersected; a new
// dependency's candidates are fetched from the registry and filtered). It
// does not prefer installed versions. The result is topologically sorted
// (dependencies first, stable with respect to discovery order); a cycle is
// an error ("Circular dependency detected involving package '<id>'").
func Solve(ctx context.Context, reg Registry, roots []Requirement, logf Logf) ([]Selected, error) {
	return nil, ErrNotImplemented
}

// TopologicalSort orders nodes so dependencies come first, visiting nodes
// in the given order (IDependencySolver.TopologicalSort).
func TopologicalSort(nodes []Selected) ([]Selected, error) {
	return nil, ErrNotImplemented
}
