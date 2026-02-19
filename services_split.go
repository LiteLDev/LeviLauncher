package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/mods"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
)

type ContentService struct {
	manager contentService
}

func NewContentService(mc *Minecraft) *ContentService {
	s := &ContentService{}
	if mc != nil {
		s.manager = mc.contentManager
	}
	return s
}

func (s *ContentService) GetContentRoots(name string) types.ContentRoots {
	return mcservice.GetContentRoots(name)
}

func (s *ContentService) GetContentCounts(name string) ContentCounts {
	c := mcservice.GetContentCounts(name)
	return ContentCounts{
		Worlds:        c.Worlds,
		ResourcePacks: c.ResourcePacks,
		BehaviorPacks: c.BehaviorPacks,
	}
}

func (s *ContentService) ListPacksForVersion(versionName string, player string) []packages.Pack {
	if s.manager == nil {
		return []packages.Pack{}
	}
	return s.manager.ListPacksForVersion(versionName, player)
}

func (s *ContentService) ImportMcpack(name string, data []byte, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcpack(name, data, overwrite)
}

func (s *ContentService) ImportMcpackPath(name string, path string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcpackPath(name, path, overwrite)
}

func (s *ContentService) ImportMcaddon(name string, data []byte, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcaddon(name, data, overwrite)
}

func (s *ContentService) ImportMcaddonPath(name string, path string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcaddonPath(name, path, overwrite)
}

func (s *ContentService) ImportMcaddonWithPlayer(name string, player string, data []byte, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcaddonWithPlayer(name, player, data, overwrite)
}

func (s *ContentService) ImportMcaddonPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcaddonPathWithPlayer(name, player, path, overwrite)
}

func (s *ContentService) ImportMcpackWithPlayer(name string, player string, fileName string, data []byte, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcpackWithPlayer(name, player, fileName, data, overwrite)
}

func (s *ContentService) ImportMcpackPathWithPlayer(name string, player string, path string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcpackPathWithPlayer(name, player, path, overwrite)
}

func (s *ContentService) IsMcpackSkinPackPath(path string) bool {
	if s.manager == nil {
		return false
	}
	return s.manager.IsMcpackSkinPackPath(path)
}

func (s *ContentService) IsMcpackSkinPack(data []byte) bool {
	if s.manager == nil {
		return false
	}
	return s.manager.IsMcpackSkinPack(data)
}

func (s *ContentService) ImportMcworld(name string, player string, fileName string, data []byte, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcworld(name, player, fileName, data, overwrite)
}

func (s *ContentService) ImportMcworldPath(name string, player string, path string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.ImportMcworldPath(name, player, path, overwrite)
}

func (s *ContentService) GetPackInfo(dir string) types.PackInfo {
	if s.manager == nil {
		return types.PackInfo{}
	}
	return s.manager.GetPackInfo(dir)
}

func (s *ContentService) UpdateResourcePackMaterialBins(versionName string, packPath string) ResourcePackMaterialUpdateResult {
	if s.manager == nil {
		return ResourcePackMaterialUpdateResult{Error: "ERR_ACCESS_VERSIONS_DIR"}
	}
	result := s.manager.UpdateResourcePackMaterialBins(versionName, packPath)
	return ResourcePackMaterialUpdateResult{
		HasMaterialBin: result.HasMaterialBin,
		TotalCount:     result.TotalCount,
		UpdatedCount:   result.UpdatedCount,
		SkippedCount:   result.SkippedCount,
		FailedCount:    result.FailedCount,
		Error:          result.Error,
	}
}

func (s *ContentService) CheckResourcePackMaterialCompatibility(versionName string, packPath string) ResourcePackMaterialCompatResult {
	if s.manager == nil {
		return ResourcePackMaterialCompatResult{Error: "ERR_ACCESS_VERSIONS_DIR"}
	}
	result := s.manager.CheckResourcePackMaterialCompatibility(versionName, packPath)
	return ResourcePackMaterialCompatResult{
		HasMaterialBin:      result.HasMaterialBin,
		Compatible:          result.Compatible,
		NeedsUpdate:         result.NeedsUpdate,
		PackMaterialPath:    result.PackMaterialPath,
		PackMaterialVersion: result.PackMaterialVersion,
		GameMaterialPath:    result.GameMaterialPath,
		GameMaterialVersion: result.GameMaterialVersion,
		Error:               result.Error,
	}
}

func (s *ContentService) DeletePack(name string, path string) string {
	if s.manager == nil {
		return "ERR_INVALID_PATH"
	}
	return s.manager.DeletePack(name, path)
}

func (s *ContentService) DeleteWorld(name string, path string) string {
	if s.manager == nil {
		return "ERR_INVALID_PATH"
	}
	return s.manager.DeleteWorld(name, path)
}

func (s *ContentService) ListScreenshots(versionName string, player string) []ScreenshotInfo {
	if s.manager == nil {
		return []ScreenshotInfo{}
	}
	mgr := s.manager.ListScreenshots(versionName, player)
	out := make([]ScreenshotInfo, len(mgr))
	for i, v := range mgr {
		out[i] = ScreenshotInfo{
			Name:        v.Name,
			Path:        v.Path,
			Dir:         v.Dir,
			CaptureTime: v.CaptureTime,
		}
	}
	return out
}

func (s *ContentService) DeleteScreenshot(versionName string, player string, path string) string {
	if s.manager == nil {
		return "ERR_INVALID_PATH"
	}
	return s.manager.DeleteScreenshot(versionName, player, path)
}

type ModsService struct{}

func NewModsService(_ *Minecraft) *ModsService {
	return &ModsService{}
}

func (s *ModsService) GetMods(name string) []types.ModInfo {
	return mods.GetMods(name)
}

func (s *ModsService) ImportModZip(name string, data []byte, overwrite bool) string {
	return mods.ImportZipToMods(name, data, overwrite)
}

func (s *ModsService) ImportModZipPath(name string, path string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_OPEN_ZIP"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_OPEN_ZIP"
	}
	return mods.ImportZipToMods(name, b, overwrite)
}

func (s *ModsService) ImportModDll(name string, fileName string, data []byte, modName string, modType string, version string, overwrite bool) string {
	return mods.ImportDllToMods(name, fileName, data, modName, modType, version, overwrite)
}

func (s *ModsService) ImportModDllPath(name string, path string, modName string, modType string, version string, overwrite bool) string {
	if strings.TrimSpace(path) == "" {
		return "ERR_WRITE_FILE"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "ERR_WRITE_FILE"
	}
	return mods.ImportDllToMods(name, filepath.Base(path), b, modName, modType, version, overwrite)
}

func (s *ModsService) DeleteMod(name string, modName string) string {
	return mods.DeleteMod(name, modName)
}

func (s *ModsService) DisableMod(name string, modName string) string {
	return mods.DisableMod(name, modName)
}

func (s *ModsService) EnableMod(name string, modName string) string {
	return mods.EnableMod(name, modName)
}

func (s *ModsService) IsModEnabled(name string, modName string) bool {
	return mods.IsModEnabled(name, modName)
}

func (s *ModsService) OpenModsExplorer(name string) {
	mcservice.OpenModsExplorer(name)
}
