package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMetadataDropsRemovedRendererSetting(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, metaFileName)
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
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "enableRenderDragon") {
		t.Fatal("removed renderer setting was written back")
	}
}
