package mcservice

import (
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/explorer"
    "github.com/liteldev/LeviLauncher/internal/utils"
)

func OpenModsExplorer(name string) { _ = explorer.OpenMods(name) }

func OpenWorldsExplorer(name string, isPreview bool) {
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
                _ = explorer.OpenPath(wp)
                return
            }
        }
        if utils.DirExists(users) {
            _ = explorer.OpenPath(users)
            return
        }
    }
    legacy := filepath.Join(utils.GetMinecraftGDKDataPath(isPreview), "worlds")
    _ = explorer.OpenPath(legacy)
}

func OpenPathDir(dir string) {
    d := strings.TrimSpace(dir)
    if d == "" {
        return
    }
    _ = explorer.OpenPath(d)
}

func OpenGameDataExplorer(isPreview bool) {
    base := utils.GetMinecraftGDKDataPath(isPreview)
    _ = explorer.OpenPath(base)
}