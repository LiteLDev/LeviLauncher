package main

// Command-line mode. LeviLauncher.exe without a subcommand opens the GUI; with
// one it runs headless and exits. The command set is kept minimal:
//
//	list                    list the installed instances
//	launch <instance>       launch an instance
//	call [method [args...]] invoke any backend method the GUI uses, or list them
//	help                    print usage
//
// "call" is what guarantees parity with the GUI: every action the GUI performs
// goes through one of the bound service methods, and "call" reaches each of
// them by reflection, so new GUI features are available here without new code.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	win "golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/app"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/storage"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const cliUsage = `Usage:
  LeviLauncher.exe                        Open the launcher window
  LeviLauncher.exe list                   List installed instances
  LeviLauncher.exe launch <instance>      Launch an instance
  LeviLauncher.exe call                   List every backend method the GUI uses
  LeviLauncher.exe call <method> [args]   Invoke one of them
  LeviLauncher.exe help                   Show this help

Arguments to "call" map to the method's parameters in order: a string
parameter takes the argument as is, any other type takes it as JSON
(true, 42, ["a","b"], {"k":"v"}). The result is printed to stdout, as JSON
unless it is a plain string; events (progress etc.) go to stderr as JSON lines.
Downloads and other background tasks are waited for before exiting.

Exit status: 0 on success, 1 when the call fails (a Go error or an "ERR_..."
code), 2 on a usage error. Add --debug to also print the launcher log.

Examples:
  LeviLauncher.exe launch "1.21.50"
  LeviLauncher.exe call GetMods "1.21.50"
  LeviLauncher.exe call SetMinimizeToTray true
  LeviLauncher.exe call ImportMcpackPath "1.21.50" C:\packs\foo.mcpack false
`

// isCLIInvocation reports whether the first argument is a subcommand. Flags
// (--launch=, --debug, --self-update=, WebView2 options...) and the bare
// "debug" switch keep their GUI meaning.
func isCLIInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := strings.TrimSpace(args[0])
	switch strings.ToLower(first) {
	case "-h", "--help", "/?", "-?", "/h":
		return true
	}
	if first == "" || strings.HasPrefix(first, "-") || strings.HasPrefix(first, "/") || isDebugArg(first) {
		return false
	}
	return true
}

// cliMethod is one bound service method callable from the command line.
type cliMethod struct {
	service string
	name    string
	fn      reflect.Value
}

var contextType = reflect.TypeFor[context.Context]()
var errorType = reflect.TypeFor[error]()

// cliHiddenMethods are exported for Go callers but not bound to the frontend
// (marked wails:ignore), so they are not part of the GUI surface.
var cliHiddenMethods = map[string]bool{
	"StartupEssential":     true,
	"StartupDeferred":      true,
	"LocalImageMiddleware": true,
	"Attach":               true,
	"RecordLauncherStart":  true,
}

// cliAsyncMethods return before their work is done; the CLI waits for one of
// the listed events before exiting. The first event of each list reports
// success, the others failure.
var cliAsyncMethods = map[string][]string{
	"StartMsixvcDownload":        {msixvc.EventDownloadDone, msixvc.EventDownloadError},
	"ResumeMsixvcDownload":       {msixvc.EventDownloadDone, msixvc.EventDownloadError},
	"StartUWPDownload":           {msixvc.EventDownloadDone, msixvc.EventDownloadError},
	"StartFileDownload":          {app.EventFileDownloadDone, app.EventFileDownloadError},
	"EnsureGameInputInteractive": {app.EventGameInputEnsureDone},
	"EnsureVcRuntimeInteractive": {vcruntime.EventEnsureDone},
}

// cliExtraEvents are emitted by the backend but not registered in init.
var cliExtraEvents = []string{
	app.EventFileDownloadStatus,
	app.EventFileDownloadProgress,
	app.EventFileDownloadDone,
	app.EventFileDownloadError,
}

func collectCLIMethods(services ...any) map[string]cliMethod {
	methods := map[string]cliMethod{}
	for _, svc := range services {
		v := reflect.ValueOf(svc)
		t := v.Type()
		service := t.Elem().Name()
		for i := 0; i < t.NumMethod(); i++ {
			m := t.Method(i)
			if cliHiddenMethods[m.Name] || strings.HasPrefix(m.Name, "Service") {
				continue
			}
			methods[m.Name] = cliMethod{service: service, name: m.Name, fn: v.Method(i)}
		}
	}
	return methods
}

func lookupCLIMethod(methods map[string]cliMethod, name string) (cliMethod, bool) {
	if i := strings.LastIndex(name, "."); i >= 0 {
		m, ok := lookupCLIMethod(methods, name[i+1:])
		if ok && !strings.EqualFold(m.service, name[:i]) {
			return cliMethod{}, false
		}
		return m, ok
	}
	if m, ok := methods[name]; ok {
		return m, true
	}
	for k, m := range methods {
		if strings.EqualFold(k, name) {
			return m, true
		}
	}
	return cliMethod{}, false
}

// signature renders a method as "Service.Name(type, ...) result".
func (m cliMethod) signature() string {
	t := m.fn.Type()
	var params []string
	for i := 0; i < t.NumIn(); i++ {
		if t.In(i) == contextType {
			continue
		}
		params = append(params, t.In(i).String())
	}
	var results []string
	for i := 0; i < t.NumOut(); i++ {
		results = append(results, t.Out(i).String())
	}
	s := fmt.Sprintf("%s.%s(%s)", m.service, m.name, strings.Join(params, ", "))
	switch len(results) {
	case 0:
	case 1:
		s += " " + results[0]
	default:
		s += " (" + strings.Join(results, ", ") + ")"
	}
	return s
}

// buildArgs converts command-line arguments to the method's parameters.
func (m cliMethod) buildArgs(ctx context.Context, args []string) ([]reflect.Value, error) {
	t := m.fn.Type()
	var in []reflect.Value
	next := 0
	for i := 0; i < t.NumIn(); i++ {
		pt := t.In(i)
		if pt == contextType {
			in = append(in, reflect.ValueOf(ctx))
			continue
		}
		if next >= len(args) {
			return nil, fmt.Errorf("not enough arguments\n  %s", m.signature())
		}
		arg := args[next]
		next++
		if pt.Kind() == reflect.String {
			in = append(in, reflect.ValueOf(arg).Convert(pt))
			continue
		}
		pv := reflect.New(pt)
		if err := json.Unmarshal([]byte(arg), pv.Interface()); err != nil {
			return nil, fmt.Errorf("argument %d (%s): %v\n  %s", next, pt, err, m.signature())
		}
		in = append(in, pv.Elem())
	}
	if next < len(args) {
		return nil, fmt.Errorf("too many arguments\n  %s", m.signature())
	}
	return in, nil
}

// cliOutput serialises writes to stdout and stderr: events arrive on many
// goroutines.
type cliOutput struct {
	mu     sync.Mutex
	stdout io.Writer
	stderr io.Writer
}

func (o *cliOutput) out(format string, a ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stdout, format, a...)
}

func (o *cliOutput) err(format string, a ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.stderr, format, a...)
}

// printResult prints a call's results and reports whether it failed.
func (o *cliOutput) printResult(results []reflect.Value) (failed bool) {
	for _, r := range results {
		if r.Type() == errorType {
			if !r.IsNil() {
				o.err("error: %v\n", r.Interface())
				failed = true
			}
			continue
		}
		if r.Kind() == reflect.String {
			switch s := r.String(); {
			case strings.HasPrefix(s, "ERR_"):
				o.err("error: %s\n", s)
				failed = true
			case s != "":
				o.out("%s\n", s)
			}
			continue
		}
		b, err := json.MarshalIndent(r.Interface(), "", "  ")
		if err != nil {
			o.err("error: cannot encode result: %v\n", err)
			failed = true
			continue
		}
		o.out("%s\n", b)
	}
	return failed
}

func runCLI(args []string) int {
	debug := false
	var rest []string
	for _, a := range args {
		if strings.EqualFold(a, "--debug") {
			debug = true
			continue
		}
		rest = append(rest, a)
	}
	args = rest

	setupCLIConsole()
	o := &cliOutput{stdout: os.Stdout, stderr: os.Stderr}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if debug {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(io.Discard)
	}

	if len(args) == 0 {
		o.out("%s", cliUsage)
		return 0
	}
	cmd := strings.ToLower(args[0])
	args = args[1:]
	switch cmd {
	case "help", "-h", "--help", "/?", "-?", "/h":
		o.out("%s", cliUsage)
		return 0
	case "list", "launch", "call":
	default:
		o.err("unknown command %q\n\n%s", cmd, cliUsage)
		return 2
	}
	if cmd == "launch" && len(args) != 1 {
		o.err("usage: LeviLauncher.exe launch <instance>\n")
		return 2
	}
	if cmd == "list" && len(args) != 0 {
		o.err("usage: LeviLauncher.exe list\n")
		return 2
	}

	// Same preparation as the GUI, minus the window and the single-instance
	// lock: the CLI may run next to an open launcher.
	switch outcome, err := storage.Prepare(); {
	case err != nil && outcome == storage.Current:
		o.err("error: storage migration could not be undone: %v\n", err)
		return 1
	case outcome == storage.Legacy:
		log.Printf("storage migration deferred: %v", err)
	}
	if err := apppath.EnsureDirs(); err != nil {
		o.err("error: create launcher directories: %v\n", err)
		return 1
	}
	if _, err := config.Load(); err != nil {
		o.err("error: load config: %v\n", err)
		return 1
	}
	update.Init()

	mc := app.NewMinecraft()
	contentService := app.NewContentService(mc)
	modsService := app.NewModsService(mc)
	userService := app.NewUserService(mc)
	versionService := app.NewVersionService(mc)

	logOut := io.Discard
	if debug {
		logOut = os.Stderr
	}
	wailsApp := application.New(application.Options{
		Name:     "LeviLauncher",
		Icon:     appIcon,
		Logger:   slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: slog.LevelDebug})),
		LogLevel: slog.LevelDebug,
		Services: []application.Service{
			application.NewService(mc),
			application.NewService(contentService),
			application.NewService(modsService),
			application.NewService(userService),
			application.NewService(versionService),
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: apppath.WebView2Dir(),
		},
	})

	// StartupEssential moves to the executable's directory; paths given on
	// the command line are relative to where the user is.
	cwd, _ := os.Getwd()
	mc.StartupEssential()
	if cwd != "" {
		_ = os.Chdir(cwd)
	}

	exitCode := 0
	run := func() int {
		switch cmd {
		case "list":
			return cliList(o, versionService)
		case "launch":
			code := versionService.LaunchVersionByName(args[0])
			if code != "" {
				o.err("error: %s\n", code)
				return 1
			}
			o.out("launched %s\n", args[0])
			return 0
		default:
			methods := collectCLIMethods(mc, contentService, modsService, userService, versionService)
			if len(args) == 0 {
				cliListMethods(o, methods)
				return 0
			}
			return cliCall(o, wailsApp, userService, methods, args[0], args[1:])
		}
	}

	// The command runs inside the Wails event loop so that methods which
	// need the UI thread (the Microsoft account picker) work as in the GUI.
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					o.err("error: %v\n", r)
					exitCode = 1
				}
				wailsApp.Quit()
			}()
			exitCode = run()
		}()
	})
	if err := wailsApp.Run(); err != nil {
		o.err("error: %v\n", err)
		return 1
	}
	return exitCode
}

func cliList(o *cliOutput, vs *app.VersionService) int {
	metas := vs.ListVersionMetasWithRegistered()
	if len(metas) == 0 {
		o.err("no instances installed\n")
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	tw := tabwriter.NewWriter(o.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tTYPE\tPACKAGE")
	for _, m := range metas {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.Name, m.GameVersion, m.Type, m.PackageType)
	}
	_ = tw.Flush()
	return 0
}

func cliListMethods(o *cliOutput, methods map[string]cliMethod) {
	var lines []string
	for _, m := range methods {
		lines = append(lines, m.signature())
	}
	sort.Strings(lines)
	o.out("%s\n", strings.Join(lines, "\n"))
}

func cliCall(o *cliOutput, wailsApp *application.App, us *app.UserService, methods map[string]cliMethod, name string, args []string) int {
	m, ok := lookupCLIMethod(methods, name)
	if !ok {
		o.err("unknown method %q; run \"LeviLauncher.exe call\" to list them\n", name)
		return 2
	}
	ctx := wailsApp.Context()
	in, err := m.buildArgs(ctx, args)
	if err != nil {
		o.err("%v\n", err)
		return 2
	}

	// Stream events to stderr and catch the one that ends a background task.
	waitFor := cliAsyncMethods[m.name]
	type finish struct {
		name string
		data any
	}
	finished := make(chan finish, 1)
	names := append(append([]string{}, appEventNames...), cliExtraEvents...)
	for _, n := range names {
		wailsApp.Event.On(n, func(e *application.CustomEvent) {
			data, _ := json.Marshal(e.Data)
			o.err("%s\n", mustJSON(map[string]any{"event": e.Name, "data": json.RawMessage(orNull(data))}))
			for _, w := range waitFor {
				if w == e.Name {
					select {
					case finished <- finish{e.Name, e.Data}:
					default:
					}
				}
			}
		})
	}

	if m.name == "SignIn" {
		if err := attachSignInWindow(wailsApp, us); err != nil {
			o.err("error: %v\n", err)
			return 1
		}
	}

	results := m.fn.Call(in)
	if o.printResult(results) {
		return 1
	}
	if len(waitFor) == 0 {
		return 0
	}
	select {
	case ev := <-finished:
		if ev.name != waitFor[0] {
			return 1
		}
		// Done events that carry a bool report whether the task succeeded.
		if ok, isBool := ev.data.(bool); isBool && !ok {
			return 1
		}
		return 0
	case <-ctx.Done():
		return 1
	}
}

// attachSignInWindow gives the account picker an owner window, as the GUI's
// main window does.
func attachSignInWindow(wailsApp *application.App, us *app.UserService) error {
	w := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "main",
		Title:         "LeviLauncher",
		Width:         420,
		Height:        140,
		DisableResize: true,
		HTML: `<html><body style="font-family:Segoe UI,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">` +
			`Sign in with the Microsoft account window...</body></html>`,
	})
	deadline := time.Now().Add(10 * time.Second)
	for w.NativeWindow() == nil {
		if time.Now().After(deadline) {
			return errors.New("the sign-in window did not open")
		}
		time.Sleep(50 * time.Millisecond)
	}
	us.Attach(w)
	return nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func orNull(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

// setupCLIConsole points stdout, stderr and stdin at the parent console. The
// release build is a GUI-subsystem program, so it starts without one; streams
// the caller redirected to a file or pipe are kept. Validity is checked before
// attaching: AttachConsole fills in empty standard handles itself, but the Go
// os.Std* files were bound at startup and still wrap the empty ones.
func setupCLIConsole() {
	type stream struct {
		which uint32
		name  string
		flag  int
		file  **os.File
	}
	var missing []stream
	for _, s := range []stream{
		{win.STD_OUTPUT_HANDLE, "CONOUT$", os.O_WRONLY, &os.Stdout},
		{win.STD_ERROR_HANDLE, "CONOUT$", os.O_WRONLY, &os.Stderr},
		{win.STD_INPUT_HANDLE, "CONIN$", os.O_RDONLY, &os.Stdin},
	} {
		if !stdHandleValid(s.which) {
			missing = append(missing, s)
		}
	}
	if len(missing) == 0 {
		return
	}
	if err := attachOrAllocateConsole(); err != nil {
		return
	}
	_, _, _ = procSetConsoleOutputCP.Call(uintptr(CP_UTF8))
	for _, s := range missing {
		if f, err := os.OpenFile(s.name, s.flag, 0); err == nil {
			*s.file = f
			_ = win.SetStdHandle(s.which, win.Handle(f.Fd()))
		}
	}
}

func stdHandleValid(which uint32) bool {
	h, err := win.GetStdHandle(which)
	if err != nil || h == 0 || h == win.InvalidHandle {
		return false
	}
	t, err := win.GetFileType(h)
	return err == nil && t != win.FILE_TYPE_UNKNOWN
}
