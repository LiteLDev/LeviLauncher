package engine

import "github.com/liteldev/LeviLauncher/internal/lipcore/report"

// Measured progress of the engine. Every value comes from counted work
// (packages, package files, objects); the installer reports its own place,
// remove and scripts bars, and fetch.DownloadFile its download bars.

// engineBar sends the updates of one engine-level bar (resolve, task).
// Methods are no-ops on a nil bar and after done, so a deferred done backs up
// an explicit one.
type engineBar struct {
	rep    report.Reporter
	p      report.Progress
	closed bool
}

// newBar opens a bar with a fresh id and reports its first update
// (Current 0).
func (e *Engine) newBar(stage report.Stage, unit report.Unit, message string, total int64) *engineBar {
	b := &engineBar{
		rep: e.rep,
		p:   report.Progress{ID: e.newID(), Stage: stage, Message: message, Total: total, Unit: unit},
	}
	report.Report(b.rep, b.p)
	return b
}

// update reports cur as the amount done, working on item when not empty.
func (b *engineBar) update(cur int64, item string) {
	if b == nil || b.closed {
		return
	}
	b.p.Current = cur
	if item != "" {
		b.p.Item = item
	}
	report.Report(b.rep, b.p)
}

// step reports cur package steps done, now working on pkg.
func (b *engineBar) step(cur int64, pkg, message string) {
	if b == nil || b.closed {
		return
	}
	b.p.Package = pkg
	b.p.Message = message
	b.update(cur, pkg)
}

// done reports the last update of the bar with the amount done so far.
func (b *engineBar) done() {
	if b == nil || b.closed {
		return
	}
	b.closed = true
	b.p.Done = true
	report.Report(b.rep, b.p)
}

// doneAt is done with cur as the amount done when it is more than the amount
// last reported, so a bar ended early still counts finished work.
func (b *engineBar) doneAt(cur int64) {
	if b == nil || b.closed {
		return
	}
	if cur > b.p.Current {
		b.p.Current = cur
	}
	b.done()
}

// reportBar sends p with report.Report, keeping lip's percentages for a
// legacy reporter (one without ReportProgress) where the fallback would
// differ: the fallback turns the Done update of an indeterminate bar into
// 100, so the Done update of a failed or abandoned bar is not sent to it,
// and it drops indeterminate updates, so with legacyOpen the bar's first
// update (Current 0) is sent as 0.
func reportBar(r report.Reporter, p report.Progress, failed, legacyOpen bool) {
	if _, ok := r.(report.ProgressReporter); !ok {
		if p.Done && failed {
			return
		}
		if p.Total <= 0 && !p.Done {
			if legacyOpen && p.Current == 0 {
				r.Progress(p.ID, p.Message, 0)
			}
			return
		}
	}
	report.Report(r, p)
}
