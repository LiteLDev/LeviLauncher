package versions

import "strings"

// Windows 10 Edition used a different executable name before 1.0.
func IsMinecraftExecutable(name string) bool {
	return strings.EqualFold(name, "Minecraft.Windows.exe") || strings.EqualFold(name, "Minecraft.Win10.DX11.exe")
}
