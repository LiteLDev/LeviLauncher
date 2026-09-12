package contentmgr

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func testUWPRoots(base string) types.ContentRoots {
	root := filepath.Join(base, "LocalState", "games", "com.mojang")
	return types.ContentRoots{
		PackageType: "uwp", ComMojangRoot: root,
		Worlds:        filepath.Join(root, "minecraftWorlds"),
		ResourcePacks: filepath.Join(root, "resource_packs"),
		BehaviorPacks: filepath.Join(root, "behavior_packs"),
		SkinPacks:     filepath.Join(root, "skin_packs"),
		Screenshots:   filepath.Join(root, "Screenshots"),
	}
}

func TestPlayerContentDirUWPAndGDK(t *testing.T) {
	uwp := testUWPRoots(t.TempDir())
	for kind, expected := range map[string]string{"minecraftWorlds": uwp.Worlds, "skin_packs": uwp.SkinPacks, "Screenshots": uwp.Screenshots} {
		for _, player := range []string{"", "old-gdk-player"} {
			if got := playerContentDir(uwp, player, kind); got != expected {
				t.Fatalf("UWP %s player %q: got %q, want %q", kind, player, got, expected)
			}
		}
	}
	gdk := types.ContentRoots{UsersRoot: filepath.Join(t.TempDir(), "Users")}
	if got, expected := playerContentDir(gdk, "123", "minecraftWorlds"), filepath.Join(gdk.UsersRoot, "123", "games", "com.mojang", "minecraftWorlds"); got != expected {
		t.Fatalf("GDK path: got %q, want %q", got, expected)
	}
	for _, player := range []string{"", ".", "..", "../outside", `..\outside`, `C:\outside`} {
		if got := playerContentDir(gdk, player, "minecraftWorlds"); got != "" {
			t.Fatalf("invalid GDK player %q resolved to %q", player, got)
		}
	}
}

func TestUWPWorldImportTransferAndDeleteWithoutPlayer(t *testing.T) {
	root := t.TempDir()
	source := testUWPRoots(filepath.Join(root, "release"))
	target := testUWPRoots(filepath.Join(root, "preview"))
	manager := New(Deps{GetContentRoots: func(name string) types.ContentRoots {
		if name == "source" {
			return source
		}
		return target
	}})
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	f, err := zw.Create("level.dat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("test-world")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := manager.ImportMcworld("source", "", "world.mcworld", archive.Bytes(), false); err != "" {
		t.Fatalf("import: %s", err)
	}
	entries, err := os.ReadDir(source.Worlds)
	if err != nil || len(entries) != 1 {
		t.Fatalf("imported worlds: %v, %v", entries, err)
	}
	world := filepath.Join(source.Worlds, entries[0].Name())
	if err := manager.TransferWorldToVersion("source", "", world, "target", ""); err != "" {
		t.Fatalf("transfer: %s", err)
	}
	if data, err := os.ReadFile(filepath.Join(target.Worlds, entries[0].Name(), "level.dat")); err != nil || string(data) != "test-world" {
		t.Fatalf("transferred data: %q, %v", data, err)
	}
	if err := manager.DeleteWorld("source", source.Worlds); err != "ERR_INVALID_PATH" {
		t.Fatalf("must reject deleting content root: %s", err)
	}
	if err := manager.DeleteWorld("source", world); err != "" {
		t.Fatalf("delete: %s", err)
	}
	if _, err := os.Stat(world); !os.IsNotExist(err) {
		t.Fatalf("source world still exists: %v", err)
	}
}

func TestUWPSkinsAndScreenshotsWithoutPlayer(t *testing.T) {
	roots := testUWPRoots(t.TempDir())
	manager := New(Deps{GetContentRoots: func(string) types.ContentRoots { return roots }})
	if got := manager.versionSkinDir("old-uwp", roots); got != roots.SkinPacks {
		t.Fatalf("old UWP skin target: %q", got)
	}
	for _, dir := range []string{roots.Screenshots, filepath.Join(roots.Screenshots, "world"), filepath.Join(roots.SkinPacks, "skin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"flat.png", filepath.Join("world", "nested.jpg")} {
		if err := os.WriteFile(filepath.Join(roots.Screenshots, file), []byte("image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := manager.ListScreenshots("uwp", ""); len(got) != 2 {
		t.Fatalf("expected flat and nested screenshots, got %+v", got)
	}
	if err := manager.DeleteScreenshot("uwp", "", filepath.Join(roots.Screenshots, "flat.png")); err != "" {
		t.Fatal(err)
	}
	if got := manager.ListScreenshots("uwp", ""); len(got) != 1 {
		t.Fatalf("remaining screenshots: %+v", got)
	}
	if err := manager.DeletePack("uwp", filepath.Join(roots.SkinPacks, "skin")); err != "" {
		t.Fatal(err)
	}
	if err := manager.DeletePack("uwp", roots.SkinPacks); err != "ERR_INVALID_PACKAGE" {
		t.Fatalf("must retain skin root: %s", err)
	}
}
