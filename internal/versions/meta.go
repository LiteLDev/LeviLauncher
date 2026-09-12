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

	"github.com/liteldev/LeviLauncher/internal/utils"
)

var _ = reflect.TypeOf(VersionMeta{})
var _ = reflect.TypeOf(metaFileName)

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

const metaFileName = "version.json"

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

func metaPath(versionDir string) string { return filepath.Join(versionDir, metaFileName) }

func WriteMeta(versionDir string, meta VersionMeta) error {
	if !utils.DirExists(versionDir) {
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			return err
		}
	}
	f, err := os.Create(metaPath(versionDir))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(meta)
}

func ReadMeta(versionDir string) (VersionMeta, error) {
	var m VersionMeta
	f, err := os.Open(metaPath(versionDir))
	if err != nil {
		return m, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	err = dec.Decode(&m)
	if err == nil {
		m.PackageType = DetectPackageType(versionDir, m)
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
