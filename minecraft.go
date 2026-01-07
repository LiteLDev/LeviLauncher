package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/curseforge/client"
	cursetypes "github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/gdk"
	"github.com/liteldev/LeviLauncher/internal/lang"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/mods"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const curseForgeAPIKey = "$2a$10$QNbi2vCXdtq6WXROpLih9OSMR2KKpL47XoOHPPzK33OoKBp7Yzhbi"

const (
	EventGameInputEnsureStart      = "gameinput.ensure.start"
	EventGameInputEnsureDone       = "gameinput.ensure.done"
	EventGameInputDownloadStart    = "gameinput.download.start"
	EventGameInputDownloadProgress = "gameinput.download.progress"
	EventGameInputDownloadDone     = "gameinput.download.done"
	EventGameInputDownloadError    = "gameinput.download.error"

	EventFileDownloadStatus   = "file_download_status"
	EventFileDownloadProgress = "file_download_progress"
	EventFileDownloadDone     = "file_download_done"
	EventFileDownloadError    = "file_download_error"
)

type FileDownloadProgress struct {
	Downloaded int64
	Total      int64
	Dest       string
}

var fileDownloader = downloader.NewManager(
	downloader.Events{
		Status:   EventFileDownloadStatus,
		Progress: EventFileDownloadProgress,
		Done:     EventFileDownloadDone,
		Error:    EventFileDownloadError,
		ProgressFactory: func(p downloader.DownloadProgress) any {
			return FileDownloadProgress{Downloaded: p.Downloaded, Total: p.Total, Dest: p.Dest}
		},
	},
	downloader.Options{Throttle: 250 * time.Millisecond, Resume: false, RemoveOnCancel: true},
)

type GameInputDownloadProgress struct {
	Downloaded int64
	Total      int64
}

func (a *Minecraft) StartFileDownload(url string, filename string) string {
	tempDir := filepath.Join(os.TempDir(), "LeviLauncher", "Downloads")
	_ = os.MkdirAll(tempDir, 0755)
	dest := filepath.Join(tempDir, filename)
	return fileDownloader.Start(a.ctx, url, dest)
}

func (a *Minecraft) CancelFileDownload() {
	fileDownloader.Cancel()
}

func (a *Minecraft) GetDriveStats(root string) map[string]uint64 {
	return mcservice.GetDriveStats(root)
}

func (a *Minecraft) FetchHistoricalVersions(preferCN bool) map[string]interface{} {
	return mcservice.FetchHistoricalVersions(preferCN)
}

type KnownFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *Minecraft) ListKnownFolders() []KnownFolder {
	var out []KnownFolder
	for _, k := range mcservice.ListKnownFolders() {
		out = append(out, KnownFolder{Name: k.Name, Path: k.Path})
	}
	return out
}

type ContentCounts struct {
	Worlds        int `json:"worlds"`
	ResourcePacks int `json:"resourcePacks"`
	BehaviorPacks int `json:"behaviorPacks"`
}

func (a *Minecraft) GetContentRoots(name string) types.ContentRoots {
	return mcservice.GetContentRoots(name)
}

func (a *Minecraft) GetContentCounts(name string) ContentCounts {
	c := mcservice.GetContentCounts(name)
	return ContentCounts{Worlds: c.Worlds, ResourcePacks: c.ResourcePacks, BehaviorPacks: c.BehaviorPacks}
}

func (a *Minecraft) ListPacksForVersion(versionName string, player string) []packages.Pack {
	roots := a.GetContentRoots(versionName)
	if roots.ResourcePacks == "" && roots.BehaviorPacks == "" {
		return []packages.Pack{}
	}
	var skinPacksDirs []string

	if roots.ResourcePacks != "" {
		sharedSkins := filepath.Join(filepath.Dir(roots.ResourcePacks), "skin_packs")
		if utils.DirExists(sharedSkins) {
			skinPacksDirs = append(skinPacksDirs, sharedSkins)
		}
	}

	if player != "" && roots.UsersRoot != "" {
		userSkins := filepath.Join(roots.UsersRoot, player, "games", "com.mojang", "skin_packs")
		if utils.DirExists(userSkins) {
			skinPacksDirs = append(skinPacksDirs, userSkins)
		}
		userSkinsSimple := filepath.Join(roots.UsersRoot, player, "skin_packs")
		if utils.DirExists(userSkinsSimple) {
			skinPacksDirs = append(skinPacksDirs, userSkinsSimple)
		}
	}

	packs, err := a.packManager.LoadPacksForVersion(versionName, roots.ResourcePacks, roots.BehaviorPacks, skinPacksDirs...)
	if err != nil {
		return []packages.Pack{}
	}
	return packs
}

func (a *Minecraft) LaunchVersionByName(name string) string {
	return a.launchVersionInternal(name, true)
}

func (a *Minecraft) LaunchVersionByNameForce(name string) string {
	return a.launchVersionInternal(name, false)
}

func (a *Minecraft) CreateDesktopShortcut(name string) string {
	return mcservice.CreateDesktopShortcut(name)
}

func (a *Minecraft) SaveVersionMeta(name string, gameVersion string, typeStr string, enableIsolation bool, enableConsole bool, enableEditorMode bool) string {
	return mcservice.SaveVersionMeta(name, gameVersion, typeStr, enableIsolation, enableConsole, enableEditorMode)
}

func (a *Minecraft) ListVersionMetas() []versions.VersionMeta { return mcservice.ListVersionMetas() }
func (a *Minecraft) ListVersionMetasWithRegistered() []versions.VersionMeta {
	metas := mcservice.ListVersionMetas()
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return metas
	}
	var releaseLoc, previewLoc string
	if info, e := registry.GetAppxInfo("MICROSOFT.MINECRAFTUWP"); e == nil && info != nil {
		releaseLoc = strings.ToLower(filepath.Clean(strings.TrimSpace(info.InstallLocation)))
	}
	if info, e := registry.GetAppxInfo("Microsoft.MinecraftWindowsBeta"); e == nil && info != nil {
		previewLoc = strings.ToLower(filepath.Clean(strings.TrimSpace(info.InstallLocation)))
	}
	for i := range metas {
		m := &metas[i]
		isPreview := strings.EqualFold(strings.TrimSpace(m.Type), "preview")
		dir := filepath.Join(vdir, strings.TrimSpace(m.Name))
		norm := strings.ToLower(filepath.Clean(strings.TrimSpace(dir)))
		if isPreview {
			m.Registered = previewLoc != "" && norm == strings.ToLower(filepath.Clean(previewLoc))
		} else {
			m.Registered = releaseLoc != "" && norm == strings.ToLower(filepath.Clean(releaseLoc))
		}
	}
	return metas
}

func (a *Minecraft) GetVersionMeta(name string) versions.VersionMeta {
	return mcservice.GetVersionMeta(name)
}

func (a *Minecraft) ListInheritableVersionNames(versionType string) []string {
	return mcservice.ListInheritableVersionNames(versionType)
}

func (a *Minecraft) CopyVersionDataFromVersion(sourceName string, targetName string) string {
	return mcservice.CopyVersionDataFromVersion(sourceName, targetName)
}

func (a *Minecraft) CopyVersionDataFromGDK(isPreview bool, targetName string) string {
	return mcservice.CopyVersionDataFromGDK(isPreview, targetName)
}

func (a *Minecraft) SaveVersionLogoDataUrl(name string, dataUrl string) string {
	return mcservice.SaveVersionLogoDataUrl(name, dataUrl)
}

func (a *Minecraft) SaveVersionLogoFromPath(name string, filePath string) string {
	return mcservice.SaveVersionLogoFromPath(name, filePath)
}

func (a *Minecraft) GetVersionLogoDataUrl(name string) string {
	return mcservice.GetVersionLogoDataUrl(name)
}

func (a *Minecraft) RemoveVersionLogo(name string) string { return mcservice.RemoveVersionLogo(name) }

func (a *Minecraft) ValidateVersionFolderName(name string) string {
	return mcservice.ValidateVersionFolderName(name)
}

func (a *Minecraft) RenameVersionFolder(oldName string, newName string) string {
	return mcservice.RenameVersionFolder(oldName, newName)
}

func (a *Minecraft) DeleteVersionFolder(name string) string {
	return mcservice.DeleteVersionFolder(name)
}

type Minecraft struct {
	ctx         context.Context
	curseClient client.CurseClient
	packManager *packages.PackManager
}

func NewMinecraft() *Minecraft {
	return &Minecraft{
		curseClient: client.NewCurseClient(curseForgeAPIKey),
		packManager: packages.NewPackManager(),
	}
}

func (a *Minecraft) startup() {
	a.ctx = application.Get().Context()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	os.Chdir(exeDir)
	launch.EnsureGamingServicesInstalled(a.ctx)
	mcservice.ReconcileRegisteredFlags()
}

func (a *Minecraft) EnsureGameInputInteractive() { go gameinput.EnsureInteractive(a.ctx) }

func (a *Minecraft) IsGameInputInstalled() bool { return gameinput.IsInstalled() }

func (a *Minecraft) IsGamingServicesInstalled() bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err == nil {
		return true
	}
	return false
}

func (a *Minecraft) StartMsixvcDownload(url string) string {
	return mcservice.StartMsixvcDownload(a.ctx, url)
}
func (a *Minecraft) ResumeMsixvcDownload() { mcservice.ResumeMsixvcDownload() }
func (a *Minecraft) CancelMsixvcDownload() { mcservice.CancelMsixvcDownload() }

func (a *Minecraft) InstallExtractMsixvc(name string, folderName string, isPreview bool) string {
	return mcservice.InstallExtractMsixvc(a.ctx, name, folderName, isPreview)
}

func (a *Minecraft) ResolveDownloadedMsixvc(version string, versionType string) string {
	return mcservice.ResolveDownloadedMsixvc(version, versionType)
}

func (a *Minecraft) DeleteDownloadedMsixvc(version string, versionType string) string {
	return mcservice.DeleteDownloadedMsixvc(version, versionType)
}

// GDK helpers
func (a *Minecraft) IsGDKInstalled() bool { return gdk.IsInstalled() }

func (a *Minecraft) StartGDKDownload(url string) string { return gdk.StartDownload(a.ctx, url) }

func (a *Minecraft) CancelGDKDownload() { gdk.CancelDownload() }

func (a *Minecraft) InstallGDKFromZip(zipPath string) string {
	return gdk.InstallFromZip(a.ctx, zipPath)
}

func (a *Minecraft) RegisterVersionWithWdapp(name string, isPreview bool) string {
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	folder := filepath.Join(vdir, strings.TrimSpace(name))
	if e := gdk.UnregisterIfExists(isPreview); e != "" {
	}
	msg := gdk.RegisterVersionFolder(folder)
	if msg != "" {
		return msg
	}
	pkg := "MICROSOFT.MINECRAFTUWP"
	if isPreview {
		pkg = "Microsoft.MinecraftWindowsBeta"
	}
	if info, e := registry.GetAppxInfo(pkg); e == nil && info != nil {
		normalize := func(p string) string {
			s := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
			s = strings.TrimPrefix(s, `\\?\`)
			s = strings.TrimPrefix(s, `\??\`)
			return strings.TrimSuffix(s, string(os.PathSeparator))
		}
		expected := normalize(filepath.Join(vdir, strings.TrimSpace(name)))
		loc := normalize(info.InstallLocation)
		flag := loc != "" && loc == expected
		if m, er := versions.ReadMeta(folder); er == nil {
			m.Registered = flag
			_ = versions.WriteMeta(folder, m)
		}
	}
	mcservice.ReconcileRegisteredFlags()
	return msg
}

func (a *Minecraft) UnregisterVersionByName(name string) string {
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	folder := filepath.Join(vdir, strings.TrimSpace(name))
	msg := gdk.UnregisterVersionFolder(folder)
	if msg != "" {
		return msg
	}
	if m, er := versions.ReadMeta(folder); er == nil {
		m.Registered = false
		_ = versions.WriteMeta(folder, m)
	}
	mcservice.ReconcileRegisteredFlags()
	return ""
}

type VersionStatus struct {
	Version      string `json:"version"`
	IsInstalled  bool   `json:"isInstalled"`
	IsDownloaded bool   `json:"isDownloaded"`
	Type         string `json:"type"`
}

func (a *Minecraft) GetInstallerDir() string { return mcservice.GetInstallerDir() }

func (a *Minecraft) GetVersionsDir() string { return mcservice.GetVersionsDir() }

func (a *Minecraft) GetVersionStatus(version string, versionType string) VersionStatus {
	s := mcservice.GetVersionStatus(version, versionType)
	return VersionStatus{Version: s.Version, Type: s.Type, IsInstalled: s.IsInstalled, IsDownloaded: s.IsDownloaded}
}

func (a *Minecraft) GetAllVersionsStatus(versions []map[string]interface{}) []VersionStatus {
	var results []VersionStatus
	for _, s := range mcservice.GetAllVersionsStatus(versions) {
		results = append(results, VersionStatus{Version: s.Version, Type: s.Type, IsInstalled: s.IsInstalled, IsDownloaded: s.IsDownloaded})
	}
	return results
}

func (a *Minecraft) OpenModsExplorer(name string) { mcservice.OpenModsExplorer(name) }

func (a *Minecraft) OpenWorldsExplorer(name string, isPreview bool) {
	mcservice.OpenWorldsExplorer(name, isPreview)
}

func (a *Minecraft) OpenPathDir(dir string) { mcservice.OpenPathDir(dir) }

func (a *Minecraft) OpenGameDataExplorer(isPreview bool) { mcservice.OpenGameDataExplorer(isPreview) }

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

func (a *Minecraft) DisableMod(name string, modName string) string {
	return mods.DisableMod(name, modName)
}

func (a *Minecraft) EnableMod(name string, modName string) string {
	return mods.EnableMod(name, modName)
}

func (a *Minecraft) IsModEnabled(name string, modName string) bool {
	return mods.IsModEnabled(name, modName)
}

func (a *Minecraft) ImportMcpack(name string, data []byte, overwrite bool) string {
	roots := a.GetContentRoots(name)
	return content.ImportMcpackToDirs2(data, "", roots.ResourcePacks, roots.BehaviorPacks, "", overwrite)
}

func (a *Minecraft) ImportMcpackPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := a.GetContentRoots(name)
	return content.ImportMcpackToDirs2(b, filepath.Base(path), roots.ResourcePacks, roots.BehaviorPacks, "", overwrite)
}

func (a *Minecraft) ImportMcaddon(name string, data []byte, overwrite bool) string {
	roots := a.GetContentRoots(name)
	return content.ImportMcaddonToDirs2(data, roots.ResourcePacks, roots.BehaviorPacks, "", overwrite)
}

func (a *Minecraft) ImportMcaddonPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := a.GetContentRoots(name)
	return content.ImportMcaddonToDirs2(b, roots.ResourcePacks, roots.BehaviorPacks, "", overwrite)
}

func (a *Minecraft) ImportMcaddonWithPlayer(name string, player string, data []byte, overwrite bool) string {
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcaddonToDirs2(data, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (a *Minecraft) ImportMcaddonPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcaddonToDirs2(b, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (a *Minecraft) ImportMcpackWithPlayer(name string, player string, fileName string, data []byte, overwrite bool) string {
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcpackToDirs2(data, fileName, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (a *Minecraft) ImportMcpackPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	skinDir := ""
	if users != "" && strings.TrimSpace(player) != "" {
		skinDir = filepath.Join(users, player, "games", "com.mojang", "skin_packs")
	}
	return content.ImportMcpackToDirs2(b, filepath.Base(path), roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
}

func (a *Minecraft) IsMcpackSkinPackPath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return content.IsMcpackSkinPack(b)
}

func (a *Minecraft) IsMcpackSkinPack(data []byte) bool {
	return content.IsMcpackSkinPack(data)
}

func (a *Minecraft) ImportMcworld(name string, player string, fileName string, data []byte, overwrite bool) string {
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users == "" || strings.TrimSpace(player) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	wp := filepath.Join(users, player, "games", "com.mojang", "minecraftWorlds")
	return content.ImportMcworldToDir(data, fileName, wp, overwrite)
}

func (a *Minecraft) ImportMcworldPath(name string, player string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	roots := a.GetContentRoots(name)
	users := strings.TrimSpace(roots.UsersRoot)
	if users == "" || strings.TrimSpace(player) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	wp := filepath.Join(users, player, "games", "com.mojang", "minecraftWorlds")
	return content.ImportMcworldToDir(b, filepath.Base(path), wp, overwrite)
}

func (a *Minecraft) GetPackInfo(dir string) types.PackInfo {
	return content.ReadPackInfoFromDir(dir)
}

func (a *Minecraft) DeletePack(name string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "ERR_INVALID_PATH"
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}
	roots := a.GetContentRoots(name)
	allowed := []string{strings.TrimSpace(roots.ResourcePacks), strings.TrimSpace(roots.BehaviorPacks)}
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	if usersRoot != "" {
		ents := a.ListDir(usersRoot)
		for _, e := range ents {
			if !e.IsDir {
				continue
			}
			nm := strings.TrimSpace(e.Name)
			if nm == "" || strings.EqualFold(nm, "Shared") {
				continue
			}
			sp := filepath.Join(usersRoot, nm, "games", "com.mojang", "skin_packs")
			allowed = append(allowed, sp)
		}
	}
	absTarget, _ := filepath.Abs(p)
	lowT := strings.ToLower(absTarget)
	ok := false
	for _, r := range allowed {
		if strings.TrimSpace(r) == "" {
			continue
		}
		absRoot, _ := filepath.Abs(r)
		lowR := strings.ToLower(absRoot)
		if lowT != lowR && strings.HasPrefix(lowT, lowR+string(os.PathSeparator)) {
			ok = true
			break
		}
	}
	if !ok {
		return "ERR_INVALID_PACKAGE"
	}
	if err := os.RemoveAll(absTarget); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func (a *Minecraft) DeleteWorld(name string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "ERR_INVALID_PATH"
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "ERR_INVALID_PATH"
	}
	roots := a.GetContentRoots(name)
	usersRoot := strings.TrimSpace(roots.UsersRoot)
	allowed := []string{}
	if usersRoot != "" {
		ents := a.ListDir(usersRoot)
		for _, e := range ents {
			if !e.IsDir {
				continue
			}
			nm := strings.TrimSpace(e.Name)
			if nm == "" || strings.EqualFold(nm, "Shared") {
				continue
			}
			wp := filepath.Join(usersRoot, nm, "games", "com.mojang", "minecraftWorlds")
			allowed = append(allowed, wp)
		}
	}
	absTarget, _ := filepath.Abs(p)
	lowT := strings.ToLower(absTarget)
	ok := false
	for _, r := range allowed {
		if strings.TrimSpace(r) == "" {
			continue
		}
		absRoot, _ := filepath.Abs(r)
		lowR := strings.ToLower(absRoot)
		if lowT != lowR && strings.HasPrefix(lowT, lowR+string(os.PathSeparator)) {
			ok = true
			break
		}
	}
	if !ok {
		return "ERR_INVALID_PATH"
	}
	if err := os.RemoveAll(absTarget); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func (a *Minecraft) ListDrives() []string { return mcservice.ListDrives() }

func (a *Minecraft) ListDir(path string) []types.FileEntry { return mcservice.ListDir(path) }

func (a *Minecraft) GetPathSize(path string) int64 { return mcservice.GetPathSize(path) }

func (a *Minecraft) GetPathModTime(path string) int64 { return mcservice.GetPathModTime(path) }

func (a *Minecraft) GetWorldLevelName(worldDir string) string {
	return mcservice.GetWorldLevelName(worldDir)
}

func (a *Minecraft) SetWorldLevelName(worldDir string, name string) string {
	return mcservice.SetWorldLevelName(worldDir, name)
}

func (a *Minecraft) GetWorldIconDataUrl(worldDir string) string {
	return mcservice.GetWorldIconDataUrl(worldDir)
}

func (a *Minecraft) BackupWorld(worldDir string) string { return mcservice.BackupWorld(worldDir) }

func (a *Minecraft) BackupWorldWithVersion(worldDir string, versionName string) string {
	return mcservice.BackupWorldWithVersion(worldDir, versionName)
}

func (a *Minecraft) ReadWorldLevelDatFields(worldDir string) map[string]any {
	return mcservice.ReadWorldLevelDatFields(worldDir)
}

func (a *Minecraft) WriteWorldLevelDatFields(worldDir string, args map[string]any) string {
	return mcservice.WriteWorldLevelDatFields(worldDir, args)
}

func (a *Minecraft) ReadWorldLevelDatFieldsAt(worldDir string, path []string) map[string]any {
	return mcservice.ReadWorldLevelDatFieldsAt(worldDir, path)
}

func (a *Minecraft) WriteWorldLevelDatFieldsAt(worldDir string, args map[string]any) string {
	return mcservice.WriteWorldLevelDatFieldsAt(worldDir, args)
}

func (a *Minecraft) WriteTempFile(name string, data []byte) string {
	tempDir := filepath.Join(os.TempDir(), "LeviLauncher", "TempImports")
	_ = os.MkdirAll(tempDir, 0755)
	outPath := filepath.Join(tempDir, name)
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return ""
	}
	return outPath
}

func (a *Minecraft) RemoveTempFile(path string) string {
	if path == "" {
		return ""
	}
	_ = os.Remove(path)
	return ""
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

func (a *Minecraft) CreateFolder(parent string, name string) string {
	return mcservice.CreateFolder(parent, name)
}

func (a *Minecraft) GetLanguageNames() []types.LanguageJson {
	return lang.GetLanguageNames()
}

func (a *Minecraft) GetAppVersion() string { return update.GetAppVersion() }

func (a *Minecraft) GetIsBeta() bool { return update.IsBeta() }

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
	return mcservice.TestMirrorLatencies(urls, timeoutMs)
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
	application.Get().Event.Emit(launch.EventMcLaunchStart, struct{}{})
	_ = vcruntime.EnsureForVersion(a.ctx, dir)
	//_ = preloader.EnsureForVersion(a.ctx, dir)
	//_ = peeditor.EnsureForVersion(a.ctx, dir)
	//_ = peeditor.RunForVersion(a.ctx, dir)

	var args []string
	toRun := exe
	var gameVer string
	var enableConsole bool
	if m, err := versions.ReadMeta(dir); err == nil {
		enableConsole = m.EnableConsole
		p := peeditor.PrepareExecutableForLaunch(a.ctx, dir, m.EnableConsole)
		if strings.TrimSpace(p) != "" {
			toRun = p
		}
		if m.Registered {
			isPreview := strings.EqualFold(strings.TrimSpace(m.Type), "preview")
			protocol := "minecraft://"
			if isPreview {
				protocol = "minecraft-preview://"
			}
			url := protocol
			if m.EnableEditorMode {
				url = protocol + "creator/?Editor=true"
			}

			c := exec.Command("cmd", "/c", "start", "", url)
			if err := c.Start(); err != nil {
				return "ERR_LAUNCH_GAME"
			}
			gameVer = strings.TrimSpace(m.GameVersion)
			discord.SetPlayingVersion(gameVer)
			go launch.MonitorMinecraftWindow(a.ctx)
			return ""
		}
		if m.EnableEditorMode {
			args = []string{"-Editor", "true"}
		}
		gameVer = strings.TrimSpace(m.GameVersion)
	}
	if checkRunning {
		if isProcessRunningAtPath(toRun) {
			return "ERR_GAME_ALREADY_RUNNING"
		}
	}
	cmd := exec.Command(toRun, args...)
	cmd.Dir = filepath.Dir(toRun)
	if enableConsole {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:       false,
			NoInheritHandles: true,
		}
	}
	if err := cmd.Start(); err != nil {
		return "ERR_LAUNCH_GAME"
	}
	discord.SetPlayingVersion(gameVer)
	go launch.MonitorMinecraftWindow(a.ctx)
	return ""
}

func (a *Minecraft) GetBaseRoot() string { return mcservice.GetBaseRoot() }

func (a *Minecraft) SetBaseRoot(root string) string { return mcservice.SetBaseRoot(root) }

func (a *Minecraft) GetDisableDiscordRPC() bool { return mcservice.GetDisableDiscordRPC() }

func (a *Minecraft) SetDisableDiscordRPC(disable bool) string {
	return mcservice.SetDisableDiscordRPC(disable)
}

func (a *Minecraft) ResetBaseRoot() string { return mcservice.ResetBaseRoot() }

func (a *Minecraft) CanWriteToDir(path string) bool { return mcservice.CanWriteToDir(path) }

func (a *Minecraft) ReconcileRegisteredFlags() { mcservice.ReconcileRegisteredFlags() }

func (a *Minecraft) GetCurseForgeGameVersions(gameID string) ([]cursetypes.GameVersion, error) {
	resp, err := a.curseClient.GetGameVersions(a.ctx, gameID)
	if err != nil {
		return nil, err
	}
	var result []cursetypes.GameVersion
	seen := make(map[string]bool)
	for _, dt := range resp.Data {
		for _, v := range dt.Versions {
			if !seen[v.Name] {
				seen[v.Name] = true
				result = append(result, v)
			}
		}
	}
	return cursetypes.GameVersions(result).Sort(), nil
}

func (a *Minecraft) GetCurseForgeCategories(gameID string) ([]cursetypes.Categories, error) {
	resp, err := a.curseClient.GetCategories(a.ctx, gameID)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (a *Minecraft) SearchCurseForgeMods(gameID string, gameVersion string, classID int, categoryIDs []int, searchFilter string, sortField int, sortOrder int, pageSize int, index int) (*cursetypes.ModsResponse, error) {
	var opts []client.ModsQueryOption
	if gameID != "" {
		opts = append(opts, client.WithModsGameID(gameID))
	}
	if gameVersion != "" {
		opts = append(opts, client.WithModsGameVersion(gameVersion))
	}

	if classID > 0 {
		opts = append(opts, client.WithModsClassID(strconv.Itoa(classID)))
	}

	if len(categoryIDs) > 0 {
		var strIDs []string
		for _, id := range categoryIDs {
			strIDs = append(strIDs, strconv.Itoa(id))
		}
		jsonStr := "[" + strings.Join(strIDs, ",") + "]"
		opts = append(opts, client.WithModsCategoryIDs(jsonStr))
	}

	if searchFilter != "" {
		opts = append(opts, client.WithModsSeatchFilter(searchFilter))
	}
	if sortField > 0 {
		opts = append(opts, client.WithModsSortField(sortField))
	}
	if sortOrder == 1 {
		opts = append(opts, client.WithModsSortOrder("asc"))
	} else {
		opts = append(opts, client.WithModsSortOrder("desc"))
	}
	if pageSize > 0 {
		opts = append(opts, client.WithModsPageSize(int64(pageSize)))
	}
	if index > 0 {
		opts = append(opts, client.WithModsIndex(int64(index)))
	}

	return a.curseClient.GetMods(a.ctx, opts...)
}

func (a *Minecraft) GetCurseForgeModsByIDs(modIDs []int64) (*cursetypes.ModsResponse, error) {
	req := &client.GetModsByIdsListRequest{
		ModIds: modIDs,
	}

	return a.curseClient.GetModsByIDs(a.ctx, req)
}

func (a *Minecraft) GetCurseForgeModDescription(modID int64) (*cursetypes.StringResponse, error) {
	return a.curseClient.GetModDescription(a.ctx, modID)
}

func (a *Minecraft) GetCurseForgeModFiles(modID int64) (*cursetypes.GetModFilesResponse, error) {
	return a.curseClient.GetModFiles(a.ctx, modID)
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
