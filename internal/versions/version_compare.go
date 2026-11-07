package versions

import (
	"strconv"
	"strings"
)

func parseNumericGameVersion(value string) ([4]uint64, bool) {
	var version [4]uint64
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) > len(version) {
		return version, false
	}
	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return version, false
		}
		number, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return version, false
		}
		version[i] = number
	}
	return version, true
}

func gameVersionAtLeast(value, minimum string) bool {
	version, valid := parseNumericGameVersion(value)
	minVersion, minValid := parseNumericGameVersion(minimum)
	if !valid || !minValid {
		return false
	}
	for i, part := range version {
		if part != minVersion[i] {
			return part > minVersion[i]
		}
	}
	return true
}

// CompareGameVersions orders two game versions ("1.21.50.07"): -1 when a is
// older than b, 1 when newer, 0 when equal. A version that does not parse
// counts as older than any that does; two such versions compare as text.
func CompareGameVersions(a, b string) int {
	va, okA := parseNumericGameVersion(a)
	vb, okB := parseNumericGameVersion(b)
	switch {
	case okA && okB:
		for i := range va {
			if va[i] != vb[i] {
				if va[i] < vb[i] {
					return -1
				}
				return 1
			}
		}
		return 0
	case okA:
		return 1
	case okB:
		return -1
	}
	return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
}
