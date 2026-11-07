package compat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/graph"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// ToothLockFileName is the tooth_lock.json of lip v0.x at the root of a
// workspace (ToothLockMigration.FileName).
const ToothLockFileName = "tooth_lock.json"

// ToothLockMember is a member of "packages" of a tooth_lock.json
// (ToothLockMigration.Member).
type ToothLockMember struct {
	// Manifest is its "manifest", a tooth.json (a doc value).
	Manifest any
	Variant  string
	Locked   bool
	Files    []string
}

// ToothLock is a tooth_lock.json that protocol/compatibility.md §3 reads.
type ToothLock struct {
	Members []ToothLockMember
}

// ReadToothLock is ToothLockMigration.Read: the members of
// <root>/tooth_lock.json when it is a JSON object with format_version 3 and
// an array "packages" of objects with "manifest" (a JSON object with
// format_version 1, 2 or 3), "variant" (a string), "locked" (a boolean) and
// "files" (an array of strings); false otherwise, also when it is missing or
// unreadable.
func ReadToothLock(root string) (*ToothLock, bool) {
	path := filepath.Join(root, ToothLockFileName)
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	v, err := doc.ParseJSON(data, ToothLockFileName)
	if err != nil {
		return nil, false
	}
	return toothLockMembers(v)
}

func toothLockMembers(root any) (*ToothLock, bool) {
	if fv, ok := FormatVersion(root); !ok || fv != 3 {
		return nil, false
	}
	packages, _ := root.(*doc.Object).Get("packages")
	list, ok := packages.([]any)
	if !ok {
		return nil, false
	}
	tl := &ToothLock{Members: []ToothLockMember{}}
	for _, item := range list {
		p, ok := item.(*doc.Object)
		if !ok {
			return nil, false
		}
		manifest, ok := p.Get("manifest")
		if !ok {
			return nil, false
		}
		if fv, ok := FormatVersion(manifest); !ok || fv < 1 || fv > 3 {
			return nil, false
		}
		variant, _ := p.Get("variant")
		vs, ok := variant.(string)
		if !ok {
			return nil, false
		}
		locked, _ := p.Get("locked")
		lb, ok := locked.(bool)
		if !ok {
			return nil, false
		}
		files, _ := p.Get("files")
		fl, ok := files.([]any)
		if !ok {
			return nil, false
		}
		m := ToothLockMember{Manifest: doc.Clone(manifest), Variant: vs, Locked: lb, Files: []string{}}
		for _, f := range fl {
			s, ok := f.(string)
			if !ok {
				return nil, false
			}
			m.Files = append(m.Files, s)
		}
		tl.Members = append(tl.Members, m)
	}
	return tl, true
}

// CommitOf gives the commit the tag of a remote repository points to (peeled
// to the commit): repo is the package id, tag "v<version>".
type CommitOf func(repo, tag string) (string, error)

// DeriveWorkspace is ToothLockMigration.Derive: the workspace file
// protocol/compatibility.md §3 derives from the members of a tooth_lock.json
// at the root of the workspace root for an environment: for each member whose
// manifest Section 2 reads, for the environment, with a variant "variant", in
// installation order, a package with that manifest, its variant, "locked" as
// explicit, the ref "<tooth>[#variant]", the commit commitOf gives for the
// tag "v<version>" (failing when it gives none) and the files below the
// workspace, workspace-relative and '/'-separated. With a nil commitOf (for
// reading only, as lip list does), no commit is recorded.
//
// Members with an invalid variant, a manifest that cannot be read, or no
// entry for the variant are left out. An error is a commit that cannot be
// obtained, or a manifest lip v0.38 itself fails on unexpectedly.
func DeriveWorkspace(tl *ToothLock, root, platform, runtime string, commitOf CommitOf) (*pkgmodel.LockFile, error) {
	type member struct {
		locked  pkgmodel.LockedPackage
		entry   *pkgmodel.Entry
		version semver.Version
	}
	var packages []member
	for _, m := range tl.Members {
		if m.Variant != "" && !ref.IsValidVariant(m.Variant) {
			continue
		}
		t, err := ReadToothJSON(m.Manifest)
		var pf *pkgmodel.Document
		if err == nil {
			pf, err = t.Derive(platform, runtime, nil)
		}
		if err != nil {
			if fe := (*FormatError)(nil); errors.As(err, &fe) {
				continue
			}
			return nil, err
		}
		entry := pf.SelectEntry(m.Variant, platform, runtime)
		if entry == nil {
			continue
		}
		commit := ""
		if commitOf != nil {
			c, err := commitOf(pf.ID, "v"+pf.Version)
			if err != nil || c == "" {
				msg := fmt.Sprintf("lip cannot convert %s, written by lip v0.x, to the workspace"+
					" file: the commit of the tag v%s of %s cannot be found. The tag may have been"+
					" deleted, or the repository cannot be reached. Check your network connection"+
					" and GitHub proxy, then try again.",
					filepath.Join(root, ToothLockFileName), pf.Version, pf.ID)
				if err != nil {
					return nil, fmt.Errorf("%s %w", msg, err)
				}
				return nil, fmt.Errorf("%s", msg)
			}
			commit = strings.ToLower(c)
		}
		files := []string{}
		for _, f := range m.Files {
			if rel, ok := relativePath(root, f); ok {
				files = append(files, rel)
			}
		}
		packages = append(packages, member{
			locked: pkgmodel.LockedPackage{
				Manifest: pf.Raw,
				Variant:  m.Variant,
				Explicit: m.Locked,
				Ref:      ref.Identity(pf.ID, m.Variant),
				Commit:   commit,
				Files:    files,
			},
			entry:   entry,
			version: t.Version,
		})
	}

	// Installation order (protocol/workspace.md §5): a package after the
	// packages that satisfy its dependencies.
	sort.SliceStable(packages, func(i, j int) bool {
		a, b := ref.Fold(packages[i].locked.Ref), ref.Fold(packages[j].locked.Ref)
		if a != b {
			return a < b
		}
		return semver.CompareSortOrder(packages[i].version, packages[j].version) < 0
	})
	order := graph.InstallOrderIndices(len(packages), func(i int) []int {
		var deps []int
		for _, d := range packages[i].entry.Deps {
			r, _, ok := ref.TryParse(d.Key, false)
			if !ok {
				continue
			}
			rng, err := semver.ParseVersionRange(d.Range)
			if err != nil {
				continue
			}
			identity := ref.Identity(r.Target, r.Variant)
			for j := range packages {
				if ref.NamesEqual(packages[j].locked.Ref, identity) && rng.Contains(packages[j].version) {
					deps = append(deps, j)
				}
			}
		}
		return deps
	})

	lf := &pkgmodel.LockFile{
		Format:   pkgmodel.Format,
		Platform: platform,
		Runtime:  runtime,
		Packages: make([]pkgmodel.LockedPackage, 0, len(order)),
	}
	for _, i := range order {
		lf.Packages = append(lf.Packages, packages[i].locked)
	}
	return lf, nil
}

// relativePath is ToothLockMigration.RelativePath: a recorded path as the
// path relative to the workspace it names, '/'-separated, when it is below
// the workspace (relative to the workspace when not absolute).
func relativePath(root, file string) (string, bool) {
	path := file
	if !isPathRooted(file) {
		path = filepath.Join(root, file)
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == "." || rel == ".." || isPathRooted(rel) ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	// Path.GetFullPath and Path.GetRelativePath keep a trailing separator.
	if strings.HasSuffix(file, "/") || strings.HasSuffix(file, string(filepath.Separator)) {
		rel += string(filepath.Separator)
	}
	return filepath.ToSlash(rel), true
}

// isPathRooted is Path.IsPathRooted: on Windows, a path starting with a
// separator or a drive letter and ':'.
func isPathRooted(path string) bool {
	if filepath.Separator == '\\' {
		if len(path) > 0 && (path[0] == '\\' || path[0] == '/') {
			return true
		}
		return len(path) >= 2 && path[1] == ':' &&
			((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z'))
	}
	return strings.HasPrefix(path, "/")
}
