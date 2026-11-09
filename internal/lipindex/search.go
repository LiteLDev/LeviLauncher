package lipindex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Full-text search over the catalogue (the shared contract's A3, ported
// from levimc.org mods/src/lib/search.ts): every query word must appear in
// one of a mod's fields, and the fields weigh in by importance. Substring
// matching finds Chinese text (领地) as well as parts of names (lamina ->
// LeviLamina).

type searchField struct {
	text   string
	weight int
}

// searchFields is the text a mod is found by: name x3; repository path and
// owner x2; its featured tags' labels in every language and its visible
// tags x2; description x1. Lower case.
func searchFields(p *Package) []searchField {
	var words []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			words = append(words, s)
		}
	}
	for _, f := range FeaturedTagsOf(p.Tags) {
		for _, l := range f.Tag.Labels {
			add(l)
		}
	}
	for _, t := range VisibleTags(p.Tags) {
		add(t)
	}
	repo := strings.Join(strings.Split(p.Identifier, "/")[1:], " ")
	fields := []searchField{
		{p.Name, 3},
		{repo + " " + p.Owner, 2},
		{strings.Join(words, " "), 2},
		{p.Description, 1},
	}
	for i := range fields {
		fields[i].text = strings.ToLower(fields[i].text)
	}
	return fields
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// matchQuality is how well token matches text: 3 a whole word, 2 the start
// of a word, 1 anywhere, 0 not at all.
func matchQuality(text, token string) int {
	best := 0
	for from := 0; from <= len(text); {
		i := strings.Index(text[from:], token)
		if i < 0 {
			break
		}
		at := from + i
		start := at == 0
		if !start {
			r, _ := utf8.DecodeLastRuneInString(text[:at])
			start = !isWordRune(r)
		}
		end := at+len(token) >= len(text)
		if !end {
			r, _ := utf8.DecodeRuneInString(text[at+len(token):])
			end = !isWordRune(r)
		}
		q := 1
		if start {
			q = 2
			if end {
				q = 3
			}
		}
		best = max(best, q)
		if best == 3 {
			break
		}
		from = at + 1
	}
	return best
}

func isQuerySeparator(r rune) bool { return unicode.IsSpace(r) || r == ',' || r == '，' || r == '、' }

// SearchCatalog returns the relevance scores of the packages matching
// query by identifier (higher is better), or nil for an empty query. Every
// word of the query has to match; each adds the weight of the fields it
// matches times how well it matches, and an exact name scores highest.
func SearchCatalog(pkgs []*Package, query string) map[string]int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var tokens []string
	for _, t := range strings.FieldsFunc(q, isQuerySeparator) {
		if !contains(tokens, t) {
			tokens = append(tokens, t)
		}
	}
	scores := map[string]int{}
	for _, p := range pkgs {
		fields := searchFields(p)
		total := 0
		all := true
		for _, token := range tokens {
			score := 0
			for _, f := range fields {
				score += f.weight * matchQuality(f.text, token)
			}
			if score == 0 {
				all = false
				break
			}
			total += score
		}
		if !all {
			continue
		}
		if fields[0].text == q {
			total += 100
		}
		scores[p.Identifier] = total
	}
	return scores
}
