//go:build darwin

package gameruntime

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/types"
)

// Scan searches common installation locations without running discovered
// binaries, reading game files, or modifying any Wine prefix.
func Scan(ctx context.Context) ([]types.WineInstallation, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	binDirs := append(filepath.SplitList(os.Getenv("PATH")), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", filepath.Join(home, ".local/bin"))
	appDirs := []string{"/Applications", filepath.Join(home, "Applications")}
	current, err := LoadSettings()
	if err != nil {
		return nil, err
	}
	return scanLocations(ctx, appDirs, binDirs, home, current)
}

func scanLocations(ctx context.Context, appDirs, binDirs []string, home string, current types.WineSettings) ([]types.WineInstallation, error) {
	result := []types.WineInstallation{}
	seen := map[string]bool{}
	prefix := current.Prefix
	if info, err := os.Stat(filepath.Join(prefix, "drive_c")); err != nil || !info.IsDir() {
		prefix = filepath.Join(home, ".wine")
	}
	add := func(name string, s types.WineSettings, supported bool) {
		info, err := os.Stat(s.Executable)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			return
		}
		real, err := filepath.EvalSymlinks(s.Executable)
		if err != nil {
			return
		}
		key := real + "\x00" + s.App + "\x00" + s.Prefix
		if seen[key] {
			return
		}
		seen[key] = true
		if s.Graphics == "" {
			s.Graphics = "auto"
		}
		if s.Sync == "" {
			s.Sync = "auto"
		}
		status := "ready"
		if !supported {
			status = "unsupported"
		} else if info, err := os.Stat(filepath.Join(s.Prefix, "drive_c")); err != nil || !info.IsDir() {
			status = "needs_prefix"
		}
		result = append(result, types.WineInstallation{Name: name, Settings: s, Status: status})
	}
	if current.Executable != "" {
		name := "Wine"
		if current.App != "" {
			name = strings.TrimSuffix(filepath.Base(current.App), ".app")
		}
		if (Profile{Wine: current.Executable}).crossOverRoot() != "" {
			if _, err := os.Stat(filepath.Join(current.Prefix, "cxbottle.conf")); err == nil {
				add("CrossOver · "+filepath.Base(current.Prefix), current, true)
			}
		} else {
			add(name, current, true)
		}
	}
	for _, root := range appDirs {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if walkErr != nil {
				return nil
			} // An unreadable app does not abort other roots.
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				for _, layout := range []struct {
					path               string
					bundled, supported bool
				}{
					{"Contents/SharedSupport/wine/bin/wine", true, true},
					{"Contents/Resources/wine/bin/wine", false, true},
					{"Contents/Frameworks/wswine.bundle/bin/wine", false, false},
					{"Contents/SharedSupport/CrossOver/bin/wine", false, true},
				} {
					s := types.WineSettings{Executable: filepath.Join(path, layout.path), Prefix: prefix}
					if (Profile{Wine: s.Executable}).crossOverRoot() != "" {
						for _, bottle := range crossOverBottles(home) {
							s.Prefix = bottle
							add(name+" · "+filepath.Base(bottle), s, true)
						}
						continue
					}
					if layout.bundled {
						s.App = path
						s.Prefix = filepath.Join(path, "Contents/SharedSupport/prefix")
					}
					add(name, s, layout.supported)
				}
				if entry.IsDir() {
					return filepath.SkipDir
				}
			}
			// Do not traverse arbitrary app data or entire user directories.
			if entry.IsDir() && relative != "." && (strings.HasPrefix(entry.Name(), ".") || len(strings.Split(relative, string(filepath.Separator))) >= 4) {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, dir := range binDirs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range []string{"wine", "wine64"} {
			path := filepath.Join(dir, name)
			if (Profile{Wine: path}).crossOverRoot() != "" {
				for _, bottle := range crossOverBottles(home) {
					add("CrossOver · "+filepath.Base(bottle), types.WineSettings{Executable: path, Prefix: bottle}, true)
				}
			} else {
				add("Wine · "+dir, types.WineSettings{Executable: path, Prefix: prefix}, true)
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Status != result[j].Status {
			return result[i].Status == "ready" || result[j].Status == "unsupported"
		}
		return result[i].Settings.Executable < result[j].Settings.Executable
	})
	return result, nil
}

func crossOverBottles(home string) []string {
	root := filepath.Join(home, "Library/Application Support/CrossOver/Bottles")
	entries, _ := os.ReadDir(root)
	var bottles []string
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(path, "cxbottle.conf")); err == nil {
			bottles = append(bottles, path)
		}
	}
	if len(bottles) == 0 {
		bottles = append(bottles, filepath.Join(root, "default"))
	}
	return bottles
}
