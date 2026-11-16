//go:build darwin || linux

package main

import (
	"embed"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/app"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log/slog"
	"os"
	"runtime"
	"time"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if err := apppath.EnsureDirs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	update.Init()
	mc := app.NewMinecraft()
	user := app.NewUserService(mc)
	versions := app.NewVersionService(mc)
	icon := appIcon
	if runtime.GOOS == "darwin" {
		// Let macOS load the padded, multi-resolution ICNS from the bundle.
		// Overriding it with the full-bleed Windows PNG makes the Dock icon huge.
		icon = nil
	}
	a := application.New(application.Options{
		Name: "LeviLauncher", LogLevel: slog.LevelWarn,
		Description: "Minecraft Bedrock launcher", Icon: icon,
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Server:         application.ServerOptions{WriteTimeout: 30 * time.Minute},
		Services:       []application.Service{application.NewService(mc), application.NewService(app.NewContentService(mc)), application.NewService(app.NewModsService(mc)), application.NewService(user), application.NewService(versions)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: mc.LocalImageMiddleware},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "org.levimc.levilauncher.native"},
	})
	mc.StartupEssential()
	w := a.Window.NewWithOptions(application.WebviewWindowOptions{Name: "main", Title: "LeviLauncher", Width: 1180, Height: 780, MinWidth: 960, MinHeight: 600, URL: "/", EnableFileDrop: true})
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if launch.CloseBlocked() && !launch.QuitRequested() {
			event.Cancel()
		}
	})
	user.Attach(w)
	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
