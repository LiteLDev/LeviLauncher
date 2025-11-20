package main

import (
	"embed"
	"log"
	"os"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/preloader"
	"github.com/liteldev/LeviLauncher/internal/update"
    "github.com/liteldev/LeviLauncher/internal/vcruntime"
    "github.com/liteldev/LeviLauncher/internal/types"
    "github.com/liteldev/LeviLauncher/internal/mcservice"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	//minecraft
	application.RegisterEvent[struct{}](EventGameInputEnsureStart)
	application.RegisterEvent[struct{}](EventGameInputEnsureDone)
	application.RegisterEvent[int64](EventGameInputDownloadStart)
	application.RegisterEvent[GameInputDownloadProgress](EventGameInputDownloadProgress)
	application.RegisterEvent[struct{}](EventGameInputDownloadDone)
	application.RegisterEvent[string](EventGameInputDownloadError)
    application.RegisterEvent[string](mcservice.EventExtractError)
    application.RegisterEvent[string](mcservice.EventExtractDone)
    application.RegisterEvent[types.ExtractProgress](mcservice.EventExtractProgress)
	// launch
	application.RegisterEvent[struct{}](launch.EventMcLaunchStart)
	application.RegisterEvent[struct{}](launch.EventMcLaunchDone)
	application.RegisterEvent[struct{}](launch.EventMcLaunchFailed)
	application.RegisterEvent[struct{}](launch.EventGamingServicesMissing)
	//msixvc
	application.RegisterEvent[string](msixvc.EventDownloadStatus)
	application.RegisterEvent[msixvc.DownloadProgress](msixvc.EventDownloadProgress)
	application.RegisterEvent[string](msixvc.EventDownloadDone)
	application.RegisterEvent[string](msixvc.EventDownloadError)
	application.RegisterEvent[bool](msixvc.EventAppxInstallLoading)
	//preloader
	application.RegisterEvent[struct{}](preloader.EventEnsureStart)
	application.RegisterEvent[bool](preloader.EventEnsureDone)
	// peeditor
	application.RegisterEvent[struct{}](peeditor.EventEnsureStart)
	application.RegisterEvent[bool](peeditor.EventEnsureDone)
	// vcruntime
	application.RegisterEvent[struct{}](vcruntime.EventEnsureStart)
	application.RegisterEvent[vcruntime.EnsureProgress](vcruntime.EventEnsureProgress)
	application.RegisterEvent[bool](vcruntime.EventEnsureDone)
	// app update
	application.RegisterEvent[string](update.EventAppUpdateStatus)
	application.RegisterEvent[update.AppUpdateProgress](update.EventAppUpdateProgress)
	application.RegisterEvent[string](update.EventAppUpdateError)
}

func main() {
	c, _ := config.Load()
	extractor.Init()
	update.Init()
	discord.Init()
	mc := Minecraft{}
	app := application.New(application.Options{
		Name:        "LeviLauncher",
		Description: "A Minecraft Launcher",
		Services: []application.Service{
			application.NewService(&mc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	mc.startup()

	initialURL := "/"
	var autoLaunchVersion string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--self-update=") {
			initialURL = "/#/updating"
			break
		}
		if strings.HasPrefix(arg, "--launch=") {
			autoLaunchVersion = strings.TrimSpace(strings.TrimPrefix(arg, "--launch="))
		}
	}

	if strings.TrimSpace(autoLaunchVersion) != "" && initialURL == "/" {
		_ = mc.LaunchVersionByName(autoLaunchVersion)
		return
	}

	w := 1024
	h := 640
	if c.WindowWidth > 0 {
		if c.WindowWidth < 960 {
			w = 960
		} else {
			w = c.WindowWidth
		}
	}
	if c.WindowHeight > 0 {
		if c.WindowHeight < 600 {
			h = 600
		} else {
			h = c.WindowHeight
		}
	}
	if c.WindowWidth == 0 || c.WindowHeight == 0 {
		c.WindowWidth = w
		c.WindowHeight = h
		_ = config.Save(c)
	}
    windows := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "LeviLauncher",
		Width:     w,
		Height:    h,
		MinWidth:  960,
		MinHeight: 600,
		Mac:       application.MacWindow{},
		Frameless: true,
		BackgroundColour: application.RGBA{
			Red:   0,
			Green: 0,
			Blue:  0,
			Alpha: 0,
		},
		Windows: application.WindowsWindow{
			BackdropType: application.Acrylic,
		},
        URL: initialURL,
    })

    if strings.TrimSpace(autoLaunchVersion) != "" {
        go func() {
            _ = mc.LaunchVersionByName(autoLaunchVersion)
        }()
    }

	windows.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		w := windows.Width()
		h := windows.Height()

		c, _ := config.Load()
		if w > 0 && h > 0 {
			c.WindowWidth = w
			c.WindowHeight = h
			_ = config.Save(c)
		}
	})
	err := app.Run()

	if err != nil {
		log.Fatal(err.Error())
	}

}
