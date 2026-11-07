package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	winreg "github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
)

// Seams replaced by tests: starting the game and reading system state.
var (
	launchGame = func(ctx context.Context, name string, checkRunning bool) string {
		return versionlaunch.New(false).Launch(ctx, name, checkRunning)
	}
	launchGamePIDs       = launch.GameProcessIDs
	launchWindowUp       = launch.GameWindowVisible
	launchWaitPID        = launch.WaitPIDExit
	launchWaitGameExit   = launch.WaitGameExit
	launchNeedsMigration = leviloader.NeedsMigration
	launchRunMigration   = leviloader.RunMigration
	launchVCInstalled    = vcruntime.IsInstalled
	launchGIInstalled    = gameinput.IsInstalled
	launchGSInstalled    = func() bool { _, err := winreg.GetAppxInfo("Microsoft.GamingServices"); return err == nil }
	launchSleep          = func(ctx context.Context, d time.Duration) {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
		case <-t.C:
		}
	}
	// launchPollInterval is how often --detach looks for the game process.
	launchPollInterval = 250 * time.Millisecond
)

// launchGSStoreURL opens the Gaming Services page of the Store.
const launchGSStoreURL = "ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN"

func init() {
	RegisterBackendErrors(map[string]BackendError{
		"ERR_UWP_LAUNCH": {Exit: ExitFailure, Message: "Windows could not start the UWP game"},
		"ERR_UWP_PREPARE": {Exit: ExitFailure, Message: "could not prepare the UWP game files for registration",
			Hint: "run again with -v for details, or check that the instance folder is writable"},
	})
	Register(&Command{
		Path:    "launch",
		Aliases: []string{"play"},
		Summary: "start the game and wait until its window is up",
		Args:    "[<instance>]",
		Description: `Starts Minecraft from an instance (default: the default instance) and waits
until the game window is visible. A UWP instance that is not registered
with Windows is registered first; replacing another registration needs
confirmation or --yes. Missing prerequisites (Visual C++ runtime, GameInput,
Gaming Services) are reported as warnings and never block the launch.

With --wait-exit the command stays until the game exits and returns the
game's own exit code. Ctrl-C stops waiting; the game keeps running.`,
		Examples: []string{"play", "launch Survival --wait-exit", "play 1.21.50 --detach -q"},
		Flags: []*Flag{
			BoolFlag("force", "", "start a second copy even if this instance's game is running (GDK only)"),
			BoolFlag("detach", "", "return as soon as the game process exists"),
			BoolFlag("wait-exit", "", "stay until the game exits and return its exit code"),
			DurationFlag("timeout", "", "how long to wait for the game to start").WithDefault("3m"),
		},
		MaxArgs: 1,
		Run:     runLaunch,
	})
}

// launchResult is the JSON document of launch.
type launchResult struct {
	Instance    string `json:"instance"`
	PID         *int   `json:"pid"`
	PackageType string `json:"packageType"`
	Channel     string `json:"channel"`
	Registered  bool   `json:"registered"`
	EditorMode  bool   `json:"editorMode"`
	State       string `json:"state"` // started, running, exited
	ExitCode    *int   `json:"exitCode,omitempty"`
	DurationMs  int64  `json:"durationMs"`
}

func runLaunch(c *Ctx) error {
	force, detach, waitExit := c.Bool("force"), c.Bool("detach"), c.Bool("wait-exit")
	if detach && waitExit {
		return UsageError("--detach and --wait-exit contradict each other")
	}
	timeout := c.Duration("timeout")
	if timeout <= 0 {
		return UsageError("--timeout must be positive")
	}
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	if force && inst.IsUWP() {
		return Errorf(ExitConflict, "ERR_FORCE_UNSUPPORTED", "--force is not supported for UWP instances: Windows runs one copy of a UWP game").
			WithHint("%s stop %s", ProgramName, quoteArg(inst.Name))
	}
	if !force {
		if pids := launchGamePIDs(inst.Dir); len(pids) > 0 {
			e := Errorf(ExitConflict, "ERR_GAME_ALREADY_RUNNING", "Minecraft is already running from %s (pid %s)", inst.Name, launchJoinPIDs(pids))
			if inst.IsUWP() {
				return e.WithHint("%s stop %s", ProgramName, quoteArg(inst.Name))
			}
			return e.WithHint("%s stop %s, or start a second copy with --force", ProgramName, quoteArg(inst.Name))
		}
	}

	launchMigrateLoader(c)
	launchAdvisories(c, inst)
	// With --force the copies already running are not this launch: the
	// window and PID reported must be those of the new copy.
	var before map[uint32]bool
	if force {
		before = map[uint32]bool{}
		for _, pid := range launchGamePIDs(inst.Dir) {
			before[pid] = true
		}
	}

	res := launchResult{Instance: inst.Name, PackageType: inst.PackageType, Channel: inst.Channel,
		Registered: inst.Meta.Registered, EditorMode: inst.Meta.EnableEditorMode}
	registeredNow := false
	if inst.IsUWP() {
		reg, err := instRegisterFlow(c, inst)
		if err != nil {
			return err
		}
		res.Registered, registeredNow = reg.Registered, reg.Changed
		if reg.Changed {
			c.Notice("Registered %s as Minecraft (%s)", inst.Name, reg.Channel)
		}
	}

	start := time.Now()
	p := c.Progress()
	p.Phase("%s", launchDescription(inst))
	// The progress line keeps this description: the generic "Starting
	// Minecraft" phase of the start event is not rendered over it.
	next := c.Progress().Sink()
	filtered := eventsink.Func(func(name string, data ...any) {
		if name == launch.EventMcLaunchStart {
			return
		}
		next.Emit(name, data...)
	})
	// The monitor's own limits follow --timeout: either phase may use all
	// of it, so a window that comes late after a fast process start is still
	// waited for; the wait below bounds the total.
	monitorCtx := launch.WithMonitorTimeouts(c.Context(), timeout, timeout)
	ctx, wait := launch.Watch(monitorCtx, filtered)
	code := launchGame(ctx, inst.Name, !force)
	if bc, _, _ := SplitBackendCode(code); bc == "ERR_UWP_NOT_REGISTERED" && inst.IsUWP() && !registeredNow {
		// The registration was lost since it was checked: register again
		// and retry once, as the window's retry does.
		p.Done()
		reg, err := instRegisterFlow(c, inst)
		if err != nil {
			return err
		}
		res.Registered = reg.Registered
		ctx, wait = launch.Watch(monitorCtx, filtered)
		p.Phase("%s", launchDescription(inst))
		code = launchGame(ctx, inst.Name, !force)
	}
	if e := launchError(inst, code); e != nil {
		return e
	}
	if err := c.Context().Err(); err != nil {
		return err
	}

	if detach {
		pid, err := launchWaitProcess(c, inst, before, wait, start.Add(timeout))
		p.Done()
		if err != nil {
			return err
		}
		res.State, res.PID = "started", pid
		res.DurationMs = time.Since(start).Milliseconds()
		if pid != nil {
			c.Notice("Minecraft started (pid %d)", *pid)
		} else {
			c.Notice("Minecraft started")
		}
		return c.Render(res, launchPIDs(res.PID), nil)
	}

	p.Phase("Waiting for the game window")
	if len(before) > 0 {
		// The monitor sees the running copy's window at once: follow the
		// new copy instead.
		pid, err := launchWaitNewWindow(c, inst, before, start, timeout)
		p.Done()
		if err != nil {
			return err
		}
		res.PID = pid
	} else {
		out, err := wait(timeout)
		p.Done()
		switch {
		case errors.Is(err, launch.ErrWaitTimeout):
			return launchTimeoutError(timeout)
		case err != nil:
			return err
		case out.Code != "":
			return launchError(inst, out.Code)
		}
		if pids := launchGamePIDs(inst.Dir); len(pids) > 0 {
			pid := int(pids[0])
			res.PID = &pid
		}
	}
	res.State = "running"
	res.DurationMs = time.Since(start).Milliseconds()
	if res.PID != nil {
		c.Notice("Game window is up (pid %d, %.1fs)", *res.PID, time.Since(start).Seconds())
	} else {
		c.Notice("Game window is up (%.1fs)", time.Since(start).Seconds())
	}
	if !waitExit {
		return c.Render(res, launchPIDs(res.PID), nil)
	}

	c.Notice("Waiting for Minecraft to exit (Ctrl-C stops waiting; the game keeps running)")
	exitCode, known, err := launchWaitExit(c.Context(), inst, res.PID)
	if err != nil {
		return err
	}
	res.State = "exited"
	res.DurationMs = time.Since(start).Milliseconds()
	if known {
		res.ExitCode = &exitCode
		c.Notice("Minecraft exited with code %d", exitCode)
	} else {
		c.Notice("Minecraft exited")
	}
	if err := c.Render(res, launchPIDs(res.PID), nil); err != nil {
		return err
	}
	if known && exitCode != 0 {
		return ExitCode(exitCode)
	}
	return nil
}

// launchDescription is the progress line of a launch:
// "Starting Minecraft 1.21.50.07 (GDK, release, LeviLamina 1.4.2)".
func launchDescription(inst Instance) string {
	parts := []string{strings.ToUpper(inst.PackageType)}
	if inst.Channel != "" {
		parts = append(parts, inst.Channel)
	}
	if v := LoaderVersion(inst); v != "" {
		parts = append(parts, "LeviLamina "+v)
	}
	if inst.Meta.EnableEditorMode {
		parts = append(parts, "editor mode")
	}
	gv := inst.Meta.GameVersion
	if gv == "" {
		gv = inst.Name
	}
	return fmt.Sprintf("Starting Minecraft %s (%s)", gv, strings.Join(parts, ", "))
}

// launchError maps a backend launch result to an error with hints.
func launchError(inst Instance, code string) *Error {
	e := Backend(code)
	if e == nil {
		return nil
	}
	name := quoteArg(inst.Name)
	switch e.Code {
	case "ERR_GAME_ALREADY_RUNNING":
		e.Message = fmt.Sprintf("Minecraft is already running from %s", inst.Name)
		if inst.IsUWP() {
			e.Hint = fmt.Sprintf("%s stop %s", ProgramName, name)
		} else {
			e.Hint = fmt.Sprintf("%s stop %s, or start a second copy with --force", ProgramName, name)
		}
	case "ERR_UWP_NOT_REGISTERED":
		e.Hint = fmt.Sprintf("%s instance register %s", ProgramName, name)
	case "ERR_LAUNCH_GAME", "ERR_UWP_LAUNCH":
		e.Hint = fmt.Sprintf("run again with -v for details, or check the mods of the instance: %s mod list -i %s", ProgramName, name)
	case "ERR_LAUNCH_TIMEOUT":
		e.Hint = fmt.Sprintf("the game may still be starting; check with: %s ps", ProgramName)
	case "ERR_NOT_FOUND_EXE":
		e.Message = fmt.Sprintf("the game executable is missing from %s", inst.Name)
	}
	return e
}

func launchTimeoutError(timeout time.Duration) *Error {
	return Backend("ERR_LAUNCH_TIMEOUT").WithDetail(fmt.Sprintf("no game window after %s", timeout)).
		WithHint("the game may still be starting; check with: %s ps", ProgramName)
}

// launchNewPIDs returns the game PIDs of inst that are not in before.
func launchNewPIDs(inst Instance, before map[uint32]bool) []uint32 {
	var out []uint32
	for _, pid := range launchGamePIDs(inst.Dir) {
		if !before[pid] {
			out = append(out, pid)
		}
	}
	return out
}

// launchPoll sleeps one poll interval, or returns ctx.Err() when ctx ends.
func launchPoll(ctx context.Context) error {
	t := time.NewTimer(launchPollInterval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// launchWaitProcess waits until a game process of inst that is not in
// before exists (--detach), the monitor reports a failure, or deadline
// passes.
func launchWaitProcess(c *Ctx, inst Instance, before map[uint32]bool, wait func(time.Duration) (launch.Outcome, error), deadline time.Time) (*int, error) {
	for {
		if pids := launchNewPIDs(inst, before); len(pids) > 0 {
			pid := int(pids[0])
			return &pid, nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil, Backend("ERR_LAUNCH_TIMEOUT").WithDetail("no game process appeared")
		}
		out, err := wait(min(launchPollInterval, left))
		switch {
		case errors.Is(err, launch.ErrWaitTimeout):
			continue
		case err != nil:
			return nil, err
		case out.Code != "":
			return nil, launchError(inst, out.Code)
		case out.Up && len(before) > 0:
			// The monitor saw a copy that was already running: keep
			// looking for the new one.
			if err := launchPoll(c.Context()); err != nil {
				return nil, err
			}
		case out.Up:
			// The window is up already; the process exists even when it
			// was not found by path (it is reported once found).
			if pids := launchGamePIDs(inst.Dir); len(pids) > 0 {
				pid := int(pids[0])
				return &pid, nil
			}
			return nil, nil
		}
	}
}

// launchWaitNewWindow waits until a game process of inst that is not in
// before shows its window. It fails when that process exits first or
// timeout passes since start.
func launchWaitNewWindow(c *Ctx, inst Instance, before map[uint32]bool, start time.Time, timeout time.Duration) (*int, error) {
	deadline := start.Add(timeout)
	seen := false
	for {
		pids := launchNewPIDs(inst, before)
		switch {
		case len(pids) > 0 && launchWindowUp(pids):
			pid := int(pids[0])
			return &pid, nil
		case len(pids) > 0:
			seen = true
		case seen:
			return nil, launchError(inst, "ERR_LAUNCH_GAME").WithDetail("the new game process exited")
		}
		if !time.Now().Before(deadline) {
			return nil, launchTimeoutError(timeout)
		}
		if err := launchPoll(c.Context()); err != nil {
			return nil, err
		}
	}
}

// launchWaitExit follows the game until it exits. With exactly one game
// process it returns that process's exit code; otherwise it waits for every
// process of the instance and the code is unknown.
func launchWaitExit(ctx context.Context, inst Instance, pid *int) (int, bool, error) {
	pids := launchGamePIDs(inst.Dir)
	if len(pids) == 1 {
		code, err := launchWaitPID(ctx, pids[0])
		if err == nil {
			// Another copy (--force) may still run; wait for all of them.
			if err := launchWaitGameExit(ctx, inst.Dir); err != nil {
				return 0, false, err
			}
			return int(int32(code)), true, nil
		}
		if ctx.Err() != nil {
			return 0, false, ctx.Err()
		}
	}
	if err := launchWaitGameExit(ctx, inst.Dir); err != nil {
		return 0, false, err
	}
	return 0, false, nil
}

func launchPIDs(pid *int) []string {
	if pid == nil {
		return nil
	}
	return []string{fmt.Sprint(*pid)}
}

func launchJoinPIDs(n []uint32) string {
	s := make([]string, len(n))
	for i, v := range n {
		s[i] = fmt.Sprint(v)
	}
	return strings.Join(s, ",")
}

// launchMigrateLoader runs the one-time loader migration when it is still
// pending, unless a game of the launcher is running (it rewrites game
// executables). Failure is a warning: this launch patches its own instance.
func launchMigrateLoader(c *Ctx) {
	if !launchNeedsMigration() {
		return
	}
	for _, p := range minecraftProcesses() {
		if p.IsLauncher {
			c.Verbosef("Skipping the one-time loader migration while Minecraft is running")
			return
		}
	}
	p := c.Progress()
	p.Phase("Migrating the loader of every instance (one time)")
	_, err := launchRunMigration(c.Context())
	p.Done()
	if err != nil && c.Context().Err() == nil {
		c.Warn("the one-time loader migration failed: %v", err)
	}
}

// launchAdvisory is one missing prerequisite.
type launchAdvisory struct {
	what, fix string
}

// launchCheckAdvisories ports the window's launch-time dependency checks:
// the Visual C++ runtime for every instance (rechecked once after 800 ms,
// as the window does), GameInput and Gaming Services for GDK instances.
// They are advisory and never block a launch.
func launchCheckAdvisories(ctx context.Context, inst Instance) []launchAdvisory {
	var (
		wg               sync.WaitGroup
		vc, gi, gs       = true, true, true
		checkGDK         = !inst.IsUWP()
		vcAdvice, others []launchAdvisory
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		vc = launchVCInstalled()
		if !vc {
			launchSleep(ctx, 800*time.Millisecond)
			vc = launchVCInstalled()
		}
	}()
	if checkGDK {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gi = launchGIInstalled()
			gs = launchGSInstalled()
		}()
	}
	wg.Wait()
	if !vc {
		vcAdvice = append(vcAdvice, launchAdvisory{"the Visual C++ runtime is not installed; the game or its mods may fail to start",
			ProgramName + " doctor --fix vcruntime"})
	}
	if !gi {
		others = append(others, launchAdvisory{"GameInput is not installed; controllers and some input may not work",
			ProgramName + " doctor --fix gameinput"})
	}
	if !gs {
		others = append(others, launchAdvisory{"Gaming Services is not installed; signing in and the game may not work",
			"install it from the Microsoft Store: " + launchGSStoreURL})
	}
	return append(vcAdvice, others...)
}

func launchAdvisories(c *Ctx, inst Instance) {
	for _, a := range launchCheckAdvisories(c.Context(), inst) {
		c.Warn("%s (fix: %s)", a.what, a.fix)
	}
}
