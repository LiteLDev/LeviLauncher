//go:build darwin || linux

package lip

import (
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/lipcore/engine"
)

func configureHostEngine(options *engine.Options) {
	options.Platform = "win-x64"
	options.RunCommand = gameruntime.RunHook
}
