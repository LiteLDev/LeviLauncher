package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/storage"
)

// Run executes a command-line invocation (os.Args[1:]) and returns the
// process exit code. It attaches to the parent console first.
func Run(args []string) int {
	setupConsole()
	return run(args, &Streams{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		StdinConsole: isConsole(os.Stdin), StdoutConsole: isConsole(os.Stdout), StderrConsole: isConsole(os.Stderr),
	})
}

// bootstrap prepares storage and settings like the window does, minus the
// window, WebView2 and the single-instance lock. It migrates the legacy
// layout only when no window runs and no other launcher process is
// migrating; otherwise it runs on the layout that holds the user's data.
// Replaced in tests.
var bootstrap = func() error {
	switch outcome, err := storage.PrepareShared(GUIRunning); {
	case err != nil && outcome == storage.Current:
		return Errorf(ExitFailure, "ERR_STORAGE", "the launcher data migration could not be completed or undone: %v", err)
	case outcome == storage.Legacy:
		log.Printf("storage migration deferred, running on the legacy layout %s: %v", apppath.LegacyMode(), err)
	case outcome == storage.Deferred:
		log.Printf("storage migration of legacy data deferred: %v", err)
	}
	if err := apppath.EnsureDirs(); err != nil {
		return Errorf(ExitFailure, "ERR_STORAGE", "cannot create the launcher folders: %v", err)
	}
	if _, err := config.Load(); err != nil {
		return FromError(err)
	}
	return nil
}

// interruptCancelled runs once the first Ctrl-C has cancelled the command's
// context; tests replace it to wait for that.
var interruptCancelled = func() {}

// notifyInterrupt delivers Ctrl-C; replaced in tests.
var notifyInterrupt = func(ch chan<- os.Signal) func() {
	signal.Notify(ch, os.Interrupt)
	return func() { signal.Stop(ch) }
}

// exit terminates the process on a second Ctrl-C; replaced in tests.
var exit = os.Exit

func run(args []string, s *Streams) (code int) {
	errMu := &sync.Mutex{}
	vals := newFlagValues()
	g := Globals{}
	// Errors before the command is known are printed with what is known of
	// the globals.
	fail := func(err error) int {
		readGlobals(vals, &g)
		// The parse stops at the bad word or flag: honour a --json given
		// after it, so scripts can still parse the error.
		if !g.JSON {
			g.JSON = jsonRequested(args)
		}
		return printError(s.Stderr, errMu, g, FromError(err))
	}

	t := registry()
	r, err := resolveCommand(t, args, vals)
	if err != nil {
		return fail(err)
	}

	var cmdFlags []*Flag
	if r.cmd != nil {
		cmdFlags = r.cmd.Flags
	}
	p := &parser{flags: append(append([]*Flag{}, cmdFlags...), globalFlags...), values: vals}
	positional, _, err := p.parse(r.rest, false)
	if err != nil {
		if r.cmd != nil {
			if e := FromError(err); e.Hint == "" {
				e.Hint = fmt.Sprintf("run '%s help %s'", ProgramName, r.cmd.Path)
			}
		}
		return fail(err)
	}
	readGlobals(vals, &g)

	switch {
	case vals.vals["help"] == true:
		// "instance lst -h": a verb typo, not help for the default verb.
		if r.viaDefault && len(positional) > 0 {
			if err := mistypedVerb(r.cmd.noun, positional[0], false); err != nil {
				return fail(err)
			}
		}
		switch {
		case r.cmd != nil:
			writeCommandHelp(s.Stdout, r.cmd)
		case r.noun != nil:
			writeNounHelp(s.Stdout, r.noun)
		default:
			writeOverview(s.Stdout, t)
		}
		return ExitOK
	case r.cmd == nil && r.noun != nil:
		writeNounHelp(s.Stderr, r.noun)
		return ExitUsage
	case r.cmd == nil && vals.vals["version"] == true:
		r.cmd = t.find("version")
		if r.cmd == nil {
			return fail(UsageError("no version command"))
		}
	case r.cmd == nil:
		if len(positional) > 0 {
			return fail(unknownCommand(t, positional[0], t.topWords()))
		}
		writeOverview(s.Stderr, t)
		return ExitUsage
	}

	c := r.cmd
	if len(positional) < c.MinArgs || (c.MaxArgs != Unlimited && len(positional) > c.MaxArgs) {
		// "instance remov foo" reaches the default verb with too many
		// arguments: report the mistyped verb, not the default's usage.
		if r.viaDefault && len(positional) > 0 {
			if err := mistypedVerb(c.noun, positional[0], false); err != nil {
				return fail(err)
			}
		}
		return fail(argCountError(c, len(positional)))
	}
	if err := vals.applyDefaults(c.Flags); err != nil {
		return fail(err)
	}

	// Logging: -v tees the launcher log to stderr; otherwise it is dropped.
	prevLog := log.Writer()
	if g.Verbose {
		log.SetOutput(lockedWriter{mu: errMu, w: s.Stderr})
	} else {
		log.SetOutput(io.Discard)
	}
	defer log.SetOutput(prevLog)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var interrupted atomic.Bool
	sigs := make(chan os.Signal, 2)
	stop := notifyInterrupt(sigs)
	defer stop()
	go func() {
		count := 0
		for range sigs {
			count++
			interrupted.Store(true)
			if count == 1 {
				errMu.Lock()
				fmt.Fprintln(s.Stderr, "\nInterrupted; stopping (press Ctrl-C again to force)")
				errMu.Unlock()
				cancel()
				interruptCancelled()
				continue
			}
			exit(ExitInterrupted)
		}
	}()

	cx := &Ctx{
		Cmd: c, Args: positional, G: g,
		Stdout: s.Stdout, Stderr: lockedWriter{mu: errMu, w: s.Stderr}, Stdin: s.Stdin,
		ctx: ctx, errw: s.Stderr, streams: s, values: vals, errMu: errMu,
	}
	prog := cx.Progress()
	eventsink.SetDefault(prog.Sink())
	defer eventsink.SetDefault(nil)
	defer setBackendScope(c.Path)()

	if !c.NoBootstrap {
		if err := bootstrap(); err != nil {
			return printError(s.Stderr, errMu, g, FromError(err))
		}
		// Ctrl-C during a slow bootstrap: do not start the command.
		if ctx.Err() != nil {
			return printError(s.Stderr, errMu, g, errInterrupted())
		}
	}

	err = safeRun(c, cx)
	prog.Done()
	if interrupted.Load() {
		// A command's own exit code (a batch's partial failure) is replaced
		// by the interruption.
		var ec ExitCode
		e := FromError(err)
		if e == nil || errors.Is(err, context.Canceled) || errors.As(err, &ec) {
			e = errInterrupted()
		}
		e.Exit = ExitInterrupted
		return printError(s.Stderr, errMu, g, e)
	}
	if err == nil {
		return ExitOK
	}
	var ec ExitCode
	if errors.As(err, &ec) {
		return int(ec)
	}
	return printError(s.Stderr, errMu, g, FromError(err))
}

// jsonRequested reports whether args, up to a bare "--", ask for --json
// (the last of --json, --json=<bool> and --no-json wins).
func jsonRequested(args []string) bool {
	on := false
	for _, a := range args {
		if a == "--" {
			break
		}
		name, val, hasVal := strings.Cut(a, "=")
		switch name {
		case "--json":
			on = true
			if hasVal {
				on, _ = strconv.ParseBool(val)
			}
		case "--no-json":
			on = false
		}
	}
	return on
}

func errInterrupted() *Error {
	return &Error{Code: "ERR_INTERRUPTED", Message: "interrupted", Exit: ExitInterrupted}
}

func readGlobals(v *flagValues, g *Globals) {
	str := func(k string) string { s, _ := v.vals[k].(string); return s }
	b := func(k string) bool { x, _ := v.vals[k].(bool); return x }
	g.Instance, g.Player = str("instance"), str("player")
	g.JSON, g.Quiet, g.Verbose = b("json"), b("quiet"), b("verbose")
	g.Yes, g.NoInput = b("yes"), b("no-input")
}

func argCountError(c *Command, n int) error {
	var e *Error
	switch {
	case c.MaxArgs == 0:
		e = UsageError("%s takes no arguments", c.Path)
	case n < c.MinArgs:
		e = UsageError("%s needs more arguments", c.Path)
	default:
		e = UsageError("%s got too many arguments", c.Path)
	}
	usage := ProgramName + " " + c.Path
	if c.Args != "" {
		usage += " " + c.Args
	}
	return e.WithHint("usage: %s", usage)
}

// safeRun runs the command, turning a panic into an internal error.
func safeRun(c *Command, cx *Ctx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in %s: %v\n%s", c.Path, r, debug.Stack())
			err = Errorf(ExitFailure, "ERR_INTERNAL", "internal error: %v", r).
				WithHint("please report this at https://github.com/LiteLDev/LeviLauncher/issues")
		}
	}()
	return c.Run(cx)
}

// printError writes e to stderr and returns its exit code.
func printError(w io.Writer, mu *sync.Mutex, g Globals, e *Error) int {
	if e == nil {
		return ExitOK
	}
	code := e.Exit
	if code == 0 {
		code = ExitFailure
	}
	mu.Lock()
	defer mu.Unlock()
	if g.JSON {
		body := map[string]any{"code": e.Code, "message": e.Message}
		if e.Hint != "" {
			body["hint"] = e.Hint
		}
		if g.Verbose && e.Detail != "" {
			body["detail"] = e.Detail
		}
		b, _ := json.Marshal(map[string]any{"error": body})
		fmt.Fprintf(w, "%s\n", b)
		return code
	}
	fmt.Fprintf(w, "error: %s\n", strings.TrimSpace(e.Message))
	if e.Hint != "" {
		fmt.Fprintf(w, "hint: %s\n", e.Hint)
	}
	if g.Verbose && e.Detail != "" {
		fmt.Fprintf(w, "detail: %s\n", e.Detail)
	}
	if e.Code != "" {
		fmt.Fprintf(w, "[%s]\n", e.Code)
	}
	return code
}

// lockedWriter serialises writes with the stderr lock.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
