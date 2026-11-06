package content

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func packArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for name, body := range files {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestPackReplacementPreservesInstalledCopyOnFailure(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "resource_packs")
	files := map[string]string{
		"wrapped/manifest.json":     `{"header":{"uuid":"test-pack"},"modules":[{"type":"resources"}]}`,
		"wrapped/item.txt":          "original",
		"wrapped/../../escaped.txt": "outside",
	}
	original := packArchive(t, files)
	if result := ImportMcpackToDirs(original, res, "", "", false); result != "" {
		t.Fatal(result)
	}
	if result := ImportMcpackToDirs(original, res, "", "", false); result != "ERR_DUPLICATE_UUID" {
		t.Fatalf("duplicate: %q", result)
	}
	check := func(want string) {
		t.Helper()
		entries, err := os.ReadDir(res)
		if err != nil || len(entries) != 1 {
			t.Fatalf("installed copies: %v, %v", entries, err)
		}
		got, err := os.ReadFile(filepath.Join(res, entries[0].Name(), "item.txt"))
		if err != nil || string(got) != want {
			t.Fatalf("installed content: %q, %v", got, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); !os.IsNotExist(err) {
			t.Fatalf("entry escaped the pack directory: %v", err)
		}
	}
	files["wrapped/item.txt"] = "replacement payload"
	replacement := packArchive(t, files)
	corrupt := bytes.Clone(replacement)
	corrupt[bytes.Index(corrupt, []byte("replacement payload"))] ^= 0xff
	if result := ImportMcpackToDirs(corrupt, res, "", "", true); result != "ERR_WRITE_FILE" {
		t.Fatalf("invalid CRC: %q", result)
	}
	check("original")
	if result := ImportMcpackToDirs(replacement, res, "", "", true); result != "" {
		t.Fatal(result)
	}
	check("replacement payload")
}

func TestAddonImportsEmbeddedAndUnpackedModules(t *testing.T) {
	dir := t.TempDir()
	res, bp, skin := filepath.Join(dir, "resources"), filepath.Join(dir, "behavior"), filepath.Join(dir, "skins")
	pack := packArchive(t, map[string]string{"manifest.json": `{"modules":[{"type":"resources"}]}`})
	addon := packArchive(t, map[string]string{
		"nested.mcpack":         string(pack),
		"scripts/manifest.json": `{"modules":[{"type":"data"},{"type":"script"}]}`,
		"skin/manifest.json":    `{"modules":[{"type":"skin_pack"}]}`,
	})
	if result := ImportMcaddonToDirs(addon, res, bp, skin, false); result != "" {
		t.Fatal(result)
	}
	for _, root := range []string{res, bp, skin} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 {
			t.Fatalf("%s: %v, %v", root, entries, err)
		}
	}
	skinOnly := packArchive(t, map[string]string{"skin/manifest.json": `{"modules":[{"type":"skin_pack"}]}`})
	if result := ImportMcaddonToDirs(skinOnly, res, bp, "", false); result != "ERR_NO_PLAYER" {
		t.Fatalf("skin without player: %q", result)
	}
}

func TestConcurrentImportOfSamePack(t *testing.T) {
	root := t.TempDir()
	pack := packArchive(t, map[string]string{
		"manifest.json": `{"header":{"uuid":"same-pack"},"modules":[{"type":"resources"}]}`,
	})
	start := make(chan struct{})
	results := make(chan string, 2)
	for range 2 {
		go func() {
			<-start
			results <- ImportMcpackToDirs(pack, root, "", "", false)
		}()
	}
	close(start)
	first, second := <-results, <-results
	if !((first == "" && second == "ERR_DUPLICATE_UUID") || (second == "" && first == "ERR_DUPLICATE_UUID")) {
		t.Fatalf("concurrent imports returned %q and %q", first, second)
	}
}
