package contentmgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/materialbin"
)

func TestCheckResourcePackMaterialCompatibilityBatchPreservesOrderAndResults(
	t *testing.T,
) {
	manager, gameMaterialPath := setupMaterialCompatibilityTest(t, "Demo")
	writeMaterialFixture(t, gameMaterialPath, 18)

	compatiblePack := filepath.Join(t.TempDir(), "compatible")
	incompatiblePack := filepath.Join(t.TempDir(), "incompatible")
	noMaterialPack := filepath.Join(t.TempDir(), "no-material")
	invalidPack := filepath.Join(t.TempDir(), "invalid")
	writePackMaterialFixture(t, compatiblePack, "Compatible.material.bin", 18)
	writePackMaterialFixture(t, incompatiblePack, "Incompatible.material.bin", 19)
	if err := os.MkdirAll(noMaterialPack, 0o755); err != nil {
		t.Fatalf("mkdir no-material pack: %v", err)
	}
	invalidPath := filepath.Join(
		invalidPack,
		"renderer",
		"materials",
		"Invalid.material.bin",
	)
	if err := os.MkdirAll(filepath.Dir(invalidPath), 0o755); err != nil {
		t.Fatalf("mkdir invalid material dir: %v", err)
	}
	if err := os.WriteFile(invalidPath, []byte("invalid"), 0o644); err != nil {
		t.Fatalf("write invalid material: %v", err)
	}

	got := manager.CheckResourcePackMaterialCompatibilityBatch(
		"Demo",
		[]string{compatiblePack, noMaterialPack, incompatiblePack, invalidPack},
	)
	if len(got) != 4 {
		t.Fatalf("result length = %d, want 4", len(got))
	}

	if !got[0].HasMaterialBin ||
		!got[0].Compatible ||
		got[0].NeedsUpdate ||
		got[0].PackMaterialVersion != 18 ||
		got[0].GameMaterialVersion != 18 ||
		got[0].Error != "" {
		t.Fatalf("compatible result = %#v", got[0])
	}
	if got[1].HasMaterialBin || !got[1].Compatible || got[1].Error != "" {
		t.Fatalf("no-material result = %#v", got[1])
	}
	if !got[2].HasMaterialBin ||
		got[2].Compatible ||
		!got[2].NeedsUpdate ||
		got[2].PackMaterialVersion != 19 ||
		got[2].GameMaterialVersion != 18 ||
		got[2].Error != "" {
		t.Fatalf("incompatible result = %#v", got[2])
	}
	if !got[3].HasMaterialBin ||
		!got[3].Compatible ||
		got[3].Error != "ERR_READ_PACK_MATERIALBIN" {
		t.Fatalf("invalid material result = %#v", got[3])
	}
}

func TestCheckResourcePackMaterialCompatibilityBatchResolvesGameLazily(
	t *testing.T,
) {
	manager, gameMaterialPath := setupMaterialCompatibilityTest(t, "Demo")
	materialPack := filepath.Join(t.TempDir(), "material")
	noMaterialPack := filepath.Join(t.TempDir(), "no-material")
	writePackMaterialFixture(t, materialPack, "Pack.material.bin", 18)
	if err := os.MkdirAll(noMaterialPack, 0o755); err != nil {
		t.Fatalf("mkdir no-material pack: %v", err)
	}

	got := manager.CheckResourcePackMaterialCompatibilityBatch(
		"Demo",
		[]string{noMaterialPack, materialPack},
	)
	if len(got) != 2 {
		t.Fatalf("result length = %d, want 2", len(got))
	}
	if got[0].HasMaterialBin || !got[0].Compatible || got[0].Error != "" {
		t.Fatalf("no-material result = %#v", got[0])
	}
	if !got[1].HasMaterialBin ||
		!got[1].Compatible ||
		got[1].Error != "ERR_READ_GAME_RENDERCHUNK" ||
		got[1].GameMaterialPath != gameMaterialPath {
		t.Fatalf("missing game material result = %#v", got[1])
	}
}

func TestCheckResourcePackMaterialCompatibilityBatchPreservesInvalidInputSemantics(
	t *testing.T,
) {
	manager := New(Deps{})
	got := manager.CheckResourcePackMaterialCompatibilityBatch(
		"",
		[]string{"pack", ""},
	)
	if len(got) != 2 {
		t.Fatalf("result length = %d, want 2", len(got))
	}
	for index, result := range got {
		if !result.Compatible || result.Error != "invalid input" {
			t.Fatalf("result %d = %#v", index, result)
		}
	}
}

func TestCheckResourcePackMaterialCompatibilityBatchPreservesDuplicates(
	t *testing.T,
) {
	manager, gameMaterialPath := setupMaterialCompatibilityTest(t, "Demo")
	writeMaterialFixture(t, gameMaterialPath, 18)
	packPath := filepath.Join(t.TempDir(), "pack")
	writePackMaterialFixture(t, packPath, "Pack.material.bin", 18)

	got := manager.CheckResourcePackMaterialCompatibilityBatch(
		"Demo",
		[]string{packPath, packPath},
	)
	if len(got) != 2 || got[0] != got[1] {
		t.Fatalf("duplicate results = %#v", got)
	}
}

func TestCheckResourcePackMaterialCompatibilityBatchReturnsNonNilEmptySlice(
	t *testing.T,
) {
	manager := New(Deps{})
	got := manager.CheckResourcePackMaterialCompatibilityBatch("Demo", nil)
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("result length = %d, want 0", len(got))
	}
}

func setupMaterialCompatibilityTest(
	t *testing.T,
	versionName string,
) (*Manager, string) {
	t.Helper()

	baseRoot := filepath.Join(t.TempDir(), "base")
	apppath.SetBaseRootOverride(baseRoot)
	t.Cleanup(func() {
		apppath.SetBaseRootOverride("")
	})

	versionsDir, err := apppath.VersionsDir()
	if err != nil {
		t.Fatalf("resolve versions dir: %v", err)
	}
	gameMaterialPath := filepath.Join(
		versionsDir,
		versionName,
		"data",
		"renderer",
		"materials",
		"RenderChunk.material.bin",
	)
	return New(Deps{}), gameMaterialPath
}

func writePackMaterialFixture(
	t *testing.T,
	packPath string,
	name string,
	version uint64,
) {
	t.Helper()
	writeMaterialFixture(
		t,
		filepath.Join(packPath, "renderer", "materials", name),
		version,
	)
}

func writeMaterialFixture(t *testing.T, path string, version uint64) {
	t.Helper()

	definition := &materialbin.CompiledMaterialDefinition{
		Version:           version,
		EncryptionVariant: materialbin.EncryptionVariantNone,
		Name:              "Test/Material",
	}
	data, err := definition.MarshalBinary(version)
	if err != nil {
		t.Fatalf("marshal material version %d: %v", version, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir material dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write material: %v", err)
	}
}
