package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/gdk"
	"github.com/liteldev/LeviLauncher/internal/launchercore"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

type UserService struct{}

func NewUserService(_ *Minecraft) *UserService {
	return &UserService{}
}

func (s *UserService) GetGamertagByXuid(xuidStr string) string {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return ""
	}
	tag, err := launchercore.GetGamertagByXuid(xuid)
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserId() string {
	id, err := launchercore.GetLocalUserId()
	if err != nil {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

func (s *UserService) GetLocalUserGamertag() string {
	tag, err := launchercore.GetLocalUserGamertag()
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserGamerPicture(size int) string {
	bin, err := launchercore.GetLocalUserGamerPicture(size)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(bin)
}

func (s *UserService) GetUserGamertagMap(usersRoot string) map[string]string {
	usersRoot = strings.TrimSpace(usersRoot)
	if usersRoot == "" {
		return map[string]string{}
	}

	cachePath := filepath.Join(config.ConfigDir(), "user_gamertag_map.json")
	out := map[string]string{}
	loadMap := func(p string) {
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			return
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			return
		}
		for k, v := range m {
			kk := strings.TrimSpace(k)
			vv := strings.TrimSpace(v)
			if kk == "" || vv == "" {
				continue
			}
			if strings.TrimSpace(out[kk]) == "" {
				out[kk] = vv
			}
		}
	}

	loadMap(cachePath)

	entries, err := os.ReadDir(usersRoot)
	if err != nil {
		return out
	}

	changed := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		obf := strings.TrimSpace(e.Name())
		if obf == "" || strings.EqualFold(obf, "shared") || obf == "9556213259376595538" {
			continue
		}
		if v := strings.TrimSpace(out[obf]); v != "" {
			continue
		}

		ss := filepath.Join(usersRoot, obf, "games", "com.mojang", "Screenshots")
		xuidDirs, err := os.ReadDir(ss)
		if err != nil {
			continue
		}

		var xuidStr string
		for _, xd := range xuidDirs {
			if xd.IsDir() {
				xuidStr = strings.TrimSpace(xd.Name())
				if xuidStr != "" {
					break
				}
			}
		}
		if xuidStr == "" {
			continue
		}

		xuid, err := strconv.ParseUint(xuidStr, 10, 64)
		if err != nil {
			continue
		}

		raw, err := launchercore.GetGamertagByXuid(xuid)
		if err != nil {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		parts := strings.SplitN(raw, "|", 2)
		gamertag := strings.TrimSpace(parts[0])
		if gamertag == "" {
			continue
		}

		key := obf
		if len(parts) == 2 {
			if k := strings.TrimSpace(parts[1]); k != "" {
				key = k
			}
		}

		if strings.TrimSpace(out[key]) != gamertag {
			out[key] = gamertag
			changed = true
		}
	}

	if changed {
		if b, err := json.Marshal(out); err == nil {
			_ = os.WriteFile(cachePath, b, 0644)
		}
	}

	return out
}

func (s *UserService) ResetSession() string {
	if err := launchercore.ResetSession(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *UserService) XUserGetState() int {
	state, err := launchercore.XUserGetState()
	if err != nil {
		return 1
	}
	return int(state)
}

type UserStatistics struct {
	MinutesPlayed     int64   `json:"minutesPlayed"`
	BlockBroken       int64   `json:"blockBroken"`
	MobKilled         int64   `json:"mobKilled"`
	DistanceTravelled float64 `json:"distanceTravelled"`
}

func (s *UserService) GetAggregatedUserStatistics(xuidStr string) UserStatistics {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return UserStatistics{}
	}
	mp, bb, mk, dt, err := launchercore.GetAggregatedUserStatisticsByXuid(xuid)
	if err != nil {
		return UserStatistics{}
	}
	return UserStatistics{
		MinutesPlayed:     mp,
		BlockBroken:       bb,
		MobKilled:         mk,
		DistanceTravelled: dt,
	}
}

// normalizePath lowercases, cleans, and strips UNC prefixes and trailing
// separators so that paths from the Windows registry (which may carry \\?\
// or \??\ prefixes) compare equal to regular paths.
func normalizePath(p string) string {
	norm := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	norm = strings.TrimPrefix(norm, `\\?\`)
	norm = strings.TrimPrefix(norm, `\??\`)
	return strings.TrimSuffix(norm, string(os.PathSeparator))
}

type VersionService struct {
	launcher    launchService
	ctxProvider func() context.Context
}

func NewVersionService(mc *Minecraft) *VersionService {
	if mc == nil {
		return &VersionService{}
	}
	return &VersionService{
		launcher: mc.launcher,
		// NOTE: mc.ctx is set later in mc.startup(), so ctxProvider may return
		// nil before startup completes. launchContext() falls back to
		// context.Background() in that case, which means Wails app-level
		// context (events, etc.) won't be available for early callers.
		ctxProvider: func() context.Context {
			return mc.ctx
		},
	}
}

func (s *VersionService) launchContext() context.Context {
	if s.ctxProvider != nil {
		if ctx := s.ctxProvider(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (s *VersionService) LaunchVersionByName(name string) string {
	if s.launcher == nil {
		return "ERR_LAUNCH_GAME"
	}
	return s.launcher.Launch(s.launchContext(), name, true)
}

func (s *VersionService) LaunchVersionByNameForce(name string) string {
	if s.launcher == nil {
		return "ERR_LAUNCH_GAME"
	}
	return s.launcher.Launch(s.launchContext(), name, false)
}

func (s *VersionService) CreateDesktopShortcut(name string) string {
	return mcservice.CreateDesktopShortcut(name)
}

func (s *VersionService) SaveVersionMeta(name string, gameVersion string, typeStr string, enableIsolation bool, enableConsole bool, enableEditorMode bool, enableRenderDragon bool, enableCtrlRReloadResources bool, launchArgs string, envVars string) string {
	return mcservice.SaveVersionMeta(name, gameVersion, typeStr, enableIsolation, enableConsole, enableEditorMode, enableRenderDragon, enableCtrlRReloadResources, launchArgs, envVars)
}

func (s *VersionService) ListVersionMetas() []versions.VersionMeta {
	return mcservice.ListVersionMetas()
}

func (s *VersionService) ListVersionMetasWithRegistered() []versions.VersionMeta {
	metas := mcservice.ListVersionMetas()
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return metas
	}
	var releaseLoc, previewLoc string
	if info, e := registry.GetAppxInfo("MICROSOFT.MINECRAFTUWP"); e == nil && info != nil {
		releaseLoc = normalizePath(info.InstallLocation)
	}
	if info, e := registry.GetAppxInfo("Microsoft.MinecraftWindowsBeta"); e == nil && info != nil {
		previewLoc = normalizePath(info.InstallLocation)
	}
	for i := range metas {
		m := &metas[i]
		isPreview := strings.EqualFold(strings.TrimSpace(m.Type), "preview")
		norm := normalizePath(filepath.Join(vdir, strings.TrimSpace(m.Name)))
		if isPreview {
			m.Registered = previewLoc != "" && norm == previewLoc
		} else {
			m.Registered = releaseLoc != "" && norm == releaseLoc
		}
	}
	return metas
}

func (s *VersionService) GetVersionMeta(name string) versions.VersionMeta {
	return mcservice.GetVersionMeta(name)
}

func (s *VersionService) ListInheritableVersionNames(versionType string) []string {
	return mcservice.ListInheritableVersionNames(versionType)
}

func (s *VersionService) CopyVersionDataFromVersion(sourceName string, targetName string) string {
	return mcservice.CopyVersionDataFromVersion(sourceName, targetName)
}

func (s *VersionService) CopyVersionDataFromGDK(isPreview bool, targetName string) string {
	return mcservice.CopyVersionDataFromGDK(isPreview, targetName)
}

func (s *VersionService) SaveVersionLogoDataUrl(name string, dataUrl string) string {
	return mcservice.SaveVersionLogoDataUrl(name, dataUrl)
}

func (s *VersionService) SaveVersionLogoFromPath(name string, filePath string) string {
	return mcservice.SaveVersionLogoFromPath(name, filePath)
}

func (s *VersionService) GetVersionLogoDataUrl(name string) string {
	return mcservice.GetVersionLogoDataUrl(name)
}

func (s *VersionService) RemoveVersionLogo(name string) string {
	return mcservice.RemoveVersionLogo(name)
}

func (s *VersionService) ValidateVersionFolderName(name string) string {
	return mcservice.ValidateVersionFolderName(name)
}

func (s *VersionService) RenameVersionFolder(oldName string, newName string) string {
	return mcservice.RenameVersionFolder(oldName, newName)
}

func (s *VersionService) DeleteVersionFolder(name string) string {
	return mcservice.DeleteVersionFolder(name)
}

func (s *VersionService) RegisterVersionWithWdapp(name string, isPreview bool) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	folder := filepath.Join(vdir, strings.TrimSpace(name))
	_ = gdk.UnregisterIfExists(isPreview)
	msg := gdk.RegisterVersionFolder(folder)
	if msg != "" {
		return msg
	}
	pkg := "MICROSOFT.MINECRAFTUWP"
	if isPreview {
		pkg = "Microsoft.MinecraftWindowsBeta"
	}
	if info, e := registry.GetAppxInfo(pkg); e == nil && info != nil {
		expected := normalizePath(filepath.Join(vdir, strings.TrimSpace(name)))
		loc := normalizePath(info.InstallLocation)
		flag := loc != "" && loc == expected
		if m, er := versions.ReadMeta(folder); er == nil {
			m.Registered = flag
			_ = versions.WriteMeta(folder, m)
		}
	}
	mcservice.ReconcileRegisteredFlags()
	return msg
}

func (s *VersionService) UnregisterVersionByName(name string) string {
	vdir, err := apppath.VersionsDir()
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

func (s *VersionService) GetInstallerDir() string {
	return mcservice.GetInstallerDir()
}

func (s *VersionService) GetVersionsDir() string {
	return mcservice.GetVersionsDir()
}

func (s *VersionService) GetVersionStatus(version string, versionType string) VersionStatus {
	st := mcservice.GetVersionStatus(version, versionType)
	return VersionStatus{
		Version:      st.Version,
		IsInstalled:  st.IsInstalled,
		IsDownloaded: st.IsDownloaded,
		Type:         st.Type,
	}
}

func (s *VersionService) GetAllVersionsStatus(versionsData []map[string]interface{}) []VersionStatus {
	raw := mcservice.GetAllVersionsStatus(versionsData)
	out := make([]VersionStatus, 0, len(raw))
	for _, st := range raw {
		out = append(out, VersionStatus{
			Version:      st.Version,
			IsInstalled:  st.IsInstalled,
			IsDownloaded: st.IsDownloaded,
			Type:         st.Type,
		})
	}
	return out
}

func (s *VersionService) ReconcileRegisteredFlags() {
	mcservice.ReconcileRegisteredFlags()
}
