package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/eventsink"
)

type fakeSystem struct {
	vc, gi, gs, dev, migrated, writable, account, elevated, wv bool
	vcCalls                                                    int
	fixes                                                      []string
	vcInstallResult                                            bool
	vcError                                                    string
	devFixOK                                                   bool
}

func installFakeSystem(t *testing.T) *fakeSystem {
	t.Helper()
	f := &fakeSystem{vc: true, gi: true, gs: true, dev: true, migrated: true, writable: true, account: true, wv: true}
	saved := doc
	savedGames := suGameRunning
	t.Cleanup(func() { doc = saved; suGameRunning = savedGames })
	suGameRunning = func() []string { return nil }
	doc.vcInstalled = func() bool { f.vcCalls++; return f.vc }
	doc.giInstalled = func() bool { return f.gi }
	doc.gsInstalled = func() bool { return f.gs }
	doc.devMode = func() bool { return f.dev }
	doc.needsMigration = func() bool { return !f.migrated }
	doc.canWrite = func(string) bool { return f.writable }
	doc.baseRoot = func() string { return `C:\Data` }
	doc.accountSaved = func() bool { return f.account }
	doc.elevated = func() bool { return f.elevated }
	doc.webview2 = func() bool { return f.wv }
	doc.sleep = func(time.Duration) {}
	doc.fixVC = func(ctx context.Context) {
		f.fixes = append(f.fixes, fixVCRuntime)
		if f.vcError != "" {
			eventsink.From(ctx).Emit("vcruntime.download.error", f.vcError)
			return
		}
		f.vc = f.vcInstallResult
	}
	doc.fixGI = func(context.Context) { f.fixes = append(f.fixes, fixGameInput); f.gi = true }
	doc.fixDevMode = func() bool { f.fixes = append(f.fixes, fixDeveloperMode); f.dev = f.devFixOK; return f.devFixOK }
	doc.migrate = func(context.Context) (int, error) {
		f.fixes = append(f.fixes, fixLoaderMigration)
		f.migrated = true
		return 2, nil
	}
	return f
}

func TestDoctorAllOK(t *testing.T) {
	h := newHarness(t)
	installFakeSystem(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	code, out, errOut := h.run("doctor")
	if code != 0 || !strings.Contains(out, "[ok]      Visual C++ runtime") || strings.Contains(out, "[missing]") {
		t.Fatalf("doctor: %d %q %q", code, out, errOut)
	}
	code, out, _ = h.run("doctor", "--json")
	var checks []doctorCheck
	if code != 0 || json.Unmarshal([]byte(out), &checks) != nil || len(checks) < 9 {
		t.Fatalf("--json: %d %q", code, out)
	}
}

func TestDoctorRequiredChecks(t *testing.T) {
	h := newHarness(t)
	f := installFakeSystem(t)
	// No GDK instance: a missing GameInput is not a failure.
	f.gi = false
	if code, out, _ := h.run("doctor"); code != 0 || !strings.Contains(out, "[missing] GameInput: needed by GDK instances") {
		t.Fatalf("no GDK instance: %d %q", code, out)
	}
	h.add("Survival", "1.21.50", "release", "gdk")
	code, out, errOut := h.run("doctor")
	if code != ExitFailure || !strings.Contains(errOut, "[ERR_DOCTOR_FAILED]") || !strings.Contains(out, "doctor --fix gameinput") {
		t.Fatalf("GDK instance: %d %q %q", code, out, errOut)
	}
	// The VC++ check is retried once, as in the window.
	f.gi, f.vc, f.vcCalls = true, false, 0
	h.run("doctor")
	if f.vcCalls != 2 {
		t.Fatalf("vc checked %d times", f.vcCalls)
	}
	if code, out, _ := h.run("doctor", "-q"); code != ExitFailure || out != "vcruntime\n" {
		t.Fatalf("-q: %d %q", code, out)
	}
}

func TestDoctorFix(t *testing.T) {
	h := newHarness(t)
	f := installFakeSystem(t)
	h.add("Survival", "1.21.50", "release", "gdk")
	f.gi, f.migrated = false, false

	if code, _, errOut := h.run("doctor", "gameinput"); code != ExitUsage || !strings.Contains(errOut, "--fix gameinput") {
		t.Fatalf("name without --fix: %d %q", code, errOut)
	}
	if code, _, errOut := h.run("doctor", "--fix", "gamenput"); code != ExitUsage || !strings.Contains(errOut, `"gameinput"`) {
		t.Fatalf("unknown fix: %d %q", code, errOut)
	}
	if code, _, _ := h.run("doctor", "--fix"); code != ExitUsage || len(f.fixes) != 0 {
		t.Fatalf("fix without --yes: %d %v", code, f.fixes)
	}
	code, out, errOut := h.run("doctor", "--fix", "gameinput", "--yes")
	if code != 0 || strings.Join(f.fixes, ",") != "gameinput" || !strings.Contains(out, "GameInput (fixed)") {
		t.Fatalf("fix gameinput: %d %v %q %q", code, f.fixes, out, errOut)
	}
	if f.migrated {
		t.Fatal("loader-migration ran although only gameinput was named")
	}
	// A bare --fix leaves Developer Mode alone without a UWP instance.
	f.dev = false
	f.fixes = nil
	code, _, _ = h.run("doctor", "--fix", "-y")
	if code != 0 || !f.migrated || strings.Join(f.fixes, ",") != "loader-migration" {
		t.Fatalf("fix all: %d %v", code, f.fixes)
	}
	f.dev = true

	// A declined administrator prompt is exit 8.
	f.vc, f.vcError = false, "The operation was canceled by the user."
	if code, _, errOut := h.run("doctor", "--fix", "vcruntime", "--yes"); code != ExitSystem || !strings.Contains(errOut, "ERR_ELEVATION_DECLINED") {
		t.Fatalf("declined: %d %q", code, errOut)
	}
	f.vc, f.dev, f.devFixOK = true, false, false
	if code, _, _ := h.run("doctor", "--fix", "developer-mode", "--yes"); code != ExitSystem {
		t.Fatalf("developer mode declined: %d", code)
	}

	// The loader migration is refused while a game runs.
	f.dev, f.migrated = true, false
	suGameRunning = func() []string { return []string{"Survival"} }
	if code, _, errOut := h.run("doctor", "--fix", "loader-migration", "--yes"); code != ExitConflict || !strings.Contains(errOut, "stop") {
		t.Fatalf("game running: %d %q", code, errOut)
	}
}

func TestDoctorInstance(t *testing.T) {
	h := newHarness(t)
	installFakeSystem(t)
	inst := h.add("Survival", "1.21.50", "release", "gdk")
	if code, out, _ := h.run("doctor", "-i", "Survival"); code != ExitFailure || !strings.Contains(out, "the game executable is missing") {
		t.Fatalf("missing exe: %d %q", code, out)
	}
	if err := os.MkdirAll(inst.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst.Dir, "Minecraft.Windows.exe"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := h.run("doctor", "-i", "survival"); code != 0 || !strings.Contains(out, "[ok]      Instance Survival: gdk 1.21.50") {
		t.Fatalf("instance ok: %d %q", code, out)
	}
}
