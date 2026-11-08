package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/types"
)

type procFakes struct {
	*instFakes
	procs  []types.ProcessInfo
	killed []int
	fail   map[int]error
	immune map[int]bool // survive a successful kill
}

func newProcFakes(t *testing.T) *procFakes {
	f := &procFakes{instFakes: newInstFakes(t), fail: map[int]error{}, immune: map[int]bool{}}
	instSave(f.instFakes, &procKill)
	instSave(f.instFakes, &procExitWait)
	procExitWait = 30 * time.Millisecond
	minecraftProcesses = func() []types.ProcessInfo { return append([]types.ProcessInfo(nil), f.procs...) }
	procKill = func(pid int) error {
		f.killed = append(f.killed, pid)
		if err := f.fail[pid]; err != nil {
			return err
		}
		if f.immune[pid] {
			return nil
		}
		kept := f.procs[:0]
		for _, p := range f.procs {
			if p.Pid != pid {
				kept = append(kept, p)
			}
		}
		f.procs = kept
		return nil
	}
	return f
}

func (f *procFakes) game(pid int, instance string) {
	p := types.ProcessInfo{Pid: pid, ExePath: `C:\Store\Minecraft.Windows.exe`}
	if instance != "" {
		p.IsLauncher, p.VersionName, p.ExePath = true, instance, `C:\i\`+instance+`\Minecraft.Windows.exe`
	}
	f.procs = append(f.procs, p)
}

func TestProcessList(t *testing.T) {
	f := newProcFakes(t)
	code, out, errOut := f.h.run("ps")
	if code != 0 || out != "" || !strings.Contains(errOut, "No Minecraft is running") {
		t.Fatalf("empty: %d %q %q", code, out, errOut)
	}
	f.game(30, "Survival")
	f.game(10, "")
	code, out, _ = f.h.run("process", "ls", "--json")
	var items []procItem
	if code != 0 || json.Unmarshal([]byte(out), &items) != nil || len(items) != 2 || items[0].PID != 10 || items[0].Instance != nil ||
		items[1].Instance == nil || *items[1].Instance != "Survival" || !items[1].FromLauncher {
		t.Fatalf("json: %d %q", code, out)
	}
	code, out, _ = f.h.run("ps", "-q")
	if code != 0 || out != "10\n30\n" {
		t.Fatalf("quiet: %d %q", code, out)
	}
	code, out, _ = f.h.run("process", "list")
	if code != 0 || !strings.HasPrefix(out, "PID") || !strings.Contains(out, "Survival") {
		t.Fatalf("table: %d %q", code, out)
	}
}

func TestProcessStop(t *testing.T) {
	f := newProcFakes(t)
	f.add("Survival", "1.21.50.07", "release", "gdk", false)
	f.add("Other", "1.21.50.07", "release", "gdk", false)

	// Nothing running is not an error.
	code, _, errOut := f.h.run("stop", "Survival")
	if code != 0 || !strings.Contains(errOut, "not running from Survival") {
		t.Fatalf("idempotent: %d %s", code, errOut)
	}
	if code, _, errOut = f.h.run("stop", "--all"); code != 0 || !strings.Contains(errOut, "No Minecraft is running") {
		t.Fatalf("--all idle: %d %s", code, errOut)
	}
	f.game(11, "Survival")
	f.game(12, "Survival")
	f.game(20, "")
	code, _, errOut = f.h.run("stop", "Survival")
	if code != ExitUsage || !strings.Contains(errOut, "--yes") || len(f.killed) != 0 {
		t.Fatalf("confirmation: %d %s", code, errOut)
	}
	code, _, errOut = f.h.run("stop", "999", "-y")
	if code != ExitNotFound || !strings.Contains(errOut, "no Minecraft process has PID 999") || !strings.Contains(errOut, "[ERR_PROCESS_NOT_MINECRAFT]") {
		t.Fatalf("foreign pid: %d %s", code, errOut)
	}
	code, out, errOut := f.h.run("stop", "survival", "-y")
	if code != 0 || out != "Stopped Survival (pid 11)\nStopped Survival (pid 12)\n" || len(f.procs) != 1 {
		t.Fatalf("stop instance: %d %q %s", code, out, errOut)
	}
	// A PID of a Store game; access denied is exit 8.
	f.fail[20] = windows.ERROR_ACCESS_DENIED
	code, _, errOut = f.h.run("process", "stop", "20", "-y")
	if code != ExitSystem || !strings.Contains(errOut, "elevated") {
		t.Fatalf("denied: %d %s", code, errOut)
	}
	delete(f.fail, 20)
	// --all with one survivor is a partial success.
	f.game(31, "Other")
	f.immune[20] = true
	code, out, errOut = f.h.run("process", "stop", "--all", "-y", "--json")
	var res []procStopResult
	if code != ExitPartial || json.Unmarshal([]byte(out), &res) != nil || len(res) != 2 || res[0].Stopped || !res[1].Stopped ||
		res[0].Error == nil || res[0].Error.Code != "ERR_PROCESS_SURVIVED" {
		t.Fatalf("partial: %d %q %s", code, out, errOut)
	}
	if code, _, _ = f.h.run("stop", "--all", "Survival"); code != ExitUsage {
		t.Fatalf("--all with args: %d", code)
	}
}

func TestProcessStopDefaultInstance(t *testing.T) {
	f := newProcFakes(t)
	f.add("Only", "1.21.50.07", "release", "gdk", false)
	f.game(5, "Only")
	code, out, errOut := f.h.run("stop", "-y", "-q")
	if code != 0 || out != "5\n" || len(f.killed) != 1 {
		t.Fatalf("default instance: %d %q %s", code, out, errOut)
	}
	if e := procKillError(errors.New("boom")); e.Code != "ERR_PROCESS_KILL_FAILED" || e.Exit != ExitFailure {
		t.Fatalf("plain error: %+v", e)
	}
	if e := procKillError(windows.ERROR_INVALID_PARAMETER); e != nil {
		t.Fatalf("gone process: %+v", e)
	}
}
