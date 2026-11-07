package lipindex

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// LeviLaminaID is the catalogue identifier of LeviLamina.
const LeviLaminaID = "github.com/LiteLDev/LeviLamina"

const leviLaminaOwnerRepo = "liteldev/levilamina"

// Package is a catalogue package as the launcher sees it: only packages
// with a "client" variant are kept. Versions, Files and LLRanges describe
// the preferred (client) variant.
type Package struct {
	// Identifier is the catalogue id ("github.com/owner/repo").
	Identifier  string   `json:"identifier"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	AvatarURL   string   `json:"avatarUrl"`
	ProjectURL  string   `json:"projectUrl"`
	License     string   `json:"license,omitempty"`
	// Stars is the GitHub star count ("hotness" in the window).
	Stars   float64 `json:"stars"`
	Updated string  `json:"updated"`
	// Versions are the preferred variant's versions, newest first.
	Versions []string `json:"versions"`
	// LLRanges are the LeviLamina ranges any version of the preferred
	// variant depends on.
	LLRanges         []string  `json:"llRanges"`
	PreferredVariant string    `json:"preferredVariant"`
	Variants         []Variant `json:"variants"`
	// Files are the preferred variant's versions with their dependencies.
	Files []File `json:"-"`
}

// Variant is one variant (tooth.json "label") of a package.
type Variant struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// PackageIdentifier is what installs this variant: "<id>#<key>".
	PackageIdentifier string   `json:"packageIdentifier"`
	Versions          []string `json:"versions"`
	LLRanges          []string `json:"llRanges"`
	Files             []File   `json:"files"`
}

// File is one version of a variant.
type File struct {
	Version           string            `json:"version"`
	Dependencies      map[string]string `json:"dependencies"`
	LLRanges          []string          `json:"llRanges"`
	OtherDependencies map[string]string `json:"otherDependencies"`
}

// SelfVariantRelation lists the "#alias" variants a package's own
// dependencies name (LegacyScriptEngine#lua...). Installed mods whose
// folder or name is such an alias belong to the package.
type SelfVariantRelation struct {
	Identifier  string   `json:"identifier"`
	PackageName string   `json:"packageName"`
	Aliases     []string `json:"aliases"`
}

// IsClientVariant reports whether a variant targets the client (its key
// contains "client"), the only variants the launcher installs.
func (v Variant) IsClientVariant() bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(v.Key)), "client")
}

// ClientVariants returns the client variants of p.
func (p *Package) ClientVariants() []Variant {
	out := []Variant{}
	for _, v := range p.Variants {
		if v.IsClientVariant() {
			out = append(out, v)
		}
	}
	return out
}

// DefaultVariant is the variant installed when none is chosen: the
// preferred variant if it is a client variant, else the first client
// variant (LIPInstallDialog resolveDefaultLIPVariant).
func (p *Package) DefaultVariant() (Variant, bool) {
	vs := p.ClientVariants()
	if len(vs) == 0 {
		return Variant{}, false
	}
	pref := strings.ToLower(strings.TrimSpace(p.PreferredVariant))
	if pref != "" {
		for _, v := range vs {
			if strings.ToLower(strings.TrimSpace(v.Key)) == pref {
				return v, true
			}
		}
	}
	return vs[0], true
}

// FindVariant returns the variant with key (case-insensitive).
func (p *Package) FindVariant(key string) (Variant, bool) {
	for _, v := range p.Variants {
		if strings.EqualFold(strings.TrimSpace(v.Key), strings.TrimSpace(key)) {
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

// Detail is a levimc.org/mods/<id>/index.json document, reduced to the
// fields the launcher uses. Values that the window reads loosely are kept
// as any.
type Detail struct {
	Tooth       string          `json:"tooth"`
	Name        any             `json:"name,omitempty"`
	Description any             `json:"description,omitempty"`
	Icon        any             `json:"icon,omitempty"`
	Stars       any             `json:"stars,omitempty"`
	Updated     any             `json:"updated,omitempty"`
	Tags        any             `json:"tags,omitempty"`
	License     any             `json:"license,omitempty"`
	Repository  any             `json:"repository,omitempty"`
	Versions    []DetailVersion `json:"versions"`
	// Readmes are not cached; only a direct fetch has them.
	Readmes []Readme `json:"readmes,omitempty"`
}

// DetailVersion is one version of a Detail.
type DetailVersion struct {
	Version  any             `json:"version"`
	Variants []DetailVariant `json:"variants"`
}

// DetailVariant is one variant of a DetailVersion.
type DetailVariant struct {
	Label        any            `json:"label"`
	Dependencies map[string]any `json:"dependencies"`
}

// Readme is a README the catalogue extracted from the repository.
type Readme struct {
	File string `json:"file"`
	Lang string `json:"lang"`
	Text string `json:"text"`
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strList(v any) []string {
	items, _ := v.([]any)
	out := []string{}
	for _, it := range items {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// number converts a JSON value to a number as JavaScript's Number() does
// for the values the catalogue uses; ok is false for missing or empty.
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		if strings.TrimSpace(x) == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
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

func variantRank(key string) int {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "client":
		return 0
	case "":
		return 1
	}
	return 2
}

func variantPackageIdentifier(identifier, key string) string {
	base := strings.TrimSpace(strings.SplitN(strings.TrimSpace(identifier), "#", 2)[0])
	if base == "" {
		return ""
	}
	return base + "#" + strings.TrimSpace(key)
}

// variantsOf groups a detail's versions by variant label
// (convertLIPModDetail + parsePackageVariants).
func variantsOf(d *Detail, identifier string) []Variant {
	type acc struct {
		files map[string]File
		order []string
	}
	byKey := map[string]*acc{}
	var keys []string
	for _, ver := range d.Versions {
		version := strings.TrimSpace(str(ver.Version))
		if version == "" {
			continue
		}
		for _, va := range ver.Variants {
			key := strings.TrimSpace(str(va.Label))
			a := byKey[key]
			if a == nil {
				a = &acc{files: map[string]File{}}
				byKey[key] = a
				keys = append(keys, key)
			}
			f := File{Version: version, Dependencies: map[string]string{}, LLRanges: []string{}, OtherDependencies: map[string]string{}}
			depKeys := make([]string, 0, len(va.Dependencies))
			for k := range va.Dependencies {
				depKeys = append(depKeys, k)
			}
			sort.Strings(depKeys)
			for _, k := range depKeys {
				nk := strings.TrimSpace(k)
				rng := strings.TrimSpace(str(va.Dependencies[k]))
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
			if _, dup := a.files[version]; !dup {
				a.order = append(a.order, version)
			}
			a.files[version] = f
		}
	}
	out := []Variant{}
	for _, key := range keys {
		a := byKey[key]
		files := make([]File, 0, len(a.order))
		for _, v := range a.order {
			files = append(files, a.files[v])
		}
		if len(files) == 0 {
			continue
		}
		sort.SliceStable(files, func(i, j int) bool { return CompareSemverLike(files[i].Version, files[j].Version) > 0 })
		v := Variant{Key: key, Label: key, PackageIdentifier: variantPackageIdentifier(identifier, key),
			Files: files, Versions: []string{}, LLRanges: []string{}}
		for _, f := range files {
			v.Versions = append(v.Versions, f.Version)
			for _, r := range f.LLRanges {
				if !contains(v.LLRanges, r) {
					v.LLRanges = append(v.LLRanges, r)
				}
			}
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := variantRank(out[i].Key), variantRank(out[j].Key); ri != rj {
			return ri < rj
		}
		return localeCompare(out[i].PackageIdentifier, out[j].PackageIdentifier) < 0
	})
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

// ParsePackage converts a detail into a Package; ok is false when the
// detail has no "client" variant (the launcher only installs those).
func ParsePackage(d *Detail) (*Package, bool) {
	if d == nil {
		return nil, false
	}
	id := strings.TrimSpace(d.Tooth)
	if id == "" {
		return nil, false
	}
	variants := variantsOf(d, id)
	if len(variants) == 0 {
		return nil, false
	}
	var preferred *Variant
	for i := range variants {
		if strings.ToLower(variants[i].Key) == "client" {
			preferred = &variants[i]
			break
		}
	}
	if preferred == nil {
		return nil, false
	}
	name := strings.TrimSpace(str(d.Name))
	if name == "" {
		name = id
	}
	stars, _ := number(d.Stars)
	p := &Package{
		Identifier:       id,
		Name:             name,
		Description:      strings.TrimSpace(str(d.Description)),
		Author:           inferAuthor(id),
		Tags:             strList(d.Tags),
		AvatarURL:        normalizeHTTPURL(str(d.Icon)),
		ProjectURL:       inferProjectURL(id),
		License:          strings.TrimSpace(str(d.License)),
		Stars:            stars,
		Updated:          strings.TrimSpace(str(d.Updated)),
		Versions:         preferred.Versions,
		LLRanges:         preferred.LLRanges,
		PreferredVariant: preferred.Key,
		Variants:         variants,
		Files:            preferred.Files,
	}
	return p, true
}

// selfVariantAliases lists the "#alias" suffixes of dependencies that point
// back to the package itself.
func selfVariantAliases(d *Detail, identifier string) []string {
	base := strings.SplitN(normalizeIdentifier(identifier), "#", 2)[0]
	if base == "" {
		return nil
	}
	set := map[string]bool{}
	for _, ver := range d.Versions {
		for _, va := range ver.Variants {
			for k := range va.Dependencies {
				n := normalizeIdentifier(k)
				if n == "" {
					continue
				}
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
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return localeCompare(out[i], out[j]) < 0 })
	return out
}

// Index is the parsed catalogue.
type Index struct {
	// Packages are the packages with a client variant, by identifier.
	Packages     []*Package
	SelfVariants []SelfVariantRelation
	byID         map[string]*Package
}

// Build parses details into an Index (parseLIPIndexResult). Packages are
// sorted by identifier so results do not depend on fetch order.
func Build(details []*Detail) *Index {
	sorted := append([]*Detail(nil), details...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Tooth < sorted[j].Tooth })
	idx := &Index{Packages: []*Package{}, SelfVariants: []SelfVariantRelation{}, byID: map[string]*Package{}}
	rel := map[string]*SelfVariantRelation{}
	var relOrder []string
	for _, d := range sorted {
		if d == nil {
			continue
		}
		p, ok := ParsePackage(d)
		if ok {
			if _, dup := idx.byID[p.Identifier]; !dup {
				idx.byID[p.Identifier] = p
				idx.Packages = append(idx.Packages, p)
			}
		}
		aliases := selfVariantAliases(d, d.Tooth)
		if len(aliases) == 0 {
			continue
		}
		id := strings.TrimSpace(d.Tooth)
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
	Version string
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
