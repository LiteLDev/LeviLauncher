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
