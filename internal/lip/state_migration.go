package lip

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// This file converts the state file of a workspace written by an earlier
// launcher version, whose built-in lip named the side "platform" and the
// platform "runtime", to the keys of lip v0.40 ("side" and "platform").
// The built-in lip follows lip v0.40, which refuses a state file with the
// old keys, so without the conversion every existing installation would
// fail its next package operation. The conversion only renames keys; the
// state file is rewritten atomically, and only when lip can read the
// result.

// migrateWorkspaceState rewrites <root>/.lip/state.json when it uses the
// keys of lip before v0.40, in the state file itself and in the package
// files it records. It reports whether it rewrote the file. A missing state
// file, one without the old keys, or one whose conversion lip could not
// read is left alone.
func migrateWorkspaceState(root string) (bool, error) {
	path, err := pkgfile.WorkspacePath(root)
	if err != nil {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// No state file, or one lip cannot read: lip reports that itself.
		return false, nil
	}
	converted, changed := migrateStateDocument(data)
	if !changed || !validWorkspace(converted, path) {
		return false, nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	mutex, err := txn.Lock(absRoot)
	if err != nil {
		if _, busy := errors.AsType[*txn.BusyError](err); busy {
			// Another lip is changing the workspace; it converts the file
			// itself, or the operation reports the busy workspace.
			return false, nil
		}
		return false, err
	}
	defer mutex.Close()

	// Another process may have converted the file before the lock was taken.
	current, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	converted, changed = migrateStateDocument(current)
	if !changed || !validWorkspace(converted, path) {
		return false, nil
	}
	if err := writeStateFile(path, converted); err != nil {
		return false, err
	}
	return true, nil
}

// validWorkspace reports whether lip reads data as a state file named path.
func validWorkspace(data []byte, path string) bool {
	_, err := pkgfile.ParseWorkspace(data, path)
	if err != nil {
		log.Printf("lip: not converting the state file %s with the keys of lip before v0.40: %v", path, err)
		return false
	}
	return true
}

// writeStateFile writes data where path is, atomically: a uniquely named
// temporary file in the same directory is renamed over the state file.
func writeStateFile(path string, data []byte) error {
	temp := filepath.Join(filepath.Dir(path), txn.NewGUID()+".tmp")
	if err := os.WriteFile(temp, data, 0o666); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("Cannot write the converted state file '%s': %w", temp, err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("Cannot replace the state file '%s' with the converted one: %w", path, err)
	}
	return nil
}

// migrateStateDocument converts the bytes of a state file to the keys of
// lip v0.40, returning the new bytes and whether they differ from data.
// Bytes that are not a JSON object, or that have no old keys, are left
// alone.
func migrateStateDocument(data []byte) ([]byte, bool) {
	// A state file with the old keys always has "runtime", the old name of
	// the platform, so a file without that member needs no conversion.
	if !bytes.Contains(data, []byte(`"runtime"`)) {
		return nil, false
	}
	v, err := doc.ParseJSONFile(data)
	if err != nil {
		return nil, false
	}
	root, ok := v.(*doc.Object)
	if !ok {
		return nil, false
	}
	converted, packagesChanged := migrateStatePackages(root)
	converted, rootChanged := renameOldKeys(converted)
	if !packagesChanged && !rootChanged {
		return nil, false
	}
	return doc.MarshalJSONFile(converted), true
}

// migrateStatePackages converts the package file each package of a state
// file records. The state file itself is returned unchanged when no package
// file has the old keys.
func migrateStatePackages(root *doc.Object) (*doc.Object, bool) {
	value, ok := root.Get("packages")
	if !ok {
		return root, false
	}
	packages, ok := value.([]any)
	if !ok {
		return root, false
	}
	converted := make([]any, len(packages))
	changed := false
	for i, p := range packages {
		converted[i] = p
		object, ok := p.(*doc.Object)
		if !ok {
			continue
		}
		manifest, ok := object.Get("manifest")
		if !ok {
			continue
		}
		convertedManifest, manifestChanged := migrateManifest(manifest)
		if !manifestChanged {
			continue
		}
		convertedPackage := doc.NewObject()
		for _, key := range object.Keys() {
			if key == "manifest" {
				convertedPackage.Set(key, convertedManifest)
				continue
			}
			member, _ := object.Get(key)
			convertedPackage.Set(key, member)
		}
		converted[i] = convertedPackage
		changed = true
	}
	if !changed {
		return root, false
	}
	out := doc.NewObject()
	for _, key := range root.Keys() {
		if key == "packages" {
			out.Set(key, converted)
			continue
		}
		member, _ := root.Get(key)
		out.Set(key, member)
	}
	return out, true
}

// migrateManifest converts the entries of the variants of a package file
// recorded in a state file. The package file itself is returned unchanged
// when no entry has the old keys.
func migrateManifest(value any) (any, bool) {
	manifest, ok := value.(*doc.Object)
	if !ok {
		return value, false
	}
	variantsValue, ok := manifest.Get("variants")
	if !ok {
		return value, false
	}
	variants, ok := variantsValue.(*doc.Object)
	if !ok {
		return value, false
	}
	converted := doc.NewObject()
	changed := false
	for _, name := range variants.Keys() {
		entriesValue, _ := variants.Get(name)
		entries, ok := entriesValue.([]any)
		if !ok {
			converted.Set(name, entriesValue)
			continue
		}
		convertedEntries := make([]any, len(entries))
		for i, entry := range entries {
			convertedEntries[i] = entry
			object, ok := entry.(*doc.Object)
			if !ok {
				continue
			}
			if convertedEntry, entryChanged := renameOldKeys(object); entryChanged {
				convertedEntries[i] = convertedEntry
				changed = true
			}
		}
		converted.Set(name, convertedEntries)
	}
	if !changed {
		return value, false
	}
	out := doc.NewObject()
	for _, key := range manifest.Keys() {
		if key == "variants" {
			out.Set(key, converted)
			continue
		}
		member, _ := manifest.Get(key)
		out.Set(key, member)
	}
	return out, true
}

// renameOldKeys returns object with the keys of lip before v0.40 replaced:
// "platform", which named the side, becomes "side", and "runtime", which
// named the platform, becomes "platform". The new key takes the place of
// the old one, so the state file keeps lip's member order. The object is
// returned unchanged when it has no old keys.
func renameOldKeys(object *doc.Object) (*doc.Object, bool) {
	if !pkgfile.UsesOldKeys(object) {
		return object, false
	}
	side, hasSide := object.Get("side")
	platform, hasPlatform := object.Get("platform")
	runtime, hasRuntime := object.Get("runtime")

	platformIsOldSide := false
	if name, ok := platform.(string); ok && pkgfile.IsSide(name) {
		platformIsOldSide = true
	}
	newSide, hasNewSide := side, hasSide
	if !hasNewSide && platformIsOldSide {
		newSide, hasNewSide = platform, true
	}
	newPlatform, hasNewPlatform := any(nil), false
	if hasPlatform && !platformIsOldSide {
		newPlatform, hasNewPlatform = platform, true
	}
	platformFromRuntime := false
	if !hasNewPlatform && hasRuntime {
		newPlatform, hasNewPlatform, platformFromRuntime = runtime, true, true
	}

	out := doc.NewObject()
	sideWritten, platformWritten := false, false
	for _, key := range object.Keys() {
		member, _ := object.Get(key)
		switch key {
		case "side":
			if hasNewSide && !sideWritten {
				out.Set("side", newSide)
				sideWritten = true
			}
		case "platform":
			if platformIsOldSide {
				// The old key of the side. Its value is written as "side" in
				// its place unless the object already had one.
				if !hasSide && hasNewSide && !sideWritten {
					out.Set("side", newSide)
					sideWritten = true
				}
				continue
			}
			if hasNewPlatform && !platformWritten {
				out.Set("platform", newPlatform)
				platformWritten = true
			}
		case "runtime":
			if platformFromRuntime && !platformWritten {
				out.Set("platform", newPlatform)
				platformWritten = true
			}
		default:
			out.Set(key, member)
		}
	}
	// Old keys the loop did not write (a "platform" of the side with an
	// existing "side" after it, say) appear once, at the end.
	if hasNewSide && !sideWritten {
		out.Set("side", newSide)
	}
	if hasNewPlatform && !platformWritten {
		out.Set("platform", newPlatform)
	}
	return out, true
}
