package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// launchFakes stands in for the game: launch emits the monitor's events
// like the real launcher does, and the process list is a variable.
type launchFakes struct {
	*instFakes
	mu         sync.Mutex
	pids       map[string][]uint32 // instance dir -> game pids
	launches   []string
	outcome    string // "" (window up), an ERR_ code for mc.launch.failed, "none"
	code       string // synchronous result of Launch
	startPID   uint32
	exitCode   uint32
	migrated   bool
	vc, gi, gs bool
}

func newLaunchFakes(t *testing.T) *launchFakes {
	f := &launchFakes{instFakes: newInstFakes(t), pids: map[string][]uint32{}, startPID: 4242, vc: true, gi: true, gs: true}
	instSave(f.instFakes, &launchGame)
	instSave(f.instFakes, &launchGamePIDs)
	instSave(f.instFakes, &launchWindowUp)
	instSave(f.instFakes, &launchWaitPID)
	instSave(f.instFakes, &launchWaitGameExit)
	instSave(f.instFakes, &launchNeedsMigration)
	instSave(f.instFakes, &launchRunMigration)
	instSave(f.instFakes, &launchVCInstalled)
	instSave(f.instFakes, &launchGIInstalled)
	instSave(f.instFakes, &launchGSInstalled)
	instSave(f.instFakes, &launchSleep)
	instSave(f.instFakes, &launchPollInterval)
	launchPollInterval = 5 * time.Millisecond
	launchGame = func(ctx context.Context, name string, checkRunning bool) string {
		f.mu.Lock()
		f.launches = append(f.launches, fmt.Sprintf("%s check=%v", name, checkRunning))
		code, outcome := f.code, f.outcome
		f.mu.Unlock()
		if code != "" {
			return code
		}
		inst, _ := lookupIn(f.h.instances, name)
		sink := eventsink.From(ctx)
		sink.Emit(launch.EventMcLaunchStart, struct{}{})
		f.mu.Lock()
		f.pids[inst.Dir] = append(f.pids[inst.Dir], f.startPID)
		f.mu.Unlock()
		go func() {
			switch outcome {
			case "":
				sink.Emit(launch.EventMcLaunchDone, struct{}{})
			case "none":
			default:
				sink.Emit(launch.EventMcLaunchFailed, outcome)
			}
		}()
		return ""
	}
	launchGamePIDs = func(dir string) []uint32 {
		f.mu.Lock()
		defer f.mu.Unlock()
		return append([]uint32(nil), f.pids[dir]...)
	}
	launchWindowUp = func([]uint32) bool { return true }
	launchWaitPID = func(ctx context.Context, pid uint32) (uint32, error) { return f.exitCode, nil }
	launchWaitGameExit = func(context.Context, string) error { return nil }
	launchNeedsMigration = func() bool { return !f.migrated }
	launchRunMigration = func(context.Context) (int, error) { f.migrated = true; return 1, nil }
	launchVCInstalled = func() bool { return f.vc }
	launchGIInstalled = func() bool { return f.gi }
	launchGSInstalled = func() bool { return f.gs }
	launchSleep = func(context.Context, time.Duration) {}
	return f
}

func TestLaunchWaitsForWindow(t *testing.T) {
	f := newLaunchFakes(t)
	f.add("Survival", "1.21.50.07", "release", "gdk", false)
	code, out, errOut := f.h.run("play", "--json")
	var res launchResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil {
		t.Fatalf("play: %d %q %s", code, out, errOut)
	}
	if res.Instance != "Survival" || res.State != "running" || res.PID == nil || *res.PID != 4242 || res.PackageType != "gdk" {
		t.Fatalf("result = %+v", res)
	}
	if !f.migrated || len(f.launches) != 1 || f.launches[0] != "Survival check=true" {
		t.Fatalf("migration %v launches %v", f.migrated, f.launches)
	}
	// Running already: refused without --force, a second copy with it.
	code, _, errOut = f.h.run("launch", "Survival")
	if code != ExitConflict || !strings.Contains(errOut, "--force") || !strings.Contains(errOut, "[ERR_GAME_ALREADY_RUNNING]") {
		t.Fatalf("already running: %d %s", code, errOut)
	}
	// The second copy is reported, not the one already running.
	f.startPID = 5151
	code, out, _ = f.h.run("launch", "Survival", "--force", "-q")
	if code != 0 || out != "5151\n" || f.launches[1] != "Survival check=false" {
		t.Fatalf("force: %d %q %v", code, out, f.launches)
	}
	f.startPID = 6161
	code, out, _ = f.h.run("launch", "Survival", "--force", "--detach", "-q")
	if code != 0 || out != "6161\n" {
		t.Fatalf("force detach: %d %q", code, out)
	}
}

func TestLaunchForceNewCopyFails(t *testing.T) {
	f := newLaunchFakes(t)
	inst := f.add("Survival", "1.21.50.07", "release", "gdk", false)
	f.pids[inst.Dir] = []uint32{100}
	// The second copy never appears: the window of the running copy must
	// not count as this launch's.
	launchGame = func(ctx context.Context, name string, checkRunning bool) string {
		eventsink.From(ctx).Emit(launch.EventMcLaunchDone, struct{}{})
		return ""
	}
	code, _, errOut := f.h.run("launch", "Survival", "--force", "--timeout", "100ms")
	if code == 0 || !strings.Contains(errOut, "[ERR_LAUNCH_TIMEOUT]") {
		t.Fatalf("no new copy: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("launch", "Survival", "--force", "--detach", "--timeout", "100ms")
	if code == 0 || !strings.Contains(errOut, "[ERR_LAUNCH_TIMEOUT]") {
		t.Fatalf("no new copy, detach: %d %s", code, errOut)
	}
	// The new copy starts and exits before showing a window.
	launchWindowUp = func([]uint32) bool { return false }
	launchGame = func(ctx context.Context, name string, checkRunning bool) string {
		f.mu.Lock()
		f.pids[inst.Dir] = append(f.pids[inst.Dir], 200)
		f.mu.Unlock()
		go func() {
			time.Sleep(30 * time.Millisecond)
			f.mu.Lock()
			f.pids[inst.Dir] = []uint32{100}
			f.mu.Unlock()
		}()
		return ""
	}
	code, _, errOut = f.h.run("launch", "Survival", "--force", "--timeout", "5s")
	if code != ExitFailure || !strings.Contains(errOut, "[ERR_LAUNCH_GAME]") {
		t.Fatalf("new copy exited: %d %s", code, errOut)
	}
}

func TestLaunchFailures(t *testing.T) {
	f := newLaunchFakes(t)
	f.add("G", "1.21.50.07", "release", "gdk", false)
	f.outcome = "ERR_LAUNCH_GAME"
	code, _, errOut := f.h.run("play")
	if code != ExitFailure || !strings.Contains(errOut, "[ERR_LAUNCH_GAME]") {
		t.Fatalf("failed: %d %s", code, errOut)
	}
	f.pids = map[string][]uint32{}
	f.outcome = "none"
	code, _, errOut = f.h.run("play", "--timeout", "20ms")
	if code != ExitTimeout || !strings.Contains(errOut, "[ERR_LAUNCH_TIMEOUT]") {
		t.Fatalf("timeout: %d %s", code, errOut)
	}
	f.pids = map[string][]uint32{}
	f.code = "ERR_NOT_FOUND_EXE"
	code, _, errOut = f.h.run("play")
	if code != ExitNotFound || !strings.Contains(errOut, "missing from G") {
		t.Fatalf("exe: %d %s", code, errOut)
	}
	for _, args := range [][]string{{"play", "--detach", "--wait-exit"}, {"play", "--timeout", "0s"}} {
		if code, _, _ = f.h.run(args...); code != ExitUsage {
			t.Errorf("%v: %d", args, code)
		}
	}
}

func TestLaunchDetachAndWaitExit(t *testing.T) {
	f := newLaunchFakes(t)
	f.add("G", "1.21.50.07", "release", "gdk", false)
	f.outcome = "none" // the window never comes; --detach does not care
	code, out, errOut := f.h.run("play", "--detach", "--json")
	var res launchResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.State != "started" || res.PID == nil {
		t.Fatalf("detach: %d %q %s", code, out, errOut)
	}
	f.pids = map[string][]uint32{}
	f.outcome = ""
	f.exitCode = 3
	code, out, _ = f.h.run("play", "--wait-exit", "--json")
	if code != 3 || json.Unmarshal([]byte(out), &res) != nil || res.State != "exited" || res.ExitCode == nil || *res.ExitCode != 3 {
		t.Fatalf("wait-exit: %d %q", code, out)
	}
	f.pids = map[string][]uint32{}
	f.exitCode = 0
	if code, _, _ = f.h.run("play", "--wait-exit"); code != 0 {
		t.Fatalf("clean exit: %d", code)
	}
}

func TestLaunchUWPRegistersFirst(t *testing.T) {
	f := newLaunchFakes(t)
	u := f.add("U", "1.20.81.1", "release", "uwp", false)
	other := f.add("Other", "1.21.50.07", "release", "gdk", false)
	f.registered["release"] = utils.CanonicalWindowsPath(other.Dir)

	code, _, errOut := f.h.run("play", "U", "--force")
	if code != ExitConflict || !strings.Contains(errOut, "not supported for UWP") {
		t.Fatalf("force on UWP: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("play", "U")
	if code != ExitUsage || !strings.Contains(errOut, "replacing instance Other") || len(f.launches) != 0 {
		t.Fatalf("displacement needs --yes: %d %s", code, errOut)
	}
	code, out, errOut := f.h.run("play", "U", "-y", "--json")
	var res launchResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || !res.Registered ||
		f.registered["release"] != utils.CanonicalWindowsPath(u.Dir) || len(f.launches) != 1 {
		t.Fatalf("register then launch: %d %q %v %s", code, out, f.calls, errOut)
	}
	// A registration lost since the check is renewed and the launch retried.
	f.pids = map[string][]uint32{}
	tries := 0
	launchGameReal := launchGame
	launchGame = func(ctx context.Context, name string, check bool) string {
		tries++
		if tries == 1 {
			delete(f.registered, "release")
			return "ERR_UWP_NOT_REGISTERED: register the selected UWP instance before launching it"
		}
		return launchGameReal(ctx, name, check)
	}
	code, _, errOut = f.h.run("play", "U")
	if code != 0 || tries != 2 {
		t.Fatalf("retry: %d tries %d %s", code, tries, errOut)
	}
}

func TestLaunchAdvisories(t *testing.T) {
	f := newLaunchFakes(t)
	f.add("G", "1.21.50.07", "release", "gdk", false)
	f.add("U", "1.20.81.1", "release", "uwp", false)
	f.vc, f.gi, f.gs = false, false, false
	f.registered["release"] = utils.CanonicalWindowsPath(f.h.instances[1].Dir)
	code, _, errOut := f.h.run("play", "G")
	if code != 0 || strings.Count(errOut, "warning: ") != 3 || !strings.Contains(errOut, "doctor --fix vcruntime") ||
		!strings.Contains(errOut, "9MWPM2CQNLHN") {
		t.Fatalf("gdk advisories: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("play", "U")
	if code != 0 || strings.Count(errOut, "warning: ") != 1 {
		t.Fatalf("uwp advisories: %d %s", code, errOut)
	}
	// Advisories never block, and -q silences them.
	f.pids = map[string][]uint32{}
	code, _, errOut = f.h.run("play", "G", "-q")
	if code != 0 || errOut != "" {
		t.Fatalf("quiet: %d %q", code, errOut)
	}
}

func TestLaunchSkipsMigrationWhileRunning(t *testing.T) {
	f := newLaunchFakes(t)
	f.add("G", "1.21.50.07", "release", "gdk", false)
	f.running["Other"] = []int{1}
	if code, _, _ := f.h.run("play"); code != 0 || f.migrated {
		t.Fatalf("migration ran while a game runs: %d %v", code, f.migrated)
	}
}

func TestLaunchDescription(t *testing.T) {
	inst := Instance{Name: "x", PackageType: "gdk", Channel: "preview", Dir: t.TempDir()}
	inst.Meta.GameVersion = "1.21.60.21"
	inst.Meta.EnableEditorMode = true
	if got := launchDescription(inst); got != "Starting Minecraft 1.21.60.21 (GDK, preview, editor mode)" {
		t.Fatalf("description = %q", got)
	}
}
