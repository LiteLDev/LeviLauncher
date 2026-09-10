package mcservice

import (
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/explorer"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func OpenWorldsExplorer(name string, isPreview bool) error {
	roots := GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users != "" {
		ents := ListDir(users)
		var firstPlayer string
		for _, e := range ents {
			if e.IsDir {
				nm := strings.TrimSpace(e.Name)
				if nm != "" && !strings.EqualFold(nm, "Shared") {
					firstPlayer = nm
					break
				}
			}
		}
		if firstPlayer != "" {
			wp := filepath.Join(users, firstPlayer, "games", "com.mojang", "minecraftWorlds")
			if utils.DirExists(wp) {
				return explorer.OpenPath(wp)
			}
		}
		if utils.DirExists(users) {
			return explorer.OpenPath(users)
		}
	}
	legacy := filepath.Join(utils.GetMinecraftGDKDataPath(isPreview), "worlds")
	return explorer.OpenPath(legacy)
}

func OpenGameDataExplorer(isPreview bool) error {
	base := utils.GetMinecraftGDKDataPath(isPreview)
	return explorer.OpenPath(base)
}
