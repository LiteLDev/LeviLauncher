package packages

import (
	"os"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type ManifestJSON struct {
	FormatVersion int `json:"format_version"`
	Header        struct {
		Name             string `json:"name"`
		Description      string `json:"description"`
		UUID             string `json:"uuid"`
		Version          []int  `json:"version"`
		MinEngineVersion []int  `json:"min_engine_version"`
	} `json:"header"`
	Modules []struct {
		Type    string `json:"type"`
		UUID    string `json:"uuid"`
		Version []int  `json:"version"`
	} `json:"modules"`
}

func (pm *PackManager) LoadPacksForVersion(versionName string, resourcePacksDir, behaviorPacksDir string, skinPacksDirs ...string) ([]Pack, error) {
	var packs []Pack

	scanDir := func(dir string, defaultType PackType) {
		if !utils.DirExists(dir) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			startPath := filepath.Join(dir, e.Name())

			var findPackRoot func(current string, depth int) string
			findPackRoot = func(current string, depth int) string {
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

			packPath := findPackRoot(startPath, 0)
			if packPath == "" {
				continue
			}

			manifestPath := filepath.Join(packPath, "manifest.json")

			manifest, err := parseManifest(manifestPath, defaultType)
			if err == nil {
				packs = append(packs, Pack{
					Manifest: manifest,
					Path:     packPath,
				})
			}
		}
	}

	scanDir(resourcePacksDir, PackTypeResources)
	scanDir(behaviorPacksDir, PackTypeBehavior)
	for _, dir := range skinPacksDirs {
		scanDir(dir, PackTypeSkins)
	}

	pm.mu.Lock()
	pm.packs[versionName] = packs
	pm.mu.Unlock()
	return packs, nil
}

func parseManifest(path string, defaultType PackType) (PackManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PackManifest{}, err
	}

	b = utils.JsonCompatBytes(b)

	var raw ManifestJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return PackManifest{}, err
	}

	pm := PackManifest{}

	pm.Identity.UUID = raw.Header.UUID
	if len(raw.Header.Version) >= 3 {
		pm.Identity.Version = SemVersion{
			Major: raw.Header.Version[0],
			Minor: raw.Header.Version[1],
			Patch: raw.Header.Version[2],
		}
	}

	pm.PackType = defaultType
	pm.Identity.PackType = defaultType

	for _, mod := range raw.Modules {
		switch strings.ToLower(mod.Type) {
		case "resources":
			pm.PackType = PackTypeResources
		case "data":
			pm.PackType = PackTypeBehavior
		case "skin_pack":
			pm.PackType = PackTypeSkins
		}
	}
	pm.Identity.PackType = pm.PackType

	if len(raw.Header.MinEngineVersion) >= 3 {
		pm.MinEngineVersion = MinEngineVersion{
			SemVersion: SemVersion{
				Major: raw.Header.MinEngineVersion[0],
				Minor: raw.Header.MinEngineVersion[1],
				Patch: raw.Header.MinEngineVersion[2],
			},
		}
	}

	pm.Name = raw.Header.Name
	pm.Description = raw.Header.Description
	pm.Location = filepath.Dir(path)

	texts := readPackTexts(pm.Location)
	if texts != nil {
		if val, ok := texts[pm.Name]; ok {
			pm.Name = val
		}
		if val, ok := texts[pm.Description]; ok {
			pm.Description = val
		}
	}

	iconPath := filepath.Join(filepath.Dir(path), "pack_icon.png")
	if utils.FileExists(iconPath) {
		pm.PackIconLocation = iconPath
	}

	return pm, nil
}

func readPackTexts(root string) map[string]string {
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
