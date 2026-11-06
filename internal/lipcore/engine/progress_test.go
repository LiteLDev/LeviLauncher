package engine

import (
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// progressRec records structured progress (and drops log lines).
type progressRec struct {
	report.Nop
	mu  sync.Mutex
	ups []report.Progress
}

func (r *progressRec) ReportProgress(p report.Progress) {
	r.mu.Lock()
	r.ups = append(r.ups, p)
	r.mu.Unlock()
}

// bars groups updates by bar id in first-update order, checking that every
// bar has a monotonic Current, a stable Stage, Unit and Total, and exactly
// one Done update, which is its last.
func (r *progressRec) bars(t *testing.T) [][]report.Progress {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	index := map[string]int{}
	var out [][]report.Progress
	for _, p := range r.ups {
		i, ok := index[p.ID]
		if !ok {
			i = len(out)
			index[p.ID] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], p)
	}
	for _, bar := range out {
		first := bar[0]
		for j, p := range bar {
			if p.Stage != first.Stage || p.Unit != first.Unit || p.Total != first.Total {
				t.Errorf("bar %s changed stage/unit/total: %+v -> %+v", p.ID, first, p)
			}
			if j > 0 && p.Current < bar[j-1].Current {
				t.Errorf("bar %s went back: %d -> %d", p.ID, bar[j-1].Current, p.Current)
			}
			if p.Done != (j == len(bar)-1) {
				t.Errorf("bar %s: update %d/%d has Done=%v", p.ID, j+1, len(bar), p.Done)
			}
		}
	}
	return out
}

func barsOf(bars [][]report.Progress, stage report.Stage) [][]report.Progress {
	var out [][]report.Progress
	for _, b := range bars {
		if b[0].Stage == stage {
			out = append(out, b)
		}
	}
	return out
}

func last(bar []report.Progress) report.Progress { return bar[len(bar)-1] }

func messages(bar []report.Progress) []string {
	var out []string
	for _, p := range bar {
		if len(out) == 0 || out[len(out)-1] != p.Message {
			out = append(out, p.Message)
		}
	}
	return out
}

func newProgressHarness(t *testing.T) (*harness, *progressRec) {
	h := newHarness(t)
	seed(h)
	rec := &progressRec{}
	h.e.Reporter = rec
	return h, rec
}

func TestInstallProgress(t *testing.T) {
	h, rec := newProgressHarness(t)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0", "example.com/d"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	bars := rec.bars(t)
	resolve := barsOf(bars, report.StageResolve)
	if len(resolve) != 2 {
		t.Fatalf("resolve bars = %d, want requested packages + solve", len(resolve))
	}
	req := resolve[0]
	if p := last(req); p.Total != 2 || p.Current != 2 || p.Unit != report.UnitPackages {
		t.Errorf("requested packages bar ends with %+v", p)
	}
	if req[1].Item != "example.com/a@1.0.0" || req[2].Item != "example.com/d" {
		t.Errorf("requested packages items %+v", req)
	}
	// The solve bar is indeterminate and counts distinct manifests: a, d and
	// b@1.1.0.
	if p := last(resolve[1]); p.Total != 0 || p.Current != 3 || p.Message != "Resolving dependencies" {
		t.Errorf("solve bar ends with %+v", p)
	}

	tasks := barsOf(bars, report.StageTask)
	if len(tasks) != 1 {
		t.Fatalf("task bars = %d", len(tasks))
	}
	task := tasks[0]
	if p := last(task); p.Total != 3 || p.Current != 3 || p.Unit != report.UnitPackages {
		t.Errorf("task bar ends with %+v", p)
	}
	want := []string{"Applying package changes", "Installing example.com/b@1.1.0", "Installing example.com/a@1.0.0", "Installing example.com/d@1.0.0"}
	if got := messages(task); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("task messages %q, want %q", got, want)
	}
	if task[1].Package != "example.com/b@1.1.0" || task[1].Item != "example.com/b@1.1.0" || task[1].Current != 0 {
		t.Errorf("first step %+v", task[1])
	}
}

func TestInstallProgressDoneOnFailure(t *testing.T) {
	h, rec := newProgressHarness(t)
	h.e.Hooks.NewPackageOps = newFailingOps(&h.calls, 1)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, false, false, false); err == nil {
		t.Fatal("want error")
	}
	tasks := barsOf(rec.bars(t), report.StageTask)
	if len(tasks) != 1 {
		t.Fatalf("task bars = %d", len(tasks))
	}
	if p := last(tasks[0]); !p.Done || p.Total != 2 || p.Current != 1 {
		t.Errorf("failed task bar ends with %+v", p)
	}
}

func TestResolveProgressDoneOnFailure(t *testing.T) {
	h, rec := newProgressHarness(t)
	// Resolving the requested package fails.
	if err := h.e.Install(ctx, []string{"example.com/d@1.0.0", "example.com/zz@1.0.0"}, false, false, false); err == nil {
		t.Fatal("want error")
	}
	bars := rec.bars(t)
	resolve := barsOf(bars, report.StageResolve)
	if len(resolve) != 1 {
		t.Fatalf("resolve bars = %d", len(resolve))
	}
	if p := last(resolve[0]); p.Total != 2 || p.Current != 1 || p.Item != "example.com/zz@1.0.0" {
		t.Errorf("failed resolve bar ends with %+v", p)
	}
	if len(barsOf(bars, report.StageTask)) != 0 {
		t.Error("task bar opened before solving")
	}

	// Solving fails: a dependency does not exist.
	h, rec = newProgressHarness(t)
	h.reg.add("example.com/f", "1.0.0", "", "example.com/missing", "*")
	if err := h.e.Install(ctx, []string{"example.com/f@1.0.0"}, false, false, false); err == nil {
		t.Fatal("want error")
	}
	resolve = barsOf(rec.bars(t), report.StageResolve)
	if len(resolve) != 2 {
		t.Fatalf("resolve bars = %d", len(resolve))
	}
	if p := last(resolve[1]); p.Total != 0 || p.Current != 1 {
		t.Errorf("failed solve bar ends with %+v", p)
	}
}

func TestDryRunHasNoTaskProgress(t *testing.T) {
	h, rec := newProgressHarness(t)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0"}, true, false, false); err != nil {
		t.Fatal(err)
	}
	if n := len(barsOf(rec.bars(t), report.StageTask)); n != 0 {
		t.Errorf("dry run task bars = %d", n)
	}
}

func TestUninstallProgress(t *testing.T) {
	h, rec := newProgressHarness(t)
	if err := h.e.Install(ctx, []string{"example.com/a@1.0.0", "example.com/d@1.0.0"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	rec.ups = nil
	if err := h.e.Uninstall(ctx, []string{"example.com/a"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	bars := rec.bars(t)
	if n := len(barsOf(bars, report.StageResolve)); n != 1 {
		t.Errorf("resolve bars = %d, want the solve bar only", n)
	}
	tasks := barsOf(bars, report.StageTask)
	if len(tasks) != 1 {
		t.Fatalf("task bars = %d", len(tasks))
	}
	if p := last(tasks[0]); p.Total != 2 || p.Current != 2 {
		t.Errorf("task bar ends with %+v", p)
	}
	want := []string{"Applying package changes", "Uninstalling example.com/a@1.0.0", "Uninstalling example.com/b@1.1.0"}
	if got := messages(tasks[0]); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("task messages %q, want %q", got, want)
	}
}

func TestNoDependenciesProgress(t *testing.T) {
	h, rec := newProgressHarness(t)
	if err := h.e.Install(ctx, []string{"example.com/d@1.0.0", "example.com/c@1.0.0"}, false, false, true); err != nil {
		t.Fatal(err)
	}
	tasks := barsOf(rec.bars(t), report.StageTask)
	if len(tasks) != 1 {
		t.Fatalf("install task bars = %d", len(tasks))
	}
	if p := last(tasks[0]); p.Total != 2 || p.Current != 2 {
		t.Errorf("install task bar ends with %+v", p)
	}

	rec.ups = nil
	// The second package is not installed: the bar ends at the failed step.
	if err := h.e.Uninstall(ctx, []string{"example.com/d", "example.com/a"}, false, false, true); err == nil {
		t.Fatal("want error")
	}
	tasks = barsOf(rec.bars(t), report.StageTask)
	if len(tasks) != 1 {
		t.Fatalf("uninstall task bars = %d", len(tasks))
	}
	if p := last(tasks[0]); p.Total != 2 || p.Current != 1 || p.Package != "example.com/a" {
		t.Errorf("uninstall task bar ends with %+v", p)
	}
}

// A reporter without ReportProgress gets determinate and Done updates as
// percentages.
func TestProgressFallsBackToPercentages(t *testing.T) {
	h := newHarness(t)
	seed(h)
	if err := h.e.Install(ctx, []string{"example.com/d@1.0.0"}, false, false, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"progress: Resolving packages 100", "progress: Resolving dependencies 100", "progress: Installing example.com/d@1.0.0 100"} {
		if !h.rep.has(want) {
			t.Errorf("missing %q in %q", want, h.rep.msgs)
		}
	}
}
