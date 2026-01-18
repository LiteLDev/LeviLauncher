package versions

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

var _ = reflect.TypeOf(VersionMeta{})
var _ = reflect.TypeOf(metaFileName)

type VersionMeta struct {
	Name               string    `json:"name"        `
	GameVersion        string    `json:"gameVersion"`
	Type               string    `json:"type"       `
	EnableIsolation    bool      `json:"enableIsolation"`
	EnableConsole      bool      `json:"enableConsole"`
	EnableEditorMode   bool      `json:"enableEditorMode"`
	EnableRenderDragon bool      `json:"enableRenderDragon"`
	CreatedAt          time.Time `json:"createdAt"`
	Registered         bool      `json:"registered,omitempty"`
}

const metaFileName = "version.json"

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
	return m, err
}

func ScanVersions(versionsRoot string) ([]VersionMeta, error) {
	entries, err := os.ReadDir(versionsRoot)
	if err != nil {
		return nil, err
	}
	var out []VersionMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(versionsRoot, e.Name())
		m, err := ReadMeta(dir)
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func ComputeVCRuntimeHash(versionDir string) (string, error) {
	path := filepath.Join(versionDir, "vcruntime140_1.dll")
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
