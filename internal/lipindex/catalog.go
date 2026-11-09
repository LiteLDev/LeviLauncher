package lipindex

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Searching and filtering the catalogue as levimc.org/mods does (the shared
// contract's A3, ported from mods/src/lib/catalog-query.ts and mirrored by
// frontend/src/utils/lipCatalog.ts). The launcher forces the side and the
// platform (TargetSide, TargetPlatform), so it has no Side or Platform
// filter; its one extra is the LeviLamina version.
//
// | Filter       | Matches                                                    |
// |--------------|------------------------------------------------------------|
// | Game version | any of them (OR), on GameVersions                          |
// | Tags         | every one (AND), none of the excluded; featured groups whole |
// | License      | any family or SPDX id (OR)                                 |
// | Dependencies | every bare id (AND), any case                              |
// | Author       | any owner (OR), any case                                   |

// hiddenPackages are parts of LeviLamina that the catalogue page hides.
var hiddenPackages = map[string]bool{"levilamina": true, "levilamina-loc": true, "crashlogger": true}

// Hidden reports whether the catalogue page hides p (LeviLamina itself and
// its own parts are managed with the loader).
func Hidden(p *Package) bool {
	name := strings.ToLower(strings.TrimSpace(p.Name))
	id := strings.ToLower(strings.TrimSpace(p.Identifier))
	parts := splitNonEmpty(id, "/")
	last := ""
	if len(parts) > 0 {
		last = parts[len(parts)-1]
	}
	return hiddenPackages[name] || hiddenPackages[id] || hiddenPackages[last]
}

// Sort keys of a catalogue search: exactly the site's.
const (
	// SortRelevance is the search score with a search, else the index's order.
	SortRelevance = "relevance"
	SortStars     = "stars"
	SortUpdated   = "updated"
	// SortNewest orders by repository creation, newest first.
	SortNewest = "newest"
)

// Sorts lists the sort keys.
var Sorts = []string{SortRelevance, SortStars, SortUpdated, SortNewest}

// Facet is the values a filter includes, and the values it excludes.
type Facet struct {
	Inc []string
	Exc []string
}

// TagFacet reads tag filter values as the site does: "-tag" excludes; a tag
// both included and excluded is excluded; hidden tags are dropped and
// aliases become their featured tag.
func TagFacet(values []string) Facet {
	var inc, exc []string
	for _, raw := range values {
		excluded := strings.HasPrefix(raw, "-")
		key := TagKey(strings.TrimPrefix(raw, "-"))
		if key == "" {
			continue
		}
		if excluded {
			if !contains(exc, key) {
				exc = append(exc, key)
			}
		} else if !contains(inc, key) {
			inc = append(inc, key)
		}
	}
	f := Facet{Inc: []string{}, Exc: exc}
	if f.Exc == nil {
		f.Exc = []string{}
	}
	for _, k := range inc {
		if !contains(exc, k) {
			f.Inc = append(f.Inc, k)
		}
	}
	return f
}

// Query is a catalogue search.
type Query struct {
	// Text is the search (SearchCatalog): every word must match.
	Text string
	// GameVersions keep packages that run on any of them (catalogue game
	// versions such as "1.21.50").
	GameVersions []string
	// Tags keep packages with every included tag key and none of the
	// excluded ones (see TagFacet).
	Tags Facet
	// License keeps packages under any of these families or SPDX ids.
	License []string
	// Deps keeps packages whose latest version depends on every one of
	// these bare ids (any case).
	Deps []string
	// Authors keeps packages of any of these owners (any case).
	Authors []string
	// LoaderVersion keeps packages whose preferred variant accepts this
	// LeviLamina version ("" = any); it needs DB.
	LoaderVersion string
	// DB is the LeviLamina version database; without it the LeviLamina
	// filter is off, as in the window.
	DB LoaderDB
	// Sort is one of Sorts ("" = SortRelevance).
	Sort string
}

var llFilterPattern = regexp.MustCompile(`(\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)`)

// NormalizeLoaderFilter extracts the semantic version from a LeviLamina
// version as the catalogue filter does ("v1.4.2" -> "1.4.2").
func NormalizeLoaderFilter(v string) string {
	n := NormalizeSemverInput(v)
	if m := llFilterPattern.FindStringSubmatch(n); m != nil {
		return m[1]
	}
	return n
}

var gameVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// CatalogGameVersion is a game version as the catalogue spells it: the
// first three numbers ("1.21.50.07" -> "1.21.50").
func CatalogGameVersion(v string) string {
	m := gameVersionPattern.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return strings.TrimSpace(v)
	}
	trim := func(s string) string {
		s = strings.TrimLeft(s, "0")
		if s == "" {
			return "0"
		}
		return s
	}
	return trim(m[1]) + "." + trim(m[2]) + "." + trim(m[3])
}

// facts are what the filters compare of a package.
type facts struct {
	tags    []string
	license string
	family  string
	deps    map[string]bool
	owner   string
}

func factsOf(p *Package) facts {
	f := facts{tags: TagKeys(p.Tags), license: strings.ToLower(p.License), family: LicenseFamily(p.License),
		deps: map[string]bool{}, owner: strings.ToLower(p.Owner)}
	for d := range p.Deps {
		f.deps[strings.ToLower(d)] = true
	}
	return f
}

type filters struct {
	gv      []string
	tags    Facet
	license []string
	deps    []string
	authors []string
}

func (q Query) filters() filters {
	return filters{gv: q.GameVersions, tags: q.Tags, license: q.License, deps: q.Deps, authors: q.Authors}
}

func anyOf(values []string, pred func(string) bool) bool {
	for _, v := range values {
		if pred(v) {
			return true
		}
	}
	return false
}

func (f filters) match(p *Package, x facts) bool {
	if len(f.gv) > 0 && !anyOf(f.gv, func(v string) bool { return contains(p.GameVersions, v) }) {
		return false
	}
	for _, k := range f.tags.Inc {
		if !contains(x.tags, k) {
			return false
		}
	}
	for _, k := range f.tags.Exc {
		if contains(x.tags, k) {
			return false
		}
	}
	if len(f.license) > 0 && !anyOf(f.license, func(v string) bool { return v == x.family || strings.ToLower(v) == x.license }) {
		return false
	}
	for _, d := range f.deps {
		if !x.deps[strings.ToLower(d)] {
			return false
		}
	}
	if len(f.authors) > 0 && !anyOf(f.authors, func(a string) bool { return strings.ToLower(a) == x.owner }) {
		return false
	}
	return true
}

// loaderFilter is the LeviLamina version filter: nil when off.
func (q Query) loaderFilter() func(*Package) bool {
	if len(q.DB) == 0 || strings.TrimSpace(q.LoaderVersion) == "" {
		return nil
	}
	ll := NormalizeLoaderFilter(q.LoaderVersion)
	return func(p *Package) bool { return len(p.LLRanges) > 0 && Compatible(ll, p.LLRanges) }
}

func updatedTime(s string) int64 {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

// sortPackages orders the results: relevance is the search score (with a
// search) then the index order; stars, updated and newest put the most or
// latest first. Ties go by identifier.
func sortPackages(list []*Package, key string, scores map[string]int) {
	cmp := func(a, b *Package) int {
		byStars := func() int {
			switch {
			case a.Stars != b.Stars:
				if b.Stars > a.Stars {
					return 1
				}
				return -1
			}
			return sign64(updatedTime(b.Updated) - updatedTime(a.Updated))
		}
		var d int
		switch key {
		case SortStars:
			d = byStars()
		case SortUpdated:
			d = sign64(updatedTime(b.Updated) - updatedTime(a.Updated))
			if d == 0 {
				d = signF(b.Stars - a.Stars)
			}
		case SortNewest:
			d = sign64(updatedTime(b.Created) - updatedTime(a.Created))
			if d == 0 {
				d = signF(b.Stars - a.Stars)
			}
		default:
			if scores != nil {
				d = scores[b.Identifier] - scores[a.Identifier]
			}
			if d == 0 {
				d = a.Order - b.Order
			}
		}
		if d != 0 {
			return d
		}
		return strings.Compare(strings.ToLower(a.Identifier), strings.ToLower(b.Identifier))
	}
	sort.SliceStable(list, func(i, j int) bool { return cmp(list[i], list[j]) < 0 })
}

func sign64(v int64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func signF(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// Visible is pkgs without the hidden ones (see Hidden).
func Visible(pkgs []*Package) []*Package {
	out := []*Package{}
	for _, p := range pkgs {
		if !Hidden(p) {
			out = append(out, p)
		}
	}
	return out
}

// Search filters and sorts packages. Hidden packages are dropped.
func Search(pkgs []*Package, q Query) []*Package {
	return Run(Visible(pkgs), q).Matched
}

// FacetOptions are the values every filter offers, from the whole catalogue.
type FacetOptions struct {
	// GameVersions, newest first.
	GameVersions []string
	// Tags: the featured tags some package has, in their table's order,
	// then every other by how many packages carry it.
	Tags []string
	// License: the families some package is under, in their order, each
	// with its SPDX ids by how many packages have them.
	License []LicenseOption
	// Deps: bare ids as the catalogue spells them, by how many packages
	// depend on them.
	Deps []string
	// Authors: owners, by how many packages they have.
	Authors []string
}

// LicenseOption is a license family and the SPDX ids under it.
type LicenseOption struct {
	Family string
	IDs    []string
}

// byFrequency lists values by how often they occur, then by name.
func byFrequency(values []string) []string {
	n := map[string]int{}
	var keys []string
	for _, v := range values {
		if _, ok := n[v]; !ok {
			keys = append(keys, v)
		}
		n[v]++
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if n[keys[i]] != n[keys[j]] {
			return n[keys[i]] > n[keys[j]]
		}
		return localeCompare(keys[i], keys[j]) < 0
	})
	return keys
}

// Options lists the values every filter offers for pkgs.
func Options(pkgs []*Package) FacetOptions {
	var o FacetOptions
	var tags, deps, authors, licenses []string
	for _, p := range pkgs {
		for _, g := range p.GameVersions {
			if !contains(o.GameVersions, g) {
				o.GameVersions = append(o.GameVersions, g)
			}
		}
		tags = append(tags, TagKeys(p.Tags)...)
		for d := range p.Deps {
			deps = append(deps, d)
		}
		authors = append(authors, p.Owner)
		licenses = append(licenses, p.License)
	}
	sort.SliceStable(o.GameVersions, func(i, j int) bool { return CompareVersions(o.GameVersions[i], o.GameVersions[j]) > 0 })
	counted := byFrequency(tags)
	for _, f := range FeaturedTags {
		if contains(counted, f.ID) {
			o.Tags = append(o.Tags, f.ID)
		}
	}
	for _, t := range counted {
		if !IsFeaturedTag(t) {
			o.Tags = append(o.Tags, t)
		}
	}
	for _, fam := range LicenseFamilies {
		var ids []string
		found := false
		for _, l := range licenses {
			if LicenseFamily(l) != fam {
				continue
			}
			found = true
			if l != "" {
				ids = append(ids, l)
			}
		}
		if found {
			o.License = append(o.License, LicenseOption{Family: fam, IDs: byFrequency(ids)})
		}
	}
	o.Deps = byFrequency(deps)
	o.Authors = byFrequency(authors)
	return o
}

// LicenseValues reads license filter values as the site's license=
// does: a family in any case becomes its id, an SPDX id takes the spelling
// some package of o has, and duplicates (any case) go. Values that are
// neither come back as unknown (the site drops them).
func LicenseValues(values []string, o FacetOptions) (known, unknown []string) {
	seen := map[string]bool{}
	add := func(v string) {
		if k := strings.ToLower(v); !seen[k] {
			seen[k] = true
			known = append(known, v)
		}
	}
	for _, v := range values {
		if IsLicenseFamily(strings.ToLower(v)) {
			add(strings.ToLower(v))
			continue
		}
		spelled := ""
		for _, lo := range o.License {
			for _, id := range lo.IDs {
				if spelled == "" && strings.EqualFold(id, v) {
					spelled = id
				}
			}
		}
		if spelled == "" {
			unknown = append(unknown, v)
			continue
		}
		add(spelled)
	}
	return known, unknown
}

// Counts are how many results each filter option would give: an OR
// filter's option (game version, license, author) as if it were the
// filter's only value; an AND filter's (tags, dependencies) as if added to
// it (an excluded tag as if included instead).
type Counts struct {
	GameVersions map[string]int
	Tags         map[string]int
	// License is keyed by family and by SPDX id.
	License map[string]int
	Deps    map[string]int
	Authors map[string]int
}

// Result is a search's results and counts.
type Result struct {
	Matched []*Package
	Counts  Counts
}

// Run applies a query to pkgs (Search drops the hidden packages first).
func Run(pkgs []*Package, q Query) Result {
	scores := SearchCatalog(pkgs, q.Text)
	ll := q.loaderFilter()
	var base []*Package
	fx := map[*Package]facts{}
	for _, p := range pkgs {
		if scores != nil {
			if _, ok := scores[p.Identifier]; !ok {
				continue
			}
		}
		if ll != nil && !ll(p) {
			continue
		}
		base = append(base, p)
		fx[p] = factsOf(p)
	}
	f := q.filters()
	matched := []*Package{}
	for _, p := range base {
		if f.match(p, fx[p]) {
			matched = append(matched, p)
		}
	}
	sortPackages(matched, q.Sort, scores)

	count := func(g filters) int {
		n := 0
		for _, p := range base {
			if g.match(p, fx[p]) {
				n++
			}
		}
		return n
	}
	withExtra := func(options, selected []string) []string {
		out := append([]string{}, options...)
		for _, s := range selected {
			if !anyOf(options, func(o string) bool { return strings.EqualFold(o, s) }) {
				out = append(out, s)
			}
		}
		return out
	}
	o := Options(pkgs)
	c := Counts{GameVersions: map[string]int{}, Tags: map[string]int{}, License: map[string]int{}, Deps: map[string]int{}, Authors: map[string]int{}}
	for _, v := range withExtra(o.GameVersions, f.gv) {
		g := f
		g.gv = []string{v}
		c.GameVersions[v] = count(g)
	}
	for _, k := range withExtra(o.Tags, append(append([]string{}, f.tags.Inc...), f.tags.Exc...)) {
		g := f
		inc := append([]string{}, f.tags.Inc...)
		if !contains(inc, k) {
			inc = append(inc, k)
		}
		var exc []string
		for _, x := range f.tags.Exc {
			if x != k {
				exc = append(exc, x)
			}
		}
		g.tags = Facet{Inc: inc, Exc: exc}
		c.Tags[k] = count(g)
	}
	licenseValues := append([]string{}, LicenseFamilies...)
	for _, lo := range o.License {
		licenseValues = append(licenseValues, lo.IDs...)
	}
	for _, v := range withExtra(licenseValues, f.license) {
		g := f
		g.license = []string{v}
		c.License[v] = count(g)
	}
	for _, d := range withExtra(o.Deps, f.deps) {
		g := f
		g.deps = append([]string{}, f.deps...)
		if !anyOf(f.deps, func(x string) bool { return strings.EqualFold(x, d) }) {
			g.deps = append(g.deps, d)
		}
		c.Deps[d] = count(g)
	}
	for _, a := range withExtra(o.Authors, f.authors) {
		g := f
		g.authors = []string{a}
		c.Authors[a] = count(g)
	}
	return Result{Matched: matched, Counts: c}
}

// ResolveGameVersion matches a game version against the catalogue's game
// versions (newest first): "latest" is the newest, an exact version itself,
// else a dotted prefix on segment boundaries ("1.21" -> the newest 1.21.x)
// or a longer version ("1.21.50.07" -> "1.21.50").
func ResolveGameVersion(options []string, want string) (string, bool) {
	w := strings.TrimSpace(want)
	if w == "" || len(options) == 0 {
		return "", false
	}
	if strings.EqualFold(w, "latest") {
		return options[0], true
	}
	w = NormalizeSemverInput(w)
	if contains(options, w) {
		return w, true
	}
	if c := CatalogGameVersion(w); contains(options, c) {
		return c, true
	}
	for _, o := range options {
		if HasVersionPrefix(o, w) {
			return o, true
		}
	}
	return "", false
}
