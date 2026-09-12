package mcservice

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestContentRootsUWPChannelsAndSharedData(t *testing.T) {
	_, _, versionsDir := setupInstanceBackupEnv(t)
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	for _, channel := range []string{"release", "beta", "preview"} {
		createTestInstance(t, versionsDir, channel, versions.VersionMeta{PackageType: "uwp", Type: channel, EnableIsolation: true})
		roots := GetContentRoots(channel)
		family := "Microsoft.MinecraftUWP_8wekyb3d8bbwe"
		if channel == "preview" {
			family = "Microsoft.MinecraftWindowsBeta_8wekyb3d8bbwe"
		}
		expected := filepath.Join(local, "Packages", family, "LocalState", "games", "com.mojang")
		if roots.ComMojangRoot != expected || roots.UsersRoot != "" || roots.IsIsolation || roots.PackageType != "uwp" {
			t.Fatalf("%s roots: %+v", channel, roots)
		}
		if roots.Worlds != filepath.Join(expected, "minecraftWorlds") || roots.SkinPacks != filepath.Join(expected, "skin_packs") || roots.Screenshots != filepath.Join(expected, "Screenshots") {
			t.Fatalf("%s direct paths: %+v", channel, roots)
		}
	}
	root := GetContentRoots("release")
	if err := os.MkdirAll(filepath.Join(root.Worlds, "world"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := GetContentCounts("beta"); got.Worlds != 1 {
		t.Fatalf("beta must share release worlds: %+v", got)
	}
	if err := os.MkdirAll(filepath.Join(root.ComMojangRoot, "minecraftpe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root.ComMojangRoot, "minecraftpe", "external_servers.txt"), []byte("1:Test:127.0.0.1:19132:1700000000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := ListServers("release", ""); err != nil || len(got) != 1 || got[0].Name != "Test" {
		t.Fatalf("UWP servers without player: %+v, %v", got, err)
	}
}

func TestContentRootsUWPDoesNotFallBackWhenLocalAppDataMissing(t *testing.T) {
	_, _, versionsDir := setupInstanceBackupEnv(t)
	createTestInstance(t, versionsDir, "uwp", versions.VersionMeta{PackageType: "uwp"})
	t.Setenv("LOCALAPPDATA", "")
	roots := GetContentRoots("uwp")
	if roots.Base != "" || roots.Worlds != "" || roots.UsersRoot != "" || roots.ResourcePacks != "" {
		t.Fatalf("unexpected fallback: %+v", roots)
	}
}
