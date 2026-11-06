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
	"sync/atomic"

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

	// barSeq numbers the progress bars this Workspace opens.
	barSeq atomic.Int64
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
				// lip's Path.GetRelativePath throws for an empty path, so a dir
				// placement fails on an uncompressed asset (whose only key is
				// ""), and Path.GetFullPath then throws for a path of spaces on
				// Windows.
				if key == "" {
					return nil, errors.New("The value cannot be an empty string. (Parameter 'path')")
				}
				// Keep compatibility with older versions of the manifest where src
				// could be empty.
				if srcFull[i] == "" {
					src := placement.Src
					if src == "" {
						src = "."
					}
					if isEffectivelyEmpty(src) {
						return nil, errPathEmpty
					}
					srcFull[i] = getFullPath(base, src)
				}
				if isEffectivelyEmpty(key) {
					return nil, errPathEmpty
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

// InstallPackage opens every asset and matches its placements, runs
// pre_install, places every asset's matched files (prefetching, creating
// directories, overwriting existing files), runs install and post_install,
// and returns the absolute paths of placed files in placement order (as
// recorded in tooth_lock.json "files"). Scripts are skipped when
// IgnoreScripts. There is no rollback on failure; on an error after
// placement started, files lists what was placed so far (lip records
// nothing in that case).
//
// Unlike lip, which writes wherever a target points, the whole package is
// rejected before any script runs or file is written when a target is not
// contained in Dir (see containment.check); the error wraps ErrUnsafePath.
//
// This changes the order of steps from lip's on purpose: lip runs
// pre_install first and then opens (downloads) and matches each asset in
// turn, while here every asset is opened and matched before pre_install.
// So an asset that fails to open or match (or a match with an empty dest)
// fails the install before pre_install runs, where lip would have run
// pre_install and then failed, leaving its side effects.
func (w *Workspace) InstallPackage(ctx context.Context, variant *manifest.Variant, open SourceOpener) (files []string, err error) {
	// Step 0: Open the assets and check every target.
	plan, err := w.planInstall(ctx, variant, open)
	if err != nil {
		return nil, err
	}

	// Step 1: Run pre-install scripts.
	if err := w.runScripts(ctx, "pre_install", variant.Scripts.PreInstall); err != nil {
		return nil, err
	}

	// Step 2: Place files.
	files, err = w.place(ctx, plan)
	if err != nil {
		return files, err
	}

	// Step 3: Run post-install scripts.
	if err := w.runScripts(ctx, "install", variant.Scripts.Install); err != nil {
		return files, err
	}
	if err := w.runScripts(ctx, "post_install", variant.Scripts.PostInstall); err != nil {
		return files, err
	}

	return files, nil
}

// place prefetches and copies every planned file and returns the full paths
// placed, in order (those placed so far on an error). It reports a
// StagePlace bar counting bytes when the size of every placed entry is known
// (Sizer), files otherwise.
func (w *Workspace) place(ctx context.Context, plan []plannedAsset) ([]string, error) {
	files := []string{}
	nFiles := 0
	for _, a := range plan {
		nFiles += len(a.matches)
	}
	if nFiles == 0 {
		return files, nil
	}

	var bar *progressBar
	var counter *byteCounter
	if totalBytes, ok := planSize(plan); ok {
		bar = w.newBar(ctx, report.StagePlace, report.UnitBytes, "Placing files", totalBytes)
		counter = newByteCounter(bar, totalBytes)
	} else {
		bar = w.newBar(ctx, report.StagePlace, report.UnitFiles, "Placing files", int64(nFiles))
	}
	// The last update counts what was copied, also on an error.
	defer func() {
		if counter != nil {
			bar.doneAt(counter.cur)
		} else {
			bar.doneAt(int64(len(files)))
		}
	}()

	for _, a := range plan {
		if err := a.src.Prefetch(ctx, distinctKeys(a.matches)); err != nil {
			return files, err
		}

		for i, m := range a.matches {
			if err := ctx.Err(); err != nil {
				return files, err
			}
			var count func(int64)
			if counter != nil {
				counter.startFile(m.Dest)
				count = counter.add
			} else {
				bar.update(int64(len(files)), m.Dest)
			}
			if err := copyToFile(a.src, m.Key, a.targets[i], count); err != nil {
				return files, err
			}
			files = append(files, a.targets[i])
		}
	}
	return files, nil
}

// planSize returns the total size of the planned matches (an entry matched
// twice is copied twice) and whether every size is known and the total is
// positive.
func planSize(plan []plannedAsset) (int64, bool) {
	var total int64
	for _, a := range plan {
		sizer, ok := a.src.(Sizer)
		if !ok {
			return 0, false
		}
		for _, m := range a.matches {
			n, ok := sizer.Size(m.Key)
			if !ok || n < 0 {
				return 0, false
			}
			total += n
		}
	}
	return total, total > 0
}

// plannedAsset is an opened asset with at least one match and the full
// path of each match's target.
type plannedAsset struct {
	src     Source
	matches []Target
	targets []string
}

// planInstall opens every asset, matches its placements (first, so lazy
// sources only download the files that are placed) and checks that every
// target is contained in Dir.
func (w *Workspace) planInstall(ctx context.Context, variant *manifest.Variant, open SourceOpener) ([]plannedAsset, error) {
	var plan []plannedAsset
	contained := w.containment()
	for _, asset := range variant.Assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		src, err := open(ctx, asset)
		if err != nil {
			return nil, err
		}
		matches, err := w.MatchPlacements(src.Keys(), asset.Placements)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			continue
		}

		targets := make([]string, len(matches))
		for i, m := range matches {
			if m.Dest == "" {
				// new FileInfo("") throws in lip.
				return nil, errors.New("The value cannot be an empty string. (Parameter 'fileName')")
			}
			targets[i] = w.FullPath(m.Dest)
			if err := contained.check(targets[i], true); err != nil {
				return nil, fmt.Errorf("Refusing to install the package: file '%s' would be placed at '%s' (destination '%s'), but %w", m.Key, targets[i], m.Dest, err)
			}
		}
		plan = append(plan, plannedAsset{src: src, matches: matches, targets: targets})
	}
	return plan, nil
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
// directory, create or truncate the file) and copy. count, when not nil,
// receives the number of bytes written as they are written.
func copyToFile(src Source, key, target string, count func(int64)) error {
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
	var dst io.Writer = f
	if count != nil {
		dst = &countingWriter{w: f, add: count}
	}
	if _, err := io.Copy(dst, r); err != nil {
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
//
// Unlike lip, a recorded file that is not contained in Dir (see
// containment.checkRecorded; the file itself may be a link) is skipped
// with a warning instead of deleted. remove_files cannot reach outside Dir:
// the walk does not descend into links, a matched link is removed itself,
// not its target, and every matched entry passes the same check (or is
// skipped with a warning).
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
	if err := w.runScripts(ctx, "pre_uninstall", variant.Scripts.PreUninstall); err != nil {
		return err
	}
	if err := w.runScripts(ctx, "uninstall", variant.Scripts.Uninstall); err != nil {
		return err
	}

	// Step 2: Remove placed files.
	contained := w.containment()
	if err := w.removeRecorded(ctx, files, preserve, contained, rep); err != nil {
		return err
	}

	// Step 3: Remove the files specified to be removed.
	for i, g := range remove {
		if err := w.removeMatching(ctx, variant.RemoveFiles[i], g, contained, rep); err != nil {
			return err
		}
	}

	// Step 4: Run post-uninstall scripts.
	return w.runScripts(ctx, "post_uninstall", variant.Scripts.PostUninstall)
}

// removeRecorded deletes the recorded files that no preserve glob matches,
// reporting a StageRemove bar over the recorded files (preserved and
// skipped files count as processed).
func (w *Workspace) removeRecorded(ctx context.Context, files []string, preserve []Matcher, contained *containment, rep report.Reporter) error {
	if len(files) == 0 {
		return nil
	}
	bar := w.newBar(ctx, report.StageRemove, report.UnitFiles, "Removing files", int64(len(files)))
	processed := 0
	defer func() { bar.doneAt(int64(processed)) }()

	for i, file := range files {
		processed = i
		if err := ctx.Err(); err != nil {
			return err
		}
		fullName := getFullPath(w.Dir, file)
		relativePath := w.relativePath(fullName)
		bar.update(int64(i), relativePath)
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

		// Unlike lip, never delete outside Dir, whatever the lock says.
		target, err := contained.checkRecorded(fullName)
		if err != nil {
			rep.Warning(fmt.Sprintf("File '%s' is not in the workspace (%v), skipping removal.", fullName, err))
			continue
		}

		if !fileExists(target) {
			rep.Warning(fmt.Sprintf("File '%s' does not exist, skipping removal.", fullName))
		}

		if err := deleteFile(target); err != nil {
			return err
		}
	}
	processed = len(files)
	return nil
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

// removeMatching deletes every entry under Dir matched by g (compiled from
// pattern) that contained accepts. It reports a StageRemove bar that is
// indeterminate during the walk, then counts the matched entries processed.
func (w *Workspace) removeMatching(ctx context.Context, pattern string, g Matcher, contained *containment, rep report.Reporter) error {
	type entry struct {
		path  string
		depth int
	}
	var toRemove []entry

	bar := w.newBar(ctx, report.StageRemove, report.UnitFiles, fmt.Sprintf("Removing files matching '%s'", pattern), 0)
	processed := 0
	defer func() { bar.doneAt(int64(processed)) }()

	root := getFullPath(w.Dir, ".")
	rootPrefix := strings.TrimRight(toSlash(root), "/") + "/"
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Match the glob against the path relative to the working directory,
		// so a pattern like `extra.log` only matches `./extra.log` instead of
		// any file with the same name in any subdirectory. WalkDir yields
		// clean paths under root, so no further normalization is needed.
		if p == root {
			return nil
		}
		relativePath, _ := strings.CutPrefix(toSlash(p), rootPrefix)
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
	bar.setTotal(int64(len(toRemove)))
	for i, e := range toRemove {
		processed = i
		if err := ctx.Err(); err != nil {
			return err
		}
		bar.update(int64(i), w.relativePath(e.path))
		if err := contained.check(e.path, false); err != nil {
			rep.Warning(fmt.Sprintf("File '%s' is not in the workspace (%v), skipping removal.", e.path, err))
			continue
		}
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
	processed = len(toRemove)
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

// runScripts runs the commands of one script hook (e.g. "pre_install"),
// reporting a StageScripts bar over the commands finished.
func (w *Workspace) runScripts(ctx context.Context, hook string, scripts []string) error {
	if w.IgnoreScripts || len(scripts) == 0 {
		return nil
	}
	run := w.RunCommand
	if run == nil {
		run = DefaultCommandRunner
	}
	rep := report.OrNop(w.Reporter)
	bar := w.newBar(ctx, report.StageScripts, report.UnitCommands, fmt.Sprintf("Running %s scripts", hook), int64(len(scripts)))
	finished := 0
	defer func() { bar.doneAt(int64(finished)) }()
	for i, script := range scripts {
		bar.update(int64(i), script)
		stdout := &lineWriter{emit: rep.Info}
		stderr := &lineWriter{emit: rep.Error}
		err := run(ctx, w.Dir, script, stdout, stderr)
		stdout.Flush()
		stderr.Flush()
		if err != nil {
			return err
		}
		finished = i + 1
	}
	return nil
}
