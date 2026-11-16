//go:build darwin || linux

package app

import (
	"context"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func (a *Minecraft) GetWineSettings() (types.WineSettings, error) { return gameruntime.LoadSettings() }
func (a *Minecraft) SaveWineSettings(s types.WineSettings) error  { return gameruntime.SaveSettings(s) }
func (a *Minecraft) CheckWineSettings(ctx context.Context, s types.WineSettings) (string, error) {
	return gameruntime.CheckSettings(ctx, s)
}
