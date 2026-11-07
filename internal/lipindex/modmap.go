package lipindex

import (
	"regexp"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/types"
)

// Source of a mod, as the mapping to the catalogue decides it.
const (
	SourceNone       = "none"       // no catalogue package matches
	SourceAmbiguous  = "ambiguous"  // several packages match
	SourceUnique     = "unique"     // exactly one package matches
	SourceLeviLamina = "levilamina" // LeviLamina itself
)

// How a unique mapping was found.
const (
	MappingNone        = "none"
	MappingName        = "name"
	MappingSelfVariant = "self_variant"
)

// ModState maps one installed mod to the catalogue (ModLIPState).
type ModState struct {
	Source        string `json:"source"`
	Identifier    string `json:"identifier"`
	IdentifierKey string `json:"identifierKey"`
	// TargetVersion is the version an update installs.
	TargetVersion string `json:"targetVersion"`
	CanUpdate     bool   `json:"canUpdate"`
	PackageName   string `json:"packageName"`
	Mapping       string `json:"mapping"`
	MatchedAlias  string `json:"matchedAlias"`
}

// InstallState is a package's state in an instance's workspace.
type InstallState struct {
	Installed        bool
	Explicit         bool
	InstalledVersion string
	Error            string
}

// IsLeviLaminaMod reports whether a mod is LeviLamina itself.
func IsLeviLaminaMod(m types.ModInfo) bool {
	return strings.ToLower(strings.TrimSpace(m.Name)) == "levilamina"
}

// ModFolder is a mod's folder, falling back to its name.
func ModFolder(m types.ModInfo) string {
	if f := strings.TrimSpace(m.Folder); f != "" {
		return f
	}
	return strings.TrimSpace(m.Name)
}

func normalizeName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

var spaces = regexp.MustCompile(`\s+`)

func normalizeMatch(s string) string {
	return spaces.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "")
}

func normalizeVersionForCompare(s string) string {
	return strings.ToLower(NormalizeSemverInput(s))
}

var (
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	nonAlnum      = regexp.MustCompile(`[^a-z0-9]+`)
	sepRun        = regexp.MustCompile(`[_-]+`)
	genericPrefix = regexp.MustCompile(`^(client|server|mod|mods|plugin|plugins|pack|package)[_-]`)
)

func slugify(s string) string {
	s = camelBoundary.ReplaceAllString(s, "$1-$2")
	s = nonAlnum.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

var genericTokens = map[string]bool{
	"client": true, "server": true, "mod": true, "mods": true,
	"plugin": true, "plugins": true, "pack": true, "package": true,
}

// aliasTokens are the words of an alias that identify a mod
// (buildAliasMatchTokens).
func aliasTokens(alias string) []string {
	n := normalizeMatch(alias)
	if n == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(v string) {
		t := normalizeMatch(v)
		if len(t) < 3 || genericTokens[t] || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	add(n)
	add(strings.ReplaceAll(n, "_", "-"))
	add(strings.ReplaceAll(n, "-", "_"))
	if stripped := genericPrefix.ReplaceAllString(n, ""); stripped != "" && stripped != n {
		add(stripped)
		add(strings.ReplaceAll(stripped, "_", "-"))
		add(strings.ReplaceAll(stripped, "-", "_"))
	}
	for _, p := range sepRun.Split(n, -1) {
		add(p)
	}
	var parts []string
	for _, p := range sepRun.Split(n, -1) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) > 1 {
		add(parts[len(parts)-1])
	}
	return out
}

// packageHints are slugs of the package name and repository that a mod's
// name must contain for a fuzzy alias match (buildPackageHints).
func packageHints(name, identifier string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(v string) {
		s := slugify(v)
		if len(s) < 4 {
			return
		}
		for _, x := range []string{s, strings.ReplaceAll(s, "-", "")} {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	add(name)
	parts := splitNonEmpty(strings.SplitN(LookupKey(identifier), "#", 2)[0], "/")
	if len(parts) > 0 {
		add(parts[len(parts)-1])
	}
	if len(parts) > 1 {
		add(parts[len(parts)-2] + "-" + parts[len(parts)-1])
	}
	return out
}

func tokenInValue(token, raw string) bool {
	value := normalizeMatch(raw)
	if token == "" || value == "" {
		return false
	}
	tvs := []string{token, strings.ReplaceAll(token, "_", "-"), strings.ReplaceAll(token, "-", "_")}
	vvs := []string{value, strings.ReplaceAll(value, "_", "-"), strings.ReplaceAll(value, "-", "_")}
	for _, t := range tvs {
		for _, v := range vvs {
			if v == t || strings.HasSuffix(v, "-"+t) || strings.HasSuffix(v, "_"+t) {
				return true
			}
			for _, w := range sepRun.Split(v, -1) {
				if w == t {
					return true
				}
			}
		}
	}
	return false
}

func hintInValue(hint, raw string) bool {
	value := normalizeMatch(raw)
	if hint == "" || value == "" {
		return false
	}
	if strings.Contains(value, hint) {
		return true
	}
	h := strings.ReplaceAll(hint, "-", "")
	return h != "" && strings.Contains(value, h)
}

type aliasCandidate struct {
	identifier    string
	identifierKey string
	lookupKey     string
	packageName   string
	alias         string
	tokens        []string
	hints         []string
}

func (c aliasCandidate) fuzzyMatches(values []string) bool {
	hit := false
	for _, t := range c.tokens {
		for _, v := range values {
			if tokenInValue(t, v) {
				hit = true
			}
		}
	}
	if !hit {
		return false
	}
	if len(c.hints) == 0 {
		return true
	}
	for _, h := range c.hints {
		for _, v := range values {
			if hintInValue(h, v) {
				return true
			}
		}
	}
	return false
}

// Mapper maps installed mods to catalogue packages.
type Mapper struct {
	candidates []aliasCandidate
	byAlias    map[string][]aliasCandidate
	byName     map[string][]*Package
	byKey      map[string]*Package
}

// NewMapper prepares the lookups of the catalogue (all packages, hidden or
// not, as the window does).
func NewMapper(idx *Index) *Mapper {
	m := &Mapper{byAlias: map[string][]aliasCandidate{}, byName: map[string][]*Package{}, byKey: map[string]*Package{}}
	if idx == nil {
		return m
	}
	for _, p := range idx.Packages {
		if n := normalizeName(p.Name); n != "" {
			m.byName[n] = append(m.byName[n], p)
		}
		if k := LookupKey(p.Identifier); k != "" {
			if _, ok := m.byKey[k]; !ok {
				m.byKey[k] = p
			}
		}
	}
	seen := map[string]bool{}
	for _, r := range idx.SelfVariants {
		id := strings.TrimSpace(r.Identifier)
		key := LookupKey(id)
		if id == "" || key == "" {
			continue
		}
		name := firstNonEmpty(strings.TrimSpace(r.PackageName), id)
		hints := packageHints(name, id)
		for _, a := range r.Aliases {
			alias := normalizeName(a)
			if alias == "" || seen[key+"::"+alias] {
				continue
			}
			seen[key+"::"+alias] = true
			c := aliasCandidate{identifier: id, identifierKey: id, lookupKey: key, packageName: name, alias: alias,
				tokens: aliasTokens(alias), hints: hints}
			m.candidates = append(m.candidates, c)
			m.byAlias[alias] = append(m.byAlias[alias], c)
		}
	}
	return m
}

func uniqueNonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" && !contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func canUpdate(modVersion, target string) bool {
	return target != "" && normalizeVersionForCompare(modVersion) != normalizeVersionForCompare(target)
}

// Map maps each mod (by folder) to the catalogue
// (buildModLIPStateByFolder). A nil mapper (catalogue not loaded) maps
// everything but LeviLamina to SourceNone.
func (m *Mapper) Map(mods []types.ModInfo) map[string]ModState {
	out := map[string]ModState{}
	for _, mod := range mods {
		folder := ModFolder(mod)
		if folder == "" {
			continue
		}
		if IsLeviLaminaMod(mod) {
			out[folder] = ModState{Source: SourceLeviLamina, Mapping: MappingNone}
			continue
		}
		if m == nil {
			out[folder] = ModState{Source: SourceNone, Mapping: MappingNone}
			continue
		}
		out[folder] = m.mapOne(mod, folder)
	}
	return out
}

func (m *Mapper) mapOne(mod types.ModInfo, folder string) ModState {
	matches := map[string]aliasCandidate{}
	var order []string
	put := func(c aliasCandidate) {
		if _, ok := matches[c.lookupKey]; !ok {
			order = append(order, c.lookupKey)
		}
		matches[c.lookupKey] = c
	}
	for _, alias := range uniqueNonEmpty(normalizeName(mod.Name), normalizeName(folder)) {
		for _, c := range m.byAlias[alias] {
			put(c)
		}
	}
	if len(matches) == 0 {
		values := uniqueNonEmpty(normalizeMatch(mod.Name), normalizeMatch(folder))
		for _, c := range m.candidates {
			if c.fuzzyMatches(values) {
				put(c)
			}
		}
	}
	switch {
	case len(matches) > 1:
		return ModState{Source: SourceAmbiguous, Mapping: MappingSelfVariant}
	case len(matches) == 1:
		c := matches[order[0]]
		pkg := m.byKey[c.lookupKey]
		st := ModState{Source: SourceNone, Mapping: MappingSelfVariant, MatchedAlias: c.alias,
			Identifier: c.identifier, IdentifierKey: c.identifierKey, PackageName: c.packageName}
		if pkg != nil {
			st.Source = SourceUnique
			st.Identifier, st.IdentifierKey = pkg.Identifier, pkg.Identifier
			st.TargetVersion = PickTargetVersion(pkg.Versions)
			st.CanUpdate = canUpdate(mod.Version, st.TargetVersion)
			st.PackageName = firstNonEmpty(strings.TrimSpace(pkg.Name), c.packageName)
		}
		return st
	}
	name := normalizeName(mod.Name)
	cands := m.byName[name]
	if name == "" || len(cands) == 0 {
		return ModState{Source: SourceNone, Mapping: MappingNone}
	}
	if len(cands) > 1 {
		return ModState{Source: SourceAmbiguous, Mapping: MappingName}
	}
	p := cands[0]
	target := PickTargetVersion(p.Versions)
	return ModState{Source: SourceUnique, Identifier: p.Identifier, IdentifierKey: p.Identifier,
		TargetVersion: target, CanUpdate: canUpdate(mod.Version, target), PackageName: p.Name, Mapping: MappingName}
}

// Candidates lists the packages that uniquely mapped mods belong to, once
// each, sorted by identifier (collectCandidateLIPIdentifiers).
func Candidates(states map[string]ModState) []string {
	byKey := map[string]string{}
	for _, st := range states {
		if st.Source != SourceUnique {
			continue
		}
		id := strings.TrimSpace(st.Identifier)
		k := LookupKey(firstNonEmpty(st.IdentifierKey, id))
		if id == "" || k == "" {
			continue
		}
		if _, ok := byKey[k]; !ok {
			byKey[k] = firstNonEmpty(st.IdentifierKey, id)
		}
	}
	out := make([]string, 0, len(byKey))
	for _, id := range byKey {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return localeCompare(out[i], out[j]) < 0 })
	return out
}

// Group is the mods that came from one catalogue package (LipGroupItem).
type Group struct {
	Identifier       string
	PackageName      string
	InstalledVersion string
	Explicit         bool
	Folders          []string
	Mods             []types.ModInfo
	ChildLabels      []string
	AllEnabled       bool
	AnyEnabled       bool
	TargetVersion    string
	CanUpdate        bool
	SelfVariant      bool
}

// Item is a mod that is not part of a group.
type Item struct {
	Folder string
	Mod    types.ModInfo
	State  ModState
}

// Listing splits mods into package groups and plain mods.
type Listing struct {
	Groups []Group
	Items  []Item
}

// BuildListing groups uniquely mapped mods by package (buildListItems). A
// group counts only when the workspace reports its package installed, or
// when it was matched by a self-variant alias and the state query failed.
// states is keyed by LookupKey. Both lists are sorted by name.
func BuildListing(mods []types.ModInfo, mapped map[string]ModState, states map[string]InstallState, enabled map[string]bool) Listing {
	type acc struct {
		g     Group
		key   string
		order int
	}
	groups := map[string]*acc{}
	var keys []string
	for _, mod := range mods {
		folder := ModFolder(mod)
		st, ok := mapped[folder]
		if folder == "" || !ok || st.Source != SourceUnique || st.Identifier == "" || st.IdentifierKey == "" {
			continue
		}
		k := LookupKey(firstNonEmpty(st.IdentifierKey, st.Identifier))
		if k == "" {
			continue
		}
		a := groups[k]
		if a == nil {
			a = &acc{key: k, g: Group{Identifier: st.Identifier,
				PackageName: firstNonEmpty(st.PackageName, mod.Name, st.Identifier), TargetVersion: st.TargetVersion}}
			groups[k] = a
			keys = append(keys, k)
		}
		a.g.Mods = append(a.g.Mods, mod)
		a.g.Folders = append(a.g.Folders, folder)
		a.g.SelfVariant = a.g.SelfVariant || st.Mapping == MappingSelfVariant
		label := normalizeName(mod.Name)
		if label == "" {
			label = normalizeName(folder)
		}
		if st.Mapping == MappingSelfVariant && st.MatchedAlias != "" {
			label = st.MatchedAlias
		}
		if label != "" {
			a.g.ChildLabels = append(a.g.ChildLabels, label)
		}
	}
	var out Listing
	managed := map[string]bool{}
	for _, k := range keys {
		a := groups[k]
		is := states[k]
		if !(is.Installed || (a.g.SelfVariant && strings.TrimSpace(is.Error) != "")) {
			continue
		}
		g := a.g
		g.Folders = uniqueNonEmpty(g.Folders...)
		g.AllEnabled = len(g.Folders) > 0
		for _, f := range g.Folders {
			if enabled[f] {
				g.AnyEnabled = true
			} else {
				g.AllEnabled = false
			}
		}
		var versions []string
		for _, mod := range g.Mods {
			if v := strings.TrimSpace(mod.Version); v != "" && !contains(versions, v) {
				versions = append(versions, v)
			}
		}
		g.InstalledVersion = strings.TrimSpace(is.InstalledVersion)
		if g.InstalledVersion == "" && len(versions) == 1 {
			g.InstalledVersion = versions[0]
		}
		g.Explicit = is.Explicit
		for _, mod := range g.Mods {
			if canUpdate(mod.Version, g.TargetVersion) {
				g.CanUpdate = true
			}
		}
		g.ChildLabels = uniqueNonEmpty(g.ChildLabels...)
		sort.Slice(g.ChildLabels, func(i, j int) bool { return localeCompare(g.ChildLabels[i], g.ChildLabels[j]) < 0 })
		g.PackageName = firstNonEmpty(g.PackageName, g.Identifier)
		for _, f := range g.Folders {
			managed[f] = true
		}
		out.Groups = append(out.Groups, g)
	}
	sort.SliceStable(out.Groups, func(i, j int) bool {
		return strings.ToLower(out.Groups[i].PackageName) < strings.ToLower(out.Groups[j].PackageName)
	})
	for _, mod := range mods {
		folder := ModFolder(mod)
		if folder == "" || managed[folder] {
			continue
		}
		st, ok := mapped[folder]
		if !ok {
			st = ModState{Source: SourceNone, Mapping: MappingNone}
		}
		out.Items = append(out.Items, Item{Folder: folder, Mod: mod, State: st})
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		return strings.ToLower(out.Items[i].Mod.Name) < strings.ToLower(out.Items[j].Mod.Name)
	})
	return out
}
