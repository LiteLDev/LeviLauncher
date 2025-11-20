package mcservice

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/liteldev/LeviLauncher/internal/content"
    "github.com/liteldev/LeviLauncher/internal/types"
    "github.com/liteldev/LeviLauncher/internal/utils"
)

func GetWorldLevelName(worldDir string) string {
    if strings.TrimSpace(worldDir) == "" {
        return ""
    }
    p := filepath.Join(worldDir, "levelname.txt")
    if !utils.FileExists(p) {
        return ""
    }
    b, err := os.ReadFile(p)
    if err != nil {
        return ""
    }
    s := strings.TrimSpace(string(b))
    if idx := strings.IndexByte(s, '\n'); idx >= 0 {
        s = strings.TrimSpace(s[:idx])
    }
    return s
}

func SetWorldLevelName(worldDir string, name string) string {
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return "ERR_INVALID_WORLD_DIR"
    }
    p := filepath.Join(worldDir, "levelname.txt")
    if err := os.WriteFile(p, []byte(strings.TrimSpace(name)), 0644); err != nil {
        return "ERR_WRITE_FILE"
    }
    return ""
}

func GetWorldIconDataUrl(worldDir string) string {
    if strings.TrimSpace(worldDir) == "" {
        return ""
    }
    p := filepath.Join(worldDir, "world_icon.jpeg")
    if !utils.FileExists(p) {
        return ""
    }
    b, err := os.ReadFile(p)
    if err != nil {
        return ""
    }
    enc := base64.StdEncoding.EncodeToString(b)
    return "data:image/jpeg;base64," + enc
}

func BackupWorld(worldDir string) string {
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return ""
    }
    level := GetWorldLevelName(worldDir)
    if level == "" {
        level = utils.GetLastDirName(worldDir)
    }
    safe := utils.SanitizeFilename(level)
    ts := time.Now().Format("20060102-150405")
    base := utils.BaseRoot()
    backupDir := filepath.Join(base, "backups", "worlds", safe)
    if err := utils.CreateDir(backupDir); err != nil {
        return ""
    }
    dest := filepath.Join(backupDir, fmt.Sprintf("%s_%s.mcworld", safe, ts))
    if err := utils.ZipDir(worldDir, dest); err != nil {
        return ""
    }
    return dest
}

func BackupWorldWithVersion(worldDir string, versionName string) string {
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return ""
    }
    level := GetWorldLevelName(worldDir)
    if level == "" {
        level = utils.GetLastDirName(worldDir)
    }
    safeWorld := utils.SanitizeFilename(level)
    folderName := utils.GetLastDirName(worldDir)
    safeFolder := utils.SanitizeFilename(folderName)
    safeVersion := utils.SanitizeFilename(strings.TrimSpace(versionName))
    if safeVersion == "" {
        safeVersion = "default"
    }
    ts := time.Now().Format("20060102-150405")
    base := utils.BaseRoot()
    backupDir := filepath.Join(base, "backups", "worlds", safeVersion, safeFolder+"_"+safeWorld)
    if err := utils.CreateDir(backupDir); err != nil {
        return ""
    }
    dest := filepath.Join(backupDir, fmt.Sprintf("%s_%s.mcworld", safeWorld, ts))
    if err := utils.ZipDir(worldDir, dest); err != nil {
        return ""
    }
    return dest
}

func ReadWorldLevelDatFields(worldDir string) map[string]any {
    res := map[string]any{}
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return res
    }
    fields, ver, err := content.ReadLevelDatFields(worldDir)
    if err != nil {
        return res
    }
    res["version"] = ver
    res["fields"] = fields
    if order, over, e2 := content.ReadLevelDatOrder(worldDir); e2 == nil && over == ver {
        res["order"] = order
    }
    return res
}

func WriteWorldLevelDatFields(worldDir string, args map[string]any) string {
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return "ERR_INVALID_WORLD_DIR"
    }
    var ver int32
    var fields []types.LevelDatField
    if v, ok := args["version"].(float64); ok {
        ver = int32(v)
    } else if v2, ok2 := args["version"].(int32); ok2 {
        ver = v2
    }
    if arr, ok := args["fields"].([]any); ok {
        for _, it := range arr {
            b, _ := json.Marshal(it)
            var f types.LevelDatField
            _ = json.Unmarshal(b, &f)
            fields = append(fields, f)
        }
    }
    if err := content.WriteLevelDatFields(worldDir, fields, ver); err != nil {
        return "ERR_WRITE_FILE"
    }
    if nm, ok := args["levelName"].(string); ok && strings.TrimSpace(nm) != "" {
        _ = os.WriteFile(filepath.Join(worldDir, "levelname.txt"), []byte(strings.TrimSpace(nm)), 0644)
    }
    return ""
}

func ReadWorldLevelDatFieldsAt(worldDir string, path []string) map[string]any {
    res := map[string]any{}
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return res
    }
    fields, ver, err := content.ReadLevelDatFieldsAt(worldDir, path)
    if err != nil {
        return res
    }
    res["version"] = ver
    res["fields"] = fields
    if order, over, e2 := content.ReadLevelDatOrderAt(worldDir, path); e2 == nil && over == ver {
        res["order"] = order
    }
    return res
}

func WriteWorldLevelDatFieldsAt(worldDir string, args map[string]any) string {
    if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
        return "ERR_INVALID_WORLD_DIR"
    }
    var ver int32
    var fields []types.LevelDatField
    var path []string
    if v, ok := args["version"].(float64); ok {
        ver = int32(v)
    } else if v2, ok2 := args["version"].(int32); ok2 {
        ver = v2
    }
    if arr, ok := args["fields"].([]any); ok {
        for _, it := range arr {
            b, _ := json.Marshal(it)
            var f types.LevelDatField
            _ = json.Unmarshal(b, &f)
            fields = append(fields, f)
        }
    }
    if p, okp := args["path"].([]any); okp {
        for _, s := range p {
            path = append(path, fmt.Sprintf("%v", s))
        }
    }
    if err := content.WriteLevelDatFieldsAt(worldDir, path, fields, ver); err != nil {
        return "ERR_WRITE_FILE"
    }
    return ""
}

func ImportMcworld(name string, player string, fileName string, data []byte, overwrite bool) string {
    roots := GetContentRoots(name)
    users := strings.TrimSpace(roots.UsersRoot)
    if users == "" || strings.TrimSpace(player) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    wp := filepath.Join(users, player, "games", "com.mojang", "minecraftWorlds")
    return content.ImportMcworldToDir(data, fileName, wp, overwrite)
}