package mcservice

import (
    "bytes"
    "encoding/base64"
    "image"
    _ "image/gif"
    _ "image/jpeg"
    "image/png"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
    "unsafe"

    "github.com/liteldev/LeviLauncher/internal/icons"
    "github.com/liteldev/LeviLauncher/internal/types"
    "github.com/liteldev/LeviLauncher/internal/utils"
    "github.com/liteldev/LeviLauncher/internal/versions"
    "golang.org/x/sys/windows"
)

type ContentCounts struct {
    Worlds        int `json:"worlds"`
    ResourcePacks int `json:"resourcePacks"`
    BehaviorPacks int `json:"behaviorPacks"`
}

func GetContentRoots(name string) types.ContentRoots {
    roots := types.ContentRoots{Base: "", UsersRoot: "", ResourcePacks: "", BehaviorPacks: "", IsIsolation: false, IsPreview: false}
    verName := strings.TrimSpace(name)
    isPreview := false
    isIsolation := false
    if verName != "" {
        if vdir, err := utils.GetVersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
            dir := filepath.Join(vdir, verName)
            if m, merr := versions.ReadMeta(dir); merr == nil {
                isIsolation = m.EnableIsolation
                isPreview = strings.EqualFold(strings.TrimSpace(m.Type), "preview")
            }
        }
    }
    roots.IsIsolation = isIsolation
    roots.IsPreview = isPreview
    gameDirName := "Minecraft Bedrock"
    if isPreview {
        gameDirName = "Minecraft Bedrock Preview"
    }
    base := ""
    if isIsolation && verName != "" {
        if vdir, err := utils.GetVersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
            base = filepath.Join(vdir, verName, gameDirName)
        }
    } else {
        base = utils.GetMinecraftGDKDataPath(isPreview)
        base = strings.TrimSpace(base)
    }
    roots.Base = base
    if strings.TrimSpace(base) == "" {
        return roots
    }
    users := filepath.Join(base, "Users")
    shared := filepath.Join(users, "Shared", "games", "com.mojang")
    roots.UsersRoot = users
    roots.ResourcePacks = filepath.Join(shared, "resource_packs")
    roots.BehaviorPacks = filepath.Join(shared, "behavior_packs")
    return roots
}

func GetContentCounts(name string) ContentCounts {
    roots := GetContentRoots(name)
    countDirs := func(path string) int {
        p := strings.TrimSpace(path)
        if p == "" {
            return 0
        }
        ents, err := os.ReadDir(p)
        if err != nil {
            return 0
        }
        n := 0
        for _, e := range ents {
            if e.IsDir() {
                n++
            }
        }
        return n
    }
    res := countDirs(roots.ResourcePacks)
    bp := countDirs(roots.BehaviorPacks)
    worlds := 0
    usersRoot := strings.TrimSpace(roots.UsersRoot)
    if usersRoot != "" {
        ents, _ := os.ReadDir(usersRoot)
        var firstPlayer string
        for _, e := range ents {
            if e.IsDir() {
                nm := strings.TrimSpace(e.Name())
                if nm != "" && !strings.EqualFold(nm, "Shared") {
                    firstPlayer = nm
                    break
                }
            }
        }
        if firstPlayer != "" {
            wp := filepath.Join(usersRoot, firstPlayer, "games", "com.mojang", "minecraftWorlds")
            worlds = countDirs(wp)
        }
    }
    return ContentCounts{Worlds: worlds, ResourcePacks: res, BehaviorPacks: bp}
}

func SaveVersionMeta(name string, gameVersion string, typeStr string, enableIsolation bool, enableConsole bool, enableEditorMode bool) string {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    dir := filepath.Join(vdir, strings.TrimSpace(name))
    meta := versions.VersionMeta{
        Name:             strings.TrimSpace(name),
        GameVersion:      strings.TrimSpace(gameVersion),
        Type:             strings.TrimSpace(typeStr),
        EnableIsolation:  enableIsolation,
        EnableConsole:    enableConsole,
        EnableEditorMode: enableEditorMode,
        CreatedAt:        time.Now(),
    }
    if err := versions.WriteMeta(dir, meta); err != nil {
        return "ERR_WRITE_TARGET"
    }
    return ""
}

func ListVersionMetas() []versions.VersionMeta {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return []versions.VersionMeta{}
    }
    metas, err := versions.ScanVersions(vdir)
    if err != nil {
        return []versions.VersionMeta{}
    }
    return metas
}

func ListInheritableVersionNames(versionType string) []string {
    vt := strings.ToLower(strings.TrimSpace(versionType))
    if vt != "release" && vt != "preview" {
        vt = "release"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return []string{}
    }
    metas, err := versions.ScanVersions(vdir)
    if err != nil || len(metas) == 0 {
        return []string{}
    }
    names := make([]string, 0, len(metas))
    for _, m := range metas {
        if !m.EnableIsolation {
            continue
        }
        mt := strings.ToLower(strings.TrimSpace(m.Type))
        if mt != vt {
            continue
        }
        gameDirName := "Minecraft Bedrock"
        if mt == "preview" {
            gameDirName = "Minecraft Bedrock Preview"
        }
        base := filepath.Join(vdir, m.Name, gameDirName)
        if !utils.DirExists(base) {
            continue
        }
        if utils.DirSize(base) <= 10*1024 {
            continue
        }
        names = append(names, m.Name)
    }
    return names
}

func CopyVersionDataFromVersion(sourceName string, targetName string) string {
    s := strings.TrimSpace(sourceName)
    t := strings.TrimSpace(targetName)
    if s == "" || t == "" {
        return "ERR_TARGET_DIR_NOT_SPECIFIED"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    sdir := filepath.Join(vdir, s)
    tdir := filepath.Join(vdir, t)
    sm, serr := versions.ReadMeta(sdir)
    if serr != nil {
        return "ERR_INHERIT_SOURCE_NOT_FOUND"
    }
    tm, terr := versions.ReadMeta(tdir)
    if terr != nil {
        return "ERR_INHERIT_TARGET_NOT_FOUND"
    }
    st := strings.ToLower(strings.TrimSpace(sm.Type))
    tt := strings.ToLower(strings.TrimSpace(tm.Type))
    if st == "" || tt == "" || st != tt {
        return "ERR_INHERIT_TYPE_MISMATCH"
    }
    gameDirName := "Minecraft Bedrock"
    if st == "preview" {
        gameDirName = "Minecraft Bedrock Preview"
    }
    srcBase := filepath.Join(sdir, gameDirName)
    dstBase := filepath.Join(tdir, gameDirName)
    if !utils.DirExists(srcBase) {
        return "ERR_INHERIT_SOURCE_NOT_FOUND"
    }
    if err := os.MkdirAll(dstBase, 0755); err != nil {
        return "ERR_CREATE_TARGET_DIR"
    }
    if err := utils.CopyDir(srcBase, dstBase); err != nil {
        return "ERR_INHERIT_COPY_FAILED"
    }
    return ""
}

func CopyVersionDataFromGDK(isPreview bool, targetName string) string {
    t := strings.TrimSpace(targetName)
    if t == "" {
        return "ERR_TARGET_DIR_NOT_SPECIFIED"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    tdir := filepath.Join(vdir, t)
    tm, terr := versions.ReadMeta(tdir)
    if terr != nil {
        return "ERR_INHERIT_TARGET_NOT_FOUND"
    }
    tt := strings.ToLower(strings.TrimSpace(tm.Type))
    if (isPreview && tt != "preview") || (!isPreview && tt != "release") {
        return "ERR_INHERIT_TYPE_MISMATCH"
    }
    gameDirName := "Minecraft Bedrock"
    if isPreview {
        gameDirName = "Minecraft Bedrock Preview"
    }
    srcBase := utils.GetMinecraftGDKDataPath(isPreview)
    if strings.TrimSpace(srcBase) == "" || !utils.DirExists(srcBase) {
        return "ERR_NO_GAME_DATA"
    }
    dstBase := filepath.Join(tdir, gameDirName)
    if err := os.MkdirAll(dstBase, 0755); err != nil {
        return "ERR_CREATE_TARGET_DIR"
    }
    if err := utils.CopyDir(srcBase, dstBase); err != nil {
        return "ERR_INHERIT_COPY_FAILED"
    }
    return ""
}

func SaveVersionLogoDataUrl(name string, dataUrl string) string {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    dir := filepath.Join(vdir, strings.TrimSpace(name))
    if !utils.DirExists(dir) {
        return "ERR_VERSION_NOT_FOUND"
    }
    du := strings.TrimSpace(dataUrl)
    lower := strings.ToLower(du)
    if !strings.HasPrefix(lower, "data:image/") || !strings.Contains(lower, ";base64,") {
        return "ERR_ICON_FORMAT"
    }
    parts := strings.SplitN(du, ",", 2)
    if len(parts) != 2 {
        return "ERR_ICON_FORMAT"
    }
    b64 := parts[1]
    raw, er := base64.StdEncoding.DecodeString(b64)
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    cfg, _, er := image.DecodeConfig(bytes.NewReader(raw))
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width != cfg.Height {
        return "ERR_ICON_NOT_SQUARE"
    }
    img, _, er := image.Decode(bytes.NewReader(raw))
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    outPath := filepath.Join(dir, "LargeLogo.png")
    f, ferr := os.Create(outPath)
    if ferr != nil {
        return "ERR_WRITE_TARGET"
    }
    defer f.Close()
    if err := png.Encode(f, img); err != nil {
        return "ERR_WRITE_TARGET"
    }
    return ""
}

func SaveVersionLogoFromPath(name string, filePath string) string {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    dir := filepath.Join(vdir, strings.TrimSpace(name))
    if !utils.DirExists(dir) {
        return "ERR_VERSION_NOT_FOUND"
    }
    p := strings.TrimSpace(filePath)
    if p == "" || !utils.FileExists(p) {
        return "ERR_ICON_DECODE"
    }
    raw, er := os.ReadFile(p)
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    cfg, _, er := image.DecodeConfig(bytes.NewReader(raw))
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width != cfg.Height {
        return "ERR_ICON_NOT_SQUARE"
    }
    img, _, er := image.Decode(bytes.NewReader(raw))
    if er != nil {
        return "ERR_ICON_DECODE"
    }
    outPath := filepath.Join(dir, "LargeLogo.png")
    f, ferr := os.Create(outPath)
    if ferr != nil {
        return "ERR_WRITE_TARGET"
    }
    defer f.Close()
    if err := png.Encode(f, img); err != nil {
        return "ERR_WRITE_TARGET"
    }
    return ""
}

func GetVersionLogoDataUrl(name string) string {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return ""
    }
    p := filepath.Join(vdir, strings.TrimSpace(name), "LargeLogo.png")
    if !utils.FileExists(p) {
        return ""
    }
    b, er := os.ReadFile(p)
    if er != nil {
        return ""
    }
    enc := base64.StdEncoding.EncodeToString(b)
    return "data:image/png;base64," + enc
}

func RemoveVersionLogo(name string) string {
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    p := filepath.Join(vdir, strings.TrimSpace(name), "LargeLogo.png")
    if utils.FileExists(p) {
        if er := os.Remove(p); er != nil {
            return "ERR_WRITE_TARGET"
        }
    }
    return ""
}

func ValidateVersionFolderName(name string) string {
    n := strings.TrimSpace(name)
    if msg := versions.ValidateFolderName(n); msg != "" {
        return msg
    }
    lower := strings.ToLower(n)
    reserved := map[string]struct{}{
        "con": {}, "prn": {}, "aux": {}, "nul": {},
        "com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {}, "com6": {}, "com7": {}, "com8": {}, "com9": {},
        "lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {}, "lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
    }
    if _, ok := reserved[lower]; ok {
        return "ERR_NAME_RESERVED"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || vdir == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    entries, err := os.ReadDir(vdir)
    if err != nil {
        return "ERR_READ_VERSIONS_DIR"
    }
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        if strings.EqualFold(e.Name(), n) {
            return "ERR_NAME_EXISTS"
        }
    }
    return ""
}

func RenameVersionFolder(oldName string, newName string) string {
    on := strings.TrimSpace(oldName)
    nn := strings.TrimSpace(newName)
    if on == "" || nn == "" {
        return "ERR_INVALID_NAME"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    oldPath := filepath.Join(vdir, on)
    newPath := filepath.Join(vdir, nn)
    if !utils.DirExists(oldPath) {
        return "ERR_NOT_FOUND_OLD"
    }
    if strings.EqualFold(on, nn) {
        m, _ := versions.ReadMeta(oldPath)
        m.Name = nn
        if er := versions.WriteMeta(oldPath, m); er != nil {
            return "ERR_WRITE_TARGET"
        }
        return ""
    }
    if utils.DirExists(newPath) {
        return "ERR_NAME_EXISTS"
    }
    if err := os.Rename(oldPath, newPath); err != nil {
        return "ERR_RENAME_FAILED"
    }
    m, _ := versions.ReadMeta(newPath)
    m.Name = nn
    if er := versions.WriteMeta(newPath, m); er != nil {
        return "ERR_WRITE_TARGET"
    }
    return ""
}

func IsProcessRunningAtPath(exePath string) bool {
    p := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
    if p == "" {
        return false
    }
    snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
    if err != nil {
        return false
    }
    defer windows.CloseHandle(snap)
    var pe windows.ProcessEntry32
    pe.Size = uint32(unsafe.Sizeof(pe))
    if err := windows.Process32First(snap, &pe); err != nil {
        return false
    }
    for {
        pid := pe.ProcessID
        h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
        if err == nil {
            buf := make([]uint16, 1024)
            size := uint32(len(buf))
            if e := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e == nil && size > 0 {
                path := windows.UTF16ToString(buf[:size])
                _ = windows.CloseHandle(h)
                norm := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
                norm = strings.TrimPrefix(norm, `\\?\`)
                norm = strings.TrimPrefix(norm, `\??\`)
                if norm == p {
                    return true
                }
            } else {
                _ = windows.CloseHandle(h)
            }
        }
        if err := windows.Process32Next(snap, &pe); err != nil {
            break
        }
    }
    return false
}

func DeleteVersionFolder(name string) string {
    n := strings.TrimSpace(name)
    if n == "" {
        return "ERR_INVALID_NAME"
    }
    vdir, err := utils.GetVersionsDir()
    if err != nil || strings.TrimSpace(vdir) == "" {
        return "ERR_ACCESS_VERSIONS_DIR"
    }
    dir := filepath.Join(vdir, n)
    if !utils.DirExists(dir) {
        return "ERR_NOT_FOUND_OLD"
    }
    exe := filepath.Join(dir, "Minecraft.Windows.exe")
    if utils.FileExists(exe) && IsProcessRunningAtPath(exe) {
        return "ERR_GAME_ALREADY_RUNNING"
    }
    if err := os.RemoveAll(dir); err != nil {
        return "ERR_DELETE_FAILED"
    }
    return ""
}

func CreateDesktopShortcut(name string) string {
    n := strings.TrimSpace(name)
    if n == "" {
        return "ERR_NAME_REQUIRED"
    }
    exePath, _ := os.Executable()
    exePath = strings.TrimSpace(exePath)
    if exePath == "" {
        return "ERR_SHORTCUT_CREATE_FAILED"
    }
    home, _ := os.UserHomeDir()
    if strings.TrimSpace(home) == "" {
        home = os.Getenv("USERPROFILE")
    }
    if strings.TrimSpace(home) == "" {
        return "ERR_SHORTCUT_CREATE_FAILED"
    }
    desktop := filepath.Join(home, "Desktop")
    if fi, err := os.Stat(desktop); err != nil || !fi.IsDir() {
        return "ERR_SHORTCUT_CREATE_FAILED"
    }
    safeName := n
    lnk := filepath.Join(desktop, "Minecraft - "+safeName+".lnk")
    args := "--launch=" + n
    workdir := filepath.Dir(exePath)
    iconPath := exePath
    if vdir, err := utils.GetVersionsDir(); err == nil && strings.TrimSpace(vdir) != "" {
        dir := filepath.Join(vdir, n)
        e := filepath.Join(dir, "Minecraft.Windows.exe")
        isPreview := false
        if m, er := versions.ReadMeta(dir); er == nil {
            isPreview = strings.EqualFold(strings.TrimSpace(m.Type), "preview")
        }
        if p := icons.EnsureVersionIcon(dir, isPreview); strings.TrimSpace(p) != "" {
            lp := strings.ToLower(p)
            if strings.HasSuffix(lp, ".ico") || strings.HasSuffix(lp, ".exe") {
                iconPath = p
            } else {
                if utils.FileExists(e) {
                    iconPath = e
                }
            }
        } else if utils.FileExists(e) {
            iconPath = e
        }
    }
    esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
    script := "$WshShell = New-Object -ComObject WScript.Shell; " +
        "$Shortcut = $WshShell.CreateShortcut('" + esc(lnk) + "'); " +
        "$Shortcut.TargetPath = '" + esc(exePath) + "'; " +
        "$Shortcut.Arguments = '" + esc(args) + "'; " +
        "$Shortcut.WorkingDirectory = '" + esc(workdir) + "'; " +
        "$Shortcut.IconLocation = '" + esc(iconPath) + "'; " +
        "$Shortcut.Save()"
    cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
    if err := cmd.Run(); err != nil {
        return "ERR_SHORTCUT_CREATE_FAILED"
    }
    return ""
}