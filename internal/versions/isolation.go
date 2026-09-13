package versions

const UWPIsolationMinVersion = "1.19.70.2"

// SupportsIsolation leaves GDK unchanged and requires a known supported UWP version.
func SupportsIsolation(packageType, gameVersion string) bool {
	if NormalizePackageType(packageType) != PackageTypeUWP {
		return true
	}
	return gameVersionAtLeast(gameVersion, UWPIsolationMinVersion)
}
