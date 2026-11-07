package versions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var _ = reflect.TypeOf(VersionMeta{})

type VersionMeta struct {
	Name             string    `json:"name"        `
	GameVersion      string    `json:"gameVersion"`
	Type             string    `json:"type"       `
	PackageType      string    `json:"packageType"`
	EnableIsolation  bool      `json:"enableIsolation"`
	EnableConsole    bool      `json:"enableConsole"`
	EnableEditorMode bool      `json:"enableEditorMode"`
	LaunchArgs       string    `json:"launchArgs"`
	EnvVars          string    `json:"envVars"`
	CreatedAt        time.Time `json:"createdAt"`
	Registered       bool      `json:"registered,omitempty"`
}

const (
	PackageTypeGDK = "gdk"
	PackageTypeUWP = "uwp"
)

// NormalizePackageType keeps metadata written before UWP support compatible.
func NormalizePackageType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), PackageTypeUWP) {
		return PackageTypeUWP
	}
	return PackageTypeGDK
}

// DetectPackageType also recognizes imported loose UWP folders. GDK registration
// generates an AppxManifest too, so MicrosoftGame.config takes precedence.
func DetectPackageType(versionDir string, meta VersionMeta) string {
	if strings.TrimSpace(meta.PackageType) != "" {
		return NormalizePackageType(meta.PackageType)
	}
	if _, err := os.Stat(filepath.Join(versionDir, "MicrosoftGame.config")); err == nil {
		return PackageTypeGDK
	}
	if _, err := os.Stat(filepath.Join(versionDir, "AppxManifest.xml")); err == nil {
		return PackageTypeUWP
	}
	return PackageTypeGDK
}

// metaPath is <instance>\.levilauncher\instance.json; legacyMetaPath is the
// version.json of instances in the legacy layout.
func metaPath(versionDir string) string {
	return filepath.Join(apppath.InstanceStateDir(versionDir), apppath.InstanceMetaFileName)
}

func legacyMetaPath(versionDir string) string {
	return filepath.Join(versionDir, apppath.LegacyInstanceMetaFileName)
}

// EnsureStateDir creates <instance>\.levilauncher with the hidden attribute,
// as lip does for .lip.
func EnsureStateDir(versionDir string) (string, error) {
	dir := apppath.InstanceStateDir(versionDir)
	if utils.DirExists(dir) {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	fsx.SetHidden(dir)
	return dir, nil
}

// WriteMeta writes the instance metadata to .levilauncher\instance.json and
// removes a legacy version.json. On the legacy layout (apppath.LegacyMode),
// which an older launcher may use again, it writes version.json, and keeps
// an instance.json that exists in step with it.
func WriteMeta(versionDir string, meta VersionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if apppath.LegacyMode() != "" {
		if err := fsx.WriteFileAtomic(legacyMetaPath(versionDir), data, 0o644); err != nil {
			return err
		}
		if utils.FileExists(metaPath(versionDir)) {
			return fsx.WriteFileAtomic(metaPath(versionDir), data, 0o644)
		}
		return nil
	}
	if _, err := EnsureStateDir(versionDir); err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(metaPath(versionDir), data, 0o644); err != nil {
		return err
	}
	_ = os.Remove(legacyMetaPath(versionDir))
	return nil
}

// readMetaBytes reads the instance metadata. When only a legacy version.json
// exists, or it is newer than instance.json (an older launcher changed the
// instance since), its content is moved to instance.json first; the legacy
// file is removed only once its content is in place, and when that fails it
// is read where it is. On the legacy layout nothing is moved: an older
// launcher reads only version.json.
func readMetaBytes(versionDir string) ([]byte, error) {
	current, legacy := metaPath(versionDir), legacyMetaPath(versionDir)
	ci, cerr := os.Stat(current)
	li, lerr := os.Stat(legacy)
	if lerr != nil {
		if cerr != nil && !os.IsNotExist(cerr) {
			return nil, cerr
		}
		return os.ReadFile(current)
	}
	if cerr == nil && !li.ModTime().After(ci.ModTime()) {
		return os.ReadFile(current)
	}
	data, err := os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	if apppath.LegacyMode() != "" {
		return data, nil
	}
	if _, serr := EnsureStateDir(versionDir); serr == nil {
		if werr := fsx.WriteFileAtomic(current, data, 0o644); werr == nil {
			_ = os.Remove(legacy)
		}
	}
	return data, nil
}

// HasMeta reports whether versionDir holds instance metadata, current or
// legacy.
func HasMeta(versionDir string) bool {
	return utils.FileExists(metaPath(versionDir)) || utils.FileExists(legacyMetaPath(versionDir))
}

func ReadMeta(versionDir string) (VersionMeta, error) {
	var m VersionMeta
	data, err := readMetaBytes(versionDir)
	if err != nil {
		return m, err
	}
	// Metadata edited by hand may start with a UTF-8 byte order mark.
	err = json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &m)
	if err == nil {
		m.PackageType = DetectPackageType(versionDir, m)
		// Expose the effective setting without rewriting imported or older metadata.
		m.EnableIsolation = m.EnableIsolation && SupportsIsolation(m.PackageType, m.GameVersion)
		m.EnableEditorMode = m.EnableEditorMode && SupportsEditorMode(m.GameVersion, m.Type)
	}
	return m, err
}

func ScanVersions(versionsRoot string) ([]VersionMeta, error) {
	entries, err := os.ReadDir(versionsRoot)
	if err != nil {
		return nil, err
	}
	var out []VersionMeta
	for _, e := range entries {
		dir := filepath.Join(versionsRoot, e.Name())
		// A version folder may be a junction into an external game directory,
		// which os.ReadDir reports as not-a-dir.
		if !e.IsDir() && !utils.ResolvesToDir(dir) {
			continue
		}
		m, err := ReadMeta(dir)
		if err != nil {
			continue
		}
		// The folder names the instance: every lookup joins the instances
		// directory with the name. A folder renamed outside the launcher, or
		// placed under a "-legacy" name by the storage migration, keeps the
		// name of the folder it came from in its metadata.
		if base := e.Name(); !strings.EqualFold(strings.TrimSpace(m.Name), base) {
			m.Name = base
			persistName(dir, base)
		}
		out = append(out, m)
	}
	return out, nil
}

// persistName writes the folder name into the stored metadata, changing
// nothing else of it.
func persistName(versionDir, name string) {
	data, err := readMetaBytes(versionDir)
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	enc, err := json.Marshal(name)
	if err != nil {
		return
	}
	raw["name"] = enc
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}
	path := metaPath(versionDir)
	if !utils.FileExists(path) {
		path = legacyMetaPath(versionDir)
	}
	_ = fsx.WriteFileAtomic(path, append(out, '\n'), 0o644)
}

func ComputeLoaderHash(versionDir string) (string, error) {
	path := filepath.Join(versionDir, "LeviLauncher.dll")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
