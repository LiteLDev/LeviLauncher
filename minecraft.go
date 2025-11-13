package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/explorer"
	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/lang"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/mods"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/preloader"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"

	"golang.org/x/sys/windows"
	winreg "golang.org/x/sys/windows/registry"

	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
)

var (
	giMu       sync.Mutex
	giEnsuring bool
)

const (
	EventGameInputEnsureStart      = "gameinput.ensure.start"
	EventGameInputEnsureDone       = "gameinput.ensure.done"
	EventGameInputDownloadStart    = "gameinput.download.start"
	EventGameInputDownloadProgress = "gameinput.download.progress"
	EventGameInputDownloadDone     = "gameinput.download.done"
	EventGameInputDownloadError    = "gameinput.download.error"
)

type GameInputDownloadProgress struct {
	Downloaded int64
	Total      int64
}

type ExtractProgress struct {
	Dir   string
	Files int64
	Bytes int64
	Ts    int64
}

func (a *Minecraft) GetDriveStats(root string) map[string]uint64 {
	res := map[string]uint64{"total": 0, "free": 0}
	r := strings.TrimSpace(root)
	if r == "" {
		return res
	}
	if !strings.HasSuffix(r, "\\") {
		r += "\\"
	}
	p, err := windows.UTF16PtrFromString(r)
	if err != nil {
		return res
	}
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err == nil {
		res["total"] = totalNumberOfBytes
		res["free"] = totalNumberOfFreeBytes
	}
	return res
}

func (a *Minecraft) FetchHistoricalVersions(preferCN bool) map[string]interface{} {
	const githubURL = "https://raw.githubusercontent.com/LiteLDev/minecraft-windows-gdk-version-db/refs/heads/main/historical_versions.json"
	const gitcodeURL = "https://github.bibk.top/LiteLDev/minecraft-windows-gdk-version-db/raw/refs/heads/main/historical_versions.json"

	urls := []string{githubURL, gitcodeURL}
	if preferCN {
		urls = []string{gitcodeURL, githubURL}
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
				return
			}
			dec := json.NewDecoder(resp.Body)
			var obj map[string]interface{}
			if derr := dec.Decode(&obj); derr != nil {
				lastErr = derr
				return
			}

			obj["_source"] = u
			urls = nil
			lastErr = nil

		}()
		if lastErr == nil && urls == nil {
			resp2, err2 := client.Get(u)
			if err2 != nil {
				lastErr = err2
				continue
			}
			defer resp2.Body.Close()
			var obj2 map[string]interface{}
			if derr2 := json.NewDecoder(resp2.Body).Decode(&obj2); derr2 != nil {
				lastErr = derr2
				continue
			}
			obj2["_source"] = u
			return obj2
		}
	}
	if lastErr != nil {
		log.Println("FetchHistoricalVersions error:", lastErr)
	}
	return map[string]interface{}{}
}

type KnownFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *Minecraft) ListKnownFolders() []KnownFolder {
	out := []KnownFolder{}
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		home = os.Getenv("USERPROFILE")
	}
	add := func(name, p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, KnownFolder{Name: name, Path: p})
		}
	}
	add("Home", home)
	if home != "" {
		add("Desktop", filepath.Join(home, "Desktop"))
	}
	return out
}

type ContentCounts struct {
	Worlds        int `json:"worlds"`
	ResourcePacks int `json:"resourcePacks"`
	BehaviorPacks int `json:"behaviorPacks"`
}

func (a *Minecraft) GetContentRoots(name string) types.ContentRoots {
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

func (a *Minecraft) GetContentCounts(name string) ContentCounts {
	roots := a.GetContentRoots(name)
	countDirs := func(path string) int {
		p := strings.TrimSpace(path)
		if p == "" {
			return 0
		}
		entries := a.ListDir(p)
		n := 0
		for _, e := range entries {
			if e.IsDir {
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
		entries := a.ListDir(usersRoot)
		var firstPlayer string
		for _, e := range entries {
			if e.IsDir {
				nm := strings.TrimSpace(e.Name)
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

func (a *Minecraft) LaunchVersionByName(name string) string {
	return a.launchVersionInternal(name, true)
}

func (a *Minecraft) LaunchVersionByNameForce(name string) string {
	return a.launchVersionInternal(name, false)
}

func isProcessRunningAtPath(exePath string) bool {
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

func (a *Minecraft) SaveVersionMeta(name string, gameVersion string, typeStr string, enableIsolation bool, enableConsole bool, enableEditorMode bool) string {
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

func (a *Minecraft) ListVersionMetas() []versions.VersionMeta {
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

func (a *Minecraft) ListInheritableVersionNames(versionType string) []string {
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

func (a *Minecraft) CopyGameToVersion(targetDir string, isPreview bool) string {
	return ""
}

func (a *Minecraft) CopyVersionDataFromVersion(sourceName string, targetName string) string {
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

func (a *Minecraft) CopyVersionDataFromGDK(isPreview bool, targetName string) string {
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

func (a *Minecraft) SaveVersionLogoDataUrl(name string, dataUrl string) string {
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
	llDir := filepath.Join(dir, "LeviLauncher")
	if err := os.MkdirAll(llDir, 0755); err != nil {
		return "ERR_WRITE_TARGET"
	}
	outPath := filepath.Join(llDir, "Logo.png")
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

func (a *Minecraft) SaveVersionLogoFromPath(name string, filePath string) string {
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
	llDir := filepath.Join(dir, "LeviLauncher")
	if err := os.MkdirAll(llDir, 0755); err != nil {
		return "ERR_WRITE_TARGET"
	}
	outPath := filepath.Join(llDir, "Logo.png")
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

func (a *Minecraft) GetVersionLogoDataUrl(name string) string {
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return ""
	}
	p := filepath.Join(vdir, strings.TrimSpace(name), "LeviLauncher", "Logo.png")
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

func (a *Minecraft) RemoveVersionLogo(name string) string {
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	p := filepath.Join(vdir, strings.TrimSpace(name), "LeviLauncher", "Logo.png")
	if utils.FileExists(p) {
		if er := os.Remove(p); er != nil {
			return "ERR_WRITE_TARGET"
		}
	}
	return ""
}

func (a *Minecraft) ValidateVersionFolderName(name string) string {
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

func (a *Minecraft) RenameVersionFolder(oldName string, newName string) string {
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

func (a *Minecraft) DeleteVersionFolder(name string) string {
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
	if utils.FileExists(exe) && isProcessRunningAtPath(exe) {
		return "ERR_GAME_ALREADY_RUNNING"
	}
	if err := os.RemoveAll(dir); err != nil {
		return "ERR_DELETE_FAILED"
	}
	return ""
}

type Minecraft struct {
	ctx context.Context
}

func NewMinecraft() *Minecraft {
	return &Minecraft{}
}

func (a *Minecraft) startup() {
	a.ctx = application.Get().Context()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	os.Chdir(exeDir)
	launch.EnsureGamingServicesInstalled(a.ctx)
}

func (a *Minecraft) IsFirstLaunch() bool {
	return false
}

func (a *Minecraft) EnsureGameInputInteractive() { go ensureGameInputInteractive(a.ctx) }

func (a *Minecraft) IsGameInputInstalled() bool {
	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\Microsoft\GameInputRedist`, winreg.READ); err == nil {
		return true
	}
	return false
}

func ensureGameInputInteractive(ctx context.Context) {
	giMu.Lock()
	if giEnsuring {
		giMu.Unlock()
		return
	}
	giEnsuring = true
	giMu.Unlock()
	defer func() {
		giMu.Lock()
		giEnsuring = false
		giMu.Unlock()
	}()

	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\Microsoft\GameInputRedist`, winreg.READ); err == nil {
		return
	}
	application.Get().Event.Emit(EventGameInputEnsureStart, struct{}{})
	type ghAsset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	type ghRelease struct {
		Assets []ghAsset `json:"assets"`
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/microsoftconnect/GameInput/releases/latest", nil)
	req.Header.Set("User-Agent", "LeviLauncher")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("gameinput latest fetch error:", err)
		return
	}
	defer resp.Body.Close()
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		log.Println("gameinput json error:", err)
		return
	}
	var dl string
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Name, "GameInputRedist.msi") {
			dl = a.URL
			break
		}
	}
	if dl == "" {
		log.Println("gameinput asset not found in latest release")
		return
	}
	dir, _ := utils.GetInstallerDir()
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Println("mkdir installers error:", err)
		return
	}
	dlPath := filepath.Join(dir, "GameInputRedist.msi")
	tmpPath := dlPath + ".part"
	if fi, err := os.Stat(dlPath); err == nil && fi.Size() > 0 {
		application.Get().Event.Emit(EventGameInputDownloadStart, fi.Size())
		application.Get().Event.Emit(EventGameInputDownloadDone, struct{}{})
		log.Println("GameInputRedist cached, size:", fi.Size())
	} else {
		req2, _ := http.NewRequest("GET", dl, nil)
		req2.Header.Set("User-Agent", "LeviLauncher")
		r2, err := http.DefaultClient.Do(req2)
		if err != nil {
			log.Println("gameinput download error:", err)
			return
		}
		if r2.StatusCode < 200 || r2.StatusCode >= 300 {
			log.Println("gameinput download bad status:", r2.Status)
			r2.Body.Close()
			return
		}
		defer r2.Body.Close()
		_ = os.Remove(tmpPath)
		f, err := os.Create(tmpPath)
		if err != nil {
			log.Println("gameinput create file error:", err)
			return
		}
		defer f.Close()
		application.Get().Event.Emit(EventGameInputDownloadStart, r2.ContentLength)
		var downloaded int64
		buf := make([]byte, 64*1024)
		for {
			n, er := r2.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					log.Println("gameinput write error:", werr)
					application.Get().Event.Emit(EventGameInputDownloadError, werr.Error())
					return
				}
				downloaded += int64(n)
				application.Get().Event.Emit(EventGameInputDownloadProgress, GameInputDownloadProgress{Downloaded: downloaded, Total: r2.ContentLength})
			}
			if er == io.EOF {
				break
			}
			if er != nil {
				log.Println("gameinput read error:", er)
				application.Get().Event.Emit(EventGameInputDownloadError, er.Error())
				return
			}
		}
		if _, stErr := os.Stat(dlPath); stErr == nil {
			_ = os.Remove(dlPath)
		}
		if err := os.Rename(tmpPath, dlPath); err != nil {
			log.Println("gameinput rename error:", err)
			return
		}
		application.Get().Event.Emit(EventGameInputDownloadDone, struct{}{})
		log.Println("GameInputRedist downloaded:", dlPath)
	}
	cmd := exec.Command("msiexec", "/i", dlPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		log.Println("failed to run GameInput installer:", err)
	}
	installed := false
	for i := 0; i < 30; i++ {
		if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\Microsoft\GameInput`, winreg.READ); err == nil {
			installed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Println("GameInput installed:", installed)
	application.Get().Event.Emit(EventGameInputEnsureDone, struct{}{})
}

func (a *Minecraft) IsGamingServicesInstalled() bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err == nil {
		return true
	}
	return false
}

func (a *Minecraft) StartMsixvcDownload(url string) string {
	return msixvc.StartDownload(a.ctx, url)
}

func (a *Minecraft) ResumeMsixvcDownload() { msixvc.Resume() }

func (a *Minecraft) CancelMsixvcDownload() { msixvc.Cancel() }

func (a *Minecraft) InstallMsixvc(name string, isPreview bool) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_MSIXVC_NOT_SPECIFIED"
	}
	return msixvc.Install(a.ctx, n, isPreview)
}

func (a *Minecraft) InstallExtractMsixvc(name string, folderName string, isPreview bool) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_MSIXVC_NOT_SPECIFIED"
	}
	inPath := n
	if !filepath.IsAbs(inPath) {
		if dir, err := utils.GetInstallerDir(); err == nil && dir != "" {
			inPath += ".msixvc"
			inPath = filepath.Join(dir, inPath)
		}
	}
	if !utils.FileExists(inPath) {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	outDir := filepath.Join(vdir, strings.TrimSpace(folderName))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	stopCh := make(chan struct{})
	go func(dir string) {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var totalBytes int64
				var files int64
				_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() {
						return nil
					}
					if fi, e := os.Stat(path); e == nil {
						totalBytes += fi.Size()
						files++
					}
					return nil
				})

				application.Get().Event.Emit("extract.progress", ExtractProgress{Dir: dir, Files: files, Bytes: totalBytes, Ts: time.Now().UnixMilli()})
			case <-stopCh:
				return
			}
		}
	}(outDir)

	rc, msg := extractor.MiHoYo(inPath, outDir)
	close(stopCh)
	if rc != 0 {
		application.Get().Event.Emit("extract.error", msg)
		if strings.TrimSpace(msg) == "" {
			msg = "ERR_APPX_INSTALL_FAILED"
		}
		return msg
	}
	_ = vcruntime.EnsureForVersion(a.ctx, outDir)
	_ = preloader.EnsureForVersion(a.ctx, outDir)
	_ = peeditor.EnsureForVersion(a.ctx, outDir)
	_ = peeditor.RunForVersion(a.ctx, outDir)
	application.Get().Event.Emit("extract.done", outDir)
	return ""
}

func (a *Minecraft) ResolveDownloadedMsixvc(version string, versionType string) string {
	dir, err := utils.GetInstallerDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".msixvc") {
			continue
		}

		ext := filepath.Ext(name)
		if strings.ToLower(ext) == ".msixvc" {
			nameNoExt := name[:len(name)-len(ext)]
			name = nameNoExt
		} else {
			name = strings.TrimSuffix(name, ".msixvc")
		}
		b := strings.TrimSpace(name)
		v := strings.TrimSpace(version)
		bl := strings.ToLower(b)
		vl := strings.ToLower(v)

		if vl == bl {
			return name
		}

	}
	return ""
}

func (a *Minecraft) DeleteDownloadedMsixvc(version string, versionType string) string {
	name := strings.TrimSpace(a.ResolveDownloadedMsixvc(version, versionType))
	if name == "" {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	dir, err := utils.GetInstallerDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return "ERR_ACCESS_INSTALLERS_DIR"
	}
	path := filepath.Join(dir, name+".msixvc")
	if !utils.FileExists(path) {
		alt := filepath.Join(dir, name)
		if utils.FileExists(alt) {
			path = alt
		}
	}
	if !utils.FileExists(path) {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	if err := os.Remove(path); err != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}

type VersionStatus struct {
	Version      string `json:"version"`
	IsInstalled  bool   `json:"isInstalled"`
	IsDownloaded bool   `json:"isDownloaded"`
	Type         string `json:"type"`
}

func (a *Minecraft) GetInstallerDir() string {
	dir, err := utils.GetInstallerDir()
	if err != nil {
		return ""
	}
	return dir
}

func (a *Minecraft) GetVersionsDir() string {
	dir, err := utils.GetVersionsDir()
	if err != nil {
		return ""
	}
	return dir
}

func (a *Minecraft) GetVersionStatus(version string, versionType string) VersionStatus {
	status := VersionStatus{
		Version:      version,
		Type:         versionType,
		IsInstalled:  false,
		IsDownloaded: false,
	}

	if name := a.ResolveDownloadedMsixvc(version, versionType); strings.TrimSpace(name) != "" {
		status.IsDownloaded = true
	}

	return status
}

func (a *Minecraft) GetAllVersionsStatus(versions []map[string]interface{}) []VersionStatus {
	var results []VersionStatus

	for _, versionData := range versions {
		version, ok := versionData["version"].(string)
		if !ok {
			version, ok = versionData["short"].(string)
			if !ok {
				continue
			}
		}

		versionType, ok := versionData["type"].(string)
		if !ok {
			versionType = "release"
		}

		status := a.GetVersionStatus(version, versionType)
		results = append(results, status)
	}

	return results
}

func (a *Minecraft) OpenModsExplorer(name string) { _ = explorer.OpenMods(name) }

func (a *Minecraft) OpenWorldsExplorer(name string, isPreview bool) {
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users != "" {
		entries := a.ListDir(users)
		var firstPlayer string
		for _, e := range entries {
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

func (a *Minecraft) OpenPathDir(dir string) {
	d := strings.TrimSpace(dir)
	if d == "" {
		return
	}
	_ = explorer.OpenPath(d)
}

func (a *Minecraft) OpenGameDataExplorer(isPreview bool) {
	base := utils.GetMinecraftGDKDataPath(isPreview)
	_ = explorer.OpenPath(base)
}

func (a *Minecraft) GetCurrentVersion(isPreview bool) string {
	info, err := registry.GetMinecraftPackage(isPreview)
	if err != nil || info == nil {
		log.Println("GetCurrentVersion:", err)
		return ""
	}
	if v, ok := info["Version"].(string); ok {
		return v
	}
	return ""
}

func (a *Minecraft) GetMods(name string) []types.ModInfo {
	return mods.GetMods(name)
}

func (a *Minecraft) ImportModZip(name string, data []byte, overwrite bool) string {
	return mods.ImportZipToMods(name, data, overwrite)
}

func (a *Minecraft) DeleteMod(name string, modName string) string {
	return mods.DeleteMod(name, modName)
}

func (a *Minecraft) ImportModDll(name string, fileName string, data []byte, modName string, modType string, version string, overwrite bool) string {
	return mods.ImportDllToMods(name, fileName, data, modName, modType, version, overwrite)
}

func (a *Minecraft) ListDrives() []string {
	drives := []string{}
	for l := 'A'; l <= 'Z'; l++ {
		root := string(l) + ":\\"
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			drives = append(drives, root)
		}
	}
	return drives
}

func (a *Minecraft) ListDir(path string) []types.FileEntry {
	list := []types.FileEntry{}
	if path == "" {
		return list
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return list
	}
	for _, e := range ents {
		list = append(list, types.FileEntry{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
			Size:  0,
		})
	}
	return list
}

func (a *Minecraft) GetWorldLevelName(worldDir string) string {
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

func (a *Minecraft) GetWorldIconDataUrl(worldDir string) string {
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

func (a *Minecraft) BackupWorld(worldDir string) string {
	if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
		return ""
	}
	level := a.GetWorldLevelName(worldDir)
	if level == "" {
		level = utils.GetLastDirName(worldDir)
	}
	safe := utils.SanitizeFilename(level)
	ts := time.Now().Format("20060102-150405")
	base := utils.BaseRoot()
	backupDir := filepath.Join(base, "backup", "worlds", safe)
	if err := utils.CreateDir(backupDir); err != nil {
		return ""
	}
	dest := filepath.Join(backupDir, fmt.Sprintf("%s_%s.mcworld", safe, ts))
	if err := utils.ZipDir(worldDir, dest); err != nil {
		return ""
	}
	return dest
}

func (a *Minecraft) BackupWorldWithVersion(worldDir string, versionName string) string {
	if strings.TrimSpace(worldDir) == "" || !utils.DirExists(worldDir) {
		return ""
	}
	level := a.GetWorldLevelName(worldDir)
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
	backupDir := filepath.Join(base, "backup", "worlds", safeVersion, safeFolder+"_"+safeWorld)
	if err := utils.CreateDir(backupDir); err != nil {
		return ""
	}
	dest := filepath.Join(backupDir, fmt.Sprintf("%s_%s.mcworld", safeWorld, ts))
	if err := utils.ZipDir(worldDir, dest); err != nil {
		return ""
	}
	return dest
}

func (a *Minecraft) ImportModZipPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	return mods.ImportZipToMods(name, b, overwrite)
}

func (a *Minecraft) ImportModDllPath(name string, path string, modName string, modType string, version string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_WRITE_FILE"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_WRITE_FILE"
	}
	return mods.ImportDllToMods(name, filepath.Base(path), b, modName, modType, version, overwrite)
}

func (a *Minecraft) CanCreateDir(dir string) bool {
	err := os.Mkdir(filepath.Join(dir, "levitest"), 0755)
	if err != nil {
		if os.IsPermission(err) {
			return false
		}
		return false
	}
	os.Remove(filepath.Join(dir, "levitest"))
	return true
}

func (a *Minecraft) GetLanguageNames() []types.LanguageJson {
	return lang.GetLanguageNames()
}

func (a *Minecraft) GetAppVersion() string { return update.GetAppVersion() }
func (a *Minecraft) GetIsBeta() bool       { return update.IsBeta() }

func (a *Minecraft) CheckUpdate() types.CheckUpdate {
	cu := update.CheckUpdate(update.GetAppVersion())
	return cu
}

func (a *Minecraft) Update() bool {
	err := update.Update(update.GetAppVersion())
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func (a *Minecraft) TestMirrorLatencies(urls []string, timeoutMs int) []map[string]interface{} {
	if timeoutMs <= 0 {
		timeoutMs = 7000
	}
	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	results := make([]map[string]interface{}, 0, len(urls))
	for _, u := range urls {
		start := time.Now()
		ok := false
		req, err := http.NewRequest("HEAD", strings.TrimSpace(u), nil)
		if err == nil {
			req.Header.Set("User-Agent", "LeviLauncher/latency")
			if resp, er := client.Do(req); er == nil {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					ok = true
				}
			}
		}
		elapsed := time.Since(start).Milliseconds()
		results = append(results, map[string]interface{}{
			"url":       u,
			"latencyMs": elapsed,
			"ok":        ok,
		})
	}
	return results
}
func (a *Minecraft) launchVersionInternal(name string, checkRunning bool) string {
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	if !utils.FileExists(exe) {
		return "ERR_NOT_FOUND_EXE"
	}
	if checkRunning {
		if isProcessRunningAtPath(exe) {
			return "ERR_GAME_ALREADY_RUNNING"
		}
	}
	_ = vcruntime.EnsureForVersion(a.ctx, dir)
	_ = preloader.EnsureForVersion(a.ctx, dir)
	_ = peeditor.EnsureForVersion(a.ctx, dir)
	_ = peeditor.RunForVersion(a.ctx, dir)
	var args []string
	if m, err := versions.ReadMeta(dir); err == nil {
		if m.EnableEditorMode {
			args = []string{"minecraft://creator/?Editor=true"}
		}
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return "ERR_LAUNCH_GAME"
	}
	go launch.MonitorMinecraftWindow(a.ctx)
	return ""
}
func (a *Minecraft) GetBaseRoot() string { return utils.BaseRoot() }
func (a *Minecraft) SetBaseRoot(root string) string {
	r := strings.TrimSpace(root)
	if r == "" {
		return "ERR_INVALID_PATH"
	}
	if err := os.MkdirAll(r, 0o755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	if err := config.Save(config.AppConfig{BaseRoot: r}); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}
func (a *Minecraft) ResetBaseRoot() string {
	if err := config.Save(config.AppConfig{BaseRoot: ""}); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}
func (a *Minecraft) CanWriteToDir(path string) bool { return utils.CanWriteDir(path) }
