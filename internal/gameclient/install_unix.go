//go:build darwin

package gameclient

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

//go:embed XboxSignInFix.dll
var clientDLL []byte

const (
	modName    = "Xbox Sign-in Fix"
	modEntry   = "XboxSignInFix.dll"
	modType    = "preload-native"
	legacyName = "LeviClient"
	legacyDLL  = "LeviClient.dll"
)

type clientManifest struct {
	Name    string `json:"name"`
	Entry   string `json:"entry"`
	Type    string `json:"type"`
	Author  string `json:"author"`
	Version string `json:"version"`
}

func Ensure(instance string) error {
	// The folder is the launcher's persistent mod identifier. Keeping it
	// lets existing instances upgrade without loading a second copy.
	dir := filepath.Join(instance, "mods", legacyName)
	manifest := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifest)
	if os.IsNotExist(err) {
		disabled := manifest + ".close"
		data, err = os.ReadFile(disabled)
		if err == nil {
			manifest = disabled
		}
	}
	if err == nil {
		var existing clientManifest
		if json.Unmarshal(data, &existing) != nil || existing.Type != modType ||
			!((existing.Name == modName && existing.Entry == modEntry) ||
				(existing.Name == legacyName && existing.Entry == legacyDLL)) {
			return fmt.Errorf("mods/LeviClient is occupied by another mod")
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if entries, readErr := os.ReadDir(dir); readErr == nil && len(entries) != 0 {
		return fmt.Errorf("mods/LeviClient has files without a client manifest")
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, modEntry)
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, clientDLL) {
		if err := fsx.WriteFileAtomic(path, clientDLL, 0644); err != nil {
			return err
		}
	}
	metadata, err := json.MarshalIndent(clientManifest{
		Name: modName, Entry: modEntry, Type: modType, Author: "wzyyyyyyy", Version: "0.1.1",
	}, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(manifest, append(metadata, '\n'), 0644)
}
