package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/explorer"
	"github.com/liteldev/LeviLauncher/internal/mods"
	"github.com/liteldev/LeviLauncher/internal/types"
)

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

func (s *ModsService) UpdateModManifest(name string, modFolder string, modName string, version string, modType string, entry string, author string) string {
	return mods.UpdateModManifest(name, modFolder, modName, version, modType, entry, author)
}

func (s *ModsService) OpenModsExplorer(name string) error {
	return explorer.OpenMods(name)
}
