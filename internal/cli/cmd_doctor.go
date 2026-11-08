package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	winreg "github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/webview2runtime"
)

// Check statuses.
const (
	doctorOK      = "ok"
	doctorWarn    = "warn"
	doctorMissing = "missing"
	doctorError   = "error"
)

// Fixes `doctor --fix` can apply.
const (
	fixVCRuntime       = "vcruntime"
	fixGameInput       = "gameinput"
	fixLoaderMigration = "loader-migration"
	fixDeveloperMode   = "developer-mode"
)

var doctorFixes = []string{fixVCRuntime, fixGameInput, fixLoaderMigration, fixDeveloperMode}

// gamingServicesStoreURL opens Gaming Services in the Microsoft Store.
const gamingServicesStoreURL = "ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN"

// doctorCheck is one line of `doctor` (an element of its --json array).
type doctorCheck struct {
	Check    string `json:"check"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix"`  // the --fix name that repairs it, or ""
	Hint     string `json:"hint"` // what to do when it cannot be fixed here
	Fixed    bool   `json:"fixed"`
}

func (d doctorCheck) failed() bool { return d.Status != doctorOK }

// doc is the system the checks and fixes look at; tests replace it.
var doc = struct {
	vcInstalled    func() bool
	giInstalled    func() bool
	gsInstalled    func() bool
	devMode        func() bool
	needsMigration func() bool
	canWrite       func(string) bool
	baseRoot       func() string
	accountSaved   func() bool
	elevated       func() bool
	webview2       func() bool
	sleep          func(time.Duration)
	fixVC          func(ctx context.Context)
	fixGI          func(ctx context.Context)
	fixDevMode     func() bool
	migrate        func(ctx context.Context) (int, error)
}{
	vcInstalled:    vcruntime.IsInstalled,
	giInstalled:    gameinput.IsInstalled,
	gsInstalled:    func() bool { _, err := winreg.GetAppxInfo("Microsoft.GamingServices"); return err == nil },
	devMode:        winreg.IsDevModeEnabled,
	needsMigration: leviloader.NeedsMigration,
	canWrite:       utils.CanWriteDir,
	baseRoot:       apppath.BaseRoot,
	accountSaved:   func() bool { return acct.hasSaved() },
	elevated:       processinfo.IsElevated,
	webview2:       webview2runtime.IsInstalled,
	sleep:          time.Sleep,
	fixVC:          vcruntime.EnsureInteractive,
	fixGI:          gameinput.EnsureInteractive,
	fixDevMode:     winreg.TryEnableDevMode,
	migrate:        leviloader.RunMigration,
}

func init() {
	Register(&Command{
		Path:    "doctor",
		Summary: "check what Minecraft and mods need on this PC; --fix repairs it",
		Args:    "[--fix [vcruntime|gameinput|loader-migration|developer-mode]...]",
		Description: `Checks the Visual C++ runtime, GameInput and Gaming Services (needed by GDK
instances), Developer Mode (needed to register UWP instances), the one-time
loader migration, the data folder, the Microsoft account, administrator rights
and the WebView2 runtime (needed by the launcher window only). With -i it also
checks one instance's game files.

--fix repairs the failed checks your instances need and the loader migration,
or only the fixes named (name developer-mode to turn it on without a UWP
instance); each fix asks first. vcruntime, gameinput and developer-mode need
administrator approval.
Gaming Services cannot be installed from here: open the Store page the hint
names.

Exit status: 0 when every required check passes, 1 when one fails, 8 when a
fix needed administrator approval that was not given.`,
		Examples: []string{"doctor", "doctor --fix", "doctor --fix gameinput -i Survival"},
		Flags:    []*Flag{BoolFlag("fix", "", "repair failed checks (all, or the ones named)")},
		MaxArgs:  Unlimited,
		Group:    GroupSystem,
		Run:      runDoctor,
	})
}

// doctorScope is what the checks are for: one instance (-i), or this PC with
// its installed instances.
type doctorScope struct {
	inst    *Instance
	gdk     bool // GDK prerequisites are required
	uwp     bool // Developer Mode is required
	running []string
}

func doctorScopeFor(c *Ctx) (doctorScope, error) {
	var s doctorScope
	if strings.TrimSpace(c.G.Instance) != "" {
		inst, err := c.ResolveInstance("")
		if err != nil {
			return s, err
		}
		s.inst = &inst
		s.gdk, s.uwp = !inst.IsUWP(), inst.IsUWP()
		return s, nil
	}
	all, err := ListInstances()
	if err != nil {
		return s, err
	}
	for _, inst := range all {
		if inst.IsUWP() {
			s.uwp = true
		} else {
			s.gdk = true
		}
	}
	return s, nil
}

func doctorRunChecks(s doctorScope) []doctorCheck {
	var out []doctorCheck
	add := func(d doctorCheck) { out = append(out, d) }

	vc := doc.vcInstalled()
	if !vc {
		doc.sleep(800 * time.Millisecond) // the window retries once too
		vc = doc.vcInstalled()
	}
	add(presence("vcruntime", "Visual C++ runtime", vc, true, fixVCRuntime, ""))

	gdkNote := ""
	if !s.gdk {
		gdkNote = "needed by GDK instances"
	}
	gi := presence("gameinput", "GameInput", doc.giInstalled(), s.gdk, fixGameInput, "")
	gi.Detail = joinDetail(gi.Detail, gdkNote)
	add(gi)
	gs := presence("gaming-services", "Gaming Services", doc.gsInstalled(), s.gdk, "",
		"install it from the Microsoft Store: start "+gamingServicesStoreURL)
	gs.Detail = joinDetail(gs.Detail, gdkNote)
	add(gs)

	dm := doctorCheck{Check: "developer-mode", Name: "Developer Mode", Status: doctorOK, Required: s.uwp}
	if !doc.devMode() {
		dm.Status, dm.Fix = doctorWarn, fixDeveloperMode
		dm.Detail = "off; needed to register UWP instances"
		dm.Hint = "or turn it on in Settings: start ms-settings:developers"
	}
	add(dm)

	lm := doctorCheck{Check: "loader-migration", Name: "Loader migration", Status: doctorOK, Detail: "done"}
	if doc.needsMigration() {
		lm.Status, lm.Fix = doctorWarn, fixLoaderMigration
		lm.Detail = "the one-time move to the new mod loader setup has not run"
	}
	add(lm)

	root := doc.baseRoot()
	dr := doctorCheck{Check: "data-root", Name: "Data folder", Status: doctorOK, Required: true, Detail: root}
	if !doc.canWrite(root) {
		dr.Status = doctorError
		dr.Detail = "cannot write to " + root
		dr.Hint = ProgramName + " config set data-root <folder>"
	}
	add(dr)

	ac := doctorCheck{Check: "account", Name: "Microsoft account", Status: doctorOK, Detail: "signed in"}
	if !doc.accountSaved() {
		ac.Status, ac.Detail = doctorWarn, "not signed in; needed to download Minecraft"
		ac.Hint = ProgramName + " account login"
	}
	add(ac)

	add(doctorCheck{Check: "elevation", Name: "Administrator", Status: doctorOK, Detail: map[bool]string{
		true: "running as administrator", false: "not running as administrator"}[doc.elevated()]})

	wv := doctorCheck{Check: "webview2", Name: "WebView2 runtime", Status: doctorOK, Detail: "needed by the launcher window only"}
	if !doc.webview2() {
		wv.Status = doctorWarn
		wv.Detail = "missing; the launcher window installs it when it starts"
	}
	add(wv)

	if s.inst != nil {
		add(doctorInstanceFiles(*s.inst))
		lv := LoaderVersion(*s.inst)
		ll := doctorCheck{Check: "loader", Name: "LeviLamina", Status: doctorOK, Detail: "not installed"}
		if lv != "" {
			ll.Detail = lv
		}
		add(ll)
	}
	return out
}

func joinDetail(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}

func presence(check, name string, ok, required bool, fix, hint string) doctorCheck {
	d := doctorCheck{Check: check, Name: name, Status: doctorOK, Required: required}
	if !ok {
		d.Status, d.Fix, d.Hint = doctorMissing, fix, hint
	}
	return d
}

// doctorInstanceFiles checks that the instance's game executable exists.
func doctorInstanceFiles(inst Instance) doctorCheck {
	d := doctorCheck{Check: "instance", Name: "Instance " + inst.Name, Status: doctorOK, Required: true}
	exe := filepath.Join(inst.Dir, "Minecraft.Windows.exe")
	if inst.IsUWP() {
		m, err := uwp.ReadManifest(inst.Dir)
		if err != nil || len(m.Applications) == 0 {
			d.Status, d.Detail = doctorError, "AppxManifest.xml is missing or invalid"
			d.Hint = "reinstall the instance"
			return d
		}
		exe = filepath.Join(inst.Dir, m.Applications[0].Executable)
	}
	if fi, err := os.Stat(exe); err != nil || fi.IsDir() {
		d.Status, d.Detail = doctorError, "the game executable is missing: "+exe
		d.Hint = "reinstall the instance"
		return d
	}
	d.Detail = fmt.Sprintf("%s %s, %s", inst.PackageType, Dash(inst.Meta.GameVersion), inst.Dir)
	return d
}

func parseDoctorFixes(c *Ctx) ([]string, error) {
	if !c.Bool("fix") {
		if len(c.Args) > 0 {
			return nil, UsageError("unexpected argument %q", c.Arg(0)).WithHint("%s doctor --fix %s", ProgramName, c.Arg(0))
		}
		return nil, nil
	}
	want := map[string]bool{}
	for _, a := range c.Args {
		n := strings.ToLower(strings.TrimSpace(a))
		ok := false
		for _, f := range doctorFixes {
			if n == f {
				ok = true
			}
		}
		if !ok {
			e := UsageError("unknown fix %q: use %s", a, strings.Join(doctorFixes, ", "))
			if s := suggest(n, doctorFixes); s != "" {
				e = e.WithHint("did you mean %q?", s)
			}
			return nil, e
		}
		want[n] = true
	}
	if len(want) == 0 {
		for _, f := range doctorFixes {
			want[f] = true
		}
	}
	var out []string
	for _, f := range doctorFixes {
		if want[f] {
			out = append(out, f)
		}
	}
	return out, nil
}

// errFixDeclined reports a fix that needed administrator approval.
func errFixDeclined(what string) *Error {
	return Errorf(ExitSystem, "ERR_ELEVATION_DECLINED", "%s needs administrator approval, which was not given", what).
		WithHint("run '%s doctor --fix' again and accept the Windows prompt", ProgramName)
}

// runInstallerFix runs a vcruntime/gameinput EnsureInteractive, rendering its
// events, and reports a download error or a declined prompt.
func runInstallerFix(c *Ctx, prefix string, run func(context.Context)) (downloadErr string) {
	var mu sync.Mutex
	next := c.Progress().Sink()
	ctx := eventsink.With(c.Context(), eventsink.Func(func(name string, data ...any) {
		if name == prefix+".download.error" && len(data) > 0 {
			mu.Lock()
			downloadErr = fmt.Sprint(data[0])
			mu.Unlock()
		}
		next.Emit(name, data...)
	}))
	run(ctx)
	c.Progress().Done()
	mu.Lock()
	defer mu.Unlock()
	return downloadErr
}

func isDeclined(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "cancel") || strings.Contains(m, "1223") || strings.Contains(m, "denied")
}

// applyFix runs one fix after asking; it returns an error only for a refusal
// that ends the command (declined prompt, game running, interrupted).
func applyFix(c *Ctx, fix string) error {
	switch fix {
	case fixVCRuntime:
		if err := c.Confirm("Install the Microsoft Visual C++ runtime (needs administrator approval)?"); err != nil {
			return err
		}
		c.Progress().Phase("Installing the Visual C++ runtime")
		if msg := runInstallerFix(c, "vcruntime", doc.fixVC); msg != "" {
			if isDeclined(msg) {
				return errFixDeclined("installing the Visual C++ runtime")
			}
			c.Warn("installing the Visual C++ runtime failed: %s", msg)
		}
	case fixGameInput:
		if err := c.Confirm("Install GameInput (needs administrator approval)?"); err != nil {
			return err
		}
		c.Progress().Phase("Installing GameInput")
		if msg := runInstallerFix(c, "gameinput", doc.fixGI); msg != "" {
			c.Warn("installing GameInput failed: %s", msg)
		}
	case fixDeveloperMode:
		if err := c.Confirm("Turn on Windows Developer Mode (needs administrator approval)?"); err != nil {
			return err
		}
		if !doc.fixDevMode() {
			return errFixDeclined("turning on Developer Mode")
		}
	case fixLoaderMigration:
		if games := suGameRunning(); len(games) > 0 {
			return ConflictError("Minecraft is running (%s)", strings.Join(games, ", ")).
				WithHint("%s stop --all", ProgramName).withCode("ERR_GAME_RUNNING")
		}
		if err := c.Confirm("Migrate the instances to the new mod loader setup (patches their game executables)?"); err != nil {
			return err
		}
		n, err := doc.migrate(c.Context())
		c.Progress().Done()
		if err != nil {
			if c.Context().Err() != nil {
				return c.Context().Err()
			}
			c.Warn("the loader migration failed: %v", err)
		} else {
			c.Verbosef("migrated %d instances", n)
		}
	}
	return c.Context().Err()
}

func runDoctor(c *Ctx) error {
	fixes, err := parseDoctorFixes(c)
	if err != nil {
		return err
	}
	scope, err := doctorScopeFor(c)
	if err != nil {
		return err
	}
	prog := c.Progress()
	prog.Phase("Checking")
	checks := doctorRunChecks(scope)
	prog.Done()

	if len(fixes) > 0 {
		wanted := map[string]bool{}
		for _, f := range fixes {
			wanted[f] = true
		}
		applied := map[string]bool{}
		named := len(c.Args) > 0
		for _, ch := range checks {
			if !ch.failed() || ch.Fix == "" || !wanted[ch.Fix] || applied[ch.Fix] {
				continue
			}
			// A bare --fix repairs what this PC's instances need; a
			// system change nothing needs (Developer Mode without a UWP
			// instance, GameInput without a GDK one) must be named.
			if !named && !ch.Required && ch.Fix != fixLoaderMigration {
				continue
			}
			applied[ch.Fix] = true
			if err := applyFix(c, ch.Fix); err != nil {
				return err
			}
		}
		if len(applied) == 0 {
			c.Notice("Nothing to fix")
		} else {
			prog.Phase("Checking again")
			after := doctorRunChecks(scope)
			prog.Done()
			for i := range after {
				if i < len(checks) && checks[i].failed() && !after[i].failed() && applied[checks[i].Fix] {
					after[i].Fixed = true
				}
			}
			checks = after
		}
	}

	var failedNames []string
	for _, ch := range checks {
		if ch.Required && ch.failed() {
			failedNames = append(failedNames, ch.Name)
		}
	}
	failed := len(failedNames)
	ids := make([]string, 0, len(checks))
	for _, ch := range checks {
		if ch.failed() {
			ids = append(ids, ch.Check)
		}
	}
	if err := c.Render(checks, ids, func(w io.Writer) error { return writeDoctor(w, checks) }); err != nil {
		return err
	}
	if failed > 0 {
		if c.G.JSON || c.G.Quiet {
			return ExitCode(ExitFailure)
		}
		e := Errorf(ExitFailure, "ERR_DOCTOR_FAILED", "%d required check(s) failed: %s", failed, strings.Join(failedNames, ", "))
		for _, ch := range checks {
			if ch.Required && ch.failed() && ch.Fix != "" {
				return e.WithHint("%s doctor --fix", ProgramName)
			}
		}
		return e
	}
	return nil
}

func writeDoctor(w io.Writer, checks []doctorCheck) error {
	for _, ch := range checks {
		line := ch.Name
		if ch.Detail != "" {
			line += ": " + ch.Detail
		}
		if ch.Fixed {
			line += " (fixed)"
		}
		var notes []string
		if ch.failed() && ch.Fix != "" {
			notes = append(notes, "fix: "+ProgramName+" doctor --fix "+ch.Fix)
		}
		if ch.failed() && ch.Hint != "" {
			notes = append(notes, ch.Hint)
		}
		if len(notes) > 0 {
			line += " (" + strings.Join(notes, "; ") + ")"
		}
		status := ch.Status
		if ch.Required && status == doctorWarn {
			status = "fail" // an instance needs it, so it is not just a warning
		}
		if _, err := fmt.Fprintf(w, "%-10s%s\n", "["+status+"]", line); err != nil {
			return err
		}
	}
	return nil
}
