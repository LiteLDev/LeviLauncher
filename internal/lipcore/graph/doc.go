// Package graph ports lip's GraphSolver
// (src/Lip.Core/Mcmod/Resolution/GraphSolver.cs): it finds the package set S
// of protocol/workspace.md §3 and orders it for installation (§5).
//
// The resolver plugs in through small interfaces mirroring what lip's
// Resolver supplies:
//
//	Resolver.ResolveLazy(ctx, source, range, onDiagnostic) Sequence
//	Candidate  identity, version, variant, commit, archive key, source Ref,
//	           dependencies, ModName (may download)
//	Ref        text, remote repository (Spec), with a git ref (Pinned),
//	           repository (Target), variant
//	Constraint nil (any version), Range (VersionRange) or ExactVersion; the
//	           solver resolves a remote repository with ExactVersion to look up
//	           one version of a member directly
//
// API:
//
//	Solve(ctx, Problem{Requests, Pinned, Resolver}) (*Solution, error)
//	    *ResolutionError when no package set exists
//	InstallOrder(set, dependenciesOf) []Candidate
//	InstallOrderIndices(count, dependenciesOf) []int
//	KeyOf(candidate) PackageKey, Fold(name)
package graph
