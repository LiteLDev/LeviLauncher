// Package lockfile reads and writes tooth_lock.json, lip 0.37.0's workspace
// state (src/Lip.Core/Entities/WorkspaceState*.cs, Services/WorkspaceService.cs,
// schemas/workspace_state.schema.json), byte-compatible with lip.
package lockfile

import (
	"errors"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("lockfile: not implemented")

// FileName is the state file name inside the workspace (instance) directory.
const FileName = "tooth_lock.json"

// State is the whole lock file. Field order is lip's serialization order.
type State struct {
	FormatVersion int       `json:"format_version"`
	FormatUUID    string    `json:"format_uuid"`
	Packages      []Package `json:"packages"`
}

// Package is one installed package.
type Package struct {
	// Files are absolute paths (FileInfo.FullName, backslashes on Windows)
	// of the files this package placed.
	Files []string `json:"files"`
	// Locked is true when the package was installed explicitly.
	Locked bool `json:"locked"`
	// Manifest is the full manifest (all variants) as installed.
	Manifest manifest.Manifest `json:"manifest"`
	// Variant is the installed variant label.
	Variant string `json:"variant"`
}

// ID returns the installed package id (Manifest.Tooth + Variant).
func (p Package) ID() manifest.PackageID {
	return manifest.PackageID{Path: p.Manifest.Tooth, Variant: p.Variant}
}

// Spec returns "path#variant@version".
func (p Package) Spec() manifest.PackageSpec {
	return manifest.PackageSpec{ID: p.ID(), Version: p.Manifest.Version}
}

// New returns an empty state with format version/uuid set.
func New() *State {
	return &State{FormatVersion: manifest.FormatVersion, FormatUUID: manifest.FormatUUID, Packages: []Package{}}
}

// Load reads <dir>/tooth_lock.json. A missing file yields New(). Like lip,
// a malformed file is reported via the returned error AND an empty state is
// returned, so callers may print the error and continue.
func Load(dir string) (*State, error) {
	return New(), ErrNotImplemented
}

// Save writes s to <dir>/tooth_lock.json exactly as lip does
// (System.Text.Json indented, see package stjson).
func (s *State) Save(dir string) error {
	return ErrNotImplemented
}

// Marshal returns the bytes Save would write.
func (s *State) Marshal() ([]byte, error) {
	return nil, ErrNotImplemented
}

// Find returns the index of the package with the given id, or -1.
func (s *State) Find(id manifest.PackageID) int {
	for i, p := range s.Packages {
		if p.ID() == id {
			return i
		}
	}
	return -1
}
