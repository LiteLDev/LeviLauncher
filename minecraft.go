package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/liteldev/LeviLauncher/internal/contentmgr"
	"github.com/liteldev/LeviLauncher/internal/curseforge/client"
	cursetypes "github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/gdk"
	"github.com/liteldev/LeviLauncher/internal/lang"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/lip"
	lipclient "github.com/liteldev/LeviLauncher/internal/lip/client"
	liptypes "github.com/liteldev/LeviLauncher/internal/lip/client/types"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed assets/curseforge.key
var curseForgeApiKey []byte

const (
	EventGameInputEnsureStart      = "gameinput.ensure.start"
	EventGameInputEnsureDone       = "gameinput.ensure.done"
	EventGameInputDownloadStart    = "gameinput.download.start"
	EventGameInputDownloadProgress = "gameinput.download.progress"
	EventGameInputDownloadDone     = "gameinput.download.done"
	EventGameInputDownloadError    = "gameinput.download.error"

	EventFileDownloadStatus   = "file.download.status"
	EventFileDownloadProgress = "file.download.progress"
	EventFileDownloadDone     = "file.download.done"
	EventFileDownloadError    = "file.download.error"

	EventVcRuntimeEnsureStart      = "vcruntime.ensure.start"
	EventVcRuntimeEnsureDone       = "vcruntime.ensure.done"
	EventVcRuntimeDownloadStart    = "vcruntime.download.start"
	EventVcRuntimeDownloadProgress = "vcruntime.download.progress"
	EventVcRuntimeDownloadDone     = "vcruntime.download.done"
	EventVcRuntimeDownloadError    = "vcruntime.download.error"
	EventVcRuntimeMissing          = "vcruntime.missing"
)

var isPreloader = false

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
	return fileDownloader.Start(a.ctx, url, dest, "")
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

func (a *Minecraft) FetchLeviLaminaVersionDB() map[string][]string {
	res, err := mcservice.FetchLeviLaminaVersionDB()
	if err != nil {
		return map[string][]string{}
	}
	return res
}

func (a *Minecraft) InstallLeviLamina(mcVersion string, targetName string) string {
	return mcservice.InstallLeviLamina(a.ctx, mcVersion, targetName)
}

func (a *Minecraft) UninstallLeviLamina(targetName string) string {
	return mcservice.UninstallLeviLamina(a.ctx, targetName)
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

type ResourcePackMaterialCompatResult struct {
	HasMaterialBin      bool   `json:"hasMaterialBin"`
	Compatible          bool   `json:"compatible"`
	NeedsUpdate         bool   `json:"needsUpdate"`
	PackMaterialPath    string `json:"packMaterialPath"`
	PackMaterialVersion uint64 `json:"packMaterialVersion"`
	GameMaterialPath    string `json:"gameMaterialPath"`
	GameMaterialVersion uint64 `json:"gameMaterialVersion"`
	Error               string `json:"error"`
}

type ResourcePackMaterialUpdateResult struct {
	HasMaterialBin bool   `json:"hasMaterialBin"`
	TotalCount     int    `json:"totalCount"`
	UpdatedCount   int    `json:"updatedCount"`
	SkippedCount   int    `json:"skippedCount"`
	FailedCount    int    `json:"failedCount"`
	Error          string `json:"error"`
}

type ScreenshotInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Dir         string `json:"dir"`
	CaptureTime int64  `json:"captureTime"`
}

func (a *Minecraft) ListServers(versionName string, player string) []types.Server {
	servers, _ := mcservice.ListServers(versionName, player)
	return servers
}

func (a *Minecraft) PingServer(host string) *mcservice.MotdBEInfo {
	info, _ := mcservice.MotdBE(host)
	return info
}

func (a *Minecraft) GetImageBase64(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	mime := http.DetectContentType(b)
	enc := base64.StdEncoding.EncodeToString(b)
	return "data:" + mime + ";base64," + enc
}

type Minecraft struct {
	ctx            context.Context
	curseClient    client.CurseClient
	lipClient      lipService
	launcher       launchService
	contentManager contentService
}

type lipService interface {
	SearchPackages(q string, perPage int, page int, sort string, order string) (*liptypes.SearchPackagesResponse, error)
	GetPackage(identifier string) (*liptypes.GetPackageResponse, error)
}

type packLoader interface {
	LoadPacksForVersion(versionName string, resourcePacksDir string, behaviorPacksDir string, skinPacksDirs ...string) ([]packages.Pack, error)
}

type launchService interface {
	Launch(ctx context.Context, versionName string, checkRunning bool) string
}

type contentService interface {
	ListPacksForVersion(versionName string, player string) []packages.Pack
	ImportMcpack(name string, data []byte, overwrite bool) string
	ImportMcpackPath(name string, path string, overwrite bool) string
	ImportMcaddon(name string, data []byte, overwrite bool) string
	ImportMcaddonPath(name string, path string, overwrite bool) string
	ImportMcaddonWithPlayer(name string, player string, data []byte, overwrite bool) string
	ImportMcaddonPathWithPlayer(name string, player string, path string, overwrite bool) string
	ImportMcpackWithPlayer(name string, player string, fileName string, data []byte, overwrite bool) string
	ImportMcpackPathWithPlayer(name string, player string, path string, overwrite bool) string
	IsMcpackSkinPackPath(path string) bool
	IsMcpackSkinPack(data []byte) bool
	ImportMcworld(name string, player string, fileName string, data []byte, overwrite bool) string
	ImportMcworldPath(name string, player string, path string, overwrite bool) string
	GetPackInfo(dir string) types.PackInfo
	UpdateResourcePackMaterialBins(versionName string, packPath string) contentmgr.MaterialUpdateResult
	CheckResourcePackMaterialCompatibility(versionName string, packPath string) contentmgr.MaterialCompatResult
	DeletePack(name string, path string) string
	DeleteWorld(name string, path string) string
	ListScreenshots(versionName string, player string) []contentmgr.ScreenshotInfo
	DeleteScreenshot(versionName string, player string, path string) string
}

type MinecraftDeps struct {
	CurseClient    client.CurseClient
	LIPClient      lipService
	PackManager    packLoader
	Launcher       launchService
	ContentManager contentService
}

func NewMinecraft() *Minecraft {
	return NewMinecraftWithDeps(MinecraftDeps{})
}

func NewMinecraftWithDeps(deps MinecraftDeps) *Minecraft {
	curseClient := deps.CurseClient
	if curseClient == nil {
		curseClient = client.NewCurseClient(resolveCurseForgeAPIKey())
	}
	lipClient := deps.LIPClient
	if lipClient == nil {
		lipClient = lipclient.NewClient()
	}
	packManager := deps.PackManager
	if packManager == nil {
		packManager = packages.NewPackManager()
	}
	launcher := deps.Launcher
	if launcher == nil {
		launcher = versionlaunch.New(isPreloader)
	}
	contentManager := deps.ContentManager
	if contentManager == nil {
		contentManager = contentmgr.New(contentmgr.Deps{
			PackLoader:      packManager,
			GetContentRoots: mcservice.GetContentRoots,
			GetVersionGameInfo: func(name string) string {
				return mcservice.GetVersionMeta(name).GameVersion
			},
			ListDir: mcservice.ListDir,
		})
	}
	return &Minecraft{
		curseClient:    curseClient,
		lipClient:      lipClient,
		launcher:       launcher,
		contentManager: contentManager,
	}
}

func resolveCurseForgeAPIKey() string {
	apiKey := os.Getenv("CURSEFORGE_API_KEY")
	if apiKey == "" {
		deobfuscated := make([]byte, len(curseForgeApiKey))
		for i, b := range curseForgeApiKey {
			deobfuscated[i] = b ^ 0xAF
		}

		decoded, err := base64.StdEncoding.DecodeString(string(deobfuscated))
		if err == nil {
			apiKey = string(decoded)
		}
	}
	return apiKey
}

func (a *Minecraft) SearchLIPPackages(q string, perPage int, page int, sort string, order string) (*liptypes.SearchPackagesResponse, error) {
	return a.lipClient.SearchPackages(q, perPage, page, sort, order)
}

func (a *Minecraft) GetLIPPackage(identifier string) (*liptypes.GetPackageResponse, error) {
	return a.lipClient.GetPackage(identifier)
}

func (a *Minecraft) InstallLip() string {
	return lip.Install()
}

func (a *Minecraft) GetLipVersion() string {
	return lip.GetVersion()
}

func (a *Minecraft) IsLipInstalled() bool {
	return lip.IsInstalled()
}

func (a *Minecraft) GetLatestLipVersion() (string, error) {
	return lip.GetLatestVersion()
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

func (a *Minecraft) EnsureVcRuntimeInteractive() { go vcruntime.EnsureInteractive(a.ctx) }

func (a *Minecraft) IsVcRuntimeInstalled() bool { return vcruntime.IsInstalled() }

func (a *Minecraft) IsGamingServicesInstalled() bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err == nil {
		return true
	}
	return false
}

func (a *Minecraft) StartMsixvcDownload(url string, md5sum string) string {
	return mcservice.StartMsixvcDownload(a.ctx, url, md5sum)
}
func (a *Minecraft) ResumeMsixvcDownload() { mcservice.ResumeMsixvcDownload() }
func (a *Minecraft) CancelMsixvcDownload() { mcservice.CancelMsixvcDownload() }
func (a *Minecraft) CancelMsixvcDownloadTask(dest string) {
	mcservice.CancelMsixvcDownloadTask(dest)
}

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

func (a *Minecraft) OpenWorldsExplorer(name string, isPreview bool) {
	mcservice.OpenWorldsExplorer(name, isPreview)
}

func (a *Minecraft) OpenPathDir(dir string) { mcservice.OpenPathDir(dir) }

func (a *Minecraft) OpenGameDataExplorer(isPreview bool) { mcservice.OpenGameDataExplorer(isPreview) }

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

func (a *Minecraft) GetBaseRoot() string { return mcservice.GetBaseRoot() }

func (a *Minecraft) SetBaseRoot(root string) string { return mcservice.SetBaseRoot(root) }

func (a *Minecraft) GetDisableDiscordRPC() bool { return mcservice.GetDisableDiscordRPC() }

func (a *Minecraft) SetDisableDiscordRPC(disable bool) string {
	return mcservice.SetDisableDiscordRPC(disable)
}

func (a *Minecraft) GetEnableBetaUpdates() bool { return mcservice.GetEnableBetaUpdates() }

func (a *Minecraft) SetEnableBetaUpdates(enable bool) string {
	return mcservice.SetEnableBetaUpdates(enable)
}

func (a *Minecraft) ResetBaseRoot() string { return mcservice.ResetBaseRoot() }

func (a *Minecraft) CanWriteToDir(path string) bool { return mcservice.CanWriteToDir(path) }

func (a *Minecraft) GetSunTimes() mcservice.SunTimes {
	return mcservice.GetSunTimes()
}

func (a *Minecraft) ListMinecraftProcesses() []types.ProcessInfo {
	return mcservice.ListMinecraftProcesses()
}

func (a *Minecraft) KillProcess(pid int) string {
	err := mcservice.KillProcess(pid)
	if err != nil {
		return err.Error()
	}
	return ""
}

func (a *Minecraft) KillAllMinecraftProcesses() string {
	err := mcservice.KillAllMinecraftProcesses()
	if err != nil {
		return err.Error()
	}
	return ""
}

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
