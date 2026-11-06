// Package workspace installs and uninstalls a single package's files and
// scripts inside a workspace (instance) directory, porting lip 0.37.0
// src/Lip.Core/Services/PackageInstaller.cs, PlacementMatcher.cs and
// Infrastructure/CommandRunner.cs. It does not touch tooth_lock.json;
// package engine owns the lock.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	Key string
	// Dest is the target path as lip's PlacementMatcher produces it
	// (Path.Combine of the placement dest and the key's name or relative
	// path, using the OS separator). It is relative to Dir; FullPath turns
	// it into the absolute path recorded in tooth_lock.json.
	Dest string
}

// MatchPlacements implements PlacementMatcher.Match for one asset: for each
// key and each placement (a key may match several placements):
//   - file: src == key -> dest; else if glob(src) matches key ->
//     dest/<basename(key)>;
//   - dir: rel = relative path from src ("" means ".") to key
//     (case-insensitive on Windows); if rel does not start with ".." ->
//     dest/rel.
//
// Results are ordered by key first and placement second. Globs are compiled
// lazily (only when an exact file match fails), as lip does.
//
// Directory placements resolve src and key against the process working
// directory (Path.GetRelativePath); use (*Workspace).MatchPlacements to
// resolve them against the workspace directory instead.
func MatchPlacements(keys []string, placements []manifest.Placement, compile GlobCompiler) ([]Target, error) {
	base, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return matchPlacements(base, keys, placements, compile)
}

// MatchPlacements is MatchPlacements with directory placements resolved
// against w.Dir (lip's working directory).
func (w *Workspace) MatchPlacements(keys []string, placements []manifest.Placement) ([]Target, error) {
	return matchPlacements(w.Dir, keys, placements, w.Glob)
}

func matchPlacements(base string, keys []string, placements []manifest.Placement, compile GlobCompiler) ([]Target, error) {
	globs := make([]Matcher, len(placements))
	srcFull := make([]string, len(placements))
	var matches []Target

	for _, key := range keys {
		for i, placement := range placements {
			switch placement.Type {
			case manifest.PlacementFile:
				if placement.Src == key {
					matches = append(matches, Target{Key: key, Dest: placement.Dest})
					continue
				}
				if globs[i] == nil {
					if compile == nil {
						return nil, errors.New("workspace: no glob compiler configured")
					}
					g, err := compile(placement.Src)
					if err != nil {
						return nil, err
					}
					globs[i] = g
				}
				if globs[i].Match(key) {
					matches = append(matches, Target{Key: key, Dest: pathCombine(placement.Dest, pathGetFileName(key))})
				}

			case manifest.PlacementDir:
				// Keep compatibility with older versions of the manifest where src
				// could be empty.
				if srcFull[i] == "" {
					src := placement.Src
					if src == "" {
						src = "."
					}
					srcFull[i] = getFullPath(base, src)
				}
				if key == "" {
					// Path.GetRelativePath throws for an empty path in lip, so a
					// dir placement fails on an uncompressed asset (whose only
					// key is "").
					return nil, errors.New("The value cannot be an empty string. (Parameter 'path')")
				}
				rel := relativePathBetween(srcFull[i], getFullPath(base, key))
				if !strings.HasPrefix(rel, "..") {
					matches = append(matches, Target{Key: key, Dest: pathCombine(placement.Dest, rel)})
				}

			default:
				return nil, fmt.Errorf("workspace: unknown placement type %q", placement.Type)
			}
		}
	}

	return matches, nil
}

// FullPath returns the absolute path lip records for a path relative to the
// workspace (FileInfo.FullName with the workspace as working directory):
// OS separators, "." and ".." collapsed, on Windows trailing dots/spaces of
// segments removed.
func (w *Workspace) FullPath(p string) string {
	return getFullPath(w.Dir, p)
}

// InstallPackage runs pre_install, places every asset's matched files
// (prefetching, creating directories, overwriting existing files), runs
// install and post_install, and returns the absolute paths of placed files
// in placement order (as recorded in tooth_lock.json "files"). Scripts are
// skipped when IgnoreScripts. There is no rollback on failure; on an error
// after placement started, files lists what was placed so far (lip records
// nothing in that case).
func (w *Workspace) InstallPackage(ctx context.Context, variant *manifest.Variant, open SourceOpener) (files []string, err error) {
	// Step 1: Run pre-install scripts.
	if err := w.runScripts(ctx, variant.Scripts.PreInstall); err != nil {
		return nil, err
	}

	// Step 2: Place files.
	files = []string{}
	for _, asset := range variant.Assets {
		src, err := open(ctx, asset)
		if err != nil {
			return files, err
		}

		// Match placements first, so lazy sources only download the files that
		// are placed.
		matches, err := w.MatchPlacements(src.Keys(), asset.Placements)
		if err != nil {
			return files, err
		}
		if len(matches) == 0 {
			continue
		}

		if err := src.Prefetch(ctx, distinctKeys(matches)); err != nil {
			return files, err
		}

		for _, m := range matches {
			if err := ctx.Err(); err != nil {
				return files, err
			}
			if m.Dest == "" {
				// new FileInfo("") throws in lip.
				return files, errors.New("The value cannot be an empty string. (Parameter 'fileName')")
			}
			target := w.FullPath(m.Dest)
			if err := copyToFile(src, m.Key, target); err != nil {
				return files, err
			}
			files = append(files, target)
		}
	}

	// Step 3: Run post-install scripts.
	if err := w.runScripts(ctx, variant.Scripts.Install); err != nil {
		return files, err
	}
	if err := w.runScripts(ctx, variant.Scripts.PostInstall); err != nil {
		return files, err
	}

	return files, nil
}

func distinctKeys(matches []Target) []string {
	seen := make(map[string]struct{}, len(matches))
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m.Key]; ok {
			continue
		}
		seen[m.Key] = struct{}{}
		keys = append(keys, m.Key)
	}
	return keys
}

// copyToFile ports the body of lip's placement loop: open the source entry,
// then FileSystemExtensions.CreateFileWithDirectory (create the parent
// directory, create or truncate the file) and copy.
func copyToFile(src Source, key, target string) error {
	r, err := src.Open(key)
	if err != nil {
		return err
	}
	defer r.Close()

	if dir := filepath.Dir(target); dir != "" {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return err
		}
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// UninstallPackage runs pre_uninstall and uninstall; deletes each recorded
// file unless a preserve_files glob matches its Dir-relative path (with "/")
// or its base name (warning when a file is already missing; empty
// directories are kept); then for each remove_files glob deletes every
// entry under Dir whose relative path (or, for directories, path + "/")
// matches, deepest first, directories recursively; then runs
// post_uninstall.
func (w *Workspace) UninstallPackage(ctx context.Context, variant *manifest.Variant, files []string) error {
	rep := report.OrNop(w.Reporter)

	// lip compiles these globs when it loads the installed manifest, before
	// running any script.
	preserve, err := w.compileAll(variant.PreserveFiles)
	if err != nil {
		return err
	}
	remove, err := w.compileAll(variant.RemoveFiles)
	if err != nil {
		return err
	}

	// Step 1: Run pre-uninstall scripts.
	if err := w.runScripts(ctx, variant.Scripts.PreUninstall); err != nil {
		return err
	}
	if err := w.runScripts(ctx, variant.Scripts.Uninstall); err != nil {
		return err
	}

	// Step 2: Remove placed files.
	for _, file := range files {
		fullName := getFullPath(w.Dir, file)
		relativePath := w.relativePath(fullName)
		name := pathGetFileName(file)

		// Also match the bare file name so that older manifests relying on
		// name-only matching keep preserving the same files.
		preserved := false
		for _, g := range preserve {
			if g.Match(relativePath) || g.Match(name) {
				preserved = true
				break
			}
		}
		if preserved {
			continue
		}

		if !fileExists(fullName) {
			rep.Warning(fmt.Sprintf("File '%s' does not exist, skipping removal.", fullName))
		}

		if err := deleteFile(fullName); err != nil {
			return err
		}
	}

	// Step 3: Remove the files specified to be removed.
	for _, g := range remove {
		if err := w.removeMatching(g); err != nil {
			return err
		}
	}

	// Step 4: Run post-uninstall scripts.
	return w.runScripts(ctx, variant.Scripts.PostUninstall)
}

func (w *Workspace) compileAll(patterns []string) ([]Matcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	if w.Glob == nil {
		return nil, errors.New("workspace: no glob compiler configured")
	}
	out := make([]Matcher, 0, len(patterns))
	for _, p := range patterns {
		g, err := w.Glob(p)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// removeMatching deletes every entry under Dir matched by g.
func (w *Workspace) removeMatching(g Matcher) error {
	type entry struct {
		path  string
		depth int
	}
	var toRemove []entry

	root := getFullPath(w.Dir, ".")
	rootPrefix := strings.TrimRight(toSlash(root), "/") + "/"
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Match the glob against the path relative to the working directory,
		// so a pattern like `extra.log` only matches `./extra.log` instead of
		// any file with the same name in any subdirectory. WalkDir yields
		// clean paths under root, so no further normalization is needed.
		if p == root {
			return nil
		}
		relativePath, _ := cutPrefixFoldOrdinal(toSlash(p), rootPrefix)
		if relativePath == "" {
			return nil
		}
		// A trailing separator is required to match a directory, so also
		// match patterns like `temp/` against directories.
		if g.Match(relativePath) || (dirExists(p) && g.Match(relativePath+"/")) {
			toRemove = append(toRemove, entry{p, strings.Count(relativePath, "/")})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Delete deeper paths first so that removing a directory recursively does
	// not invalidate the other paths collected above.
	sort.SliceStable(toRemove, func(i, j int) bool { return toRemove[i].depth > toRemove[j].depth })
	for _, e := range toRemove {
		if fileExists(e.path) {
			if err := deleteFile(e.path); err != nil {
				return err
			}
		} else if dirExists(e.path) {
			if err := os.RemoveAll(e.path); err != nil {
				return err
			}
		}
	}
	return nil
}

// relativePath ports PackageInstaller.GetWorkingDirectoryRelativePath: the
// path relative to Dir with "/" separators, "" for Dir itself, or the whole
// normalized path when it is outside Dir. The prefix test ignores case.
func (w *Workspace) relativePath(p string) string {
	rootPath := strings.TrimRight(toSlash(getFullPath(w.Dir, ".")), "/")
	fullPath := toSlash(getFullPath(w.Dir, p))

	if fullPath == rootPath {
		return ""
	}
	if rest, ok := cutPrefixFoldOrdinal(fullPath, rootPath+"/"); ok {
		return rest
	}
	return fullPath
}

// toSlash replaces the OS directory separator with "/" (lip replaces both
// DirectorySeparatorChar and AltDirectorySeparatorChar).
func toSlash(s string) string {
	if filepath.Separator != '/' {
		s = strings.ReplaceAll(s, string(filepath.Separator), "/")
	}
	return s
}

func (w *Workspace) runScripts(ctx context.Context, scripts []string) error {
	if w.IgnoreScripts {
		return nil
	}
	run := w.RunCommand
	if run == nil {
		run = DefaultCommandRunner
	}
	rep := report.OrNop(w.Reporter)
	for _, script := range scripts {
		stdout := &lineWriter{emit: rep.Info}
		stderr := &lineWriter{emit: rep.Error}
		err := run(ctx, w.Dir, script, stdout, stderr)
		stdout.Flush()
		stderr.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}
