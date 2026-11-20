package main

import (
	"embed"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/app"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/gameauth"
	"github.com/liteldev/LeviLauncher/internal/gameconsole"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"io"
	"log"
	"log/slog"
	"os"
	"time"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--attach-console" {
		os.Exit(gameconsole.RunCLI(os.Args[2:]))
	}
	if err := apppath.EnsureDirs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Finder-launched apps have no terminal; retain backend errors locally so
	// authentication failures can be diagnosed without logging credentials.
	if file, err := os.OpenFile(apppath.StartupLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
		defer file.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, file))
	}
	if _, err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	update.Init()
	mc := app.NewMinecraft()
	user := app.NewUserService(mc)
	versions := app.NewVersionService(mc)
	// macOS loads the Dock icon from the app bundle.
	a := application.New(application.Options{
		Name: "LeviLauncher", LogLevel: slog.LevelWarn,
		Description:    "Minecraft Bedrock launcher",
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Server:         application.ServerOptions{WriteTimeout: 30 * time.Minute},
		Services:       []application.Service{application.NewService(mc), application.NewService(app.NewContentService(mc)), application.NewService(app.NewModsService(mc)), application.NewService(user), application.NewService(versions), application.NewService(app.NewDiagnosticsService())},
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
	if broker, err := gameauth.Start(func(state gameauth.State) { a.Event.Emit(gameauth.Event, state) }, a.Browser.OpenURL); err != nil {
		log.Printf("Game account bridge unavailable: %v", err)
	} else {
		defer broker.Close()
	}
	if consoleBroker, err := gameconsole.Start(); err != nil {
		log.Printf("Game console bridge unavailable: %v", err)
	} else {
		defer consoleBroker.Close()
	}
	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
