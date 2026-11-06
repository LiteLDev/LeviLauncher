package lip

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// fakeProgressClock replaces the throttle's clock with one advanced by hand.
type fakeProgressClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeProgressClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func useFakeProgressClock(t *testing.T) *fakeProgressClock {
	t.Helper()
	c := &fakeProgressClock{now: time.Unix(1700000000, 0)}
	old := progressNow
	progressNow = func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.now
	}
	t.Cleanup(func() { progressNow = old })
	return c
}

// progressEvents returns the lip_task_progress payloads of events.
func progressEvents(t *testing.T, events []recordedEvent) []LipTaskProgressEvent {
	t.Helper()
	var out []LipTaskProgressEvent
	for _, ev := range events {
		if ev.name == EventLipTaskProgress {
			out = append(out, ev.data.(LipTaskProgressEvent))
		}
	}
	return out
}

// checkBar asserts that the updates of one bar never go back, keep one ID
// and end with their only Done update.
func checkBar(t *testing.T, bar []LipTaskProgressEvent) {
	t.Helper()
	if len(bar) == 0 {
		t.Fatal("bar has no updates")
	}
	for i, p := range bar {
		if p.ProgressID != bar[0].ProgressID {
			t.Fatalf("update %d has ID %q, want %q", i, p.ProgressID, bar[0].ProgressID)
		}
		if i > 0 && p.Current < bar[i-1].Current {
			t.Fatalf("update %d goes back: %d after %d", i, p.Current, bar[i-1].Current)
		}
		if p.Done != (i == len(bar)-1) {
			t.Fatalf("update %d Done=%v; want exactly one Done, last (%+v)", i, p.Done, bar)
		}
	}
}

func testTaskReporter(t *testing.T) (*taskReporter, *eventRecorder, *fakeProgressClock) {
	t.Helper()
	rec := recordEvents(t)
	clock := useFakeProgressClock(t)
	return newTaskReporter(newTaskMeta("Install", t.TempDir(), []string{"a@1.0.0"})), rec, clock
}

func TestStructuredProgressFields(t *testing.T) {
	r, rec, _ := testTaskReporter(t)
	report.Report(r, report.Progress{ID: "p1", Stage: report.StagePlace, Package: "a@1.0.0", Message: " Placing files ",
		Current: 1, Total: 4, Unit: report.UnitFiles, Item: "mods/a.dll"})
	report.Report(r, report.Progress{ID: "p2", Stage: report.StageResolve, Current: 7})
	report.Report(r, report.Progress{ID: "p2", Stage: report.StageResolve, Current: 9, Done: true})
	r.Progress("dl-1", "Downloading https://example.com/a.zip", 42.5)

	got := progressEvents(t, rec.snapshot())
	if len(got) != 4 {
		t.Fatalf("got %d updates: %+v", len(got), got)
	}
	meta := r.meta
	for i, p := range got {
		if p.TaskID != meta.TaskID || p.Method != "Install" || p.Target != meta.Target || fmt.Sprint(p.Packages) != "[a@1.0.0]" {
			t.Fatalf("update %d has wrong task fields: %+v", i, p)
		}
	}
	place := got[0]
	if place.ProgressID != "p1" || place.Stage != "place" || place.Package != "a@1.0.0" || place.Message != "Placing files" ||
		place.Current != 1 || place.Total != 4 || place.Unit != "files" || place.Item != "mods/a.dll" ||
		place.Percentage != 25 || place.Done || place.Indeterminate {
		t.Fatalf("unexpected place update: %+v", place)
	}
	if p := got[1]; p.Stage != "resolve" || p.Current != 7 || p.Total != 0 || !p.Indeterminate || p.Percentage != 0 || p.Done {
		t.Fatalf("unknown total must be indeterminate: %+v", p)
	}
	if p := got[2]; !p.Done || p.Indeterminate || p.Percentage != 100 {
		t.Fatalf("done bar with unknown total: %+v", p)
	}
	if p := got[3]; p.ProgressID != "dl-1" || p.Message != "Downloading https://example.com/a.zip" || p.Percentage != 42.5 ||
		p.Stage != "" || p.Indeterminate || p.Done || p.Total != 0 {
		t.Fatalf("legacy update: %+v", p)
	}
}

func TestProgressThrottle(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	bar := func(cur, total int64, item string) report.Progress {
		return report.Progress{ID: "b", Stage: report.StageDownload, Current: cur, Total: total, Unit: report.UnitBytes, Item: item}
	}
	steps := []struct {
		p    report.Progress
		wait time.Duration // clock advance before the update
		sent bool
	}{
		{bar(0, 100, "x"), 0, true},                       // first update
		{bar(10, 100, "x"), 10 * time.Millisecond, false}, // too soon
		{bar(20, 100, "x"), 50 * time.Millisecond, false},
		{bar(30, 100, "x"), 40 * time.Millisecond, true}, // 100ms since the last sent
		{bar(40, 100, "x"), 0, false},
		{bar(40, 200, "x"), 0, true}, // Total changed
		{bar(50, 200, "y"), 0, true}, // Item changed
		{report.Progress{ID: "b", Stage: report.StageExtract, Current: 60, Total: 200, Unit: report.UnitBytes, Item: "y"}, 0, true}, // Stage changed
		{report.Progress{ID: "b", Stage: report.StageExtract, Current: 70, Total: 200, Unit: report.UnitBytes, Item: "y"}, 0, false},
		{report.Progress{ID: "b", Stage: report.StageExtract, Current: 200, Total: 200, Unit: report.UnitBytes, Item: "y"}, 0, true}, // reached Total
		{report.Progress{ID: "b", Stage: report.StageExtract, Current: 200, Total: 200, Unit: report.UnitBytes, Item: "y", Done: true}, 0, true},
	}
	sent := 0
	for i, s := range steps {
		clock.advance(s.wait)
		r.ReportProgress(s.p)
		got := progressEvents(t, rec.snapshot())
		if s.sent {
			sent++
		}
		if len(got) != sent {
			t.Fatalf("step %d: %d updates sent, want %d", i, len(got), sent)
		}
		if s.sent && got[len(got)-1].Current != s.p.Current {
			t.Fatalf("step %d sent %+v, want Current %d", i, got[len(got)-1], s.p.Current)
		}
	}
	checkBar(t, progressEvents(t, rec.snapshot()))
	if len(r.bars) != 0 {
		t.Fatalf("Done must forget the bar: %+v", r.bars)
	}

	// A new update with a finished bar's ID opens a new bar.
	r.ReportProgress(bar(1, 100, "x"))
	if got := progressEvents(t, rec.snapshot()); len(got) != sent+1 {
		t.Fatalf("reopened bar not sent: %d updates", len(got))
	}
}

// Bars are throttled independently, and legacy updates not at all.
func TestProgressThrottlePerBar(t *testing.T) {
	r, rec, _ := testTaskReporter(t)
	for i := int64(0); i < 5; i++ {
		r.ReportProgress(report.Progress{ID: "a", Stage: report.StagePlace, Current: i, Total: 10})
		r.ReportProgress(report.Progress{ID: "b", Stage: report.StagePlace, Current: i, Total: 10})
		r.Progress("legacy", "Downloading https://example.com/a.zip", float64(i))
	}
	counts := map[string]int{}
	for _, p := range progressEvents(t, rec.snapshot()) {
		counts[p.ProgressID]++
	}
	if counts["a"] != 1 || counts["b"] != 1 || counts["legacy"] != 5 {
		t.Fatalf("unexpected update counts: %v", counts)
	}
}

// A long measured stream is thinned to the rate limit and still ends on its
// exact last values.
func TestProgressThrottleStream(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	const total = 1000
	for i := int64(0); i <= total; i++ {
		r.ReportProgress(report.Progress{ID: "s", Stage: report.StagePlace, Current: i, Total: total, Unit: report.UnitBytes, Done: i == total})
		clock.advance(time.Millisecond)
	}
	got := progressEvents(t, rec.snapshot())
	checkBar(t, got)
	// The first update (0ms), one per 100ms up to 900ms and the Done one.
	if len(got) != 11 {
		t.Fatalf("sent %d updates for a 1s stream", len(got))
	}
	if last := got[len(got)-1]; last.Current != total || last.Percentage != 100 {
		t.Fatalf("stream ended at %+v", last)
	}
}

// flush sends what the throttle holds back, once.
func TestProgressFlush(t *testing.T) {
	r, rec, _ := testTaskReporter(t)
	r.ReportProgress(report.Progress{ID: "f", Stage: report.StageRemove, Current: 1, Total: 5})
	r.ReportProgress(report.Progress{ID: "f", Stage: report.StageRemove, Current: 3, Total: 5})
	r.ReportProgress(report.Progress{ID: "g", Stage: report.StageRemove, Current: 1, Total: 5})
	r.flush()
	r.flush()
	got := progressEvents(t, rec.snapshot())
	if len(got) != 3 || got[2].ProgressID != "f" || got[2].Current != 3 {
		t.Fatalf("unexpected updates after flush: %+v", got)
	}
}

// A task's held-back progress goes out before lip_task_finished, on success
// and on error.
func TestRunPackageTaskFlushesProgress(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("fail=", fail), func(t *testing.T) {
			rec := recordEvents(t)
			useFakeProgressClock(t)
			state := useFakeEngine(t)
			state.onCall = func(ctx context.Context, r report.Reporter) error {
				for i := int64(1); i <= 3; i++ {
					report.Report(r, report.Progress{ID: "t", Stage: report.StageTask, Current: i, Total: 5, Unit: report.UnitPackages})
				}
				if fail {
					return fmt.Errorf("boom")
				}
				return nil
			}
			err := InstallPackages(context.Background(), t.TempDir(), []string{"x"})
			if (err != nil) != fail {
				t.Fatalf("err = %v", err)
			}
			events := rec.snapshot()
			if events[len(events)-1].name != EventLipTaskFinished {
				t.Fatalf("last event %s", events[len(events)-1].name)
			}
			got := progressEvents(t, events)
			if len(got) != 2 || got[0].Current != 1 || got[1].Current != 3 {
				t.Fatalf("unexpected updates: %+v", got)
			}
		})
	}
}

// Concurrent bars keep their own order and all end Done.
func TestProgressConcurrentBars(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	var wg sync.WaitGroup
	for b := 0; b < 8; b++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for i := int64(0); i <= 200; i++ {
				r.ReportProgress(report.Progress{ID: id, Stage: report.StagePlace, Current: i, Total: 200, Done: i == 200})
				clock.advance(time.Millisecond)
			}
		}(fmt.Sprint("bar-", b))
	}
	wg.Wait()
	bars := map[string][]LipTaskProgressEvent{}
	for _, p := range progressEvents(t, rec.snapshot()) {
		bars[p.ProgressID] = append(bars[p.ProgressID], p)
	}
	if len(bars) != 8 {
		t.Fatalf("got %d bars", len(bars))
	}
	for _, bar := range bars {
		checkBar(t, bar)
	}
}

func TestLogReporterIgnoresProgress(t *testing.T) {
	rec := recordEvents(t)
	report.Report(logReporter{}, report.Progress{ID: "x", Stage: report.StagePlace, Current: 1, Total: 2, Done: true})
	if len(rec.snapshot()) != 0 {
		t.Fatalf("logReporter emitted %+v", rec.snapshot())
	}
}
