// Package lipindex is the LeviLamina mod catalogue (levimc.org) client and
// the decisions the launcher makes with it: version and LeviLamina range
// matching, catalogue search, mapping installed mods to catalogue packages
// and planning an install. It is a Go port of the window's TypeScript
// (frontend/src/utils/content.ts, modIntelligenceResolver.ts,
// LeviLaminaContext.tsx, LIPInstallDialog.tsx and LIPPage.tsx) so the
// command line behaves as the window does. It has no Wails dependency.
package lipindex

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Semver is a parsed MAJOR.MINOR.PATCH[-PRE] version.
type Semver struct {
	Major, Minor, Patch uint64
	Pre                 []string
}

var semverPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// NormalizeSemverInput trims s and drops a leading "v" or "V".
func NormalizeSemverInput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	return s
}

// ParseSemver parses a strict semantic version ("v" prefix allowed).
func ParseSemver(s string) (Semver, bool) {
	m := semverPattern.FindStringSubmatch(NormalizeSemverInput(s))
	if m == nil {
		return Semver{}, false
	}
	var v Semver
	var err error
	if v.Major, err = strconv.ParseUint(m[1], 10, 64); err != nil {
		return Semver{}, false
	}
	if v.Minor, err = strconv.ParseUint(m[2], 10, 64); err != nil {
		return Semver{}, false
	}
	if v.Patch, err = strconv.ParseUint(m[3], 10, 64); err != nil {
		return Semver{}, false
	}
	if m[4] != "" {
		v.Pre = strings.Split(m[4], ".")
	}
	return v, true
}

func cmpUint(a, b uint64) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	}
	return 0
}

func compareCore(a, b Semver) int {
	if c := cmpUint(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpUint(a.Minor, b.Minor); c != 0 {
		return c
	}
	return cmpUint(a.Patch, b.Patch)
}

var digits = regexp.MustCompile(`^[0-9]+$`)

// Compare orders two versions: a release is above its pre-releases, and
// pre-release identifiers compare numerically when both are numbers.
func (a Semver) Compare(b Semver) int {
	if c := compareCore(a, b); c != 0 {
		return c
	}
	aRel, bRel := len(a.Pre) == 0, len(b.Pre) == 0
	switch {
	case aRel && bRel:
		return 0
	case aRel:
		return 1
	case bRel:
		return -1
	}
	for i := 0; ; i++ {
		if i >= len(a.Pre) && i >= len(b.Pre) {
			return 0
		}
		if i >= len(a.Pre) {
			return -1
		}
		if i >= len(b.Pre) {
			return 1
		}
		ap, bp := a.Pre[i], b.Pre[i]
		an, bn := digits.MatchString(ap), digits.MatchString(bp)
		switch {
		case an && bn:
			x, _ := strconv.ParseUint(ap, 10, 64)
			y, _ := strconv.ParseUint(bp, 10, 64)
			if c := cmpUint(x, y); c != 0 {
				return c
			}
		case an:
			return -1
		case bn:
			return 1
		case ap != bp:
			if ap > bp {
				return 1
			}
			return -1
		}
	}
}

// localeCompare approximates JavaScript's String.prototype.localeCompare
// for the ASCII identifiers the catalogue uses: case-insensitive first,
// then by case.
func localeCompare(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(b, a) // lower case sorts before upper case
}

// CompareSemverLike orders package versions: valid semantic versions above
// anything else, others by text.
func CompareSemverLike(a, b string) int {
	pa, okA := ParseSemver(a)
	pb, okB := ParseSemver(b)
	switch {
	case okA && okB:
		return pa.Compare(pb)
	case okA:
		return 1
	case okB:
		return -1
	}
	return localeCompare(NormalizeSemverInput(a), NormalizeSemverInput(b))
}

// CompareVersions is the window's dotted numeric compare (utils/version.ts):
// missing parts count as 0, numeric parts compare as numbers, others as
// text.
func CompareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	n := max(len(as), len(bs))
	for i := 0; i < n; i++ {
		ai, bi := "0", "0"
		if i < len(as) {
			ai = as[i]
		}
		if i < len(bs) {
			bi = bs[i]
		}
		if digits.MatchString(ai) && digits.MatchString(bi) {
			x, _ := strconv.ParseUint(ai, 10, 64)
			y, _ := strconv.ParseUint(bi, 10, 64)
			if c := cmpUint(x, y); c != 0 {
				return c
			}
			continue
		}
		if ai != bi {
			return localeCompare(ai, bi)
		}
	}
	return 0
}

func isWildcard(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "*" || s == "x"
}

// wildcardRange parses "1.x", "1.2.*" or "*". all reports a range matching
// everything; ok is false when token is not a wildcard range.
func wildcardRange(token string) (lo, hi Semver, all, ok bool) {
	n := NormalizeSemverInput(token)
	if n == "" {
		return
	}
	if isWildcard(n) {
		return lo, hi, true, true
	}
	parts := strings.Split(n, ".")
	if len(parts) > 3 {
		return
	}
	at := -1
	for i, p := range parts {
		if isWildcard(p) {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	for i := 0; i < at; i++ {
		if !digits.MatchString(strings.TrimSpace(parts[i])) {
			return
		}
	}
	for i := at; i < len(parts); i++ {
		if !isWildcard(parts[i]) {
			return
		}
	}
	if at == 0 {
		return lo, hi, true, true
	}
	major, _ := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	var minor uint64
	if at > 1 {
		minor, _ = strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	}
	lo = Semver{Major: major, Minor: minor}
	switch at {
	case 1:
		return lo, Semver{Major: major + 1}, false, true
	case 2:
		return lo, Semver{Major: major, Minor: minor + 1}, false, true
	}
	return Semver{}, Semver{}, false, false
}

var comparatorPattern = regexp.MustCompile(`^(>=|<=|>|<|=)\s*(.+)$`)

func tokenMatches(target Semver, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if m := comparatorPattern.FindStringSubmatch(token); m != nil {
		v, ok := ParseSemver(m[2])
		if !ok {
			return false
		}
		c := target.Compare(v)
		switch m[1] {
		case ">=":
			return c >= 0
		case "<=":
			return c <= 0
		case ">":
			return c > 0
		case "<":
			return c < 0
		}
		return c == 0
	}
	if lo, hi, all, ok := wildcardRange(token); ok {
		if all {
			return true
		}
		return compareCore(target, lo) >= 0 && compareCore(target, hi) < 0
	}
	v, ok := ParseSemver(token)
	return ok && target.Compare(v) == 0
}

// InRange reports whether the LeviLamina version satisfies a range
// expression: space-separated tokens that must all match, each a comparator
// (>=, <=, >, <, =), a wildcard (*, x, 1.x, 1.2.*) or an exact version.
func InRange(version, expr string) bool {
	target, ok := ParseSemver(version)
	if !ok {
		return false
	}
	tokens := strings.Fields(expr)
	if len(tokens) == 0 {
		return false
	}
	for _, t := range tokens {
		if !tokenMatches(target, t) {
			return false
		}
	}
	return true
}

// Compatible reports whether version satisfies any of ranges.
func Compatible(version string, ranges []string) bool {
	if len(ranges) == 0 || strings.TrimSpace(version) == "" {
		return false
	}
	for _, r := range ranges {
		if InRange(strings.TrimSpace(version), r) {
			return true
		}
	}
	return false
}

// CompatibleWithAny reports whether any of versions satisfies ranges.
func CompatibleWithAny(ranges, versions []string) bool {
	for _, v := range versions {
		if Compatible(v, ranges) {
			return true
		}
	}
	return false
}

// SupportedGameVersions lists the game versions, newest first, that have a
// LeviLamina version satisfying ranges.
func SupportedGameVersions(ranges []string, db LoaderDB) []string {
	if len(ranges) == 0 {
		return []string{}
	}
	out := []string{}
	for game, lls := range db {
		if strings.TrimSpace(game) != "" && CompatibleWithAny(ranges, lls) {
			out = append(out, strings.TrimSpace(game))
		}
	}
	sort.Slice(out, func(i, j int) bool { return CompareVersions(out[i], out[j]) > 0 })
	return out
}

// LoaderDB maps a game version ("1.21.50.07") to the LeviLamina versions
// supporting it, newest first.
type LoaderDB map[string][]string

// NormalizeLoaderDB cleans the LeviLamina version database as the window
// does: game versions lose a leading "v", LeviLamina versions are trimmed,
// deduplicated and sorted newest first (releases above pre-releases), and
// games without versions are dropped.
func NormalizeLoaderDB(raw map[string][]string) LoaderDB {
	out := LoaderDB{}
	for game, lls := range raw {
		g := NormalizeSemverInput(game)
		if g == "" {
			continue
		}
		seen := map[string]bool{}
		list := []string{}
		for _, v := range lls {
			v = strings.TrimSpace(v)
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			list = append(list, v)
		}
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool { return CompareSemverLike(list[i], list[j]) > 0 })
		out[g] = list
	}
	return out
}

// Versions returns the LeviLamina versions of a game version (exact match
// after stripping a leading "v"), newest first; nil when unsupported.
func (db LoaderDB) Versions(gameVersion string) []string {
	return db[NormalizeSemverInput(gameVersion)]
}

// Supports reports whether LeviLamina supports a game version.
func (db LoaderDB) Supports(gameVersion string) bool { return len(db.Versions(gameVersion)) > 0 }

// GameVersions lists the supported game versions, newest first.
func (db LoaderDB) GameVersions() []string {
	out := make([]string, 0, len(db))
	for g := range db {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if c := CompareVersions(out[i], out[j]); c != 0 {
			return c > 0
		}
		return out[i] > out[j]
	})
	return out
}

// ResolveGame finds the game version key matching spec: "latest" (the
// newest), an exact version (also "1.21.50.7" for "1.21.50.07"), else the
// newest key with spec as a dotted prefix on segment boundaries. ok is
// false when none matches.
func (db LoaderDB) ResolveGame(spec string) (string, bool) {
	spec = NormalizeSemverInput(spec)
	games := db.GameVersions()
	if strings.EqualFold(spec, "latest") {
		if len(games) == 0 {
			return "", false
		}
		return games[0], true
	}
	if _, ok := db[spec]; ok {
		return spec, true
	}
	for _, g := range games {
		if VersionEqual(g, spec) {
			return g, true
		}
	}
	for _, g := range games {
		if HasVersionPrefix(g, spec) {
			return g, true
		}
	}
	return "", false
}

func versionSegments(v string) []string { return strings.Split(strings.TrimSpace(v), ".") }

func segmentEqual(a, b string) bool {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)
	if ea == nil && eb == nil {
		return na == nb
	}
	return strings.EqualFold(a, b)
}

// VersionEqual reports whether two dotted versions are the same build
// number, segment by segment, so "1.21.50.7" equals "1.21.50.07".
func VersionEqual(a, b string) bool {
	sa, sb := versionSegments(a), versionSegments(b)
	if len(sa) != len(sb) {
		return false
	}
	for i := range sa {
		if !segmentEqual(sa[i], sb[i]) {
			return false
		}
	}
	return true
}

// HasVersionPrefix reports whether prefix names version on dotted segment
// boundaries: "1.21.50" matches "1.21.50.07", "1.21.5" does not, and a
// version is a prefix of itself.
func HasVersionPrefix(version, prefix string) bool {
	sv, sp := versionSegments(version), versionSegments(prefix)
	if strings.TrimSpace(prefix) == "" || len(sp) > len(sv) {
		return false
	}
	for i := range sp {
		if !segmentEqual(sv[i], sp[i]) {
			return false
		}
	}
	return true
}

// IsPrerelease reports whether a LeviLamina version is a pre-release
// ("1.8.0-rc.1"): a semver pre-release part, or "rc" anywhere, as the
// window's release-candidate warning checks.
func IsPrerelease(v string) bool {
	if s, ok := ParseSemver(v); ok && len(s.Pre) > 0 {
		return true
	}
	return IsReleaseCandidate(v)
}

// IsStable reports whether a package version is a release (no "-").
func IsStable(v string) bool { return !strings.Contains(strings.TrimSpace(v), "-") }

// IsReleaseCandidate reports whether a LeviLamina version is an rc, which
// the window never picks by default.
func IsReleaseCandidate(v string) bool { return strings.Contains(strings.ToLower(v), "rc") }
