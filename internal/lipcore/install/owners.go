package install

import (
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// FileOwners is FileOwners.cs: which package owns each file of the workspace
// while an operation is applied (protocol/workspace.md §4): the files of the
// installed packages, less those of the packages uninstalled so far, and the
// files written so far. Paths are workspace-relative, '/'-separated and by
// their long names (Installer.Owners); they are compared ignoring case on
// Windows.
type FileOwners struct {
	owners map[string]string // pathKey -> identity
}

// NewFileOwners returns owners of no file.
func NewFileOwners() *FileOwners { return &FileOwners{owners: map[string]string{}} }

// OwnerOf is the identity of the package owning the file; false when no
// package owns it.
func (o *FileOwners) OwnerOf(path string) (string, bool) {
	identity, ok := o.owners[pathKey(path)]
	return identity, ok
}

// TryAdd records that the package owns the file, unless a package owns it
// already.
func (o *FileOwners) TryAdd(path, identity string) {
	if _, ok := o.owners[pathKey(path)]; !ok {
		o.owners[pathKey(path)] = identity
	}
}

// Add records that the package owns the file, as it has written it.
func (o *FileOwners) Add(path, identity string) { o.owners[pathKey(path)] = identity }

// Release records that the package owns no file any more, as it has been
// uninstalled.
func (o *FileOwners) Release(identity string) {
	for path, owner := range o.owners {
		if ref.NamesEqual(owner, identity) {
			delete(o.owners, path)
		}
	}
}

// Owners is McmodInstaller.Owners: the owners of the files of the installed
// packages (protocol/workspace.md §4), by the long names of the files; the
// first package recording a file owns it.
func (in *Installer) Owners(root string, packages []pkgmodel.LockedPackage) *FileOwners {
	full := absRoot(root)
	owners := NewFileOwners()
	for i := range packages {
		identity := LockedIdentity(&packages[i])
		for _, recorded := range packages[i].Files {
			file := normalizeRelative(recorded)
			if long, ok := in.longName(full, file); ok {
				file = long
			}
			owners.TryAdd(file, identity)
		}
	}
	return owners
}
