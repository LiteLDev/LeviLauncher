package mods

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestUpdateModManifestUpdatesEnabledManifest(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	modDir := filepath.Join(versionsDir, "Demo", "mods", "raw_mod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod dir: %v", err)
	}
	manifestPath := filepath.Join(modDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"name":"Old","entry":"old.dll","version":"0.1.0","type":"preload-native","author":"Alice","extra":true}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if got := UpdateModManifest("Demo", "raw_mod", "New Name", "1.2.3", "native", "new.dll", "Bob"); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}

	manifest := readModsTestManifest(t, manifestPath)
	if manifest.Name != "New Name" {
		t.Fatalf("name = %q", manifest.Name)
	}
	if manifest.Version != "1.2.3" {
		t.Fatalf("version = %q", manifest.Version)
	}
	if manifest.Type != "native" {
		t.Fatalf("type = %q", manifest.Type)
	}
	if manifest.Entry != "new.dll" {
		t.Fatalf("entry = %q", manifest.Entry)
	}
	if manifest.Author != "Bob" {
		t.Fatalf("author = %q", manifest.Author)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(data), `"extra": true`) {
		t.Fatalf("expected unknown manifest fields to be preserved, got %s", data)
	}
}

func TestUpdateModManifestUpdatesDisabledManifestWithoutEnabling(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	modDir := filepath.Join(versionsDir, "Demo", "mods", "disabled_mod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod dir: %v", err)
	}
	closedPath := filepath.Join(modDir, "manifest.json.close")
	if err := os.WriteFile(closedPath, []byte(`{"name":"Old","entry":"old.dll","version":"0.1.0","type":"preload-native","author":"Alice"}`), 0o644); err != nil {
		t.Fatalf("write closed manifest: %v", err)
	}

	if got := UpdateModManifest("Demo", "disabled_mod", "Disabled Name", "2.0.0", "preload-native", "disabled.dll", ""); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}

	if _, err := os.Stat(filepath.Join(modDir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("enabled manifest should not be created, stat err=%v", err)
	}
	manifest := readModsTestManifest(t, closedPath)
	if manifest.Name != "Disabled Name" {
		t.Fatalf("name = %q", manifest.Name)
	}
	if manifest.Version != "2.0.0" {
		t.Fatalf("version = %q", manifest.Version)
	}
	if manifest.Entry != "disabled.dll" {
		t.Fatalf("entry = %q", manifest.Entry)
	}
	if manifest.Author != "" {
		t.Fatalf("author = %q, want empty", manifest.Author)
	}

	data, err := os.ReadFile(closedPath)
	if err != nil {
		t.Fatalf("read closed manifest: %v", err)
	}
	if strings.Contains(string(data), `"author"`) {
		t.Fatalf("empty author should be removed, got %s", data)
	}
}

func TestHasNamedModFindsEnabledAndDisabledManifests(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	modsDir := filepath.Join(versionsDir, "Demo", "mods")
	enabledDir := filepath.Join(modsDir, "enabled")
	disabledDir := filepath.Join(modsDir, "disabled")
	if err := os.MkdirAll(enabledDir, 0o755); err != nil {
		t.Fatalf("mkdir enabled mod dir: %v", err)
	}
	if err := os.MkdirAll(disabledDir, 0o755); err != nil {
		t.Fatalf("mkdir disabled mod dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(enabledDir, "manifest.json"),
		[]byte(`{"name":"Other Mod"}`),
		0o644,
	); err != nil {
		t.Fatalf("write enabled manifest: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(disabledDir, "manifest.json.close"),
		[]byte(`{"name":"LeviLamina"}`),
		0o644,
	); err != nil {
		t.Fatalf("write disabled manifest: %v", err)
	}

	if !HasNamedMod("Demo", "levilamina") {
		t.Fatal("expected disabled LeviLamina manifest to be detected")
	}
	if HasNamedMod("Demo", "missing") {
		t.Fatal("unexpected match for missing mod")
	}
}

func TestHasNamedModRejectsEmptyInput(t *testing.T) {
	setupModsTestVersionsDir(t)
	if HasNamedMod("", "levilamina") {
		t.Fatal("empty instance name must not match")
	}
	if HasNamedMod("Demo", "") {
		t.Fatal("empty target name must not match")
	}
}

func TestHasNamedModPreservesManifestNameWhitespaceSemantics(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	modDir := filepath.Join(versionsDir, "Demo", "mods", "spaced")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(modDir, "manifest.json"),
		[]byte(`{"name":" LeviLamina "}`),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if HasNamedMod("Demo", "levilamina") {
		t.Fatal("manifest name whitespace must not be normalized")
	}
}

func TestHasNamedModIgnoresInvalidManifest(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	modDir := filepath.Join(versionsDir, "Demo", "mods", "invalid")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(modDir, "manifest.json"),
		[]byte(`{"name":`),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if HasNamedMod("Demo", "levilamina") {
		t.Fatal("invalid manifest must not match")
	}
}

func TestImportZipToModsValidatesManifestBeforeWriting(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantError string
	}{
		{
			name: "malformed manifest",
			files: map[string]string{
				"Broken/manifest.json": `{"name":`,
				"Broken/mod.dll":       "dll",
			},
			wantError: "ERR_INVALID_MANIFEST",
		},
		{
			name: "missing required field",
			files: map[string]string{
				"Broken/manifest.json": `{"name":"Broken","entry":"mod.dll","version":"1.0.0"}`,
				"Broken/mod.dll":       "dll",
			},
			wantError: "ERR_INVALID_MANIFEST",
		},
		{
			name: "unsafe entry path",
			files: map[string]string{
				"Broken/manifest.json": `{"name":"Broken","entry":"../mod.dll","version":"1.0.0","type":"preload-native"}`,
				"mod.dll":              "dll",
			},
			wantError: "ERR_INVALID_MANIFEST",
		},
		{
			name: "missing entry file",
			files: map[string]string{
				"Broken/manifest.json": `{"name":"Broken","entry":"mod.dll","version":"1.0.0","type":"preload-native"}`,
			},
			wantError: "ERR_MANIFEST_ENTRY_NOT_FOUND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			versionsDir := setupModsTestVersionsDir(t)
			got := ImportZipToMods("Demo", makeModsTestZip(t, tc.files), false)
			if got != tc.wantError {
				t.Fatalf("ImportZipToMods() = %q, want %q", got, tc.wantError)
			}

			modsDir := filepath.Join(versionsDir, "Demo", "mods")
			entries, err := os.ReadDir(modsDir)
			if err != nil {
				t.Fatalf("read mods dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid archive wrote files: %v", entries)
			}
		})
	}
}

func TestImportZipToModsImportsValidRootAndNestedManifests(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantFolder string
	}{
		{
			name: "nested manifest",
			files: map[string]string{
				"Nested/manifest.json": `{"name":"Display Name","entry":"bin/mod.dll","version":"1.0.0","type":"preload-native"}`,
				"Nested/bin/mod.dll":   "dll",
			},
			wantFolder: "Nested",
		},
		{
			name: "root manifest uses manifest name",
			files: map[string]string{
				"manifest.json": `{"name":"Root Mod","entry":"mod.dll","version":"1.0.0","type":"preload-native"}`,
				"mod.dll":       "dll",
			},
			wantFolder: "Root Mod",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			versionsDir := setupModsTestVersionsDir(t)
			if got := ImportZipToMods("Demo", makeModsTestZip(t, tc.files), false); got != "" {
				t.Fatalf("unexpected error code: %q", got)
			}

			target := filepath.Join(versionsDir, "Demo", "mods", tc.wantFolder)
			if _, err := os.Stat(filepath.Join(target, "manifest.json")); err != nil {
				t.Fatalf("stat imported manifest: %v", err)
			}
		})
	}
}

func TestImportZipToModsInvalidOverwritePreservesExistingMod(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	target := filepath.Join(versionsDir, "Demo", "mods", "Existing")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir existing mod: %v", err)
	}
	markerPath := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(markerPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	data := makeModsTestZip(t, map[string]string{
		"Existing/manifest.json": `{"name":`,
		"Existing/mod.dll":       "dll",
	})
	if got := ImportZipToMods("Demo", data, true); got != "ERR_INVALID_MANIFEST" {
		t.Fatalf("ImportZipToMods() = %q, want ERR_INVALID_MANIFEST", got)
	}

	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read preserved marker: %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("marker content = %q, want original", content)
	}
}

func TestImportZipToModsValidOverwriteReplacesExistingMod(t *testing.T) {
	versionsDir := setupModsTestVersionsDir(t)
	target := filepath.Join(versionsDir, "Demo", "mods", "Existing")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir existing mod: %v", err)
	}
	markerPath := filepath.Join(target, "old.txt")
	if err := os.WriteFile(markerPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old marker: %v", err)
	}

	data := makeModsTestZip(t, map[string]string{
		"Existing/manifest.json": `{"name":"Existing","entry":"mod.dll","version":"2.0.0","type":"preload-native"}`,
		"Existing/mod.dll":       "new",
	})
	if got := ImportZipToMods("Demo", data, true); got != "" {
		t.Fatalf("unexpected error code: %q", got)
	}

	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("old marker should have been removed, stat err=%v", err)
	}
	content, err := os.ReadFile(filepath.Join(target, "mod.dll"))
	if err != nil {
		t.Fatalf("read imported entry: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("entry content = %q, want new", content)
	}
}

func makeModsTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func readModsTestManifest(t *testing.T, path string) types.ModManifestJson {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest types.ModManifestJson
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return manifest
}

func setupModsTestVersionsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	apppath.SetBaseRootOverride(filepath.Join(root, "base"))
	t.Cleanup(func() {
		apppath.SetBaseRootOverride("")
	})
	versionsDir, err := apppath.VersionsDir()
	if err != nil {
		t.Fatalf("resolve versions dir: %v", err)
	}
	return versionsDir
}
