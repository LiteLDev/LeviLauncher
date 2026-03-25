package main

import (
	"bufio"
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	win "golang.org/x/sys/windows"
	"gopkg.in/natefinch/npipe.v2"

	"github.com/joho/godotenv"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/resourcerules"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

var singleInstanceGuard win.Handle

const singleInstancePipe = `\\.\pipe\LeviLauncher_SingleInstance_Pipe`

const (
	SW_RESTORE          = 9
	minWindowWidth      = 960
	minWindowHeight     = 600
	defaultWindowWidth  = 1024
	defaultWindowHeight = 640
)

var (
	user32                  = win.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

type startupLogger struct {
	start time.Time
}

func newStartupLogger() *startupLogger {
	return &startupLogger{start: time.Now()}
}

func (s *startupLogger) Mark(phase string) {
	log.Printf("[startup] %s (+%dms)", phase, time.Since(s.start).Milliseconds())
}

func focusExistingWindow() {
	title, _ := win.UTF16PtrFromString("LeviLauncher")
	r1, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if r1 != 0 {
		_, _, _ = procShowWindow.Call(r1, uintptr(SW_RESTORE))
		_, _, _ = procSetForegroundWindow.Call(r1)
	}
}

func parseArgs() (initialURL string, autoLaunchVersion string, postUpdateRestart bool) {
	initialURL = "/"
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--self-update=") {
			initialURL = "/#/updating"
			break
		}
		if arg == "--post-update-restart" {
			postUpdateRestart = true
			continue
		}
		if strings.HasPrefix(arg, "--launch=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--launch="))
			v = strings.Trim(v, `"'`)
			autoLaunchVersion = v
		}
	}
	return initialURL, autoLaunchVersion, postUpdateRestart
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

func startSingleInstanceServer(versionService *VersionService) {
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
						if errCode := versionlaunch.ValidateLaunchName(payload); errCode != "" {
							log.Printf("Rejected single-instance launch payload %q: %s", payload, errCode)
							continue
						}
						go func(v string) {
							_ = versionService.LaunchVersionByNameForce(v)
						}(payload)
					}
				}
			}(conn)
		}
	}()
}

func ensureSingleInstance(autoLaunchVersion string, postUpdateRestart bool) bool {
	name, err := win.UTF16PtrFromString("Global\\LeviLauncher_SingleInstance")
	if err != nil {
		return true
	}
	tryAcquire := func() (win.Handle, error) {
		return win.CreateMutex(nil, true, name)
	}
	h, err := tryAcquire()
	if err == win.ERROR_ALREADY_EXISTS {
		if h != 0 {
			_ = win.CloseHandle(h)
		}
		if postUpdateRestart {
			for i := 0; i < 12; i++ {
				time.Sleep(250 * time.Millisecond)
				h, err = tryAcquire()
				if err == nil {
					singleInstanceGuard = h
					return true
				}
				if err != win.ERROR_ALREADY_EXISTS {
					if h != 0 {
						_ = win.CloseHandle(h)
					}
					return true
				}
				if h != 0 {
					_ = win.CloseHandle(h)
				}
			}
		}
		if !sendLaunchToExistingInstance(autoLaunchVersion) {
			focusExistingWindow()
		}
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
	application.RegisterEvent[types.InstanceBackupRestoreProgress](mcservice.EventInstanceBackupRestoreProgress)
	// launch
	application.RegisterEvent[struct{}](launch.EventMcLaunchStart)
	application.RegisterEvent[struct{}](launch.EventMcLaunchDone)
	application.RegisterEvent[string](launch.EventMcLaunchFailed)
	application.RegisterEvent[struct{}](launch.EventGamingServicesMissing)
	//msixvc
	application.RegisterEvent[msixvc.DownloadStatus](msixvc.EventDownloadStatus)
	application.RegisterEvent[msixvc.DownloadProgress](msixvc.EventDownloadProgress)
	application.RegisterEvent[msixvc.DownloadDone](msixvc.EventDownloadDone)
	application.RegisterEvent[msixvc.DownloadError](msixvc.EventDownloadError)
	application.RegisterEvent[bool](msixvc.EventAppxInstallLoading)
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
	// lip daemon task stream
	application.RegisterEvent[lip.LipTaskStartedEvent](lip.EventLipTaskStarted)
	application.RegisterEvent[lip.LipTaskLogEvent](lip.EventLipTaskLog)
	application.RegisterEvent[lip.LipTaskProgressEvent](lip.EventLipTaskProgress)
	application.RegisterEvent[lip.LipTaskFinishedEvent](lip.EventLipTaskFinished)
	application.RegisterEvent[types.FilesDroppedEvent]("files-dropped")
}

func main() {
	startup := newStartupLogger()
	startup.Mark("process start")

	_ = godotenv.Load()
	initialURL, autoLaunchVersion, postUpdateRestart := parseArgs()

	if !ensureSingleInstance(autoLaunchVersion, postUpdateRestart) {
		return
	}
	c, err := config.Load()
	if err != nil {
		log.Printf("config.Load failed: %v", err)
	}
	update.Init()
	startup.Mark("config loaded")
	mc := NewMinecraft()
	contentService := NewContentService(mc)
	modsService := NewModsService(mc)
	userService := NewUserService(mc)
	versionService := NewVersionService(mc)

	assets, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "LeviLauncher",
		Description: "A Minecraft Launcher",
		Services: []application.Service{
			application.NewService(mc),
			application.NewService(contentService),
			application.NewService(modsService),
			application.NewService(userService),
			application.NewService(versionService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	mc.startupEssential()
	startSingleInstanceServer(versionService)

	if strings.TrimSpace(autoLaunchVersion) != "" && initialURL == "/" {
		_ = versionService.LaunchVersionByName(autoLaunchVersion)
		return
	}

	w := defaultWindowWidth
	h := defaultWindowHeight
	if c.WindowWidth > 0 {
		if c.WindowWidth < minWindowWidth {
			w = minWindowWidth
		} else {
			w = c.WindowWidth
		}
	}
	if c.WindowHeight > 0 {
		if c.WindowHeight < minWindowHeight {
			h = minWindowHeight
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
		MinWidth:  minWindowWidth,
		MinHeight: minWindowHeight,
		Mac:       application.MacWindow{},
		Frameless: true,
		BackgroundColour: application.RGBA{
			Red:   248,
			Green: 250,
			Blue:  252,
			Alpha: 255,
		},
		URL:            initialURL,
		EnableFileDrop: true,
	})
	startup.Mark("window created")
	reapplyWindowMinConstraints := func() {
		windows.SetMinSize(minWindowWidth, minWindowHeight)
		currentW := windows.Width()
		currentH := windows.Height()
		targetW := currentW
		targetH := currentH
		if targetW > 0 && targetW < minWindowWidth {
			targetW = minWindowWidth
		}
		if targetH > 0 && targetH < minWindowHeight {
			targetH = minWindowHeight
		}
		if targetW != currentW || targetH != currentH {
			windows.SetSize(targetW, targetH)
		}
	}
	syncWindowResizeHandles := func() {
		isMaximised := windows.IsMaximised()
		windows.ExecJS(`if (window._wails && typeof window._wails.setResizable === "function") { window._wails.setResizable(` + strconv.FormatBool(!isMaximised) + `); }`)
		if !isMaximised {
			reapplyWindowMinConstraints()
		}
	}

	if strings.TrimSpace(autoLaunchVersion) != "" {
		go func() {
			_ = versionService.LaunchVersionByName(autoLaunchVersion)
		}()
	}

	windows.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		details := event.Context().DropTargetDetails()
		if len(files) > 0 {
			windows.EmitEvent("files-dropped", types.FilesDroppedEvent{
				Files:  files,
				Target: details.ElementID,
			})
		}
	})
	windows.OnWindowEvent(events.Common.WindowMaximise, func(_ *application.WindowEvent) {
		syncWindowResizeHandles()
	})
	windows.OnWindowEvent(events.Common.WindowUnMaximise, func(_ *application.WindowEvent) {
		syncWindowResizeHandles()
	})
	windows.OnWindowEvent(events.Common.WindowRestore, func(_ *application.WindowEvent) {
		syncWindowResizeHandles()
	})
	var deferredStartupOnce sync.Once
	windows.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(_ *application.WindowEvent) {
		syncWindowResizeHandles()
		deferredStartupOnce.Do(func() {
			startup.Mark("webview navigation completed")
			go func() {
				startup.Mark("deferred startup started")
				var wg sync.WaitGroup

				wg.Add(1)
				go func() {
					defer wg.Done()
					mc.startupDeferred()
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()
					extractor.Init()
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = resourcerules.EnsureLatestWithError(context.Background())
				}()

				if !config.GetDiscordRPCDisabled() {
					wg.Add(1)
					go func() {
						defer wg.Done()
						discord.Init()
					}()
				}

				wg.Wait()
				startup.Mark("deferred startup finished")
			}()
		})
	})
	windows.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		w := windows.Width()
		h := windows.Height()

		c, err := config.Load()
		if err != nil {
			log.Printf("config.Load failed during window close: %v", err)
		}
		if w > 0 && h > 0 {
			if w < minWindowWidth {
				w = minWindowWidth
			}
			if h < minWindowHeight {
				h = minWindowHeight
			}
			c.WindowWidth = w
			c.WindowHeight = h
			_ = config.Save(c)
		}
	})
	err = app.Run()

	if err != nil {
		log.Fatal(err.Error())
	}

	if singleInstanceGuard != 0 {
		_ = win.ReleaseMutex(singleInstanceGuard)
		_ = win.CloseHandle(singleInstanceGuard)
	}

}
