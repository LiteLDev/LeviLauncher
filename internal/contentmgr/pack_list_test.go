package contentmgr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestListPackDetailsForVersion(t *testing.T) {
	root := t.TempDir()
	roots := types.ContentRoots{
		ResourcePacks: filepath.Join(root, "resource_packs"),
		BehaviorPacks: filepath.Join(root, "behavior_packs"),
		UsersRoot:     filepath.Join(root, "Users"),
	}
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// The module determines the type even when the wrapper lives in a different root.
	dir := filepath.Join(roots.BehaviorPacks, "wrapper", "pack")
	write(dir, "manifest.json", `{"header":{"name":"pack.name","description":"pack.description","version":[1,2,3,4],"min_engine_version":"1.21"},"modules":[{"type":"resources"}]}`)
	write(filepath.Join(dir, "texts"), "en_US.lang", "pack.name=Display name\npack.description=Localized description\n")
	write(dir, "pack_icon.png", "PNG")
	stamp := time.Unix(1700000000, 0)
	if err := os.Chtimes(dir, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(roots.ResourcePacks, "broken"), "manifest.json", "{")
	shared := filepath.Join(root, "skin_packs", "shared")
	player := filepath.Join(roots.UsersRoot, "one", "games", "com.mojang", "skin_packs", "private")
	outside := filepath.Join(root, "outside", "skin_packs", "private")
	for _, skin := range []string{shared, player, outside} {
		write(skin, "manifest.json", `{"header":{"name":"Skin"},"modules":[{"type":"skin_pack"}]}`)
	}

	manager := New(Deps{PackLoader: packages.PackManager{}, GetContentRoots: func(string) types.ContentRoots { return roots }})
	infos := manager.ListPackDetailsForVersion("demo", "", packages.PackTypeResources)
	want := types.PackListEntry{PackInfo: types.PackInfo{
		Path: dir, Name: "Display name", Description: "Localized description",
		Version: "1.2.3.4", MinEngineVersion: "1.21", IconDataUrl: "data:image/png;base64,UE5H",
	}, ModTime: stamp.Unix()}
	if len(infos) != 1 || infos[0] != want {
		t.Fatalf("resource details = %+v, want %+v", infos, want)
	}
	if got := manager.ListPackDetailsForVersion("demo", "", packages.PackTypeBehavior); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty behavior list, got %+v", got)
	}
	if got := manager.ListPackDetailsForVersion("demo", "", packages.PackTypeSkins); len(got) != 1 || got[0].Path != shared {
		t.Fatalf("unexpected shared skins: %+v", got)
	}
	if got := manager.ListPackDetailsForVersion("demo", "one", packages.PackTypeSkins); len(got) != 2 || got[0].Path != shared || got[1].Path != player {
		t.Fatalf("unexpected player skins: %+v", got)
	}
	if got := manager.ListPackDetailsForVersion("demo", "../outside", packages.PackTypeSkins); len(got) != 1 || got[0].Path != shared {
		t.Fatalf("invalid player exposed private skins: %+v", got)
	}
}
