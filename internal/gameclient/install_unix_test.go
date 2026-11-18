//go:build darwin || linux

package gameclient

import (
	"bytes"
	"debug/pe"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInstallsAnX64DLLAndUpgradesOwnedClient(t *testing.T) {
	image, err := pe.NewFile(bytes.NewReader(clientDLL))
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	if image.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || image.Characteristics&pe.IMAGE_FILE_DLL == 0 {
		t.Fatal("embedded client is not an x64 DLL")
	}
	instance := t.TempDir()
	if err := Ensure(instance); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(instance, "mods", "LeviClient", "XboxSignInFix.dll")
	if err := os.WriteFile(path, []byte("old client"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(instance); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, clientDLL) {
		t.Fatal("owned client was not upgraded")
	}
}

func TestEnsureMigratesLegacyManifestAndPreservesDisabledState(t *testing.T) {
	for _, filename := range []string{"manifest.json", "manifest.json.close"} {
		t.Run(filename, func(t *testing.T) {
			instance := t.TempDir()
			dir := filepath.Join(instance, "mods", "LeviClient")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(`{"name":"LeviClient","entry":"LeviClient.dll","type":"preload-native","version":"0.1.0"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := Ensure(instance); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var got clientManifest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Name != "Xbox Sign-in Fix" || got.Entry != "XboxSignInFix.dll" || got.Author != "wzyyyyyyy" {
				t.Fatalf("unexpected manifest: %+v", got)
			}
			if filename == "manifest.json.close" {
				if _, err := os.Stat(filepath.Join(dir, "manifest.json")); !os.IsNotExist(err) {
					t.Fatal("disabled plugin was enabled")
				}
			}
		})
	}
}

func TestEnsurePreservesAnUnrelatedMod(t *testing.T) {
	for _, manifest := range []string{"", `{"name":"Other","entry":"other.dll","type":"preload-native"}`} {
		instance := t.TempDir()
		dir := filepath.Join(instance, "mods", "LeviClient")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		foreign := filepath.Join(dir, "other.dll")
		if err := os.WriteFile(foreign, []byte("preserve"), 0644); err != nil {
			t.Fatal(err)
		}
		if manifest != "" {
			if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := Ensure(instance); err == nil {
			t.Fatal("overwrote unrelated mod")
		}
		if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
			t.Fatal("unrelated mod changed")
		}
	}
}
