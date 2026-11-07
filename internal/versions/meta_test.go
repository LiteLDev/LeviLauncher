package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMetadataDropsRemovedRendererSetting(t *testing.T) {
	directory := t.TempDir()
	path := legacyMetaPath(directory)
	legacy := "{\"name\":\"Demo\",\"gameVersion\":\"1.26.0\",\"enableIsolation\":true,\"enableRenderDragon\":true,\"launchArgs\":\"--example\"}"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(directory)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "Demo" || !meta.EnableIsolation || meta.LaunchArgs != "--example" {
		t.Fatalf("unrelated metadata changed: %+v", meta)
	}
	if err := WriteMeta(directory, meta); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(metaPath(directory))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "enableRenderDragon") {
		t.Fatal("removed renderer setting was written back")
	}
}

func TestReadMetaMovesLegacyVersionJSON(t *testing.T) {
	directory := t.TempDir()
	legacy := []byte("{\"name\":\"Demo\",\"gameVersion\":\"1.21.0\",\"unknown\":1}")
	if err := os.WriteFile(legacyMetaPath(directory), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasMeta(directory) {
		t.Fatal("HasMeta = false for a legacy instance")
	}
	meta, err := ReadMeta(directory)
	if err != nil || meta.Name != "Demo" {
		t.Fatalf("ReadMeta = %+v, %v", meta, err)
	}
	moved, err := os.ReadFile(filepath.Join(directory, ".levilauncher", "instance.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The file moves byte for byte, keeping members this version ignores.
	if string(moved) != string(legacy) {
		t.Fatalf("instance.json = %s", moved)
	}
	if _, err := os.Stat(legacyMetaPath(directory)); !os.IsNotExist(err) {
		t.Fatalf("version.json left behind: %v", err)
	}
}
