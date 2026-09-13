package versions

import "strings"

// Editor was Preview-only from 1.19.80.20 and reached retail in 1.21.50.
// https://zh.minecraft.wiki/w/基岩版编辑器?variant=zh-cn
const (
	EditorReleaseMinVersion = "1.21.50"
	EditorPreviewMinVersion = "1.19.80.20"
)

func EditorMinVersion(channel string) string {
	if strings.EqualFold(strings.TrimSpace(channel), "preview") {
		return EditorPreviewMinVersion
	}
	// Legacy Beta uses the retail package, not Minecraft Preview.
	return EditorReleaseMinVersion
}

func SupportsEditorMode(gameVersion, channel string) bool {
	return gameVersionAtLeast(gameVersion, EditorMinVersion(channel))
}

func EditorLaunchURI(packageType, channel string) string {
	protocol := "minecraft:"
	if strings.EqualFold(strings.TrimSpace(channel), "preview") {
		protocol = "minecraft-preview:"
	}
	// Retail 1.21.120 / Preview 1.21.120.21 switched from UWP to GDK.
	// Use the installed package format, rather than guessing it from a version.
	if NormalizePackageType(packageType) == PackageTypeUWP {
		return protocol + "?Editor=true"
	}
	return protocol + "//creator/?Editor=true"
}
