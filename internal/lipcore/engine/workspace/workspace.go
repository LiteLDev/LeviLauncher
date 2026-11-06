// Package workspace installs and uninstalls a single package's files and
// scripts inside a workspace (instance) directory, porting lip 0.37.0
// src/Lip.Core/Services/PackageInstaller.cs, PlacementMatcher.cs and
// Infrastructure/CommandRunner.cs. It does not touch tooth_lock.json;
// package engine owns the lock.
package workspace

import (
	"context"
	"errors"
	"io"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("workspace: not implemented")

// Matcher is a compiled glob (glob.Glob in production).
type Matcher interface {
	Match(s string) bool
}

// GlobCompiler compiles a DotNet.Glob pattern.
type GlobCompiler func(pattern string) (Matcher, error)

// Source exposes one asset's files (lip ISource).
type Source interface {
	Keys() []string
	Open(key string) (io.ReadCloser, error)
	// Prefetch hints that keys are about to be read (lazy git tree sources
	// download blobs in a batch). May be a no-op.
	Prefetch(ctx context.Context, keys []string) error
}

// SourceOpener opens the source of one asset of the package being
// installed (downloading as needed).
type SourceOpener func(ctx context.Context, asset manifest.Asset) (Source, error)

// CommandRunner runs one script line in dir ("cmd.exe /c <cmd>" on Windows,
// "sh -c <cmd>" elsewhere; stdin null; stdout and stderr to the writers);
// a non-zero exit is an error.
type CommandRunner func(ctx context.Context, dir, command string, stdout, stderr io.Writer) error

// Workspace performs per-package operations in Dir.
type Workspace struct {
	// Dir is the absolute workspace directory (lip's cwd).
	Dir           string
	Reporter      report.Reporter
	Glob          GlobCompiler
	IgnoreScripts bool
	// RunCommand defaults to DefaultCommandRunner when nil.
	RunCommand CommandRunner
}

// Target is one file to place: source key -> path relative to Dir.
type Target struct {
	Key  string
	Dest string
}

// MatchPlacements implements PlacementMatcher.Match for one asset: for each
// key and each placement (a key may match several placements):
//   - file: src == key -> dest; else if glob(src) matches key ->
//     dest/<basename(key)>;
//   - dir: rel = relative path from src ("" means ".") to key
//     (case-insensitive on Windows); if rel does not start with ".." ->
//     dest/rel.
func MatchPlacements(keys []string, placements []manifest.Placement, compile GlobCompiler) ([]Target, error) {
	return nil, ErrNotImplemented
}

// InstallPackage runs pre_install, places every asset's matched files
// (prefetching, creating directories, overwriting existing files), runs
// install and post_install, and returns the absolute paths of placed files
// in placement order (as recorded in tooth_lock.json "files"). Scripts are
// skipped when IgnoreScripts. There is no rollback on failure.
func (w *Workspace) InstallPackage(ctx context.Context, variant *manifest.Variant, open SourceOpener) (files []string, err error) {
	return nil, ErrNotImplemented
}

// UninstallPackage runs pre_uninstall and uninstall; deletes each recorded
// file unless a preserve_files glob matches its Dir-relative path (with "/")
// or its base name (warning when a file is already missing; empty
// directories are kept); then for each remove_files glob deletes every
// entry under Dir whose relative path (or, for directories, path + "/")
// matches, deepest first, directories recursively; then runs
// post_uninstall.
func (w *Workspace) UninstallPackage(ctx context.Context, variant *manifest.Variant, files []string) error {
	return ErrNotImplemented
}

// DefaultCommandRunner runs commands like lip's CommandRunner.
func DefaultCommandRunner(ctx context.Context, dir, command string, stdout, stderr io.Writer) error {
	return ErrNotImplemented
}
