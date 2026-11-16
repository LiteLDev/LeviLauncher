package app

import (
	"context"
	"errors"
	"github.com/liteldev/LeviLauncher/internal/types"
)

// Keep service bindings available to the shared frontend. Windows does not
// expose this settings panel or use Wine for any existing operation.
func (a *Minecraft) GetWineSettings() (types.WineSettings, error) {
	return types.WineSettings{}, errors.New("Wine settings are only available on macOS/Linux")
}
func (a *Minecraft) SaveWineSettings(s types.WineSettings) error {
	return errors.New("Wine settings are only available on macOS/Linux")
}
func (a *Minecraft) CheckWineSettings(ctx context.Context, s types.WineSettings) (string, error) {
	return "", errors.New("Wine settings are only available on macOS/Linux")
}

func (a *Minecraft) ScanWineInstallations(ctx context.Context) ([]types.WineInstallation, error) {
	return nil, errors.New("Wine settings are only available on macOS/Linux")
}
func (a *Minecraft) GetWineInstallPlan() types.WineInstallPlan { return types.WineInstallPlan{} }
func (a *Minecraft) StartWineInstaller() error {
	return errors.New("Wine installation is only available on macOS")
}
