// Package pkgmodel holds plain types for lip's format-4 package file
// (protocol/lip.schema.toml; PackageFile and PackageEntry of
// src/Lip.Core/Mcmod/Documents.cs) and state file
// (protocol/state.schema.json; LockFile and LockedPackage of
// src/Lip.Core/Mcmod/Contracts.cs), as of lip v0.40.0. It does no
// parsing or validation.
package pkgmodel

import "strings"

const (
	// PackageFileName is the package file at the root of an archive
	// (PackageFormat.FileName).
	PackageFileName = "lip.toml"
	// Format is the "format" of package files and state files
	// (PackageFormat.Format).
	Format = 4
	// FormatKey is the key of the format (PackageFormat.FormatKey).
	FormatKey = "format"
	// ModManifestFileName is the manifest.json at the root of the mcmod asset
	// of a mod (PackageFormat.ModManifestFileName).
	ModManifestFileName = "manifest.json"
	// LipDir is the directory of lip's state in a workspace.
	LipDir = ".lip"
	// WorkspaceFile is the state file, relative to the workspace root.
	WorkspaceFile = ".lip/state.json"
	// ModsDir is the directory mods are installed into.
	ModsDir = "mods"
)

// Sides.
const (
	SideServer = "server"
	SideClient = "client"
)

// Document is a package file (protocol/package.md §3), or the one
// protocol/compatibility.md §2 derives from a tooth.json for an environment.
type Document struct {
	Format int
	// ID is the package id, a Go module path.
	ID string
	// Version is the version of the package: a strict SemVer 2.0.0 version
	// without a leading "v" (a semver.Version once parsed by the caller).
	Version string
	// Info is information for people, unused by lip; nil when absent.
	Info *Info
	// Variants are the entries of each variant ("" for the default variant),
	// in document order.
	Variants []VariantEntries
	// Raw is the package file as read (for lip.toml) or the derived one; a
	// *doc.Object (ordered JSON object) when set.
	Raw any
}

// VariantEntries are the entries of one variant.
type VariantEntries struct {
	Name    string
	Entries []Entry
}

// EntriesOf returns the entries of a variant, or nil, false when the
// document has no such variant.
func (d *Document) EntriesOf(variant string) ([]Entry, bool) {
	for _, v := range d.Variants {
		if v.Name == variant {
			return v.Entries, true
		}
	}
	return nil, false
}

// SelectEntry is PackageFile.SelectEntry: the entry of a variant whose
// side and platform are those of the environment (protocol/package.md §4
// step 2), or nil.
func (d *Document) SelectEntry(variant, side, platform string) *Entry {
	entries, ok := d.EntriesOf(variant)
	if !ok {
		return nil
	}
	for i := range entries {
		if entries[i].Side == side && entries[i].Platform == platform {
			return &entries[i]
		}
	}
	return nil
}

// Info is the "info" of a package file.
type Info struct {
	Name        string
	Description string
	Tags        []string
	// Logo is an https URL of the logo image.
	Logo string
}

// Entry is an entry of a variant (lip.schema.toml, the items of "variants").
type Entry struct {
	// Side is "server" or "client".
	Side string
	// Platform is a .NET runtime identifier.
	Platform string
	// Deps are the "deps", in document order.
	Deps   []Dep
	Assets []Asset
	// Preserve and Purge are the "preserve" and "purge" globs.
	Preserve []string
	Purge    []string
	Hooks    Hooks
	// Raw is the entry as written, which the state file records; a
	// *doc.Object when set.
	Raw any
}

// McmodAsset is the index of the asset of type "mcmod", or -1.
func (e *Entry) McmodAsset() int {
	for i, a := range e.Assets {
		if a.Type == AssetMcmod {
			return i
		}
	}
	return -1
}

// Dep is a dependency: a source key ("<id>" or "<id>#<variant>", possibly
// with "@<ref>") and a version range.
type Dep struct {
	Key   string
	Range string
}

// AssetType is the "type" of an asset.
type AssetType string

const (
	// AssetSelf is the archive of the package file itself.
	AssetSelf AssetType = "self"
	// AssetZip is a ZIP archive downloaded from its URLs.
	AssetZip AssetType = "zip"
	// AssetMcmod is a ZIP archive downloaded from its URLs, with a LeviLamina
	// manifest.json at its root, written whole into mods/<its name>/.
	AssetMcmod AssetType = "mcmod"
)

// Asset is an asset of an entry (EntryAsset).
type Asset struct {
	Type AssetType
	// URLs are HTTPS URLs of the same file, tried in order.
	URLs  []string
	Place []Placement
}

// Placement is a member of the "place" of an asset (EntryPlacement).
type Placement struct {
	Src  string
	Dest string
}

// PlacementForm is the form of a placement, which its src decides.
type PlacementForm int

const (
	PlacementFile PlacementForm = iota
	PlacementDirectory
	PlacementGlob
)

func (f PlacementForm) String() string {
	switch f {
	case PlacementFile:
		return "file"
	case PlacementDirectory:
		return "directory"
	default:
		return "glob"
	}
}

// Form is the glob form when src contains '*', '?' or '['; otherwise the
// directory form when src is "" or ends with '/'; otherwise the file form.
func (p Placement) Form() PlacementForm {
	switch {
	case strings.ContainsAny(p.Src, "*?["):
		return PlacementGlob
	case p.Src == "" || strings.HasSuffix(p.Src, "/"):
		return PlacementDirectory
	}
	return PlacementFile
}

// Hooks are the "hooks" of an entry.
type Hooks struct {
	PreInstall    []string
	PostInstall   []string
	PreUninstall  []string
	PostUninstall []string
}

// LockFile is the state file (protocol/workspace.md §7.1).
type LockFile struct {
	Format int
	// Side is "server" or "client".
	Side string
	// Platform is a .NET runtime identifier.
	Platform string
	// Packages are in installation order.
	Packages []LockedPackage
}

// LockedPackage is an installed package in the state file.
type LockedPackage struct {
	// Manifest is its package file as protocol/package.md §4 read it (for a
	// package read from a tooth.json, the derived one); a *doc.Object when
	// set. Its "id" is the package id and its "version" the version.
	Manifest any
	// Variant is the variant of its source.
	Variant string
	// Explicit is whether a request named it (as opposed to a dependency).
	Explicit bool
	// Ref is the source of the request or the key of the dependency it was
	// resolved for; a local path as the file: URI of its absolute path.
	Ref string
	// Commit is, for a package from a remote repository, the commit its
	// archive was at (lowercase hex); "" when absent.
	Commit string
	// Files are the owned files, workspace-relative, '/'-separated and in the
	// mods/ form.
	Files []string
}
