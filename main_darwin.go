//go:build darwin

package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/liteldev/LeviLauncher/internal/app"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/cli"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/deeplink"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/gameauth"
	"github.com/liteldev/LeviLauncher/internal/gameconsole"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/oslang"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

const (
	darwinMinWindowWidth      = 960
	darwinMinWindowHeight     = 600
	darwinDefaultWindowWidth  = 1180
	darwinDefaultWindowHeight = 780
	darwinStartupLogsKept     = 5
)

type darwinStartupDiagnostics struct {
	logPath         string
	logFile         *os.File
	logger          *slog.Logger
	debugMode       bool
	reportOnce      sync.Once
	startupComplete atomic.Bool
	showDialog      func(title, message string)
}

func rotateDarwinStartupLogs(logPath string) int {
	ext := filepath.Ext(logPath)
	stem := strings.TrimSuffix(logPath, ext)
	numbered := func(i int) string { return fmt.Sprintf("%s.%d%s", stem, i, ext) }
	if _, err := os.Stat(logPath); err != nil {
		return os.O_TRUNC
	}
	_ = os.Remove(numbered(darwinStartupLogsKept - 1))
	for i := darwinStartupLogsKept - 2; i >= 1; i-- {
		_ = os.Rename(numbered(i), numbered(i+1))
	}
	if err := os.Rename(logPath, numbered(1)); err != nil {
		return os.O_APPEND
	}
	return os.O_TRUNC
}

func initDarwinStartupDiagnostics(debugMode bool) *darwinStartupDiagnostics {
	logPath := apppath.StartupLogPath()
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	writers := []io.Writer{os.Stderr}
	var logFile *os.File
	// Finder-launched apps have no terminal; retain backend errors locally so
	// authentication and runtime failures can be diagnosed without logging credentials.
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|rotateDarwinStartupLogs(logPath), 0o600)
	if err == nil {
		logFile = file
		writers = append(writers, file)
	}
	logWriter := io.MultiWriter(writers...)
	log.SetOutput(logWriter)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	level := slog.LevelWarn
	if debugMode {
		level = slog.LevelDebug
	}
	diag := &darwinStartupDiagnostics{
		logPath:    logPath,
		logFile:    logFile,
		logger:     slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: level})),
		debugMode:  debugMode,
		showDialog: showDarwinStartupFailureDialog,
	}
	if err != nil {
		log.Printf("[startup] failed to open startup log %s: %v", logPath, err)
	} else {
		log.Printf("[startup] startup log path: %s", logPath)
	}
	return diag
}

func (s *darwinStartupDiagnostics) Logger() *slog.Logger {
	if s == nil {
		return nil
	}
	return s.logger
}

func (s *darwinStartupDiagnostics) Close() {
	if s == nil || s.logFile == nil {
		return
	}
	_ = s.logFile.Sync()
	_ = s.logFile.Close()
}

func (s *darwinStartupDiagnostics) flush() {
	if s == nil || s.logFile == nil {
		return
	}
	_ = s.logFile.Sync()
}

func (s *darwinStartupDiagnostics) MarkStartupComplete() {
	if s == nil {
		return
	}
	s.startupComplete.Store(true)
}

func (s *darwinStartupDiagnostics) HandleError(source string, err error) {
	if err == nil {
		return
	}
	if strings.TrimSpace(source) == "" {
		log.Printf("[startup] error: %v", err)
	} else {
		log.Printf("[startup] %s: %v", source, err)
	}
	if s == nil || s.startupComplete.Load() {
		s.flush()
		return
	}
	s.reportOnce.Do(func() {
		s.flush()
		if s.showDialog != nil {
			s.showDialog(darwinStartupFailureTitle(), darwinStartupFailureMessage(s.logPath))
		}
		s.flush()
	})
}

func (s *darwinStartupDiagnostics) HandlePanic(source string, err error, stackTrace string) {
	if err == nil {
		err = fmt.Errorf("unknown panic")
	}
	if strings.TrimSpace(stackTrace) == "" {
		stackTrace = string(debug.Stack())
	}
	log.Printf("[startup] panic in %s: %v", source, err)
	if strings.TrimSpace(stackTrace) != "" {
		log.Printf("[startup] panic stack trace:\n%s", stackTrace)
	}
	s.flush()
	if s == nil || s.startupComplete.Load() {
		return
	}
	s.reportOnce.Do(func() {
		if s.showDialog != nil {
			s.showDialog(darwinStartupFailureTitle(), darwinStartupFailureMessage(s.logPath))
		}
		s.flush()
	})
}

func darwinStartupFailureTitle() string {
	if oslang.IsChineseUI() {
		return "LeviLauncher - 启动失败"
	}
	return "LeviLauncher - Startup Failed"
}

func darwinStartupFailureMessage(logPath string) string {
	if strings.TrimSpace(logPath) == "" {
		logPath = "Unavailable"
	}
	if oslang.IsChineseUI() {
		return fmt.Sprintf("LeviLauncher 启动失败。\n\n请在提交 GitHub issue 时附上 startup.log。\n\n日志路径:\n%s", logPath)
	}
	return fmt.Sprintf("LeviLauncher failed to start.\n\nPlease attach startup.log when opening a GitHub issue.\n\nLog path:\n%s", logPath)
}

func showDarwinStartupFailureDialog(title, message string) {
	script := `on run argv
display alert (item 1 of argv) message (item 2 of argv) as critical
end run`
	_ = exec.Command("/usr/bin/osascript", "-e", script, "--", title, message).Run()
}

func darwinPanicErrorValue(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("%v", v)
}

func parseDarwinArgs(args []string) (autoLaunchVersion string, debugMode bool) {
	for _, arg := range args {
		low := strings.ToLower(strings.TrimSpace(arg))
		switch low {
		case "debug", "-debug", "--debug", "/debug":
			debugMode = true
			continue
		}
		if strings.HasPrefix(arg, "--launch=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--launch="))
			v = strings.Trim(v, `"'`)
			autoLaunchVersion = v
		}
	}
	return autoLaunchVersion, debugMode
}

func darwinDeepLinkFromArgs(args []string) (*deeplink.Request, error) {
	raw, ok := deeplink.FromArgs(args)
	if !ok {
		return nil, nil
	}
	r, err := deeplink.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func resolveDarwinWindowSize(cfg config.AppConfig) (width, height int, persistInitial bool) {
	width = darwinDefaultWindowWidth
	height = darwinDefaultWindowHeight
	if cfg.WindowWidth > 0 {
		if cfg.WindowWidth < darwinMinWindowWidth {
			width = darwinMinWindowWidth
		} else {
			width = cfg.WindowWidth
		}
	}
	if cfg.WindowHeight > 0 {
		if cfg.WindowHeight < darwinMinWindowHeight {
			height = darwinMinWindowHeight
		} else {
			height = cfg.WindowHeight
		}
	}
	return width, height, cfg.WindowWidth == 0 || cfg.WindowHeight == 0
}

func handleDarwinSecondInstanceArgs(args []string, deliverLink func(string) error, launchVersion func(string), focus func()) {
	if raw, ok := deeplink.FromArgs(args); ok {
		if err := deliverLink(raw); err != nil {
			log.Printf("Ignored second-instance levilauncher:// link: %v", err)
		}
		focus()
		return
	}
	autoLaunch, _ := parseDarwinArgs(args)
	if autoLaunch != "" {
		if errCode := versionlaunch.ValidateLaunchName(autoLaunch); errCode != "" {
			log.Printf("Rejected second-instance launch payload %q: %s", autoLaunch, errCode)
			return
		}
		launchVersion(autoLaunch)
		return
	}
	focus()
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--attach-console" {
		os.Exit(gameconsole.RunCLI(os.Args[2:]))
	}
	if cli.Wants(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:]))
	}

	autoLaunchVersion, debugMode := parseDarwinArgs(os.Args[1:])
	if err := apppath.EnsureDirs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	diagnostics := initDarwinStartupDiagnostics(debugMode)
	defer diagnostics.Close()
	defer func() {
		if recovered := recover(); recovered != nil {
			diagnostics.HandlePanic("main", darwinPanicErrorValue(recovered), string(debug.Stack()))
			os.Exit(2)
		}
	}()

	deepLink, deepLinkErr := darwinDeepLinkFromArgs(os.Args[1:])
	if deepLinkErr != nil {
		log.Printf("[startup] ignored the levilauncher:// link: %v", deepLinkErr)
	}
	if deepLink != nil {
		autoLaunchVersion = ""
	}

	c, err := config.Load()
	if err != nil {
		diagnostics.HandleError("config.Load failed", err)
		os.Exit(1)
	}
	update.Init()

	mc := app.NewMinecraft()
	user := app.NewUserService(mc)
	versions := app.NewVersionService(mc)
	if deepLink != nil {
		log.Printf("[startup] opening %s", deepLink)
		deeplink.Default.Deliver(*deepLink)
	}
	deepLinks := app.NewDeepLinkService()

	logLevel := slog.LevelWarn
	if debugMode {
		logLevel = slog.LevelDebug
	}

	// macOS loads the Dock icon from the app bundle.
	a := application.New(application.Options{
		Name:        "LeviLauncher",
		Logger:      diagnostics.Logger(),
		LogLevel:    logLevel,
		Description: "Minecraft Bedrock launcher",
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Server:      application.ServerOptions{WriteTimeout: 30 * time.Minute},
		ErrorHandler: func(err error) {
			diagnostics.HandleError("Wails error", err)
		},
		PanicHandler: func(details *application.PanicDetails) {
			if details == nil {
				diagnostics.HandlePanic("Wails panic", fmt.Errorf("panic details unavailable"), "")
				os.Exit(2)
			}
			diagnostics.HandlePanic("Wails panic", details.Error, details.FullStackTrace)
			os.Exit(2)
		},
		Services: []application.Service{
			application.NewService(mc),
			application.NewService(app.NewContentService(mc)),
			application.NewService(app.NewModsService(mc)),
			application.NewService(user),
			application.NewService(versions),
			application.NewService(app.NewDiagnosticsService()),
			application.NewService(deepLinks),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: mc.LocalImageMiddleware,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "org.levimc.levilauncher.native",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				handleDarwinSecondInstanceArgs(
					data.Args,
					func(raw string) error {
						_, err := deeplink.Default.DeliverRaw(raw)
						return err
					},
					func(name string) {
						go func() { _ = versions.LaunchVersionByNameForce(name) }()
					},
					func() { go launch.RestoreLauncherWindow() },
				)
			},
		},
	})

	// macOS hands levilauncher:// links (CFBundleURLTypes in the Info.plist
	// that build/native/macos.py writes) to the running app as an Apple Event,
	// also when the link started it. The link only opens the mod's page.
	a.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
		if _, err := deeplink.Default.DeliverRaw(event.Context().URL()); err != nil {
			log.Printf("Ignored levilauncher:// link: %v", err)
			return
		}
		go launch.RestoreLauncherWindow()
	})
	// Clicking the Dock icon while the launcher window was hidden (e.g. via
	// the "hide on game launch" setting) restores the main window.
	a.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(_ *application.ApplicationEvent) {
		go launch.RestoreLauncherWindow()
	})

	mc.StartupEssential()
	mc.RecordLauncherStart()

	winW, winH, persistInitial := resolveDarwinWindowSize(c)
	if persistInitial {
		_ = config.Update(func(c *config.AppConfig) {
			c.WindowWidth = winW
			c.WindowHeight = winH
		})
	}

	w := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:           "main",
		Title:          "LeviLauncher",
		Width:          winW,
		Height:         winH,
		MinWidth:       darwinMinWindowWidth,
		MinHeight:      darwinMinWindowHeight,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInsetUnified,
		},
		URL:            "/",
		EnableFileDrop: true,
	})

	w.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		details := event.Context().DropTargetDetails()
		if len(files) > 0 {
			target := ""
			if details != nil {
				target = details.ElementID
			}
			w.EmitEvent("files-dropped", types.FilesDroppedEvent{
				Files:  files,
				Target: target,
			})
		}
	})

	var deferredStartupOnce sync.Once
	w.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(_ *application.WindowEvent) {
		diagnostics.MarkStartupComplete()
		deferredStartupOnce.Do(func() {
			go func() {
				var wg sync.WaitGroup

				wg.Add(1)
				go func() {
					defer wg.Done()
					mc.StartupDeferred()
				}()

				if !config.GetDiscordRPCDisabled() {
					wg.Add(1)
					go func() {
						defer wg.Done()
						discord.Init()
					}()
				}

				wg.Wait()
			}()
		})
	})

	persistWindowSize := func() {
		if w.IsMinimised() || w.IsMaximised() || w.IsFullscreen() {
			return
		}
		curW := w.Width()
		curH := w.Height()
		if curW <= 0 || curH <= 0 {
			return
		}
		if curW < darwinMinWindowWidth {
			curW = darwinMinWindowWidth
		}
		if curH < darwinMinWindowHeight {
			curH = darwinMinWindowHeight
		}
		if err := config.Update(func(c *config.AppConfig) {
			c.WindowWidth = curW
			c.WindowHeight = curH
		}); err != nil {
			log.Printf("failed to persist window size: %v", err)
		}
	}
	a.OnShutdown(persistWindowSize)
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		persistWindowSize()
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

	if strings.TrimSpace(autoLaunchVersion) != "" {
		go func() {
			_ = versions.LaunchVersionByName(autoLaunchVersion)
		}()
	}

	if err := a.Run(); err != nil {
		diagnostics.HandleError("a.Run failed", err)
		os.Exit(1)
	}
}
