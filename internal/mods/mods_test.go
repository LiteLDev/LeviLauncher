package mods

import (
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
