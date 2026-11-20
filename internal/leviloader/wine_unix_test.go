//go:build darwin

package leviloader

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestWineInstanceLoaderSurvivesActivationAndMetadataRefresh(t *testing.T) {
	dir := t.TempDir()
	writeTestVersion(t, dir)
	meta := versions.VersionMeta{Name: "instance", PackageType: "gdk", EnableIsolation: true}
	if err := versions.WriteMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	loader := append(append([]byte(nil), embeddedLoader...), []byte("Wine instance fixture")...)
	if err := os.WriteFile(filepath.Join(dir, ".levilauncher", "wine-loader.dll"), loader, 0600); err != nil {
		t.Fatal(err)
	}
	for _, isolation := range []bool{true, false} {
		meta.EnableIsolation = isolation
		if err := versions.WriteMeta(dir, meta); err != nil {
			t.Fatal(err)
		}
		if _, err := PatchAndActivateForWine(context.Background(), dir); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(dir, LoaderDLLName)); err != nil || !bytes.Equal(data, loader) {
			t.Fatal("compatible Wine loader was replaced by the bundled Windows loader")
		}
		if got, err := versions.ReadMeta(dir); err != nil || got.EnableIsolation != isolation {
			t.Fatalf("instance refresh changed settings: %+v %v", got, err)
		}
		current, err := os.ReadFile(filepath.Join(dir, ".levilauncher", "instance.json"))
		legacy, legacyErr := os.ReadFile(filepath.Join(dir, "version.json"))
		if err != nil || legacyErr != nil || !bytes.Equal(current, legacy) {
			t.Fatal("runtime metadata disappeared or did not follow the saved instance settings")
		}
	}
	// Windows uses this path even when the same instance carries a Wine override.
	if _, err := PatchAndActivate(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, LoaderDLLName)); err != nil || !bytes.Equal(data, embeddedLoader) {
		t.Fatal("Wine override changed the standard Windows activation path")
	}
}

func TestWineInstanceLoaderRejectsInvalidOverride(t *testing.T) {
	dir := t.TempDir()
	writeTestVersion(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".levilauncher"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".levilauncher", "wine-loader.dll"), []byte("invalid PE"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PatchAndActivateForWine(context.Background(), dir); err == nil {
		t.Fatal("invalid Wine loader accepted")
	}
}
