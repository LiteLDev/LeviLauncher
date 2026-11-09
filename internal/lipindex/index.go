package lipindex

import (
	"net/url"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
)

// LeviLaminaID is the catalogue identifier of LeviLamina.
const LeviLaminaID = "github.com/LiteLDev/LeviLamina"

const leviLaminaOwnerRepo = "liteldev/levilamina"

// Sides of an entry of a package file (lip.toml side).
const (
	SideServer = "server"
	SideClient = "client"
)

// TargetSide is the side the launcher installs for: always the client.
const TargetSide = SideClient

// TargetPlatform is the platform the launcher installs for, the .NET RID of
// this process (engine.CurrentPlatform: "win-x64" on Windows x64). Packages
// and variants without an entry for TargetSide and TargetPlatform are not
// offered. Tests set it to the platform of their fixtures.
var TargetPlatform = engine.CurrentPlatform()

// Package is a catalogue package as the launcher sees it: only packages
// with a variant that has an entry for TargetSide and TargetPlatform are
// kept. Versions, Files and LLRanges describe the preferred variant (rule
// A2 of the catalogue contract: "client", then "", then the others by
// name). The JSON names are a contract of `--json` output: keep them.
type Package struct {
	// Identifier is the catalogue id ("github.com/owner/repo").
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	// Description is the catalogue index's: cut to 200 characters as the
	// site lists and searches it.
	Description string `json:"description"`
	// FullDescription is the detail's whole description.
	FullDescription string `json:"-"`
	// Author is the repository owner, the same as Owner (kept for scripts).
	Author string `json:"author"`
	// Owner is the repository owner: the author, at https://github.com/<owner>.
	Owner      string   `json:"owner"`
	Tags       []string `json:"tags"`
	AvatarURL  string   `json:"avatarUrl"`
	ProjectURL string   `json:"projectUrl"`
	// License is the SPDX id, after the catalogue's overrides (LicenseOf).
	License       string `json:"license,omitempty"`
	LicenseFamily string `json:"licenseFamily"`
	// Stars is the GitHub star count.
	Stars   float64 `json:"stars"`
	Updated string  `json:"updated"`
	Created string  `json:"created,omitempty"`
	// Side and Platform are what entries of the latest version run on.
	Side     []string `json:"side"`
	Platform []string `json:"platform"`
	// GameVersions are the Minecraft versions any version runs on, newest
	// first, as the catalogue works them out.
	GameVersions []string `json:"gameVersions"`
	// Deps is what the latest version depends on: bare id (as the
	// catalogue names it) -> range (several ranges joined by " || "), the
	// package itself left out, as the catalogue index's deps.
	Deps map[string]string `json:"deps"`
	// Versions are the preferred variant's versions, newest first.
	Versions []string `json:"versions"`
	// LLRanges are the LeviLamina ranges any version of the preferred
	// variant depends on.
	LLRanges         []string  `json:"llRanges"`
	PreferredVariant string    `json:"preferredVariant"`
	Variants         []Variant `json:"variants"`
	// Files are the preferred variant's versions with their dependencies.
	Files []File `json:"-"`
	// Order is the package's position in the catalogue's index (its default
	// order, "relevance" without a search).
	Order int `json:"-"`
}

// Variant is a variant of a package that has an entry for TargetSide and
// TargetPlatform.
type Variant struct {
	// Key is the variant name; "" is the default variant.
	Key string `json:"key"`
	// Label is the variant name too (kept for scripts).
	Label string `json:"label"`
	// PackageIdentifier is what installs this variant: "<id>#<key>" ("<id>#"
	// for the default variant).
	PackageIdentifier string   `json:"packageIdentifier"`
	Versions          []string `json:"versions"`
	LLRanges          []string `json:"llRanges"`
	Files             []File   `json:"files"`
}

// File is one version of a variant: the dependencies of its entries for
// TargetSide and TargetPlatform.
type File struct {
	Version           string            `json:"version"`
	Prerelease        bool              `json:"prerelease,omitempty"`
	Dependencies      map[string]string `json:"dependencies"`
	LLRanges          []string          `json:"llRanges"`
	OtherDependencies map[string]string `json:"otherDependencies"`
}

// SelfVariantRelation lists the "#alias" variants a package's own entries
// depend on (LegacyScriptEngine#client_lua...). Installed mods whose
// folder or name is such an alias belong to the package.
type SelfVariantRelation struct {
	Identifier  string   `json:"identifier"`
	PackageName string   `json:"packageName"`
	Aliases     []string `json:"aliases"`
}

// DefaultVariant is the variant installed when none is chosen: the
// preferred variant (rule A2).
func (p *Package) DefaultVariant() (Variant, bool) {
	for _, v := range p.Variants {
		if v.Key == p.PreferredVariant {
			return v, true
		}
	}
	if len(p.Variants) > 0 {
		return p.Variants[0], true
	}
	return Variant{}, false
}

// FindVariant returns the variant named key (case-insensitive; "" is the
// default variant).
func (p *Package) FindVariant(key string) (Variant, bool) {
	for _, v := range p.Variants {
		if strings.EqualFold(v.Key, strings.TrimSpace(key)) {
			return v, true
		}
	}
	return Variant{}, false
}

// LatestVersion is the version the catalogue offers as an update: the
// first stable version, else the newest (pickTargetVersion).
func (p *Package) LatestVersion() string { return PickTargetVersion(p.Versions) }

// PickTargetVersion returns the first version without "-" in a newest-first
// list, else the first one.
func PickTargetVersion(versions []string) string {
	first := ""
	for _, v := range versions {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if first == "" {
			first = v
		}
		if IsStable(v) {
			return v
		}
	}
	return first
}

// Detail is a levimc.org/api/v4/mods/<id>/index.json document, reduced to the
// fields the launcher uses.
type Detail struct {
	ID           string          `json:"id"`
	Name         string          `json:"name,omitempty"`
	Description  string          `json:"description,omitempty"`
	Owner        string          `json:"owner,omitempty"`
	Logo         string          `json:"logo,omitempty"`
	Repository   string          `json:"repository,omitempty"`
	Stars        float64         `json:"stars,omitempty"`
	Updated      string          `json:"updated,omitempty"`
	Created      string          `json:"created,omitempty"`
	License      string          `json:"license,omitempty"`
	Side         []string        `json:"side,omitempty"`
	Platform     []string        `json:"platform,omitempty"`
	GameVersions []string        `json:"gameVersions,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	Versions     []DetailVersion `json:"versions"`
	// Latest names the version the catalogue offers (its deps describe it).
	Latest *DetailLatest `json:"latest,omitempty"`
	// Dependencies names every dependency as the catalogue does, by
	// lower-case bare id.
	Dependencies map[string]DependencyRef `json:"dependencies,omitempty"`
	// Readmes are not cached; only a direct fetch has them.
	Readmes []Readme `json:"readmes,omitempty"`
	// CatalogDescription is not in the document: it is the description the
	// catalogue index lists (mods[].description, cut to 200 characters),
	// which FetchDetails sets and the Cache keeps. The search reads it.
	CatalogDescription string `json:"catalogDescription,omitempty"`
	// CatalogDeps is not in the document either: the catalogue's deps as
	// UnmarshalJSON works them out from the document's order (see
	// catalogDeps), kept by the Cache.
	CatalogDeps map[string]string `json:"catalogDeps,omitempty"`
}

// DetailVersion is one version of a Detail, newest first.
type DetailVersion struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease,omitempty"`
	// Variants maps a variant name ("" is the default) to its entries.
	Variants map[string][]Entry `json:"variants"`
}

// Entry is an entry of a variant: where it runs and what it depends on.
type Entry struct {
	Side     string `json:"side"`
	Platform string `json:"platform"`
	// Deps maps a dependency key ("<id>[#<variant>]") to a version range.
	Deps map[string]string `json:"deps"`
}

// Readme is a README the catalogue extracted from the repository.
type Readme struct {
	File string `json:"file"`
	Lang string `json:"lang"`
	Text string `json:"text"`
}

// normalizeIdentifier lowercases an identifier and strips the scheme and
// surrounding slashes (normalizeLIPIdentifier).
func normalizeIdentifier(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimLeft(s, "/")
	return strings.TrimRight(s, "/")
}

func ownerRepo(v string) string {
	n := normalizeIdentifier(v)
	if n == "" {
		return ""
	}
	base := strings.SplitN(n, "#", 2)[0]
	parts := splitNonEmpty(base, "/")
	if len(parts) < 2 {
		return base
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// IsLeviLamina reports whether an identifier names LeviLamina, with any
// variant or prefix ("github.com/LiteLDev/LeviLamina#client").
func IsLeviLamina(v string) bool {
	n := normalizeIdentifier(v)
	if n == "" {
		return false
	}
	base := strings.SplitN(n, "#", 2)[0]
	return base == leviLaminaOwnerRepo || ownerRepo(base) == leviLaminaOwnerRepo
}

func splitNonEmpty(s, sep string) []string {
	out := []string{}
	for _, p := range strings.Split(s, sep) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// compareVariantNames orders variant names as rule A2 prefers them:
// "client", the default "", then the others by name.
func compareVariantNames(a, b string) int {
	rank := func(k string) int {
		switch k {
		case "client":
			return 0
		case "":
			return 1
		}
		return 2
	}
	if d := rank(a) - rank(b); d != 0 {
		return d
	}
	return strings.Compare(a, b)
}

// QualifyingVariants are the variant names with an entry for side (and
// platform, when not ""), in rule A2's order.
func QualifyingVariants(variants map[string][]Entry, side, platform string) []string {
	out := []string{}
	for name, entries := range variants {
		for _, e := range entries {
			if e.Side == side && (platform == "" || strings.EqualFold(e.Platform, platform)) {
				out = append(out, name)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return compareVariantNames(out[i], out[j]) < 0 })
	return out
}

// PreferredVariant is the variant to install for a side and platform (rule
// A2): among those with an entry for them, "client", then the default "",
// then the others by name; ok is false when none has one.
func PreferredVariant(variants map[string][]Entry, side, platform string) (string, bool) {
	q := QualifyingVariants(variants, side, platform)
	if len(q) == 0 {
		return "", false
	}
	return q[0], true
}

// ChooseVariant is the variant to install from a detail for side and
// platform: rule A2 over the version named (when the detail has it), else
// over the newest version that has a qualifying variant.
func ChooseVariant(d *Detail, version, side, platform string) (string, bool) {
	if d == nil {
		return "", false
	}
	if want := NormalizeSemverInput(version); want != "" {
		for _, v := range d.Versions {
			if strings.EqualFold(NormalizeSemverInput(v.Version), want) {
				return PreferredVariant(v.Variants, side, platform)
			}
		}
	}
	for _, v := range d.Versions {
		if name, ok := PreferredVariant(v.Variants, side, platform); ok {
			return name, true
		}
	}
	return "", false
}

func variantPackageIdentifier(identifier, key string) string {
	base := strings.TrimSpace(strings.SplitN(strings.TrimSpace(identifier), "#", 2)[0])
	if base == "" {
		return ""
	}
	return base + "#" + strings.TrimSpace(key)
}

// variantsOf lists the variants of a detail with an entry for side and
// platform in some version, each with the versions that have one, in rule
// A2's order.
func variantsOf(d *Detail, identifier, side, platform string) []Variant {
	names := []string{}
	for _, ver := range d.Versions {
		for _, n := range QualifyingVariants(ver.Variants, side, platform) {
			if !contains(names, n) {
				names = append(names, n)
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return compareVariantNames(names[i], names[j]) < 0 })
	out := []Variant{}
	for _, key := range names {
		v := Variant{Key: key, Label: key, PackageIdentifier: variantPackageIdentifier(identifier, key),
			Versions: []string{}, LLRanges: []string{}, Files: []File{}}
		for _, ver := range d.Versions {
			version := strings.TrimSpace(ver.Version)
			if version == "" || contains(v.Versions, version) {
				continue
			}
			matched := false
			f := File{Version: version, Prerelease: ver.Prerelease, Dependencies: map[string]string{}, LLRanges: []string{},
				OtherDependencies: map[string]string{}}
			for _, e := range ver.Variants[key] {
				if e.Side != side || !strings.EqualFold(e.Platform, platform) {
					continue
				}
				matched = true
				depKeys := make([]string, 0, len(e.Deps))
				for k := range e.Deps {
					depKeys = append(depKeys, k)
				}
				sort.Strings(depKeys)
				for _, k := range depKeys {
					nk, rng := strings.TrimSpace(k), strings.TrimSpace(e.Deps[k])
					if nk == "" || rng == "" {
						continue
					}
					f.Dependencies[nk] = rng
					if IsLeviLamina(nk) {
						if !contains(f.LLRanges, rng) {
							f.LLRanges = append(f.LLRanges, rng)
						}
						continue
					}
					f.OtherDependencies[nk] = rng
				}
			}
			if !matched {
				continue
			}
			v.Files = append(v.Files, f)
			v.Versions = append(v.Versions, version)
		}
		if len(v.Files) == 0 {
			continue
		}
		// The catalogue lists versions newest first; keep that order however
		// a document lists them.
		sort.SliceStable(v.Files, func(i, j int) bool { return CompareSemverLike(v.Files[i].Version, v.Files[j].Version) > 0 })
		for i, f := range v.Files {
			v.Versions[i] = f.Version
			for _, r := range f.LLRanges {
				if !contains(v.LLRanges, r) {
					v.LLRanges = append(v.LLRanges, r)
				}
			}
		}
		out = append(out, v)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func normalizeHTTPURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

// inferAuthor is the owner segment of an identifier.
func inferAuthor(identifier string) string {
	parts := splitNonEmpty(identifier, "/")
	if len(parts) >= 3 && strings.Contains(parts[0], ".") {
		return parts[1]
	}
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// inferProjectURL is https://github.com/owner/repo for a two-part id,
// otherwise https://<id>.
func inferProjectURL(identifier string) string {
	if direct := normalizeHTTPURL(identifier); direct != "" {
		return direct
	}
	n := strings.Trim(strings.TrimSpace(identifier), "/")
	if n == "" || !strings.Contains(n, "/") {
		return ""
	}
	parts := splitNonEmpty(n, "/")
	if len(parts) == 2 && !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") {
		return normalizeHTTPURL("https://github.com/" + parts[0] + "/" + parts[1])
	}
	return normalizeHTTPURL("https://" + n)
}

// OwnerURL is the GitHub page of an owner: the author's link.
func OwnerURL(owner string) string {
	if strings.TrimSpace(owner) == "" {
		return ""
	}
	return "https://github.com/" + url.PathEscape(strings.TrimSpace(owner))
}

// ParsePackage converts a detail into a Package for TargetSide and
// TargetPlatform; ok is false when no variant of it has an entry for them
// (the launcher cannot install it).
func ParsePackage(d *Detail) (*Package, bool) {
	return ParsePackageFor(d, TargetSide, TargetPlatform)
}

// ParsePackageFor converts a detail into a Package for side and platform.
func ParsePackageFor(d *Detail, side, platform string) (*Package, bool) {
	if d == nil {
		return nil, false
	}
	id := strings.TrimSpace(d.ID)
	if id == "" {
		return nil, false
	}
	variants := variantsOf(d, id, side, platform)
	if len(variants) == 0 {
		return nil, false
	}
	preferred := variants[0]
	if key, ok := ChooseVariant(d, "", side, platform); ok {
		for _, v := range variants {
			if v.Key == key {
				preferred = v
				break
			}
		}
	}
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = id
	}
	owner := strings.TrimSpace(d.Owner)
	if owner == "" {
		owner = inferAuthor(id)
	}
	license := LicenseOf(id, strings.TrimSpace(d.License))
	project := normalizeHTTPURL(d.Repository)
	if project == "" {
		project = inferProjectURL(id)
	}
	nonNil := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	p := &Package{
		Identifier:       id,
		Name:             name,
		Description:      catalogDescription(d),
		FullDescription:  strings.TrimSpace(d.Description),
		Author:           owner,
		Owner:            owner,
		Tags:             nonNil(d.Tags),
		AvatarURL:        normalizeHTTPURL(d.Logo),
		ProjectURL:       project,
		License:          license,
		LicenseFamily:    LicenseFamily(license),
		Stars:            d.Stars,
		Updated:          strings.TrimSpace(d.Updated),
		Created:          strings.TrimSpace(d.Created),
		Side:             nonNil(d.Side),
		Platform:         nonNil(d.Platform),
		GameVersions:     nonNil(d.GameVersions),
		Deps:             latestDeps(d),
		Versions:         preferred.Versions,
		LLRanges:         preferred.LLRanges,
		PreferredVariant: preferred.Key,
		Variants:         variants,
		Files:            preferred.Files,
	}
	return p, true
}

// selfVariantAliases lists the "#alias" suffixes of the dependencies of a
// detail's entries that point back to the package itself, lower case.
func selfVariantAliases(d *Detail, identifier string) []string {
	base := strings.SplitN(normalizeIdentifier(identifier), "#", 2)[0]
	if base == "" {
		return nil
	}
	set := map[string]bool{}
	for _, ver := range d.Versions {
		for _, entries := range ver.Variants {
			for _, e := range entries {
				for k := range e.Deps {
					n := normalizeIdentifier(k)
					i := strings.Index(n, "#")
					if i < 0 || n[:i] != base {
						continue
					}
					if alias := strings.ToLower(strings.TrimSpace(n[i+1:])); alias != "" {
						set[alias] = true
					}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return localeCompare(out[i], out[j]) < 0 })
	return out
}

// Index is the parsed catalogue.
type Index struct {
	// Packages are the packages the launcher can install, in the order of
	// the details given to Build (the catalogue index's order).
	Packages     []*Package
	SelfVariants []SelfVariantRelation
	byID         map[string]*Package
}

// Build parses details into an Index (parseLIPIndexResult). Packages keep
// the order of details, which is the catalogue index's order when they come
// from FetchDetails or the Cache: "relevance" without a search.
func Build(details []*Detail) *Index {
	idx := &Index{Packages: []*Package{}, SelfVariants: []SelfVariantRelation{}, byID: map[string]*Package{}}
	rel := map[string]*SelfVariantRelation{}
	var relOrder []string
	for _, d := range details {
		if d == nil {
			continue
		}
		p, ok := ParsePackage(d)
		if ok {
			if _, dup := idx.byID[p.Identifier]; !dup {
				p.Order = len(idx.Packages)
				idx.byID[p.Identifier] = p
				idx.Packages = append(idx.Packages, p)
			}
		}
		aliases := selfVariantAliases(d, d.ID)
		if len(aliases) == 0 {
			continue
		}
		id := strings.TrimSpace(d.ID)
		if ok {
			id = p.Identifier
		}
		if id == "" {
			continue
		}
		r := rel[id]
		if r == nil {
			name := id
			if ok {
				name = p.Name
			}
			r = &SelfVariantRelation{Identifier: id, PackageName: name}
			rel[id] = r
			relOrder = append(relOrder, id)
		}
		for _, a := range aliases {
			if !contains(r.Aliases, a) {
				r.Aliases = append(r.Aliases, a)
			}
		}
		sort.Slice(r.Aliases, func(i, j int) bool { return localeCompare(r.Aliases[i], r.Aliases[j]) < 0 })
	}
	for _, id := range relOrder {
		idx.SelfVariants = append(idx.SelfVariants, *rel[id])
	}
	sort.SliceStable(idx.SelfVariants, func(i, j int) bool {
		a, b := idx.SelfVariants[i], idx.SelfVariants[j]
		return localeCompare(firstNonEmpty(a.PackageName, a.Identifier), firstNonEmpty(b.PackageName, b.Identifier)) < 0
	})
	return idx
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// Find looks a package up as the window does (exact, then without
// "#variant", then ignoring case), and also by lookup key so "owner/repo"
// finds "github.com/owner/repo".
func (idx *Index) Find(identifier string) (*Package, bool) {
	if idx == nil {
		return nil, false
	}
	target := strings.TrimSpace(identifier)
	if target == "" {
		return nil, false
	}
	if p, ok := idx.byID[target]; ok {
		return p, true
	}
	base := strings.SplitN(target, "#", 2)[0]
	if p, ok := idx.byID[base]; ok {
		return p, true
	}
	for id, p := range idx.byID {
		l := strings.ToLower(id)
		if l == strings.ToLower(target) || l == strings.ToLower(base) {
			return p, true
		}
	}
	key := LookupKey(target)
	if key == "" {
		return nil, false
	}
	for _, p := range idx.Packages {
		if LookupKey(p.Identifier) == key {
			return p, true
		}
	}
	return nil, false
}

// Add puts a package into the index (a detail fetched directly).
func (idx *Index) Add(p *Package) {
	if idx == nil || p == nil {
		return
	}
	if idx.byID == nil {
		idx.byID = map[string]*Package{}
	}
	if old, ok := idx.byID[p.Identifier]; ok {
		*old = *p
		return
	}
	idx.byID[p.Identifier] = p
	idx.Packages = append(idx.Packages, p)
}

// LookupKey is the key packages are compared by: lowercase, no scheme,
// "@version" or "#variant", and "owner/repo" as "github.com/owner/repo"
// (normalizeIdentifierLookupKey).
func LookupKey(v string) string {
	key := strings.ToLower(strings.TrimSpace(v))
	key = strings.TrimPrefix(key, "https://")
	key = strings.TrimPrefix(key, "http://")
	if i := strings.Index(key, "@"); i >= 0 {
		key = key[:i]
	}
	if i := strings.Index(key, "#"); i >= 0 {
		key = key[:i]
	}
	parts := splitNonEmpty(key, "/")
	if len(parts) == 2 && !strings.Contains(parts[0], ".") {
		return "github.com/" + strings.Join(parts, "/")
	}
	return strings.Join(parts, "/")
}

// Reference is a package reference typed by a user:
// "owner/repo", "github.com/owner/repo" or a URL, optionally followed by
// "#variant" and "@version".
type Reference struct {
	// ID is the catalogue identifier: "github.com/owner/repo".
	ID      string
	Variant string
	// HasVariant is set when the reference names a variant, "" (a "#"
	// with nothing after it) included.
	HasVariant bool
	Version    string
}

// ParseReference parses a package reference. ok is false when s is not
// shaped like a package (it needs an owner and a repository).
func ParseReference(s string) (Reference, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Reference{}, false
	}
	var r Reference
	if i := strings.LastIndex(s, "@"); i >= 0 {
		r.Version = strings.TrimSpace(s[i+1:])
		s = strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, "#"); i >= 0 {
		r.Variant = strings.TrimSpace(s[i+1:])
		r.HasVariant = true
		s = strings.TrimSpace(s[:i])
	}
	l := strings.ToLower(s)
	if strings.HasPrefix(l, "https://") {
		s = s[len("https://"):]
	} else if strings.HasPrefix(l, "http://") {
		s = s[len("http://"):]
	}
	parts := splitNonEmpty(strings.Trim(s, "/"), "/")
	switch {
	case len(parts) == 2 && !strings.Contains(parts[0], "."):
		r.ID = "github.com/" + parts[0] + "/" + parts[1]
	case len(parts) == 3 && strings.Contains(parts[0], "."):
		r.ID = strings.Join(parts, "/")
	default:
		return Reference{}, false
	}
	for _, p := range parts {
		if strings.ContainsAny(p, ` \:*?"<>|`) {
			return Reference{}, false
		}
	}
	return r, true
}

// DetailURL is the catalogue document of an identifier (buildLIPModDetailUrl).
func DetailURL(base, identifier string) string {
	path := strings.SplitN(strings.TrimSpace(identifier), "#", 2)[0]
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.TrimRight(base, "/") + "/" + strings.Join(segs, "/") + "/index.json"
}
