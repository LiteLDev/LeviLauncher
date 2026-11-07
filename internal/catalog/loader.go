package catalog

import (
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipindex"
)

// LoaderDB maps a game version ("1.21.132.01") to the LeviLamina versions
// that support it, newest first. It is lipindex's type, shared with the mod
// commands.
type LoaderDB = lipindex.LoaderDB

// NormalizeLoaderDB cleans the LeviLamina version database as the window
// does: game versions lose a leading "v", each version list is trimmed,
// de-duplicated and sorted newest first (releases above their
// pre-releases), and game versions without any version are dropped.
func NormalizeLoaderDB(raw map[string][]string) LoaderDB { return lipindex.NormalizeLoaderDB(raw) }

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

// CompareLoaderVersions orders LeviLamina versions by semantic version, a
// release above its pre-releases; a version that is not semver sorts below
// every semver one, and two such versions compare as text.
func CompareLoaderVersions(a, b string) int { return lipindex.CompareSemverLike(a, b) }

// IsPrerelease reports whether a LeviLamina version is a pre-release.
func IsPrerelease(v string) bool { return lipindex.IsPrerelease(v) }

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
	sc, okC := lipindex.ParseSemver(current)
	st, okT := lipindex.ParseSemver(target)
	if !okC || !okT {
		if strings.EqualFold(strings.TrimPrefix(current, "v"), strings.TrimPrefix(target, "v")) {
			return ActionReinstall
		}
		return ActionChange
	}
	switch st.Compare(sc) {
	case 1:
		return ActionUpgrade
	case -1:
		return ActionDowngrade
	}
	return ActionReinstall
}
