package lipindex

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

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

// Sort keys of a catalogue search.
const (
	SortStars   = "stars"
	SortUpdated = "updated"
	SortName    = "name"
)

// Query is a catalogue search (LIPPage filters).
type Query struct {
	// Text matches part of the identifier, name, description, author or a
	// tag, ignoring case.
	Text string
	// GameVersion keeps packages whose LeviLamina range is satisfied by a
	// LeviLamina version for that game ("" = any game).
	GameVersion string
	// LoaderVersion keeps packages whose range this LeviLamina version
	// satisfies ("" = any).
	LoaderVersion string
	// DB is the LeviLamina version database; without it the version
	// filters are off, as in the window.
	DB LoaderDB
	// Sort is SortStars, SortUpdated or SortName; Descending orders it.
	Sort       string
	Descending bool
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

// Search filters and sorts packages. Hidden packages are dropped.
func Search(pkgs []*Package, q Query) []*Package {
	keyword := strings.ToLower(strings.TrimSpace(q.Text))
	mapping := len(q.DB) > 0
	game := NormalizeSemverInput(q.GameVersion)
	ll := NormalizeLoaderFilter(q.LoaderVersion)
	gameActive := mapping && game != ""
	llActive := mapping && strings.TrimSpace(q.LoaderVersion) != ""
	var llByGame []string
	if gameActive {
		for _, v := range q.DB[game] {
			llByGame = append(llByGame, NormalizeLoaderFilter(v))
		}
	}
	out := []*Package{}
	for _, p := range pkgs {
		if Hidden(p) {
			continue
		}
		if gameActive && len(p.LLRanges) == 0 {
			continue
		}
		switch {
		case gameActive && !llActive:
			if !CompatibleWithAny(p.LLRanges, llByGame) {
				continue
			}
		case gameActive && llActive:
			if len(llByGame) > 0 && !contains(llByGame, ll) {
				continue
			}
			if !Compatible(ll, p.LLRanges) {
				continue
			}
		case llActive:
			if len(p.LLRanges) == 0 || !Compatible(ll, p.LLRanges) {
				continue
			}
		}
		if keyword != "" && !matchesKeyword(p, keyword) {
			continue
		}
		out = append(out, p)
	}
	sortPackages(out, q.Sort, q.Descending)
	return out
}

func matchesKeyword(p *Package, keyword string) bool {
	hay := append([]string{p.Identifier, p.Name, p.Description, p.Author}, p.Tags...)
	for _, h := range hay {
		if strings.Contains(strings.ToLower(h), keyword) {
			return true
		}
	}
	return false
}

func updatedTime(s string) int64 {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

// sortPackages sorts ascending by key, then reverses for descending, as
// the catalogue page does.
func sortPackages(list []*Package, key string, desc bool) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		switch key {
		case SortUpdated:
			return updatedTime(a.Updated) < updatedTime(b.Updated)
		case SortName:
			return localeCompare(a.Name, b.Name) < 0
		default:
			return a.Stars < b.Stars
		}
	})
	if desc {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
}

// DefaultDescending is the order a sort key starts with: name ascending,
// stars and updated descending.
func DefaultDescending(key string) bool { return key != SortName }
