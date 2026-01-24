package main

import (
	"bufio"
	"embed"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"unsafe"

	win "golang.org/x/sys/windows"
	"gopkg.in/natefinch/npipe.v2"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/gdk"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/preloader"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

var singleInstanceGuard win.Handle

const singleInstancePipe = `\\.\pipe\LeviLauncher_SingleInstance_Pipe`

const (
	SW_RESTORE = 9
)

var (
	user32                  = win.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func focusExistingWindow() {
	title, _ := win.UTF16PtrFromString("LeviLauncher")
	r1, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if r1 != 0 {
		_, _, _ = procShowWindow.Call(r1, uintptr(SW_RESTORE))
		_, _, _ = procSetForegroundWindow.Call(r1)
	}
}

func parseArgs() (initialURL string, autoLaunchVersion string) {
	initialURL = "/"
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--self-update=") {
			initialURL = "/#/updating"
			break
		}
		if strings.HasPrefix(arg, "--launch=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--launch="))
			v = strings.Trim(v, `"'`)
			autoLaunchVersion = v
		}
	}
	return initialURL, autoLaunchVersion
}

func sendLaunchToExistingInstance(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" {
		return false
	}
	for i := 0; i < 8; i++ {
		conn, err := npipe.DialTimeout(singleInstancePipe, 200*time.Millisecond)
		if err == nil && conn != nil {
			func() {
				defer conn.Close()
				_, _ = conn.Write([]byte("launch\t" + v + "\n"))
			}()
			return true
		}
		time.Sleep(120 * time.Millisecond)
	}
	return false
}

func startSingleInstanceServer(mc *Minecraft) {
	ln, err := npipe.Listen(singleInstancePipe)
	if err != nil {
		return
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				s := bufio.NewScanner(c)
				for s.Scan() {
					line := strings.TrimSpace(s.Text())
					if line == "" {
						continue
					}
					parts := strings.SplitN(line, "\t", 2)
					if len(parts) != 2 {
						continue
					}
					cmd := strings.TrimSpace(parts[0])
					payload := strings.TrimSpace(parts[1])
					if cmd == "launch" && payload != "" {
						go func(v string) {
							_ = mc.LaunchVersionByNameForce(v)
						}(payload)
					}
				}
			}(conn)
		}
	}()
}

func ensureSingleInstance(autoLaunchVersion string) bool {
	name, err := win.UTF16PtrFromString("Global\\LeviLauncher_SingleInstance")
	if err != nil {
		return true
	}
	h, err := win.CreateMutex(nil, true, name)
	if err == win.ERROR_ALREADY_EXISTS {
		_ = sendLaunchToExistingInstance(autoLaunchVersion)
		focusExistingWindow()
		return false
	}
	if err != nil {
		return true
	}
	singleInstanceGuard = h
	return true
}

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
	application.RegisterEvent[msixvc.DownloadStatus](msixvc.EventDownloadStatus)
	application.RegisterEvent[msixvc.DownloadProgress](msixvc.EventDownloadProgress)
	application.RegisterEvent[msixvc.DownloadDone](msixvc.EventDownloadDone)
	application.RegisterEvent[msixvc.DownloadError](msixvc.EventDownloadError)
	application.RegisterEvent[bool](msixvc.EventAppxInstallLoading)
	// gdk
	application.RegisterEvent[string](gdk.EventDownloadStatus)
	application.RegisterEvent[gdk.DownloadProgress](gdk.EventDownloadProgress)
	application.RegisterEvent[string](gdk.EventDownloadDone)
	application.RegisterEvent[string](gdk.EventDownloadError)
	application.RegisterEvent[string](gdk.EventInstallStart)
	application.RegisterEvent[string](gdk.EventInstallDone)
	application.RegisterEvent[string](gdk.EventInstallError)
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
	initialURL, autoLaunchVersion := parseArgs()

	if !ensureSingleInstance(autoLaunchVersion) {
		return
	}
	c, _ := config.Load()
	extractor.Init()
	update.Init()
	if !config.GetDiscordRPCDisabled() {
		discord.Init()
	}
	mc := NewMinecraft()
	startSingleInstanceServer(mc)
	app := application.New(application.Options{
		Name:        "LeviLauncher",
		Description: "A Minecraft Launcher",
		Services: []application.Service{
			application.NewService(mc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	mc.startup()

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

	if singleInstanceGuard != 0 {
		_ = win.ReleaseMutex(singleInstanceGuard)
		_ = win.CloseHandle(singleInstanceGuard)
	}

}
