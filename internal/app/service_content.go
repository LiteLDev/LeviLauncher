package app

import (
	"github.com/liteldev/LeviLauncher/internal/mcservice"
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

func (s *ContentService) TransferPackToVersion(sourceVersionName string, sourcePackPath string, targetVersionName string, overwrite bool) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.TransferPackToVersion(sourceVersionName, sourcePackPath, targetVersionName, overwrite)
}

func (s *ContentService) TransferWorldToVersion(sourceVersionName string, sourcePlayer string, sourceWorldPath string, targetVersionName string, targetPlayer string) string {
	if s.manager == nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	return s.manager.TransferWorldToVersion(sourceVersionName, sourcePlayer, sourceWorldPath, targetVersionName, targetPlayer)
}

func (s *ContentService) GetPackInfo(dir string) types.PackInfo {
	if s.manager == nil {
		return types.PackInfo{}
	}
	return s.manager.GetPackInfo(dir)
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
