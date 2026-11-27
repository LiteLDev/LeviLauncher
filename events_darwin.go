package main

import (
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	application.RegisterEvent[string](launch.EventMcLaunchPrepare)
	application.RegisterEvent[struct{}](launch.EventMcLaunchProcessReady)
}
