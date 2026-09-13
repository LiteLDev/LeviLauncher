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
