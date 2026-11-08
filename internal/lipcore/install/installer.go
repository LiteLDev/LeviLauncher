package install

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pathglob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// Installer is McmodInstaller.cs: installs and uninstalls packages
// (protocol/workspace.md §4).
//
// Installing writes the files of the assets of the entry, in order (step 2):
// each asset is mounted at its index; "self" and "zip" assets carry out
// their placements, "mcmod" assets are written whole into the mod directory
// mods/<mod name>/, manifest.json included, as it is.
//
// Ownership: a file is owned by a package when the package wrote it. A file
// that exists when it would be written and matches preserve is kept and not
// written, so it is not owned by the package; whether it exists is decided
// then, after the hook commands that ran before it.
//
// The write rules are checked twice: when planning, against the workspace as
// the earlier changes of the operation are planned to leave it, so that a
// failure changes nothing (protocol/workspace.md §6 rule 1); and at each
// write, against the workspace and the owners as they are then (FileOwners),
// which decides.
//
// The mods directory is never deleted, and purge never matches it (so a
// directory pattern naming it removes nothing). .lip, where lip keeps the
// state file, is never written, deleted or matched. On Windows, these checks
// and ownership see a path by its long names (longName), so that an 8.3
// short name such as "LIP~1" cannot name .lip or a file of another package.
// Elsewhere, on a file system that ignores case (as macOS does by default),
// they see each name that exists as the file system spells it, so that
// ".LIP" or "MODS" cannot name .lip or mods, nor "Mods/a.dll" a file
// "mods/a.dll" of another package.
//
// An Installer may be used for several operations, one at a time.
type Installer struct {
	// Reporter receives the dry-run plan (Info), the output of hook commands
	// (Info, line by line) and measured progress. nil discards them.
	Reporter report.Reporter
	// RunCommand runs hook commands; nil is DefaultCommandRunner.
	RunCommand CommandRunner

	barSeq atomic.Int64
	// caseInsensitive caches whether the file system of each workspace root
	// ignores case (ignoresCase), once known.
	caseInsensitive sync.Map
}

// Options is what InstallContext carries besides the environment.
type Options struct {
	// Root is the workspace root.
	Root string
	// Installed are the packages of the state file as the changes
	// planned before this one leave it (InstallContext.Lock.Packages): their
	// files are owned by them.
	Installed []pkgmodel.LockedPackage
	// DryRun reports the plan and fetches no content.
	DryRun bool
	// IgnoreScripts drops the hooks of the package.
	IgnoreScripts bool
}

// Package is the candidate to install (Contracts.cs Candidate): a package
// with its selected entry, the archive it is in, and how to obtain its other
// assets.
type Package struct {
	ID      string
	Variant string
	Version string
	// Entry is the selected entry (protocol/package.md §4 step 2).
	Entry *pkgmodel.Entry
	// Archive is the archive of the package, the asset of type "self".
	Archive archive.Source
	// OpenAsset obtains an asset of type "zip" or "mcmod" from the first of
	// its URLs that can be obtained (IArchiveProvider.OpenAsset). It fails
	// with *archive.UnavailableError when none can. Like lip's AssetHandle it
	// should open each asset once (the mod name may have been read from the
	// mcmod asset already).
	OpenAsset func(urls []string) (archive.Source, error)
	// ModName is the mod name of a mod (Candidate.GetModManifest), as the
	// solver read it; nil reads it from the opened mcmod asset (ReadModName).
	ModName func(ctx context.Context) (string, error)
}

// Identity is Package.Identity.
func (p *Package) Identity() string { return ref.Identity(p.ID, p.Variant) }

// Prefetcher is implemented by sources that fetch the content of files in
// batches (ISource.Prefetch; a git tree source). Other sources read files
// when they are opened.
type Prefetcher interface {
	Prefetch(ctx context.Context, keys []string) error
}

// LockedIdentity is LockedPackage.Identity: the "id" of the recorded
// package file, then '#' and the variant when that is not "".
func LockedIdentity(p *pkgmodel.LockedPackage) string {
	return ref.Identity(stringMember(p.Manifest, "id"), p.Variant)
}

// LockedVersion is LockedPackage.Version: the "version" of the recorded
// package file.
func LockedVersion(p *pkgmodel.LockedPackage) string { return stringMember(p.Manifest, "version") }

func stringMember(manifest any, name string) string {
	if obj, ok := manifest.(*doc.Object); ok {
		if v, ok := obj.Get(name); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// plannedWrite is McmodInstaller.PlannedWrite.
type plannedWrite struct {
	target   string
	source   int // the asset
	key      string
	fullPath string
	keep     bool
}

// mounted is the assets of a package mounted at their indexes
// (MountSource.cs, archive.Mount).
type mounted struct {
	sources []archive.Source
	mount   archive.Source
}

func (m *mounted) resolve(key string) (int, string) {
	i := strings.IndexByte(key, '/')
	n, _ := strconv.Atoi(key[:i])
	return n, key[i+1:]
}

// prefetch is MountSource.Prefetch: the keys grouped by asset, in order of
// first appearance.
func (m *mounted) prefetch(ctx context.Context, keys []string) error {
	var order []int
	groups := map[int][]string{}
	for _, k := range keys {
		i, inner := m.resolve(k)
		if _, ok := groups[i]; !ok {
			order = append(order, i)
		}
		groups[i] = append(groups[i], inner)
	}
	for _, i := range order {
		src := m.sources[i]
		if d, ok := src.(interface {
			Load() (archive.Source, error)
		}); ok {
			loaded, err := d.Load()
			if err != nil {
				return err
			}
			src = loaded
		}
		if p, ok := src.(Prefetcher); ok {
			if err := p.Prefetch(ctx, groups[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func label(identity, version string) string { return identity + " " + version }

// describeAsset is McmodInstaller.DescribeAsset: an asset of an entry for
// messages.
func describeAsset(entry *pkgmodel.Entry, i int) string {
	if i >= len(entry.Assets) {
		return "unknown"
	}
	asset := entry.Assets[i]
	if asset.Type == pkgmodel.AssetSelf || len(asset.URLs) == 0 {
		return fmt.Sprintf("type \"%s\", the archive of the package itself", asset.Type)
	}
	return fmt.Sprintf("type \"%s\", from %s", asset.Type, strings.Join(asset.URLs, " or "))
}

func compileAll(patterns []string) ([]*pathglob.Glob, error) {
	globs := make([]*pathglob.Glob, 0, len(patterns))
	for _, p := range patterns {
		g, err := pathglob.Compile(p)
		if err != nil {
			return nil, err
		}
		globs = append(globs, g)
	}
	return globs, nil
}

func matchesAny(globs []*pathglob.Glob, path string) bool {
	for _, g := range globs {
		if g.IsMatch(path) {
			return true
		}
	}
	return false
}

func (in *Installer) info(msg string) { report.OrNop(in.Reporter).Info(msg) }

// PlanInstall is McmodInstaller.PlanInstall: plans installing the package
// into the workspace as the view shows it, and adds the planned writes to
// the view. It obtains every asset first, and, unless DryRun, fetches the
// content of the files to write, so that a failure changes nothing; a dry
// run reports the plan instead. It fails with *Error when the package
// cannot be installed, e.g. when a file is owned by another installed
// package.
func (in *Installer) PlanInstall(ctx context.Context, view *View, opts Options, pkg *Package) (*Plan, error) {
	entry := pkg.Entry
	name := pkg.Identity()
	lbl := label(name, pkg.Version)
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	// Every asset is obtained before anything is written or run.
	m := &mounted{}
	for i, asset := range entry.Assets {
		var src archive.Source
		if asset.Type == pkgmodel.AssetSelf {
			src = pkg.Archive
		} else {
			src, err = pkg.OpenAsset(asset.URLs)
		}
		if err != nil {
			var unavailable *archive.UnavailableError
			if errors.As(err, &unavailable) {
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: lip could not obtain asset %d"+
					" (%s) of the package, so nothing was changed.", lbl, i, describeAsset(entry, i)), Err: err}
			}
			return nil, err
		}
		m.sources = append(m.sources, src)
	}
	m.mount = archive.Mount(m.sources...)

	plan, err := in.plan(ctx, pkg, m, root, opts, view)
	if err != nil {
		return nil, err
	}
	for _, w := range plan {
		view.AddFile(w.fullPath)
	}
	// Whether a file matching preserve is kept is decided when it is
	// written, after the hook commands before it (protocol/workspace.md §4);
	// the plan expects the workspace as it is.
	preserve, err := compileAll(entry.Preserve)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		for _, w := range plan {
			if w.keep {
				in.info(fmt.Sprintf("Would keep %s (preserved)", w.target))
			} else {
				in.info(fmt.Sprintf("Would write %s", w.target))
			}
		}
	} else {
		// Fetch content now, so that a fetch failure changes nothing. A file
		// expected to be kept is fetched too: it is written when a hook
		// command deleted it.
		var keys []string
		seen := map[string]bool{}
		for _, w := range plan {
			k := strconv.Itoa(w.source) + "/" + w.key
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		if err := m.prefetch(ctx, keys); err != nil {
			if isCancel(err) {
				return nil, err
			}
			return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: lip could not get the files of the"+
				" package, so nothing was changed.", lbl), Err: err}
		}
	}

	// pre_install, the writes, post_install.
	var steps []Step
	if !opts.IgnoreScripts && len(entry.Hooks.PreInstall) > 0 {
		steps = append(steps, &ScriptStep{Stage: "pre_install", Commands: entry.Hooks.PreInstall})
	}
	owned := []string{}
	for _, w := range plan {
		steps = append(steps, &WriteStep{Target: w.target, FullPath: w.fullPath,
			Source: m.mount, Key: strconv.Itoa(w.source) + "/" + w.key, Preserve: preserve,
			PlannedKeep: w.keep})
		if !w.keep {
			owned = append(owned, w.target)
		}
	}
	if !opts.IgnoreScripts && len(entry.Hooks.PostInstall) > 0 {
		steps = append(steps, &ScriptStep{Stage: "post_install", Commands: entry.Hooks.PostInstall})
	}

	return &Plan{Name: name, Version: pkg.Version, Root: root, OwnedFiles: owned, Steps: steps}, nil
}

// plan is McmodInstaller.Plan: the writes of step 2 of installing a package
// (protocol/workspace.md §4).
func (in *Installer) plan(ctx context.Context, pkg *Package, m *mounted, root string,
	opts Options, view *View) ([]plannedWrite, error) {
	entry := pkg.Entry
	lbl := label(pkg.Identity(), pkg.Version)

	modDirectory := ""
	if mcmod := entry.McmodAsset(); mcmod >= 0 {
		var modName string
		var err error
		if pkg.ModName != nil {
			modName, err = pkg.ModName(ctx)
		} else {
			modName, err = ReadModName(m.sources[mcmod], lbl)
		}
		if err != nil {
			return nil, err
		}
		modDirectory = pkgmodel.ModsDir + "/" + modName + "/"
	}

	// Target (workspace-relative) -> asset and key. Later writes of the same
	// target win.
	type source struct {
		target string
		asset  int
		key    string
	}
	var order []string           // pathKeys, first seen first
	first := map[string]string{} // pathKey -> the target as first spelled
	targets := map[string]source{}
	add := func(target string, asset int, key string) error {
		// "a/./b" is "a/b". Windows drops trailing dots and spaces, which
		// would alias another path, e.g. one in .lip or owned by another
		// package, and takes ':' for an alternate data stream.
		var segments []string
		if isSafeRelativePath(target) {
			for _, s := range strings.Split(target, "/") {
				if s != "." {
					segments = append(segments, s)
				}
			}
		}
		if len(segments) == 0 {
			return &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places a file at '%s',"+
				" which is not a valid path inside the workspace (a relative path with '/' separators"+
				" and without '..'). Report this to the author of the package.", lbl, target)}
		}
		if aliasesOnWindows(strings.Join(segments, "/")) {
			return &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places a file at"+
				" '%s', which is not a valid path on Windows (a name in it ends with '.' or a"+
				" space, or contains ':'). Report this to the author of the package.", lbl, target)}
		}
		long, ok := in.longName(root, strings.Join(segments, "/"))
		if !ok {
			return &Error{Msg: fmt.Sprintf("Cannot install %s: lip cannot tell which file '%s' names, as"+
				" it cannot read the long name of a short name in it.", lbl, target)}
		}
		target = long
		if IsLipPath(target) {
			return &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places a file at '%s',"+
				" inside %s, which is reserved for lip. Report this to the author of the package.",
				lbl, target, pkgmodel.LipDir)}
		}
		k := pathKey(target)
		if _, ok := targets[k]; !ok {
			order = append(order, k)
			first[k] = target
		}
		targets[k] = source{target, asset, key}
		return nil
	}

	// Step 2.
	keys, err := archive.ListKeys(m.mount)
	if err != nil {
		return nil, err
	}
	for i, asset := range entry.Assets {
		prefix := strconv.Itoa(i) + "/"
		var files []assetFile
		for _, k := range keys {
			if strings.HasPrefix(k, prefix) {
				files = append(files, assetFile{path: k[len(prefix):]})
			}
		}

		// The package file of the package is never written
		// (protocol/workspace.md §4 step 2): its lip.toml, or, for an archive
		// without one, the tooth.json it is read from.
		if asset.Type == pkgmodel.AssetSelf {
			packageFile := archive.ToothFileName
			for _, f := range files {
				if f.path == archive.PackageFileName {
					packageFile = archive.PackageFileName
					break
				}
			}
			kept := files[:0]
			for _, f := range files {
				if f.path != packageFile {
					kept = append(kept, f)
				}
			}
			files = kept
		}

		if asset.Type == pkgmodel.AssetMcmod {
			for _, f := range files {
				if err := add(modDirectory+f.path, i, f.path); err != nil {
					return nil, err
				}
			}
			continue
		}

		for _, placement := range asset.Place {
			if err := planPlacement(lbl, i, placement, files, func(target, key string) error {
				return add(target, i, key)
			}); err != nil {
				return nil, err
			}
		}
	}

	owners := map[string]string{}
	identity := pkg.Identity()
	for idx := range opts.Installed {
		locked := &opts.Installed[idx]
		lockedIdentity := LockedIdentity(locked)
		if ref.NamesEqual(lockedIdentity, identity) {
			continue
		}
		for _, recorded := range locked.Files {
			file := normalizeRelative(recorded)
			if long, ok := in.longName(root, file); ok {
				file = long
			}
			k := pathKey(file)
			if _, ok := owners[k]; !ok {
				owners[k] = lockedIdentity
			}
		}
	}

	var conflicts []string
	for _, k := range order {
		if owner, ok := owners[k]; ok {
			conflicts = append(conflicts, fmt.Sprintf("%s (owned by %s)", first[k], owner))
		}
	}
	if len(conflicts) > 0 {
		return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: it would overwrite files that belong"+
			" to other installed packages: %s. Uninstall those packages first, or ask the authors"+
			" of the packages to resolve the conflict.", lbl, strings.Join(conflicts, ", "))}
	}

	preserve, err := compileAll(entry.Preserve)
	if err != nil {
		return nil, err
	}
	var plan []plannedWrite
	checkedDirectories := map[string]bool{}
	for _, k := range order {
		s := targets[k]
		full := fullPath(root, s.target)
		if !isInside(root, full) {
			return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places a file at"+
				" '%s', which is outside the workspace. Report this to the author of the package.",
				lbl, s.target)}
		}
		if view.DirectoryExists(full) {
			return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: it writes the file '%s', but a"+
				" directory of that name is in the way. Move or delete that directory, then try"+
				" again.", lbl, s.target)}
		}
		for parent, ok := parentDir(full); ok && isInside(root, parent) &&
			!checkedDirectories[pathKey(parent)]; parent, ok = parentDir(parent) {
			checkedDirectories[pathKey(parent)] = true
			if view.FileExists(parent) {
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: it writes the file '%s', but"+
					" '%s' is a file, not a directory. Move or delete that file, then try again.",
					lbl, s.target, show(root, parent))}
			}
		}

		keep := view.FileExists(full) && matchesAny(preserve, s.target)
		plan = append(plan, plannedWrite{target: s.target, source: s.asset, key: s.key,
			fullPath: full, keep: keep})
	}
	return plan, nil
}

// assetFile is a file of an asset: its path in the asset.
type assetFile struct{ path string }

// planPlacement is McmodInstaller.PlanPlacement: the writes of one placement
// of a "self" or "zip" asset (protocol/workspace.md §4 step 2).
func planPlacement(lbl string, asset int, placement pkgmodel.Placement, files []assetFile,
	add func(target, key string) error) error {
	switch placement.Form() {
	case pkgmodel.PlacementFile:
		for _, f := range files {
			if f.path == placement.Src {
				return add(placement.Dest, f.path)
			}
		}
		return &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places the file '%s' of"+
			" asset %d, but the asset has no such file. Report this to the author of the package.",
			lbl, placement.Src, asset)}

	case pkgmodel.PlacementDirectory:
		matched := 0
		for _, f := range files {
			if strings.HasPrefix(f.path, placement.Src) {
				matched++
				if err := add(placement.Dest+f.path[len(placement.Src):], f.path); err != nil {
					return err
				}
			}
		}
		if matched == 0 {
			return &Error{Msg: fmt.Sprintf("Cannot install %s: its package file places the directory"+
				" '%s' of asset %d, but the asset has no file below it. Report this to the author of"+
				" the package.", lbl, placement.Src, asset)}
		}

	case pkgmodel.PlacementGlob:
		glob, err := pathglob.Compile(placement.Src)
		if err != nil {
			return err
		}
		fixedPrefix := pathglob.FixedPrefix(placement.Src)
		for _, f := range files {
			if glob.IsMatch(f.path) {
				if err := add(placement.Dest+f.path[len(fixedPrefix):], f.path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// PlanUninstall is McmodInstaller.PlanUninstall: plans uninstalling the
// installed package, whose entry (McmodInstaller.LockedEntry: the entry of
// its variant in its recorded package file for the environment) the caller
// has selected, from the workspace as the view shows it, and adds the
// planned deletions to the view. A dry run reports the plan.
func (in *Installer) PlanUninstall(ctx context.Context, view *View, opts Options,
	pkg *pkgmodel.LockedPackage, entry *pkgmodel.Entry) (*Plan, error) {
	identity := LockedIdentity(pkg)
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	preserve, err := compileAll(entry.Preserve)
	if err != nil {
		return nil, err
	}
	purge, err := compileAll(entry.Purge)
	if err != nil {
		return nil, err
	}

	// The paths to delete are planned against the workspace before
	// pre_uninstall runs; a DeleteMatchesStep deletes what matches only after
	// hook commands ran. Each deletion is checked again against the owners
	// when it is carried out (Apply).
	ownedByOthers := map[string]bool{}
	for idx := range opts.Installed {
		other := &opts.Installed[idx]
		if ref.NamesEqual(LockedIdentity(other), identity) {
			continue
		}
		for _, f := range other.Files {
			file := normalizeRelative(f)
			if long, ok := in.longName(root, file); ok {
				file = long
			}
			ownedByOthers[pathKey(file)] = true
		}
	}

	// 2. Owned files not matching preserve.
	var files []string
	fileSet := map[string]bool{}
	ownedToDelete := []string{}
	for _, recorded := range pkg.Files {
		// A path whose long name cannot be read is not deleted: it might name
		// .lip.
		file, ok := in.longName(root, normalizeRelative(recorded))
		if !ok {
			file = ""
		}
		if !isSafeRelativePath(file) || aliasesOnWindows(file) || IsLipPath(file) ||
			ownedByOthers[pathKey(file)] {
			continue
		}
		if matchesAny(preserve, file) {
			continue
		}
		ownedToDelete = append(ownedToDelete, file)
		if view.FileExists(fullPath(root, file)) && !fileSet[pathKey(file)] {
			fileSet[pathKey(file)] = true
			files = append(files, file)
		}
	}

	// 3. Paths matching purge that no other package owns.
	var directories []string
	directorySet := map[string]bool{}
	for _, glob := range purge {
		matches, err := in.enumerateMatches(ctx, root, glob, view)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if match.isDirectory {
				if !IsModsDirectory(match.path) && !directorySet[pathKey(match.path)] {
					directorySet[pathKey(match.path)] = true
					directories = append(directories, match.path)
				}
			} else if !ownedByOthers[pathKey(match.path)] && !fileSet[pathKey(match.path)] {
				fileSet[pathKey(match.path)] = true
				files = append(files, match.path)
			}
		}
	}

	if opts.DryRun {
		for _, f := range files {
			in.info("Would delete " + f)
		}
		for _, d := range directories {
			in.info("Would delete " + d + "/ (if empty after deletion)")
		}
	}

	var steps []Step
	if !opts.IgnoreScripts && len(entry.Hooks.PreUninstall) > 0 {
		steps = append(steps, &ScriptStep{Stage: "pre_uninstall", Commands: entry.Hooks.PreUninstall})
	}

	for _, f := range files {
		full := fullPath(root, f)
		view.DeleteFile(full)
		steps = append(steps, &DeleteFileStep{FullPath: full})
	}

	// Directory patterns delete whole directories, except what other
	// packages own. Deepest first, so that a matched directory containing
	// matched directories empties bottom-up.
	ordered := append([]string(nil), directories...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return strings.Count(ordered[i], "/") > strings.Count(ordered[j], "/")
	})
	for _, d := range ordered {
		full := fullPath(root, d)
		if !view.DirectoryExists(full) {
			continue
		}
		empty, err := view.IsEmpty(full)
		if err != nil {
			return nil, err
		}
		if empty {
			view.DeleteDirectory(full)
			steps = append(steps, &DeleteDirectoryStep{FullPath: full})
		}
	}

	// 4. Directories that became empty because of the deletions above.
	for _, path := range append(append([]string(nil), files...), directories...) {
		for dir, ok := parentDir(fullPath(root, path)); ok && isInside(root, dir); dir, ok = parentDir(dir) {
			if !view.DirectoryExists(dir) {
				break
			}
			empty, err := view.IsEmpty(dir)
			if err != nil {
				return nil, err
			}
			if !empty || IsModsDirectory(relativePath(root, dir)) {
				break
			}
			view.DeleteDirectory(dir)
			steps = append(steps, &DeleteDirectoryStep{FullPath: dir})
		}
	}

	// Without hook commands, the workspace is as planned when the deletions
	// run.
	if !opts.IgnoreScripts {
		steps = append(steps, &DeleteMatchesStep{OwnedFiles: ownedToDelete, Purge: purge})
	}

	if !opts.IgnoreScripts && len(entry.Hooks.PostUninstall) > 0 {
		steps = append(steps, &ScriptStep{Stage: "post_uninstall", Commands: entry.Hooks.PostUninstall})
	}

	return &Plan{Name: identity, Version: LockedVersion(pkg), Root: root, Steps: steps}, nil
}

// match is a path enumerateMatches found.
type match struct {
	path        string
	isDirectory bool
}

// enumerateMatches is McmodInstaller.EnumerateMatches: the workspace paths
// matching the glob, as (workspace-relative path, is directory); everything
// below a matched directory is included. Only the glob's literal leading
// directory is walked, only as deep as the pattern can reach, and never
// through links.
func (in *Installer) enumerateMatches(ctx context.Context, root string, glob *pathglob.Glob, view *View) ([]match, error) {
	pattern := glob.Pattern()
	segments := strings.Split(strings.TrimRight(pattern, "/"), "/")
	for _, s := range segments {
		if s == "." || s == ".." {
			// No workspace-relative path has such a segment; never look
			// outside the workspace.
			return nil, nil
		}
	}
	if startsWithDrive(pattern) || aliasesOnWindows(pattern) {
		// Nor a name that Windows cannot hold, which Path.GetFullPath would
		// turn into another path (e.g. "mods./" into mods, ".lip./x" into
		// .lip/x): lip would match what the pattern does not name.
		return nil, nil
	}

	literal := 0
	for literal < len(segments) && !pathglob.HasMeta(segments[literal]) {
		literal++
	}

	if literal == len(segments) && !glob.MatchesDirectories() {
		// A literal file path.
		if file, ok := in.longName(root, pattern); ok && !IsLipPath(file) &&
			view.FileExists(fullPath(root, file)) {
			return []match{{file, false}}, nil
		}
		return nil, nil
	}

	// The walk starts at the long name of the literal leading directory, so
	// that the paths it finds are checked by their long names; the glob is
	// matched against them as it spells that directory.
	var start string
	for _, s := range segments[:literal] {
		start += s + "/"
	}
	longStart, ok := in.longName(root, start)
	if !ok {
		return nil, nil
	}
	startFull := fullPath(root, longStart)
	if (start != "" && !isInside(root, startFull)) || !view.DirectoryExists(startFull) ||
		IsLipPath(longStart) {
		return nil, nil
	}

	maxDepth := len(segments) - literal // 0: unlimited
	for _, s := range segments[literal:] {
		if s == "**" {
			maxDepth = 0
		}
	}

	var result []match
	var walk func(directory string, all bool, depth int) error
	walk = func(directory string, all bool, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := view.Enumerate(directory)
		if err != nil {
			return err
		}
		for _, e := range entries {
			path := relativePath(root, e.FullName)
			if IsLipPath(path) {
				continue
			}
			// A path below the long name of the start is matched as the glob
			// spells the start. A path that the start as spelled does not lead
			// to, as when Windows drops the trailing dots of a name in it, is
			// matched as it is.
			spelled := path
			if longStart != start && hasPathPrefix(path, longStart) {
				spelled = start + path[len(longStart):]
			}
			suffix := ""
			if e.IsDirectory {
				suffix = "/"
			}
			// A path below mods does not match only because mods does
			// (protocol/workspace.md §4).
			minSegments := 0
			if slash := strings.IndexByte(path, '/'); slash >= 0 && IsModsDirectory(path[:slash]) {
				minSegments = 2
			}
			matched := all || (!(e.IsDirectory && IsModsDirectory(path)) && glob.IsMatchBelow(spelled+suffix, minSegments))
			if matched {
				result = append(result, match{path, e.IsDirectory})
			}
			if e.IsDirectory && !e.IsLink && (matched || maxDepth == 0 || depth < maxDepth) {
				if err := walk(e.FullName, matched, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if literal == len(segments) {
		// A literal directory path. mods is never matched, so a pattern
		// naming it removes nothing below it either.
		if IsModsDirectory(longStart) {
			return nil, nil
		}
		result = append(result, match{strings.TrimRight(longStart, "/"), true})
		if err := walk(startFull, true, 1); err != nil {
			return nil, err
		}
		return result, nil
	}

	if err := walk(startFull, false, 1); err != nil {
		return nil, err
	}
	return result, nil
}

// Apply is McmodInstaller.Apply: runs the steps of a plan, making every
// file change through the transaction, and returns the files the package
// owns afterwards (workspace-relative, '/'-separated; none for an
// uninstall). Each write is checked against the write rules as the
// workspace and owners are then (protocol/workspace.md §4), and adds the
// file it writes to owners; a file that another package owns then is not
// deleted. It fails with *Error when a step fails, or with the context's
// error when ctx is cancelled; the caller then rolls the transaction back.
// The plan's WrittenPaths should be among the transaction's reserved paths
// from the start (txn.Options.Reserved) with those of every other plan of
// the operation; Apply reserves them too.
func (in *Installer) Apply(ctx context.Context, tx *txn.Transaction, plan *Plan, owners *FileOwners) ([]string, error) {
	owned := []string{}
	ownedSet := map[string]bool{}
	for _, p := range plan.WrittenPaths() {
		tx.Reserve(p)
	}
	pkg := plan.Name
	if plan.Version != "" {
		pkg += "@" + plan.Version
	}

	var place, remove *progressBar
	var counter *byteCounter
	placed, removed := 0, 0
	closeBars := func() {
		if place != nil {
			if counter != nil {
				place.doneAt(counter.cur)
			} else {
				place.doneAt(int64(placed))
			}
			place = nil
		}
		if remove != nil {
			remove.doneAt(int64(removed))
			remove = nil
		}
	}
	defer closeBars()

	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch s := step.(type) {
		case *ScriptStep:
			closeBars()
			if len(s.Commands) > 0 {
				// A hook command runs only once the journal exists
				// (protocol/workspace.md §7.4).
				if err := tx.Begin(); err != nil {
					return nil, err
				}
			}
			if err := in.runScripts(ctx, pkg, plan.Name, s.Stage, s.Commands, plan.Root); err != nil {
				return nil, err
			}

		case *WriteStep:
			if place == nil {
				place, counter = in.openPlaceBar(pkg, plan)
			}
			// The write rules see the target by its long names as the
			// workspace is now: a directory that an earlier write or a hook
			// command created can give a short name in it a meaning it did not
			// have when the plan was made (protocol/workspace.md §4).
			target, ok := in.longName(plan.Root, s.Target)
			if !ok {
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: lip cannot tell which file '%s'"+
					" names, as it cannot read the long name of a short name in it, so it does not"+
					" write it.", plan.Name, s.Target)}
			}
			if IsLipPath(target) {
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: it writes '%s', inside %s, which"+
					" is reserved for lip. Report this to the author of the package.", plan.Name, target,
					pkgmodel.LipDir)}
			}
			if owner, ok := owners.OwnerOf(target); ok && !ref.NamesEqual(owner, plan.Name) {
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: it writes '%s', which belongs to"+
					" %s. Uninstall one of the packages first, or ask the authors of the packages to"+
					" resolve the conflict.", plan.Name, target, owner)}
			}

			var count func(int64)
			if counter != nil {
				counter.startFile(s.Target)
				count = counter.add
			} else {
				place.update(int64(placed), s.Target)
			}

			// An existing file matching preserve is kept, as it is now.
			full := fullPath(plan.Root, target)
			if matchesAny(s.Preserve, target) && fileExists(full) {
				placed++
				continue
			}

			if err := writeFile(ctx, tx, s, full, count); err != nil {
				if isCancel(err) {
					return nil, err
				}
				return nil, &Error{Msg: fmt.Sprintf("Cannot install %s: lip cannot write '%s'.",
					plan.Name, target), Err: err}
			}
			placed++
			owners.Add(target, plan.Name)
			if !ownedSet[pathKey(target)] {
				ownedSet[pathKey(target)] = true
				owned = append(owned, target)
			}

		case *DeleteFileStep:
			if remove == nil {
				remove = in.openRemoveBar(pkg, plan)
			}
			remove.update(int64(removed), show(plan.Root, s.FullPath))
			// An earlier package of the operation may own it now, and a hook
			// command may have deleted it.
			if !in.ownedByAnother(plan, owners, s.FullPath) && fileExists(s.FullPath) {
				if err := tx.DeleteFile(s.FullPath); err != nil {
					if errors.Is(err, txn.ErrBug) {
						return nil, err
					}
					return nil, &Error{Msg: fmt.Sprintf("Cannot uninstall %s: lip cannot delete '%s'.",
						plan.Name, show(plan.Root, s.FullPath)), Err: err}
				}
			}
			removed++

		case *DeleteDirectoryStep:
			if remove == nil {
				remove = in.openRemoveBar(pkg, plan)
			}
			remove.update(int64(removed), show(plan.Root, s.FullPath))
			if err := tx.DeleteDirectoryOnCommit(s.FullPath); err != nil {
				if errors.Is(err, txn.ErrBug) {
					return nil, err
				}
				return nil, &Error{Msg: fmt.Sprintf("Cannot uninstall %s: lip cannot record that it deletes"+
					" the directory '%s'.", plan.Name, show(plan.Root, s.FullPath)), Err: err}
			}
			removed++

		case *DeleteMatchesStep:
			if err := in.deleteMatches(ctx, plan, s, tx, owners); err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("lip planned a step it does not know (%T). This is a bug in lip.", step)
		}
	}
	return owned, nil
}

// deleteMatches is McmodInstaller.DeleteMatches: deletes what uninstallation
// steps 2-4 match in the workspace as it is now (protocol/workspace.md §6
// rule 4), beyond what the plan already deleted. A file that another package
// owns now is kept, and so is a matched directory holding it.
func (in *Installer) deleteMatches(ctx context.Context, plan *Plan, step *DeleteMatchesStep,
	tx *txn.Transaction, owners *FileOwners) error {
	now := NewView(nil)
	files := append([]string(nil), step.OwnedFiles...)
	var directories []string
	for _, glob := range step.Purge {
		matches, err := in.enumerateMatches(ctx, plan.Root, glob, now)
		if err != nil {
			return err
		}
		for _, m := range matches {
			if m.isDirectory {
				if !IsModsDirectory(m.path) {
					directories = append(directories, m.path)
				}
			} else {
				files = append(files, m.path)
			}
		}
	}

	var deleted []string
	distinct := map[string]bool{}
	for _, file := range files {
		if distinct[pathKey(file)] {
			continue
		}
		distinct[pathKey(file)] = true
		full := fullPath(plan.Root, file)
		if !fileExists(full) || in.ownedByAnother(plan, owners, full) {
			continue
		}
		if err := tx.DeleteFile(full); err != nil {
			if errors.Is(err, txn.ErrBug) {
				return err
			}
			return &Error{Msg: fmt.Sprintf("Cannot uninstall %s: lip cannot delete '%s'.",
				plan.Name, show(plan.Root, full)), Err: err}
		}
		deleted = append(deleted, file)
	}

	// Matched directories, and the directories these deletions leave empty,
	// are deleted when the operation commits if they are empty then.
	scheduled := map[string]bool{}
	schedule := func(full string) error {
		if scheduled[pathKey(full)] {
			return nil
		}
		scheduled[pathKey(full)] = true
		if !dirExists(full) {
			return nil
		}
		if err := tx.DeleteDirectoryOnCommit(full); err != nil {
			if errors.Is(err, txn.ErrBug) {
				return err
			}
			return &Error{Msg: fmt.Sprintf("Cannot uninstall %s: lip cannot record that it deletes the"+
				" directory '%s'.", plan.Name, show(plan.Root, full)), Err: err}
		}
		return nil
	}
	for _, d := range directories {
		if err := schedule(fullPath(plan.Root, d)); err != nil {
			return err
		}
	}
	for _, path := range append(deleted, directories...) {
		for dir, ok := parentDir(fullPath(plan.Root, path)); ok && isInside(plan.Root, dir); dir, ok = parentDir(dir) {
			if IsModsDirectory(relativePath(plan.Root, dir)) {
				break
			}
			if err := schedule(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// ownedByAnother is McmodInstaller.OwnedByAnother: whether a package other
// than the one of the plan owns the file now, so that uninstalling the
// package keeps it (protocol/workspace.md §4).
func (in *Installer) ownedByAnother(plan *Plan, owners *FileOwners, full string) bool {
	owner, ok := owners.OwnerOf(relativePath(plan.Root, full))
	return ok && !ref.NamesEqual(owner, plan.Name)
}

// writeFile writes the file of a write step through the transaction,
// counting the bytes read from the source; the copy also stops when ctx is
// cancelled (the transaction's own context aside).
func writeFile(ctx context.Context, tx *txn.Transaction, s *WriteStep, full string, count func(int64)) error {
	r, err := s.Source.Open(s.Key)
	if err != nil {
		return err
	}
	defer r.Close()
	return tx.WriteFile(full, &countingReader{ctx: ctx, r: r, add: count})
}

// openPlaceBar opens the StagePlace bar of the writes of a plan: bytes when
// the size of every written file is known (and the total positive), files
// otherwise.
func (in *Installer) openPlaceBar(pkg string, plan *Plan) (*progressBar, *byteCounter) {
	var total int64
	files := 0
	sized := true
	for _, step := range plan.Steps {
		if w, ok := step.(*WriteStep); ok {
			files++
			n, ok := w.Source.Size(w.Key)
			if !ok || n < 0 {
				sized = false
			}
			total += n
		}
	}
	if sized && total > 0 {
		bar := in.newBar(pkg, report.StagePlace, report.UnitBytes, "Placing files", total)
		return bar, newByteCounter(bar, total)
	}
	return in.newBar(pkg, report.StagePlace, report.UnitFiles, "Placing files", int64(files)), nil
}

// openRemoveBar opens the StageRemove bar of the deletions of a plan.
func (in *Installer) openRemoveBar(pkg string, plan *Plan) *progressBar {
	n := 0
	for _, step := range plan.Steps {
		switch step.(type) {
		case *DeleteFileStep, *DeleteDirectoryStep:
			n++
		}
	}
	return in.newBar(pkg, report.StageRemove, report.UnitFiles, "Removing files", int64(n))
}

// runScripts is McmodInstaller.RunScripts: runs the commands of a hook in
// turn in the workspace root, reporting a StageScripts bar over the commands
// finished and their output as Info lines.
func (in *Installer) runScripts(ctx context.Context, pkg, name, stage string, commands []string, root string) error {
	run := in.RunCommand
	if run == nil {
		run = DefaultCommandRunner
	}
	rep := report.OrNop(in.Reporter)
	bar := in.newBar(pkg, report.StageScripts, report.UnitCommands,
		fmt.Sprintf("Running %s scripts", stage), int64(len(commands)))
	finished := 0
	defer func() { bar.doneAt(int64(finished)) }()
	for i, command := range commands {
		bar.update(int64(i), command)
		output := &lineWriter{emit: rep.Info}
		err := run(ctx, root, command, output)
		output.Flush()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil && isCancel(err) {
				return ctxErr
			}
			return &Error{Msg: fmt.Sprintf("In the %s hook of %s, the command '%s' failed. lip undoes"+
				" the file changes of this operation, but not what hook commands changed.",
				stage, name, command), Err: err}
		}
		finished = i + 1
	}
	return nil
}
