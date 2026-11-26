package update

import "testing"

func TestSelectMacDMGAsset(t *testing.T) {
	rel := githubRelease{
		TagName: "v1.2.0",
		Assets: []githubReleaseAsset{
			{Name: "LeviLauncher.zip", BrowserDownloadURL: "https://example.com/LeviLauncher.zip"},
			{Name: "LeviLauncher-macos-amd64-installer.dmg", BrowserDownloadURL: "https://example.com/amd64.dmg", Size: 100},
			{Name: "LeviLauncher-macos-arm64-installer.dmg", BrowserDownloadURL: "https://example.com/arm64.dmg", Size: 200},
		},
	}

	a, ok := selectMacDMGAsset(rel, "arm64")
	if !ok || a.Name != "LeviLauncher-macos-arm64-installer.dmg" {
		t.Fatalf("expected arm64 DMG, got %+v (ok=%v)", a, ok)
	}

	a, ok = selectMacDMGAsset(rel, "amd64")
	if !ok || a.Name != "LeviLauncher-macos-amd64-installer.dmg" {
		t.Fatalf("expected amd64 DMG, got %+v (ok=%v)", a, ok)
	}

	noDMG := githubRelease{
		TagName: "v1.2.0",
		Assets: []githubReleaseAsset{
			{Name: "LeviLauncher.zip", BrowserDownloadURL: "https://example.com/LeviLauncher.zip"},
		},
	}
	if _, ok := selectMacDMGAsset(noDMG, "arm64"); ok {
		t.Fatal("expected no DMG to be matched")
	}
}
