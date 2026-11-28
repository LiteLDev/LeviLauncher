package app

import (
	"context"
	"errors"
	"github.com/liteldev/LeviLauncher/internal/types"
)

// Keep service bindings available to the shared frontend. Windows does not
// expose this settings panel or use Wine for any existing operation.
func (a *Minecraft) GetWineSettings() (types.WineSettings, error) {
	return types.WineSettings{}, errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) SaveWineSettings(s types.WineSettings) error {
	return errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) CheckWineSettings(ctx context.Context, s types.WineSettings) (string, error) {
	return "", errors.New("Wine settings are only available on macOS")
}

func (a *Minecraft) ScanWineInstallations(ctx context.Context) ([]types.WineInstallation, error) {
	return nil, errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) GetWineDownloadSettings() (types.WineDownloadSettings, error) {
	return types.WineDownloadSettings{}, errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) SaveWineDownloadSettings(types.WineDownloadSettings) error {
	return errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) WineNeedsRosetta(context.Context) bool { return false }
func (a *Minecraft) StartWineInstaller(context.Context, bool) (types.WineSettings, error) {
	return types.WineSettings{}, errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) InstallWineVersion(context.Context, bool, string) (types.WineSettings, error) {
	return types.WineSettings{}, errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) ListWineRuntimeReleases(context.Context) ([]types.WineRuntimeRelease, error) {
	return nil, errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) ListInstalledWineVersions() ([]types.ManagedWineVersion, error) {
	return nil, errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) SwitchWineVersion(context.Context, string) (types.WineSettings, error) {
	return types.WineSettings{}, errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) RemoveWineVersion(string) error {
	return errors.New("Wine installation is only available on macOS")
}
func (a *Minecraft) GetLegacyWineInfo() (types.LegacyWineInfo, error) {
	return types.LegacyWineInfo{}, nil
}
func (a *Minecraft) RemoveLegacyWineRuntime() error {
	return errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) GetWineShaderCacheInfo() (types.WineShaderCacheInfo, error) {
	return types.WineShaderCacheInfo{}, nil
}
func (a *Minecraft) ClearWineShaderCache() error {
	return errors.New("Wine settings are only available on macOS")
}
func (a *Minecraft) KillWineProcesses(context.Context) error {
	return errors.New("Wine settings are only available on macOS")
}
