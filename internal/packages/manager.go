package packages

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type RawManifest struct {
	FormatVersion int `json:"format_version"`
	Header        struct {
		Name             string      `json:"name"`
		Description      string      `json:"description"`
		UUID             string      `json:"uuid"`
		Version          interface{} `json:"version"`            // []int or string
		MinEngineVersion interface{} `json:"min_engine_version"` // []int or string
	} `json:"header"`
	Modules []struct {
		Type    string      `json:"type"`
		UUID    string      `json:"uuid"`
		Version interface{} `json:"version"` // []int or string
	} `json:"modules"`
}

func (PackManager) LoadPacksForVersion(_ string, resourcePacksDir, behaviorPacksDir string, skinPacksDirs ...string) ([]Pack, error) {
	var packs []Pack
	scanPackManifests(resourcePacksDir, behaviorPacksDir, skinPacksDirs, func(dir string, kind PackType, raw *RawManifest) {
		packs = append(packs, Pack{Path: dir, Manifest: raw.packManifest(dir, kind)})
	})
	return packs, nil
}

func (PackManager) LoadPackDetails(kind PackType, resourcePacksDir, behaviorPacksDir string, skinPacksDirs ...string) []types.PackListEntry {
	entries := []types.PackListEntry{}
	scanPackManifests(resourcePacksDir, behaviorPacksDir, skinPacksDirs, func(dir string, found PackType, raw *RawManifest) {
		if found != kind {
			return
		}
		entry := types.PackListEntry{PackInfo: raw.ReadPackInfo(dir)}
		if stat, err := os.Stat(dir); err == nil {
			entry.ModTime = stat.ModTime().Unix()
		}
		entries = append(entries, entry)
	})
	return entries
}

func scanPackManifests(resourcePacksDir, behaviorPacksDir string, skinPacksDirs []string, visit func(string, PackType, *RawManifest)) {
	scanDir := func(dir string, defaultType PackType) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			startPath := filepath.Join(dir, e.Name())

			packPath := findPackRoot(startPath, 0)
			if packPath == "" {
				continue
			}

			b, err := os.ReadFile(filepath.Join(packPath, "manifest.json"))
			if err != nil {
				continue
			}
			var raw RawManifest
			if json.Unmarshal(utils.JsonCompatBytes(b), &raw) == nil {
				visit(packPath, raw.packType(defaultType), &raw)
			}
		}
	}

	scanDir(resourcePacksDir, PackTypeResources)
	scanDir(behaviorPacksDir, PackTypeBehavior)
	for _, dir := range skinPacksDirs {
		scanDir(dir, PackTypeSkins)
	}
}

func findPackRoot(current string, depth int) string {
	if depth > 3 {
		return ""
	}

	mPath := filepath.Join(current, "manifest.json")
	if utils.FileExists(mPath) {
		return current
	}

	subs, err := os.ReadDir(current)
	if err != nil {
		return ""
	}
	for _, sub := range subs {
		if sub.IsDir() {
			found := findPackRoot(filepath.Join(current, sub.Name()), depth+1)
			if found != "" {
				return found
			}
		}
	}
	return ""
}

func (raw *RawManifest) packType(kind PackType) PackType {
	for _, mod := range raw.Modules {
		switch strings.ToLower(mod.Type) {
		case "resources":
			kind = PackTypeResources
		case "data":
			kind = PackTypeBehavior
		case "skin_pack":
			kind = PackTypeSkins
		}
	}
	return kind
}

func (raw *RawManifest) packManifest(dir string, kind PackType) PackManifest {
	pm := PackManifest{}

	pm.Identity.UUID = raw.Header.UUID

	version := ParseVersion(raw.Header.Version)
	if len(version) >= 3 {
		pm.Identity.Version = SemVersion{
			Major: version[0],
			Minor: version[1],
			Patch: version[2],
		}
	}

	pm.PackType = kind
	pm.Identity.PackType = kind

	minEngineVersion := ParseVersion(raw.Header.MinEngineVersion)
	if len(minEngineVersion) >= 3 {
		pm.MinEngineVersion = MinEngineVersion{
			SemVersion: SemVersion{
				Major: minEngineVersion[0],
				Minor: minEngineVersion[1],
				Patch: minEngineVersion[2],
			},
		}
	}

	pm.Name = raw.Header.Name
	pm.Description = raw.Header.Description
	pm.Location = dir

	texts := ReadPackTexts(pm.Location)
	if texts != nil {
		if val, ok := texts[pm.Name]; ok {
			pm.Name = val
		}
		if val, ok := texts[pm.Description]; ok {
			pm.Description = val
		}
	}

	iconPath := filepath.Join(dir, "pack_icon.png")
	if utils.FileExists(iconPath) {
		pm.PackIconLocation = iconPath
	}

	return pm
}

func formatPackVersion(value any) string {
	version := ParseVersion(value)
	parts := make([]string, len(version))
	for i, n := range version {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}

// ReadPackInfo enriches the parsed manifest with its display texts and icon.
func (mf *RawManifest) ReadPackInfo(dir string) types.PackInfo {
	info := types.PackInfo{
		Path:             dir,
		Name:             strings.TrimSpace(mf.Header.Name),
		Description:      strings.TrimSpace(mf.Header.Description),
		Version:          formatPackVersion(mf.Header.Version),
		MinEngineVersion: formatPackVersion(mf.Header.MinEngineVersion),
	}
	if info.Name == "pack.name" || info.Description == "pack.description" {
		texts := ReadPackTexts(dir)
		if v, ok := texts[info.Name]; info.Name == "pack.name" && ok {
			info.Name = strings.TrimSpace(v)
		}
		if v, ok := texts[info.Description]; info.Description == "pack.description" && ok {
			info.Description = strings.TrimSpace(v)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "pack_icon.png")); err == nil {
		info.IconDataUrl = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
	}
	return info
}

func ReadPackTexts(root string) map[string]string {
	textsDir := filepath.Join(root, "texts")
	if !utils.DirExists(textsDir) {
		return nil
	}
	langFile := filepath.Join(textsDir, "en_US.lang")
	if !utils.FileExists(langFile) {
		lj := filepath.Join(textsDir, "languages.json")
		if utils.FileExists(lj) {
			if b, err := os.ReadFile(lj); err == nil {
				var langs []string
				_ = json.Unmarshal(utils.JsonCompatBytes(b), &langs)
				if len(langs) > 0 {
					cand := filepath.Join(textsDir, strings.TrimSpace(langs[0])+".lang")
					if utils.FileExists(cand) {
						langFile = cand
					}
				}
			}
		}
	}
	if !utils.FileExists(langFile) {
		ents, err := os.ReadDir(textsDir)
		if err == nil {
			for _, e := range ents {
				if e.IsDir() {
					continue
				}
				name := strings.ToLower(strings.TrimSpace(e.Name()))
				if strings.HasSuffix(name, ".lang") {
					langFile = filepath.Join(textsDir, e.Name())
					break
				}
			}
		}
	}
	if !utils.FileExists(langFile) {
		return nil
	}
	b, err := os.ReadFile(langFile)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	m := make(map[string]string, 16)
	for _, ln := range lines {
		l := strings.TrimSpace(strings.TrimRight(ln, "\r"))
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if idx := strings.IndexRune(l, '#'); idx >= 0 {
			l = strings.TrimSpace(l[:idx])
		}
		if l == "" {
			continue
		}
		if eq := strings.IndexRune(l, '='); eq >= 0 {
			k := strings.TrimSpace(l[:eq])
			v := strings.TrimSpace(l[eq+1:])
			if k != "" {
				m[k] = v
			}
		}
	}
	return m
}

func ParseVersion(v interface{}) []int {
	switch val := v.(type) {
	case []interface{}:
		res := make([]int, len(val))
		for i, x := range val {
			if f, ok := x.(float64); ok {
				res[i] = int(f)
			}
		}
		return res
	case []int:
		return val
	case string:
		parts := strings.Split(val, ".")
		res := make([]int, 0, len(parts))
		for _, p := range parts {
			if i, err := strconv.Atoi(p); err == nil {
				res = append(res, i)
			}
		}
		return res
	}
	return nil
}
