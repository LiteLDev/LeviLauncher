package lipindex

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"
)

// DependencyRef is a value of a detail's "dependencies" (keyed by the
// lower-case bare id): a dependency as the catalogue names it. URL is set
// when it has a page; ID is then the id of that page (the current name of a
// renamed repository, the registry's spelling).
type DependencyRef struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Owner string `json:"owner,omitempty"`
	URL   string `json:"url,omitempty"`
}

// DetailLatest is a detail's "latest": the version (and the variant, for
// the server) the catalogue offers.
type DetailLatest struct {
	Variant string `json:"variant"`
	Version string `json:"version"`
}

// UnmarshalJSON decodes a detail and works out its catalogue deps
// (CatalogDeps) from the document itself: the site's depsOf joins the
// ranges of a dependency in the order of the document's variants, entries
// and keys, which a Go map does not keep. A document that already has
// "catalogDeps" (the Cache) keeps them.
func (d *Detail) UnmarshalJSON(b []byte) error {
	type plain Detail
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*d = Detail(p)
	if d.CatalogDeps != nil {
		return nil
	}
	i := latestVersionIndex(d)
	if i < 0 {
		return nil
	}
	var raw struct {
		Versions []struct {
			Variants json.RawMessage `json:"variants"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(b, &raw); err != nil || i >= len(raw.Versions) {
		return nil
	}
	pairs, err := orderedDeps(raw.Versions[i].Variants)
	if err != nil {
		// The plain decode accepted it; latestDeps falls back to the maps.
		return nil
	}
	d.CatalogDeps = catalogDeps(d, pairs)
	return nil
}

// depPair is a dependency key of an entry and its range.
type depPair struct {
	key, rng string
}

// orderedObject lists the members of a JSON object in the document's order.
// null or nothing is an empty object.
func orderedObject(raw json.RawMessage) ([]string, []json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, nil, errors.New("lipindex: not a JSON object")
	}
	var keys []string
	var values []json.RawMessage
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, errors.New("lipindex: bad JSON object key")
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)
		values = append(values, v)
	}
	return keys, values, nil
}

// orderedDeps lists the dependency keys and ranges of a version's
// "variants" in the document's order: variant by variant, entry by entry,
// key by key (the site's entriesOf(doc) and Object.entries(entry.deps)).
func orderedDeps(variants json.RawMessage) ([]depPair, error) {
	_, lists, err := orderedObject(variants)
	if err != nil {
		return nil, err
	}
	out := []depPair{}
	for _, list := range lists {
		var entries []struct {
			Deps json.RawMessage `json:"deps"`
		}
		if err := json.Unmarshal(list, &entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			keys, values, err := orderedObject(e.Deps)
			if err != nil {
				return nil, err
			}
			for j, k := range keys {
				var r string
				if err := json.Unmarshal(values[j], &r); err != nil {
					return nil, err
				}
				out = append(out, depPair{key: k, rng: r})
			}
		}
	}
	return out, nil
}

// mapDeps lists a version's dependency keys and ranges from its maps, for
// a Detail that was not decoded from a document: variants in rule A2's
// order, keys sorted.
func mapDeps(v DetailVersion) []depPair {
	names := make([]string, 0, len(v.Variants))
	for n := range v.Variants {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return compareVariantNames(names[i], names[j]) < 0 })
	out := []depPair{}
	for _, n := range names {
		for _, e := range v.Variants[n] {
			keys := make([]string, 0, len(e.Deps))
			for k := range e.Deps {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				out = append(out, depPair{key: k, rng: e.Deps[k]})
			}
		}
	}
	return out
}

// latestVersionIndex is the version the catalogue's deps describe: the one
// "latest" names, else the first (newest) one; -1 when there is none.
func latestVersionIndex(d *Detail) int {
	want := ""
	if d.Latest != nil {
		want = strings.TrimSpace(d.Latest.Version)
	}
	first := -1
	for i, v := range d.Versions {
		s := strings.TrimSpace(v.Version)
		if s == "" {
			continue
		}
		if first < 0 {
			first = i
		}
		if want != "" && s == want {
			return i
		}
	}
	return first
}

// catalogDeps is the site's depsOf (levimc.org mods/src/server/catalog.ts)
// as far as a detail allows: each bare id is named as the catalogue names
// it (a dependency with a page, "url" in the detail's "dependencies", by
// the id of that page: the registry's spelling, the current name of a
// renamed repository, the listed copy of a duplicate), the package itself
// left out after that, several ranges joined by " || " in the order given.
//
// What the detail cannot tell: the site names a dependency without a page
// (an unlisted or excluded package of the registry) by the registry's
// spelling, but the detail's ref of it is lower case and has no url, so
// such a dependency keeps the key's spelling, as LeviLauncher-Android's
// LipIndex.latestDeps does too. The two differ only when a lip.toml spells
// such an id in another case than the registry.
func catalogDeps(d *Detail, pairs []depPair) map[string]string {
	canonical := map[string]string{}
	for key, ref := range d.Dependencies {
		if id := strings.TrimSpace(ref.ID); id != "" && strings.TrimSpace(ref.URL) != "" {
			canonical[strings.ToLower(strings.TrimSpace(key))] = id
		}
	}
	self := strings.TrimSpace(d.ID)
	ranges := map[string][]string{}
	for _, p := range pairs {
		bare := strings.SplitN(p.key, "#", 2)[0]
		if strings.TrimSpace(bare) == "" {
			continue
		}
		target := bare
		if id, ok := canonical[strings.ToLower(strings.TrimSpace(bare))]; ok {
			target = id
		}
		if strings.EqualFold(target, self) {
			continue
		}
		if !contains(ranges[target], p.rng) {
			ranges[target] = append(ranges[target], p.rng)
		}
	}
	out := make(map[string]string, len(ranges))
	for target, list := range ranges {
		out[target] = strings.Join(list, " || ")
	}
	return out
}

// latestDeps is the catalogue index's deps of a detail (mods[].deps): what
// the latest version depends on, by bare id as the site names it.
func latestDeps(d *Detail) map[string]string {
	if d.CatalogDeps != nil {
		out := make(map[string]string, len(d.CatalogDeps))
		for k, v := range d.CatalogDeps {
			out[k] = v
		}
		return out
	}
	i := latestVersionIndex(d)
	if i < 0 {
		return map[string]string{}
	}
	return catalogDeps(d, mapDeps(d.Versions[i]))
}

// descriptionMax is the length, in code points, the catalogue index cuts
// descriptions to (the site's DESCRIPTION_MAX).
const descriptionMax = 200

func isJSSpace(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' }

// truncateDescription is the site's truncate: the trimmed text, cut to 199
// code points, trimmed at the end and followed by "…" when it is longer
// than 200.
func truncateDescription(text string) string {
	chars := []rune(strings.TrimFunc(text, isJSSpace))
	if len(chars) <= descriptionMax {
		return string(chars)
	}
	return strings.TrimRightFunc(string(chars[:descriptionMax-1]), isJSSpace) + "…"
}

// catalogDescription is the description the catalogue index lists and the
// search reads: the index's own (set by FetchDetails), else the detail's
// cut as the index cuts it.
func catalogDescription(d *Detail) string {
	if s := strings.TrimSpace(d.CatalogDescription); s != "" {
		return s
	}
	return truncateDescription(d.Description)
}
