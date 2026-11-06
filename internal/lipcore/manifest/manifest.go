// Package manifest models tooth.json format 3 (lip 0.37.0
// src/Lip.Core/Entities/PackageManifest*.cs, schemas/manifest.schema.json)
// and package identifiers/specs (PackageId.cs, PackageSpec.cs).
//
// Parse accepts only format 3 documents; callers convert older formats and
// render templates with package migrate first.
package manifest

import (
	"errors"

	"github.com/liteldev/LeviLauncher/internal/lipcore/glob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
	"golang.org/x/mod/module"
)

// Hooks into sibling packages; tests may replace them to stay independent
// of package semver/glob.
var (
	// NormalizeVersion parses a manifest version with SemVersionStyles.Any
	// and returns SemVersion.ToString().
	NormalizeVersion = func(s string) (string, error) {
		v, err := semver.ParseAny(s)
		if err != nil {
			return "", err
		}
		return v.String(), nil
	}
	// NormalizeRange parses a dependency range (SemVersionRange.Parse) and
	// returns the text lip writes back.
	NormalizeRange = func(s string) (string, error) {
		r, err := semver.ParseRange(s)
		if err != nil {
			return "", err
		}
		return r.String(), nil
	}
	// ValidateGlob reports whether a preserve/remove/platform pattern parses.
	ValidateGlob = func(pattern string) error {
		_, err := glob.Compile(pattern)
		return err
	}
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("manifest: not implemented")

const (
	FormatVersion = 3
	FormatUUID    = "289f771f-2c9a-4d73-9f3f-8492495a924d"
)

// Manifest is a tooth.json format 3 document. Field order is the
// serialization order used by lip (and therefore by tooth_lock.json).
type Manifest struct {
	FormatVersion int    `json:"format_version"`
	FormatUUID    string `json:"format_uuid"`
	// Tooth is the package path (Go module path rules, e.g.
	// "github.com/LiteLDev/LeviLamina").
	Tooth string `json:"tooth"`
	// Version is the SemVer version in the normalized form lip writes
	// (SemVersion.ToString of the value parsed with SemVersionStyles.Any).
	Version  string    `json:"version"`
	Info     Info      `json:"info"`
	Variants []Variant `json:"variants"`
}

type Info struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	AvatarURL   string   `json:"avatar_url"`
}

type Variant struct {
	// Label must match ^([a-z0-9_]+(/[a-z0-9_]+)?)?$.
	Label string `json:"label"`
	// Platform is "" (any), a .NET RID such as "win-x64", or a glob over RIDs.
	Platform      string       `json:"platform"`
	Dependencies  Dependencies `json:"dependencies"`
	Assets        []Asset      `json:"assets"`
	PreserveFiles []string     `json:"preserve_files"`
	RemoveFiles   []string     `json:"remove_files"`
	Scripts       Scripts      `json:"scripts"`
}

// Dependency is one "<package id>": "<range>" entry.
type Dependency struct {
	// ID is a package id string ("path" or "path#variant").
	ID string
	// Range is the range text as lip writes it back.
	Range string
}

// Dependencies preserves insertion order (System.Text.Json writes
// Dictionary entries in insertion order). It (un)marshals as a JSON object.
type Dependencies []Dependency

func (d Dependencies) MarshalJSON() ([]byte, error)         { return nil, ErrNotImplemented }
func (d *Dependencies) UnmarshalJSON(data []byte) error     { return ErrNotImplemented }
func (d Dependencies) MarshalSTJSON(w *stjson.Writer) error { return ErrNotImplemented }

type AssetType string

const (
	AssetSelf         AssetType = "self"
	AssetTar          AssetType = "tar"
	AssetTgz          AssetType = "tgz"
	AssetUncompressed AssetType = "uncompressed"
	AssetZip          AssetType = "zip"
)

type Asset struct {
	Type       AssetType   `json:"type"`
	URLs       []string    `json:"urls"`
	Placements []Placement `json:"placements"`
}

type PlacementType string

const (
	PlacementFile PlacementType = "file"
	PlacementDir  PlacementType = "dir"
)

type Placement struct {
	Type PlacementType `json:"type"`
	Src  string        `json:"src"`
	// Dest is relative to the workspace; rooted, fully qualified or ".."
	// paths are rejected by Validate.
	Dest string `json:"dest"`
}

type Scripts struct {
	PreInstall    []string `json:"pre_install"`
	Install       []string `json:"install"`
	PostInstall   []string `json:"post_install"`
	PreUninstall  []string `json:"pre_uninstall"`
	Uninstall     []string `json:"uninstall"`
	PostUninstall []string `json:"post_uninstall"`
}

// Parse decodes a format 3 manifest, fills defaults (empty strings/lists,
// as lip's records do), normalizes Version and validates it (Validate).
func Parse(data []byte) (*Manifest, error) {
	return nil, ErrNotImplemented
}

// Validate checks format version/uuid, tooth path (module.CheckPath rules),
// version, tag/label patterns, placement dest safety and glob patterns.
func (m *Manifest) Validate() error {
	return ErrNotImplemented
}

// MarshalIndented encodes m like lip (System.Text.Json, indented); this is
// also the form embedded in tooth_lock.json.
func (m *Manifest) MarshalIndented() ([]byte, error) {
	return nil, ErrNotImplemented
}

// PlatformMatcher reports whether the variant platform pattern matches rid
// (glob.Match in production).
type PlatformMatcher func(pattern, rid string) bool

// ResolveVariant implements PackageManifest.GetVariant: select variants
// whose Label == label and whose Platform is "", == rid, or a glob matching
// rid; error if none. The result has Label=label, Platform=rid, merged
// Dependencies (duplicate keys are an error), concatenated Assets,
// PreserveFiles, RemoveFiles, and each script list taken from the single
// selected variant where it is non-empty (error if more than one).
func (m *Manifest) ResolveVariant(label, rid string, match PlatformMatcher) (*Variant, error) {
	return nil, ErrNotImplemented
}

// PackageID is "path" or "path#variant" (regex ^(?<path>[^#]+)(?:#(?<label>[^#]*))?$).
type PackageID struct {
	Path    string
	Variant string
}

// ParsePackageID parses s.
func ParsePackageID(s string) (PackageID, error) {
	return PackageID{}, ErrNotImplemented
}

// String formats as lip's PackageId.ToString: "path" when Variant is empty,
// else "path#variant".
func (id PackageID) String() string {
	if id.Variant == "" {
		return id.Path
	}
	return id.Path + "#" + id.Variant
}

// PackageSpec is "id@version" (regex ^(?<id>[^@]+)@(?<version>[^@]+)$).
// Version is kept as text; callers parse it strictly with semver.Parse.
type PackageSpec struct {
	ID      PackageID
	Version string
}

// ParsePackageSpec parses s.
func ParsePackageSpec(s string) (PackageSpec, error) {
	return PackageSpec{}, ErrNotImplemented
}

func (s PackageSpec) String() string { return s.ID.String() + "@" + s.Version }

// checkPath validates a package path with Go module path rules
// (lip uses a port of golang.org/x/mod/module.CheckPath).
func checkPath(path string) error { return module.CheckPath(path) }
