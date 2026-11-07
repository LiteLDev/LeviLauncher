package versions

import (
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

func WriteMeta(versionDir string, meta VersionMeta) error {
	if _, err := EnsureStateDir(versionDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(metaPath(versionDir), append(data, '\n'), 0o644); err != nil {
		return err
	}
	_ = os.Remove(legacyMetaPath(versionDir))
	return nil
}

// readMetaBytes reads instance.json, moving a legacy version.json there
// first. The legacy file is removed only once its content is in place; when
// that fails, it is read where it is.
func readMetaBytes(versionDir string) ([]byte, error) {
	data, err := os.ReadFile(metaPath(versionDir))
	if err == nil || !os.IsNotExist(err) {
		return data, err
	}
	legacy := legacyMetaPath(versionDir)
	data, err = os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	if _, serr := EnsureStateDir(versionDir); serr == nil {
		if werr := fsx.WriteFileAtomic(metaPath(versionDir), data, 0o644); werr == nil {
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
	err = json.Unmarshal(data, &m)
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
		out = append(out, m)
	}
	return out, nil
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
