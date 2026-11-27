//go:build darwin

package app

import (
	"context"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
)

func (a *Minecraft) GetWineSettings() (types.WineSettings, error) { return gameruntime.LoadSettings() }
func (a *Minecraft) SaveWineSettings(s types.WineSettings) error  { return gameruntime.SaveSettings(s) }
func (a *Minecraft) CheckWineSettings(ctx context.Context, s types.WineSettings) (string, error) {
	return gameruntime.CheckSettings(ctx, s)
}

func (a *Minecraft) ScanWineInstallations(ctx context.Context) ([]types.WineInstallation, error) {
	return gameruntime.Scan(ctx)
}
func (a *Minecraft) GetWineDownloadSettings() (types.WineDownloadSettings, error) {
	return gameruntime.LoadDownloadSettings()
}
func (a *Minecraft) SaveWineDownloadSettings(settings types.WineDownloadSettings) error {
	return gameruntime.SaveDownloadSettings(settings)
}
func (a *Minecraft) WineNeedsRosetta(ctx context.Context) bool {
	return gameruntime.NeedsRosetta(ctx)
}
func (a *Minecraft) StartWineInstaller(ctx context.Context, consent bool) (types.WineSettings, error) {
	return gameruntime.InstallManagedWine(ctx, consent, func(progress types.WineInstallProgress) {
		eventsink.From(ctx).Emit("wine.install.progress", progress)
	}, versionlaunch.PrepareWine)
}
func (a *Minecraft) GetLegacyWineInfo() (types.LegacyWineInfo, error) {
	return gameruntime.GetLegacyWineInfo()
}
func (a *Minecraft) RemoveLegacyWineRuntime() error {
	return gameruntime.RemoveLegacyWineRuntime()
}
func (a *Minecraft) GetWineShaderCacheInfo() (types.WineShaderCacheInfo, error) {
	return gameruntime.GetWineShaderCacheInfo()
}
func (a *Minecraft) ClearWineShaderCache() error {
	return gameruntime.ClearWineShaderCache()
}
func (a *Minecraft) KillWineProcesses(ctx context.Context) error {
	return gameruntime.KillWineProcesses(ctx)
}
