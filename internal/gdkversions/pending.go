package gdkversions

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Historical entries win as a whole, including their verified MD5 and mirrors.
// Remember every unmatched Store entry, even when Store has moved on again.
func mergeVersions(history, pending, latest []Version) ([]Version, []Version) {
	merged := make([]Version, 0, len(history)+len(pending)+len(latest))
	remaining := make([]Version, 0)
	seen := make(map[string]bool)
	for _, version := range history {
		key := versionKey(version.Version)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, version)
		}
	}
	// Keep the first-seen timestamp stable but refresh a pending version's URLs.
	updates := make(map[string]Version, len(latest))
	for _, version := range latest {
		updates[versionKey(version.Version)] = version
	}
	for _, version := range append(append([]Version(nil), pending...), latest...) {
		key := versionKey(version.Version)
		if seen[key] {
			continue
		}
		seen[key] = true
		if update, ok := updates[key]; ok {
			version.URLs = update.URLs
		}
		merged = append(merged, version)
		remaining = append(remaining, version)
	}
	return merged, remaining
}

func readPending(filename string) Catalog {
	var pending Catalog
	raw, err := os.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(raw, &pending)
	}
	if err != nil && !os.IsNotExist(err) {
		log.Printf("GDK pending catalog read failed: %v", err)
		return Catalog{}
	}
	return pending
}

func writePending(filename string, pending Catalog) {
	if filename == "" {
		return
	}
	raw, err := json.Marshal(pending)
	if err != nil {
		return
	}
	previous, readErr := os.ReadFile(filename)
	if bytes.Equal(raw, previous) || (os.IsNotExist(readErr) && len(pending.ReleaseVersions)+len(pending.PreviewVersions) == 0) {
		return
	}
	if err = savePending(filename, raw); err != nil {
		log.Printf("GDK pending catalog write failed: %v", err)
	}
}

func savePending(filename string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".gdk-pending-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filename)
}
