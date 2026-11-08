package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
)

type fakeUpdate struct {
	version  string
	latest   string
	checkErr error
	applyErr error
	applied  int
	beta     bool
	gui      bool
	games    []string
}

func installFakeUpdate(t *testing.T) *fakeUpdate {
	t.Helper()
	f := &fakeUpdate{version: "1.6.2"}
	sv, sc, sa, sg, sgame := suVersion, suCheck, suApply, suGUIRunning, suGameRunning
	t.Cleanup(func() { suVersion, suCheck, suApply, suGUIRunning, suGameRunning = sv, sc, sa, sg, sgame })
	suVersion = func() string { return f.version }
	suCheck = func(_ context.Context, _ string, beta bool) (types.CheckUpdate, error) {
		f.beta = beta
		if f.checkErr != nil {
			return types.CheckUpdate{}, f.checkErr
		}
		if f.latest == "" {
			return types.CheckUpdate{Version: f.version}, nil
		}
		return types.CheckUpdate{IsUpdate: true, Version: f.latest, Body: "line1\nline2"}, nil
	}
	suApply = func(context.Context, string, bool) (update.ApplyResult, error) {
		f.applied++
		if f.applyErr != nil {
			return update.ApplyResult{}, f.applyErr
		}
		return update.ApplyResult{From: "v" + f.version, To: f.latest}, nil
	}
	suGUIRunning = func() bool { return f.gui }
	suGameRunning = func() []string { return f.games }
	return f
}

func TestSelfUpdateCheck(t *testing.T) {
	h := newHarness(t)
	f := installFakeUpdate(t)
	if code, out, _ := h.run("self-update", "--check"); code != 0 || out != "Up to date (v1.6.2)\n" {
		t.Fatalf("up to date: %d %q", code, out)
	}
	f.latest = "v1.7.0"
	code, out, _ := h.run("self-update", "--check", "--beta")
	if code != 0 || !strings.HasPrefix(out, "Update available: v1.6.2 -> v1.7.0\n") || !strings.Contains(out, "line2") || !f.beta {
		t.Fatalf("available: %d %q beta=%v", code, out, f.beta)
	}
	code, out, _ = h.run("self-update", "--check", "--json")
	var r selfUpdateResult
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil || !r.UpdateAvailable || r.Latest != "v1.7.0" || r.Updated || f.applied != 0 {
		t.Fatalf("--json: %d %q", code, out)
	}
	f.checkErr = errors.New("ERR_UPDATE_RELEASE_UNAVAILABLE: dial tcp")
	if code, _, _ := h.run("self-update", "--check"); code != ExitNetwork {
		t.Fatalf("offline: %d", code)
	}
}

func TestSelfUpdateApply(t *testing.T) {
	h := newHarness(t)
	f := installFakeUpdate(t)
	// Up to date: idempotent, nothing applied, no confirmation.
	if code, _, _ := h.run("self-update"); code != 0 || f.applied != 0 {
		t.Fatalf("up to date: %d applied=%d", code, f.applied)
	}
	f.latest = "v1.7.0"
	f.gui = true
	if code, _, errOut := h.run("self-update", "--yes"); code != ExitConflict || !strings.Contains(errOut, "ERR_LAUNCHER_RUNNING") || f.applied != 0 {
		t.Fatalf("window running: %d %q", code, errOut)
	}
	f.gui, f.games = false, []string{"Survival"}
	if code, _, _ := h.run("self-update", "--yes"); code != ExitConflict || f.applied != 0 {
		t.Fatalf("game running: %d", code)
	}
	f.games = nil
	if code, _, errOut := h.run("self-update"); code != ExitUsage || !strings.Contains(errOut, "ERR_CONFIRMATION_REQUIRED") || f.applied != 0 {
		t.Fatalf("no --yes: %d %q", code, errOut)
	}
	code, out, _ := h.run("self-update", "--yes")
	if code != 0 || f.applied != 1 || !strings.HasPrefix(out, "Updated v1.6.2 -> v1.7.0") {
		t.Fatalf("apply: %d %q", code, out)
	}
	f.applyErr = errors.New("ERR_UPDATE_ADMIN_REQUIRED: the launcher folder is not writable")
	if code, _, errOut := h.run("self-update", "--yes"); code != ExitSystem || !strings.Contains(errOut, "elevated") {
		t.Fatalf("admin: %d %q", code, errOut)
	}
	f.applyErr = errors.New("ERR_UPDATE_NOT_AVAILABLE: no update available")
	if code, out, _ := h.run("self-update", "--yes"); code != 0 || !strings.HasPrefix(out, "Up to date") {
		t.Fatalf("raced: %d %q", code, out)
	}
}

func TestSelfUpdateDevBuild(t *testing.T) {
	h := newHarness(t)
	f := installFakeUpdate(t)
	f.version, f.latest = "0.0.0-dev", "v9.9.9"
	code, out, errOut := h.run("self-update", "--yes")
	if code != 0 || f.applied != 0 || !strings.Contains(errOut, "development build") || !strings.HasPrefix(out, "Up to date") {
		t.Fatalf("dev build: %d %q %q", code, out, errOut)
	}
}
