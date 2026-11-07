// Package manifest models tooth.json format 3 (lip 0.37.0
// src/Lip.Core/Entities/PackageManifest*.cs, schemas/manifest.schema.json)
// and package identifiers/specs (PackageId.cs, PackageSpec.cs).
//
// Parse accepts only format 3 documents; callers convert older formats and
// render templates with package migrate first.
package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"

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
	// NormalizeGlob parses a preserve/remove pattern and returns the text lip
	// writes back (GlobListJsonConverter serialises Glob.ToString, which
	// re-renders the tokens).
	NormalizeGlob = func(pattern string) (string, error) {
		g, err := glob.Compile(pattern)
		if err != nil {
			return "", err
		}
		return g.Format(), nil
	}
)

// ErrNotImplemented is returned by skeleton stubs.
//
// Deprecated: kept for API compatibility; nothing returns it any more.
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

type AssetType string

const (
	AssetSelf         AssetType = "self"
	AssetTar          AssetType = "tar"
	AssetTgz          AssetType = "tgz"
	AssetUncompressed AssetType = "uncompressed"
	AssetZip          AssetType = "zip"
)

// assetTypes lists the AssetType members in C# declaration order (their
// numeric values, which JsonStringEnumConverter also accepts on read).
var assetTypes = []AssetType{AssetSelf, AssetTar, AssetTgz, AssetUncompressed, AssetZip}

type Asset struct {
	Type AssetType `json:"type"`
	// URLs are written normalized like Flurl's Url.ToString (see
	// NormalizeURL); empty entries are dropped on read.
	URLs       []string    `json:"urls"`
	Placements []Placement `json:"placements"`
}

type PlacementType string

const (
	PlacementFile PlacementType = "file"
	PlacementDir  PlacementType = "dir"
)

var placementTypes = []PlacementType{PlacementFile, PlacementDir}

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

// Regexes from lip. .NET's "$" (without RegexOptions.Multiline) also
// matches before a final "\n", which "\n?$" reproduces.
var (
	tagRegex     = regexp.MustCompile(`^[a-z0-9-]+(:[a-z0-9-]+)?\n?$`)
	variantRegex = regexp.MustCompile(`^([a-z0-9_]+(/[a-z0-9_]+)?)?\n?$`)
	idRegex      = regexp.MustCompile(`^([^#]+)(?:#([^#]*))?$`)
	specRegex    = regexp.MustCompile(`^([^@]+)@([^@]+)$`)
)

// IsValidVariant reports whether label is a valid variant label
// (PackageId.IsValidVariant).
func IsValidVariant(label string) bool { return variantRegex.MatchString(label) }

// IsValidTag reports whether tag matches ^[a-z0-9-]+(:[a-z0-9-]+)?$.
func IsValidTag(tag string) bool { return tagRegex.MatchString(tag) }

// windowsPaths selects .NET's Windows path rules for IsValidDest.
var windowsPaths = runtime.GOOS == "windows"

// IsValidDest implements PackageManifestAssetPlacement.IsValidDst: the
// destination must not be fully qualified, rooted or contain "..".
// Rootedness follows .NET Path.IsPathRooted on the current OS.
func IsValidDest(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if windowsPaths {
		if len(path) >= 1 && (path[0] == '/' || path[0] == '\\') {
			return false
		}
		if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
			return false
		}
		return true
	}
	return !strings.HasPrefix(path, "/")
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// Validate checks format version/uuid, tooth path (module.CheckPath rules),
// version, tag/label patterns, dependency ids/ranges (including duplicate
// ids), asset and placement types, placement dest safety and
// preserve/remove glob patterns. Like lip, platform patterns are only
// interpreted by ResolveVariant.
func (m *Manifest) Validate() error {
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported format version: %d", m.FormatVersion)
	}
	if m.FormatUUID != FormatUUID {
		return fmt.Errorf("unsupported format UUID: %s", m.FormatUUID)
	}
	if err := checkPath(m.Tooth); err != nil {
		return fmt.Errorf("invalid package path: %s: %w", m.Tooth, err)
	}
	if _, err := NormalizeVersion(m.Version); err != nil {
		return fmt.Errorf("invalid package version: %s: %w", m.Version, err)
	}
	for _, tag := range m.Info.Tags {
		if !IsValidTag(tag) {
			return fmt.Errorf("invalid package tag: %s", tag)
		}
	}
	for i := range m.Variants {
		if err := m.Variants[i].validate(); err != nil {
			return fmt.Errorf("variant %d: %w", i, err)
		}
	}
	return nil
}

func (v *Variant) validate() error {
	if !IsValidVariant(v.Label) {
		return fmt.Errorf("invalid label: %s", v.Label)
	}
	seen := make(map[PackageID]bool, len(v.Dependencies))
	for _, d := range v.Dependencies {
		id, err := ParsePackageID(d.ID)
		if err != nil {
			return err
		}
		if seen[id] {
			return fmt.Errorf("duplicate dependency: %s", id)
		}
		seen[id] = true
		if _, err := NormalizeRange(d.Range); err != nil {
			return fmt.Errorf("invalid version range for %s: %s: %w", d.ID, d.Range, err)
		}
	}
	for _, a := range v.Assets {
		if !isAssetType(a.Type) {
			return fmt.Errorf("invalid asset type: %q", a.Type)
		}
		for _, p := range a.Placements {
			if !isPlacementType(p.Type) {
				return fmt.Errorf("invalid placement type: %q", p.Type)
			}
			if !IsValidDest(p.Dest) {
				return fmt.Errorf("invalid destination path: %s", p.Dest)
			}
		}
	}
	for _, list := range [][]string{v.PreserveFiles, v.RemoveFiles} {
		for _, pattern := range list {
			if err := ValidateGlob(pattern); err != nil {
				return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
		}
	}
	return nil
}

func isAssetType(t AssetType) bool {
	for _, x := range assetTypes {
		if x == t {
			return true
		}
	}
	return false
}

func isPlacementType(t PlacementType) bool {
	for _, x := range placementTypes {
		if x == t {
			return true
		}
	}
	return false
}

// MarshalIndented encodes m like lip (System.Text.Json, indented); this is
// also the form embedded in tooth_lock.json.
func (m *Manifest) MarshalIndented() ([]byte, error) {
	return stjson.Marshal(m)
}

// MarshalSTJSON writes m in lip's property order. Like lip's records, the
// format version/uuid are always the current constants, nil lists are
// written as [] and URLs are written in Flurl's normalized form.
func (m Manifest) MarshalSTJSON(w *stjson.Writer) error {
	w.BeginObject()
	w.Key("format_version")
	w.Int(FormatVersion)
	w.Key("format_uuid")
	w.String(FormatUUID)
	w.Key("tooth")
	w.String(m.Tooth)
	w.Key("version")
	w.String(m.Version)
	w.Key("info")
	if err := w.Value(m.Info); err != nil {
		return err
	}
	w.Key("variants")
	w.BeginArray()
	for _, v := range m.Variants {
		w.BeginObject()
		w.Key("label")
		w.String(v.Label)
		w.Key("platform")
		w.String(v.Platform)
		w.Key("dependencies")
		if err := v.Dependencies.MarshalSTJSON(w); err != nil {
			return err
		}
		w.Key("assets")
		w.BeginArray()
		for _, a := range v.Assets {
			w.BeginObject()
			w.Key("type")
			w.String(string(a.Type))
			w.Key("urls")
			w.BeginArray()
			for _, u := range a.URLs {
				w.String(NormalizeURL(u))
			}
			w.EndArray()
			w.Key("placements")
			if err := w.Value(a.Placements); err != nil {
				return err
			}
			w.EndObject()
		}
		w.EndArray()
		w.Key("preserve_files")
		if err := w.Value(v.PreserveFiles); err != nil {
			return err
		}
		w.Key("remove_files")
		if err := w.Value(v.RemoveFiles); err != nil {
			return err
		}
		w.Key("scripts")
		if err := w.Value(v.Scripts); err != nil {
			return err
		}
		w.EndObject()
	}
	w.EndArray()
	w.EndObject()
	return nil
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
	var selected []*Variant
	for i := range m.Variants {
		v := &m.Variants[i]
		if v.Label != label {
			continue
		}
		if v.Platform != "" && v.Platform != rid && (match == nil || !match(v.Platform, rid)) {
			continue
		}
		selected = append(selected, v)
	}
	// lip's "all selected variants mismatch both label and platform" check
	// only holds for an empty selection.
	if len(selected) == 0 {
		return nil, fmt.Errorf("no variant found for label '%s' and platform '%s'", label, rid)
	}

	out := &Variant{
		Label:         label,
		Platform:      rid,
		Dependencies:  Dependencies{},
		Assets:        []Asset{},
		PreserveFiles: []string{},
		RemoveFiles:   []string{},
	}
	seen := map[PackageID]bool{}
	for _, v := range selected {
		for _, d := range v.Dependencies {
			id, err := ParsePackageID(d.ID)
			if err != nil {
				return nil, err
			}
			if seen[id] {
				return nil, fmt.Errorf("an item with the same key has already been added. Key: %s", id)
			}
			seen[id] = true
			out.Dependencies = append(out.Dependencies, d)
		}
		out.Assets = append(out.Assets, v.Assets...)
		out.PreserveFiles = append(out.PreserveFiles, v.PreserveFiles...)
		out.RemoveFiles = append(out.RemoveFiles, v.RemoveFiles...)
	}

	pick := func(name string, get func(*Scripts) []string) ([]string, error) {
		var found []string
		n := 0
		for _, v := range selected {
			if s := get(&v.Scripts); len(s) > 0 {
				found = s
				n++
			}
		}
		if n > 1 {
			return nil, fmt.Errorf("multiple selected variants define %s scripts", name)
		}
		if found == nil {
			return []string{}, nil
		}
		return append([]string(nil), found...), nil
	}
	var err error
	sc := &out.Scripts
	for _, f := range []struct {
		name string
		dst  *[]string
		get  func(*Scripts) []string
	}{
		{"pre_install", &sc.PreInstall, func(s *Scripts) []string { return s.PreInstall }},
		{"install", &sc.Install, func(s *Scripts) []string { return s.Install }},
		{"post_install", &sc.PostInstall, func(s *Scripts) []string { return s.PostInstall }},
		{"pre_uninstall", &sc.PreUninstall, func(s *Scripts) []string { return s.PreUninstall }},
		{"uninstall", &sc.Uninstall, func(s *Scripts) []string { return s.Uninstall }},
		{"post_uninstall", &sc.PostUninstall, func(s *Scripts) []string { return s.PostUninstall }},
	} {
		if *f.dst, err = pick(f.name, f.get); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// PackageID is "path" or "path#variant" (regex ^(?<path>[^#]+)(?:#(?<label>[^#]*))?$).
type PackageID struct {
	Path    string
	Variant string
}

// ParsePackageID parses s. The path must pass module.CheckPath and the
// variant must be a valid label.
func ParsePackageID(s string) (PackageID, error) {
	m := idRegex.FindStringSubmatch(s)
	if m == nil {
		return PackageID{}, fmt.Errorf("invalid package id: %s", s)
	}
	id := PackageID{Path: m[1], Variant: m[2]}
	if err := checkPath(id.Path); err != nil {
		return PackageID{}, fmt.Errorf("invalid package path: %s: %w", id.Path, err)
	}
	if !IsValidVariant(id.Variant) {
		return PackageID{}, fmt.Errorf("invalid package variant: %s", id.Variant)
	}
	return id, nil
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
	m := specRegex.FindStringSubmatch(s)
	if m == nil {
		return PackageSpec{}, fmt.Errorf("invalid package spec: %s", s)
	}
	id, err := ParsePackageID(m[1])
	if err != nil {
		return PackageSpec{}, err
	}
	return PackageSpec{ID: id, Version: m[2]}, nil
}

func (s PackageSpec) String() string { return s.ID.String() + "@" + s.Version }

// checkPath validates a package path with Go module path rules
// (lip uses a port of golang.org/x/mod/module.CheckPath).
func checkPath(path string) error { return module.CheckPath(path) }
