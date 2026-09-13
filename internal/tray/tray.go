package tray

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/launch"
)

var (
	mu     sync.Mutex
	app    *application.App
	handle *application.SystemTray

	showLabel = "Show Launcher"
	exitLabel = "Quit Launcher"
)

// Setup installs the tray icon. The launcher language lives in the frontend, so
// the menu starts out in English and is retranslated by SetLabels as soon as
// the frontend reports which language it renders in.
func Setup(a *application.App, icon []byte) {
	mu.Lock()
	defer mu.Unlock()

	app = a
	handle = a.SystemTray.New()
	handle.SetIcon(icon)
	handle.SetTooltip("LeviLauncher")
	handle.OnClick(launch.RestoreLauncherWindow)
	handle.OnDoubleClick(launch.RestoreLauncherWindow)
	applyMenu()
}

// SetLabels retranslates the tray menu.
func SetLabels(show, exit string) {
	mu.Lock()
	defer mu.Unlock()

	showLabel = show
	exitLabel = exit
	applyMenu()
}

// applyMenu rebuilds the menu from the current labels. Callers hold mu.
func applyMenu() {
	menu := app.NewMenu()
	menu.Add(showLabel).OnClick(func(_ *application.Context) {
		launch.RestoreLauncherWindow()
	})
	menu.AddSeparator()
	menu.Add(exitLabel).OnClick(func(_ *application.Context) {
		launch.QuitLauncher()
	})
	handle.SetMenu(menu)
}
