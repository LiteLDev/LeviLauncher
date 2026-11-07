package catalog

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LoaderDB maps a game version ("1.21.132.01") to the LeviLamina versions
// that support it, newest first.
type LoaderDB map[string][]string

// NormalizeLoaderDB cleans the LeviLamina version database as the window
// does: game versions lose a leading "v", each version list is trimmed,
// de-duplicated and sorted newest first (releases above their
// pre-releases), and game versions without any version are dropped.
func NormalizeLoaderDB(raw map[string][]string) LoaderDB {
	out := LoaderDB{}
	for game, list := range raw {
		g := normalizeGameKey(game)
		if g == "" {
			continue
		}
		seen := map[string]bool{}
		var vs []string
		for _, v := range list {
			v = strings.TrimSpace(v)
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			vs = append(vs, v)
		}
		if len(vs) == 0 {
			continue
		}
		sort.SliceStable(vs, func(i, j int) bool { return CompareLoaderVersions(vs[i], vs[j]) > 0 })
		out[g] = vs
	}
	return out
}

func normalizeGameKey(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return v
}

// Versions returns the LeviLamina versions for a game version (exact match
// after stripping a leading "v"), newest first; nil when unsupported.
func (db LoaderDB) Versions(gameVersion string) []string {
	return db[normalizeGameKey(gameVersion)]
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

// ResolveGame finds the game version key matching spec: exact, else the
// newest key with spec as a dotted prefix. ok is false when none matches.
func (db LoaderDB) ResolveGame(spec string) (string, bool) {
	spec = normalizeGameKey(spec)
	if _, ok := db[spec]; ok {
		return spec, true
	}
	for _, g := range db.GameVersions() {
		if VersionEqual(g, spec) {
			return g, true
		}
	}
	for _, g := range db.GameVersions() {
		if HasVersionPrefix(g, spec) {
			return g, true
		}
	}
	return "", false
}

// LoaderLatest returns the newest version of list (sorted newest first),
// pre-releases included, as the instance settings page offers it.
func LoaderLatest(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// LoaderLatestStable returns the newest version of list that is not a
// pre-release, "" when there is none.
func LoaderLatestStable(list []string) string {
	for _, v := range list {
		if !IsPrerelease(v) {
			return v
		}
	}
	return ""
}

// FindLoaderVersion finds want in list, ignoring case and a leading "v".
func FindLoaderVersion(list []string, want string) (string, bool) {
	w := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(want), "v"), "V")
	if w == "" {
		return "", false
	}
	for _, v := range list {
		t := strings.TrimSpace(v)
		if strings.EqualFold(t, w) || strings.EqualFold(strings.TrimPrefix(strings.TrimPrefix(t, "v"), "V"), w) {
			return t, true
		}
	}
	return "", false
}

type semver struct {
	major, minor, patch int
	pre                 []string
}

var semverPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func parseSemver(v string) (semver, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	m := semverPattern.FindStringSubmatch(v)
	if m == nil {
		return semver{}, false
	}
	s := semver{}
	s.major, _ = strconv.Atoi(m[1])
	s.minor, _ = strconv.Atoi(m[2])
	s.patch, _ = strconv.Atoi(m[3])
	if m[4] != "" {
		s.pre = strings.Split(m[4], ".")
	}
	return s, true
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func compareSemver(a, b semver) int {
	if c := cmpInt(a.major, b.major); c != 0 {
		return c
	}
	if c := cmpInt(a.minor, b.minor); c != 0 {
		return c
	}
	if c := cmpInt(a.patch, b.patch); c != 0 {
		return c
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < max(len(a.pre), len(b.pre)); i++ {
		if i >= len(a.pre) {
			return -1
		}
		if i >= len(b.pre) {
			return 1
		}
		ai, bi := a.pre[i], b.pre[i]
		an, aerr := strconv.Atoi(ai)
		bn, berr := strconv.Atoi(bi)
		aNum, bNum := aerr == nil && isDigits(ai), berr == nil && isDigits(bi)
		switch {
		case aNum && bNum:
			if c := cmpInt(an, bn); c != 0 {
				return c
			}
		case aNum:
			return -1
		case bNum:
			return 1
		case ai != bi:
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// CompareLoaderVersions orders LeviLamina versions by semantic version, a
// release above its pre-releases; a version that is not semver sorts below
// every semver one, and two such versions compare as text.
func CompareLoaderVersions(a, b string) int {
	sa, okA := parseSemver(a)
	sb, okB := parseSemver(b)
	switch {
	case okA && okB:
		return compareSemver(sa, sb)
	case okA:
		return 1
	case okB:
		return -1
	}
	return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
}

// IsPrerelease reports whether a LeviLamina version is a pre-release
// ("1.8.0-rc.1"): a semver pre-release part, or "rc" anywhere, as the
// window's release-candidate warning checks.
func IsPrerelease(v string) bool {
	if s, ok := parseSemver(v); ok && len(s.pre) > 0 {
		return true
	}
	return strings.Contains(strings.ToLower(v), "rc")
}

// Loader actions.
const (
	ActionInstall   = "install"
	ActionUpgrade   = "upgrade"
	ActionDowngrade = "downgrade"
	ActionReinstall = "reinstall"
	ActionChange    = "change" // versions that do not compare
)

// LoaderAction says what installing target over current ("" when none is
// installed) does.
func LoaderAction(current, target string) string {
	current, target = strings.TrimSpace(current), strings.TrimSpace(target)
	if current == "" {
		return ActionInstall
	}
	sc, okC := parseSemver(current)
	st, okT := parseSemver(target)
	if !okC || !okT {
		if strings.EqualFold(strings.TrimPrefix(current, "v"), strings.TrimPrefix(target, "v")) {
			return ActionReinstall
		}
		return ActionChange
	}
	switch compareSemver(st, sc) {
	case 1:
		return ActionUpgrade
	case -1:
		return ActionDowngrade
	}
	return ActionReinstall
}
