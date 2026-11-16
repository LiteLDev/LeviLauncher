//go:build darwin || linux

package vcruntime

import (
	"context"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

func IsInstalled() bool {
	return gameruntime.HasSystemDLL("vcruntime140.dll") && gameruntime.HasSystemDLL("msvcp140.dll")
}
func EnsureInteractive(ctx context.Context) {
	if !IsInstalled() {
		eventsink.From(ctx).Emit(EventMissing, "Configure the selected Wine environment's Windows VC runtime before launching the game")
	}
}
func EnsureStartupInteractive(context.Context) bool { return IsInstalled() }
