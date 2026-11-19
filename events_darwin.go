package main

import (
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	application.RegisterEvent[types.WineInstallProgress]("wine.install.progress")
}
