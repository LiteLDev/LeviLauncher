// Package lockfile reads and writes tooth_lock.json, lip 0.37.0's workspace
// state (src/Lip.Core/Entities/WorkspaceState*.cs, Services/WorkspaceService.cs,
// schemas/workspace_state.schema.json), byte-compatible with lip.
package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
)

// ErrNotImplemented is returned by skeleton stubs.
//
// Deprecated: kept for API compatibility; nothing returns it any more.
var ErrNotImplemented = errors.New("lockfile: not implemented")

// FileName is the state file name inside the workspace (instance) directory.
const FileName = "tooth_lock.json"

// State is the whole lock file. Field order is lip's serialization order.
type State struct {
	FormatVersion int       `json:"format_version"`
	FormatUUID    string    `json:"format_uuid"`
	Packages      []Package `json:"packages"`
}

// Package is one installed package.
type Package struct {
	// Files are absolute paths (FileInfo.FullName, backslashes on Windows)
	// of the files this package placed.
	Files []string `json:"files"`
	// Locked is true when the package was installed explicitly.
	Locked bool `json:"locked"`
	// Manifest is the full manifest (all variants) as installed.
	Manifest manifest.Manifest `json:"manifest"`
	// Variant is the installed variant label.
	Variant string `json:"variant"`
}

// ID returns the installed package id (Manifest.Tooth + Variant).
func (p Package) ID() manifest.PackageID {
	return manifest.PackageID{Path: p.Manifest.Tooth, Variant: p.Variant}
}

// Spec returns "path#variant@version".
func (p Package) Spec() manifest.PackageSpec {
	return manifest.PackageSpec{ID: p.ID(), Version: p.Manifest.Version}
}

// New returns an empty state with format version/uuid set.
func New() *State {
	return &State{FormatVersion: manifest.FormatVersion, FormatUUID: manifest.FormatUUID, Packages: []Package{}}
}

// Load reads <dir>/tooth_lock.json. A missing file yields New(). Like lip,
// a malformed file is reported via the returned error AND an empty state is
// returned, so callers may print the error and continue.
func Load(dir string) (*State, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return New(), nil
		}
		return New(), fmt.Errorf("failed to load workspace state from '%s': %w", FileName, err)
	}
	s, err := Unmarshal(data)
	if err != nil {
		return New(), fmt.Errorf("failed to load workspace state from '%s': %w", FileName, err)
	}
	return s, nil
}

// Unmarshal decodes lock file bytes with System.Text.Json's rules for
// lip's WorkspaceState: format_version/format_uuid are optional but must
// match when present, "packages" and every package property are required,
// and each manifest is decoded like manifest.Parse.
func Unmarshal(data []byte) (*State, error) {
	const what = "workspace state"
	o, err := manifest.DecodeObject(manifest.TrimBOM(data), what)
	if err != nil {
		return nil, err
	}
	if fv, ok, err := o.Int("format_version", what); err != nil {
		return nil, err
	} else if ok && fv != manifest.FormatVersion {
		return nil, fmt.Errorf("unsupported format version: %d", fv)
	}
	if _, ok := o["format_uuid"]; ok {
		uuid, err := o.String("format_uuid", what, true)
		if err != nil {
			return nil, err
		}
		if uuid != manifest.FormatUUID {
			return nil, fmt.Errorf("unsupported format UUID: %s", uuid)
		}
	}
	if _, ok := o["packages"]; !ok {
		return nil, fmt.Errorf("%s: missing required property 'packages'", what)
	}
	items, err := o.List("packages", what)
	if err != nil {
		return nil, err
	}
	s := New()
	for i, raw := range items {
		p, err := decodePackage(raw, fmt.Sprintf("packages[%d]", i))
		if err != nil {
			return nil, err
		}
		s.Packages = append(s.Packages, p)
	}
	return s, nil
}

func decodePackage(raw json.RawMessage, what string) (Package, error) {
	var p Package
	o, err := manifest.DecodeObject(raw, what)
	if err != nil {
		return p, err
	}
	if p.Files, err = o.StringList("files", what, true, false); err != nil {
		return p, err
	}
	if p.Locked, err = o.Bool("locked", what); err != nil {
		return p, err
	}
	m, ok := o["manifest"]
	if !ok {
		return p, fmt.Errorf("%s: missing required property 'manifest'", what)
	}
	mf, err := manifest.Parse(m)
	if err != nil {
		return p, fmt.Errorf("%s.manifest: %w", what, err)
	}
	p.Manifest = *mf
	if p.Variant, err = o.String("variant", what, true); err != nil {
		return p, err
	}
	if !manifest.IsValidVariant(p.Variant) {
		return p, fmt.Errorf("invalid package variant: %s", p.Variant)
	}
	return p, nil
}

// Save writes s to <dir>/tooth_lock.json exactly as lip does
// (System.Text.Json indented, see package stjson), creating dir if needed.
// Unlike lip, the file is replaced atomically (temp file + rename) so an
// interrupted write cannot leave a truncated lock file behind.
func (s *State) Save(dir string) error {
	data, err := s.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, FileName+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = replaceFile(tmpName, filepath.Join(dir, FileName))
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// Rename retry budget for replaceFile; rename is replaced in tests.
var (
	rename         = os.Rename
	renameAttempts = 20
	renameBackoff  = 25 * time.Millisecond
)

// replaceFile renames tmp over dst. On Windows the rename fails while a
// reader (e.g. a concurrent List) has dst open, with "Access is denied", so
// that error is retried briefly.
func replaceFile(tmp, dst string) error {
	var err error
	for i := 0; i < renameAttempts; i++ {
		if i > 0 {
			time.Sleep(renameBackoff)
		}
		if err = rename(tmp, dst); err == nil || !errors.Is(err, fs.ErrPermission) {
			return err
		}
	}
	return err
}

// Marshal returns the bytes Save would write.
func (s *State) Marshal() ([]byte, error) {
	return stjson.Marshal(s)
}

// MarshalSTJSON writes s like lip: the format version/uuid are always the
// current constants and a nil package list is written as [].
func (s *State) MarshalSTJSON(w *stjson.Writer) error {
	w.BeginObject()
	w.Key("format_version")
	w.Int(manifest.FormatVersion)
	w.Key("format_uuid")
	w.String(manifest.FormatUUID)
	w.Key("packages")
	if err := w.Value(s.Packages); err != nil {
		return err
	}
	w.EndObject()
	return nil
}

// Find returns the index of the package with the given id, or -1.
func (s *State) Find(id manifest.PackageID) int {
	for i, p := range s.Packages {
		if p.ID() == id {
			return i
		}
	}
	return -1
}
