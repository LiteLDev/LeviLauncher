package update

import "testing"

func TestSelectUpdateReleaseSkipsMissingMacDMG(t *testing.T) {
	dmg := githubReleaseAsset{
		Name: "LeviLauncher-macos-arm64-installer.dmg", BrowserDownloadURL: "https://example.com/installer.dmg",
	}
	releases := []githubRelease{
		{TagName: "v1.5.0-beta.2", Prerelease: true},
		{TagName: "v1.5.0-beta.1", Prerelease: true, Assets: []githubReleaseAsset{dmg}},
		{TagName: "v1.4.0", Assets: []githubReleaseAsset{{Name: dmg.Name}}},
		{TagName: "v1.3.0", Assets: []githubReleaseAsset{dmg}},
	}
	for _, tc := range []struct {
		host string
		beta bool
		want string
	}{
		{"darwin", false, "v1.3.0"},
		{"darwin", true, "v1.5.0-beta.1"},
		{"windows", false, "v1.4.0"},
		{"windows", true, "v1.5.0-beta.2"},
	} {
		got, ok := selectUpdateReleaseForPlatform(releases, "v1.2.0", tc.beta, tc.host, "arm64")
		if !ok || got.TagName != tc.want {
			t.Errorf("%s with beta=%v: got %q, want %q", tc.host, tc.beta, got.TagName, tc.want)
		}
	}
	if got, ok := selectUpdateReleaseForPlatform(releases, "v1.3.0", false, "darwin", "arm64"); ok {
		t.Fatalf("offered %s when no newer stable release contains a DMG", got.TagName)
	}
}

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
