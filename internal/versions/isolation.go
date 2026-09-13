package versions

import (
	"strconv"
	"strings"
)

const UWPIsolationMinVersion = "1.19.70.2"

// SupportsIsolation leaves GDK unchanged and requires a known supported UWP version.
func SupportsIsolation(packageType, gameVersion string) bool {
	if NormalizePackageType(packageType) != PackageTypeUWP {
		return true
	}
	parts := strings.Split(strings.TrimSpace(gameVersion), ".")
	if len(parts) > 4 {
		return false
	}
	var version [4]uint64
	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return false
		}
		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return false
		}
		version[i] = value
	}
	minimum := strings.Split(UWPIsolationMinVersion, ".")
	for i, part := range version {
		minPart, _ := strconv.ParseUint(minimum[i], 10, 32)
		if part != minPart {
			return part > minPart
		}
	}
	return true
}
