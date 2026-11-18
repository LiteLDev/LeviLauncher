//go:build darwin || linux

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

//go:embed LeviClient.dll
var clientDLL []byte

func Ensure(instance string) error {
	dir := filepath.Join(instance, "mods", "LeviClient")
	manifest := filepath.Join(dir, "manifest.json")
	if data, err := os.ReadFile(manifest); err == nil {
		var existing struct{ Name, Entry, Type string }
		if json.Unmarshal(data, &existing) != nil || existing.Name != "LeviClient" || existing.Entry != "LeviClient.dll" || existing.Type != "preload-native" {
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
	path := filepath.Join(dir, "LeviClient.dll")
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, clientDLL) {
		if err := fsx.WriteFileAtomic(path, clientDLL, 0644); err != nil {
			return err
		}
	}
	return fsx.WriteFileAtomic(manifest, []byte("{\n  \"name\": \"LeviClient\",\n  \"entry\": \"LeviClient.dll\",\n  \"version\": \"0.1.0\",\n  \"type\": \"preload-native\"\n}\n"), 0644)
}
