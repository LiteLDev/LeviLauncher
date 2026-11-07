package lip

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// fakeProgressClock replaces the throttle's clock and timers with ones
// advanced by hand: advance only moves the clock, fireDue runs the scheduled
// sends that are due.
type fakeProgressClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeProgressTimer
}

type fakeProgressTimer struct {
	due  time.Time
	f    func()
	done bool // stopped or fired
}

func (c *fakeProgressClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// fireDue runs the scheduled sends due by now, outside c.mu (they read the
// clock).
func (c *fakeProgressClock) fireDue() {
	c.mu.Lock()
	var due []func()
	for _, tm := range c.timers {
		if !tm.done && !tm.due.After(c.now) {
			tm.done = true
			due = append(due, tm.f)
		}
	}
	c.mu.Unlock()
	for _, f := range due {
		f()
	}
}

// scheduled reports how many sends are scheduled and not yet stopped or run.
func (c *fakeProgressClock) scheduled() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, tm := range c.timers {
		if !tm.done {
			n++
		}
	}
	return n
}

func useFakeProgressClock(t *testing.T) *fakeProgressClock {
	t.Helper()
	c := &fakeProgressClock{now: time.Unix(1700000000, 0)}
	oldNow, oldAfter := progressNow, progressAfterFunc
	progressNow = func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.now
	}
	progressAfterFunc = func(d time.Duration, f func()) func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		tm := &fakeProgressTimer{due: c.now.Add(d), f: f}
		c.timers = append(c.timers, tm)
		return func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			stopped := !tm.done
			tm.done = true
			return stopped
		}
	}
	t.Cleanup(func() { progressNow, progressAfterFunc = oldNow, oldAfter })
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
		{bar(40, 200, "x"), 0, true},  // Total changed
		{bar(50, 200, "y"), 0, false}, // a new Item alone is throttled
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

// close ends every open bar with one Done update carrying its latest
// values, once, and drops what comes after it.
func TestProgressClose(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	r.ReportProgress(report.Progress{ID: "f", Stage: report.StageRemove, Current: 1, Total: 5})
	r.ReportProgress(report.Progress{ID: "f", Stage: report.StageRemove, Current: 3, Total: 5})
	r.ReportProgress(report.Progress{ID: "g", Stage: report.StageRemove, Current: 1})
	r.close()
	r.close()
	r.ReportProgress(report.Progress{ID: "h", Stage: report.StageRemove, Current: 1, Total: 5})
	clock.advance(time.Second)
	clock.fireDue()
	got := progressEvents(t, rec.snapshot())
	if len(got) != 4 {
		t.Fatalf("unexpected updates after close: %+v", got)
	}
	if p := got[2]; p.ProgressID != "f" || p.Current != 3 || p.Total != 5 || !p.Done || p.Percentage != 60 {
		t.Fatalf("held-back bar closed as %+v", p)
	}
	if p := got[3]; p.ProgressID != "g" || p.Current != 1 || !p.Done || p.Indeterminate {
		t.Fatalf("open bar closed as %+v", p)
	}
	if clock.scheduled() != 0 {
		t.Fatal("close left a send scheduled")
	}
	bars := map[string][]LipTaskProgressEvent{}
	for _, p := range got {
		bars[p.ProgressID] = append(bars[p.ProgressID], p)
	}
	for _, bar := range bars {
		checkBar(t, bar)
	}
}

// A per-file bar names a new Item in every update; that alone does not get
// past the throttle.
func TestProgressThrottlePerFileItems(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	const files = 20000
	for i := int64(1); i <= files; i++ {
		r.ReportProgress(report.Progress{ID: "c", Stage: report.StageCacheClean, Current: i, Total: files,
			Unit: report.UnitFiles, Item: fmt.Sprintf("cache/%d", i), Done: i == files})
		clock.advance(10 * time.Microsecond) // 0.2s for all files
		clock.fireDue()
	}
	got := progressEvents(t, rec.snapshot())
	checkBar(t, got)
	if len(got) > 5 {
		t.Fatalf("%d files sent %d updates", files, len(got))
	}
	if last := got[len(got)-1]; last.Current != files || last.Item != fmt.Sprintf("cache/%d", files) {
		t.Fatalf("bar ended at %+v", last)
	}
}

// A held-back update goes out when its interval ends, even if its bar stalls.
func TestProgressTrailingSend(t *testing.T) {
	r, rec, clock := testTaskReporter(t)
	bar := func(cur int64) report.Progress {
		return report.Progress{ID: "d", Stage: report.StageDownload, Current: cur, Total: 100 << 20, Unit: report.UnitBytes}
	}
	r.ReportProgress(bar(40 << 20))
	clock.advance(50 * time.Millisecond)
	r.ReportProgress(bar(45 << 20))
	r.ReportProgress(bar(49 << 20)) // replaces 45MB, schedules no second send
	if n := clock.scheduled(); n != 1 {
		t.Fatalf("%d sends scheduled", n)
	}
	clock.advance(49 * time.Millisecond)
	clock.fireDue()
	if got := progressEvents(t, rec.snapshot()); len(got) != 1 {
		t.Fatalf("sent early: %+v", got)
	}
	clock.advance(time.Millisecond)
	clock.fireDue()
	got := progressEvents(t, rec.snapshot())
	if len(got) != 2 || got[1].Current != 49<<20 {
		t.Fatalf("stalled bar shows %+v", got)
	}

	// An update forwarded in the meantime cancels the scheduled send, and a
	// stale send that runs anyway changes nothing.
	clock.advance(10 * time.Millisecond)
	r.ReportProgress(bar(50 << 20))
	r.mu.Lock()
	staleSeq := r.bars["d"].seq
	r.mu.Unlock()
	clock.advance(100 * time.Millisecond)
	r.ReportProgress(bar(60 << 20)) // interval over: sent, cancels the send
	if n := clock.scheduled(); n != 0 {
		t.Fatalf("%d sends left scheduled", n)
	}
	r.ReportProgress(bar(61 << 20)) // held, new send scheduled
	r.mu.Lock()
	b := r.bars["d"]
	r.mu.Unlock()
	r.sendPending("d", b, staleSeq)
	got = progressEvents(t, rec.snapshot())
	if len(got) != 3 || got[2].Current != 60<<20 {
		t.Fatalf("unexpected updates: %+v", got)
	}
	r.ReportProgress(report.Progress{ID: "d", Stage: report.StageDownload, Current: 61 << 20, Total: 100 << 20, Done: true})
	clock.advance(time.Second)
	clock.fireDue()
	got = progressEvents(t, rec.snapshot())
	if len(got) != 4 {
		t.Fatalf("send after Done: %+v", got)
	}
	checkBar(t, got)
}

// A bar the engine leaves open is closed with its held-back values before
// lip_task_finished, on success and on error.
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
			if len(got) != 2 || got[0].Current != 1 || got[1].Current != 3 || !got[1].Done {
				t.Fatalf("unexpected updates: %+v", got)
			}
			checkBar(t, got)
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
