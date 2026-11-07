package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/types"
)

// Seams replaced by tests.
var (
	procKill = mcservice.KillProcess
	// procExitWait is how long stop waits for killed processes to vanish
	// from the process list.
	procExitWait = 3 * time.Second
)

func init() {
	RegisterBackendErrors(map[string]BackendError{
		"ERR_PROCESS_ACCESS_DENIED": {Exit: ExitSystem, Message: "access to the Minecraft process was denied",
			Hint: "run the command from an elevated (administrator) terminal"},
		"ERR_PROCESS_NOT_MINECRAFT": {Exit: ExitNotFound, Message: "no Minecraft process has that PID", Hint: "LeviLauncher.exe ps"},
	})
	Register(&Command{
		Path:    "process list",
		Summary: "list running Minecraft processes, also those not started by the launcher",
		Description: `INSTANCE is the instance a process runs from, or "-" for a Minecraft the
launcher did not start (for example the Microsoft Store one).`,
		Examples: []string{"ps", "process list --json", "process ls -q"},
		Run:      runProcessList,
	})
	Register(&Command{
		Path:    "process stop",
		Summary: "force-close running Minecraft (progress since the last autosave is lost)",
		Args:    "[<instance>|<pid>...]",
		Description: `Each argument is an instance (every process running from it) or a PID of
a Minecraft process. With no argument, stops the default instance's game.
--all stops every Minecraft, including ones the launcher did not start.
Closing a game that is not running is not an error.`,
		Examples: []string{"stop", "stop Survival --yes", "process stop 12345", "process stop --all -y"},
		Flags:    []*Flag{BoolFlag("all", "", "stop every Minecraft process")},
		MaxArgs:  Unlimited,
		Run:      runProcessStop,
	})
}

// procItem is one element of process list --json.
type procItem struct {
	PID          int     `json:"pid"`
	Instance     *string `json:"instance"`
	ExePath      string  `json:"exePath"`
	FromLauncher bool    `json:"fromLauncher"`
}

func procItems(ps []types.ProcessInfo) []procItem {
	out := make([]procItem, 0, len(ps))
	for _, p := range ps {
		it := procItem{PID: p.Pid, ExePath: p.ExePath, FromLauncher: p.IsLauncher}
		if p.IsLauncher && p.VersionName != "" {
			name := p.VersionName
			it.Instance = &name
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func (it procItem) label() string {
	if it.Instance != nil {
		return fmt.Sprintf("%s (pid %d)", *it.Instance, it.PID)
	}
	return fmt.Sprintf("Minecraft not started by the launcher (pid %d)", it.PID)
}

func runProcessList(c *Ctx) error {
	items := procItems(minecraftProcesses())
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = strconv.Itoa(it.PID)
	}
	if len(items) == 0 {
		c.Notice("No Minecraft is running.")
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			inst := "-"
			if it.Instance != nil {
				inst = *it.Instance
			}
			rows[i] = []string{strconv.Itoa(it.PID), inst, it.ExePath}
		}
		return Table(w, []string{"PID", "INSTANCE", "PATH"}, rows)
	})
}

// procStopResult is one element of process stop --json.
type procStopResult struct {
	procItem
	Stopped bool       `json:"stopped"`
	Error   *procError `json:"error,omitempty"`
}

type procError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// procTargets resolves the arguments of process stop to processes. An
// all-digit argument is a PID when a Minecraft process has it, otherwise an
// instance name.
func procTargets(c *Ctx, running []procItem) ([]procItem, error) {
	byPID := map[int]procItem{}
	for _, it := range running {
		byPID[it.PID] = it
	}
	var out []procItem
	seen := map[int]bool{}
	add := func(it procItem) {
		if !seen[it.PID] {
			seen[it.PID] = true
			out = append(out, it)
		}
	}
	if c.Bool("all") {
		if len(c.Args) > 0 || strings.TrimSpace(c.G.Instance) != "" {
			return nil, UsageError("--all takes no instance or PID")
		}
		return running, nil
	}
	args := c.Args
	if len(args) == 0 {
		inst, err := c.ResolveInstance("")
		if err != nil {
			return nil, err
		}
		args = []string{inst.Name}
	}
	for _, a := range args {
		if pid, err := strconv.Atoi(strings.TrimSpace(a)); err == nil {
			if it, ok := byPID[pid]; ok {
				add(it)
				continue
			}
			if _, lerr := LookupInstance(a); lerr != nil {
				return nil, Errorf(ExitNotFound, "ERR_PROCESS_NOT_MINECRAFT", "no Minecraft process has PID %d", pid).
					WithHint("%s ps", ProgramName)
			}
		}
		inst, err := LookupInstance(a)
		if err != nil {
			return nil, err
		}
		found := false
		for _, it := range running {
			if it.Instance != nil && strings.EqualFold(*it.Instance, inst.Name) {
				add(it)
				found = true
			}
		}
		if !found {
			c.Notice("Minecraft is not running from %s", inst.Name)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

// procKillError wraps the plain-text error of a kill into a code.
func procKillError(err error) *Error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return Backend("ERR_PROCESS_ACCESS_DENIED").WithDetail(err.Error())
	}
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		// The process is gone (OpenProcess on a dead PID).
		return nil
	}
	return Errorf(ExitFailure, "ERR_PROCESS_KILL_FAILED", "could not stop the process: %v", err)
}

func runProcessStop(c *Ctx) error {
	running := procItems(minecraftProcesses())
	targets, err := procTargets(c, running)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if c.Bool("all") {
			c.Notice("No Minecraft is running.")
		}
		return c.Render([]procStopResult{}, nil, nil)
	}
	labels := make([]string, len(targets))
	for i, t := range targets {
		labels[i] = t.label()
	}
	q := "Force-close Minecraft " + strings.Join(labels, ", ") + ", losing progress since the last autosave?"
	if err := c.Confirm(q); err != nil {
		return err
	}

	results := make([]procStopResult, len(targets))
	killErrs := make([]*Error, len(targets))
	for i, t := range targets {
		results[i] = procStopResult{procItem: t}
		if kerr := procKill(t.PID); kerr != nil {
			if e := procKillError(kerr); e != nil {
				results[i].Error, killErrs[i] = &procError{e.Code, e.Message}, e
			}
		}
	}
	// A kill returns before the process is gone, and a bulk kill can
	// partially fail: read the list again until the killed ones vanish.
	alive := procAlive(targets, results)
	deadline := time.Now().Add(procExitWait)
	for len(alive) > 0 && time.Now().Before(deadline) {
		select {
		case <-c.Context().Done():
			return c.Context().Err()
		case <-time.After(150 * time.Millisecond):
		}
		alive = procAlive(targets, results)
	}
	var failed []*Error
	for i := range results {
		r := &results[i]
		switch {
		case killErrs[i] != nil:
			failed = append(failed, killErrs[i])
		case alive[r.PID]:
			e := Errorf(ExitPartial, "ERR_PROCESS_SURVIVED", "%s is still running after being stopped", r.label())
			r.Error = &procError{e.Code, e.Message}
			failed = append(failed, e)
		default:
			r.Stopped = true
		}
	}
	if len(targets) == 1 && len(failed) == 1 {
		return failed[0]
	}
	var ids []string
	for _, r := range results {
		if r.Stopped {
			ids = append(ids, strconv.Itoa(r.PID))
		} else if len(targets) > 1 && r.Error != nil {
			c.errf("error: %s: %s [%s]\n", r.label(), r.Error.Message, r.Error.Code)
		}
	}
	if err := c.Render(results, ids, func(w io.Writer) error {
		for _, r := range results {
			if r.Stopped {
				name := "Minecraft"
				if r.Instance != nil {
					name = *r.Instance
				}
				fmt.Fprintf(w, "Stopped %s (pid %d)\n", name, r.PID)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(failed) > 0 && len(failed) == len(targets) {
		// Every kill failed: report the most specific cause.
		for _, e := range failed {
			if e.Exit == ExitSystem {
				return e
			}
		}
	}
	return instBatchExit(c, len(targets), len(failed), "processes", "stopped")
}

// procAlive returns the targets without a kill error that still run.
func procAlive(targets []procItem, results []procStopResult) map[int]bool {
	now := map[int]bool{}
	for _, p := range minecraftProcesses() {
		now[p.Pid] = true
	}
	out := map[int]bool{}
	for i, t := range targets {
		if results[i].Error == nil && now[t.PID] {
			out[t.PID] = true
		}
	}
	return out
}
