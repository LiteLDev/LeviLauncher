package workspace

import (
	"context"
	"fmt"
	"io"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// Sizer is optionally implemented by a Source that knows the size of its
// entries before they are read (archive headers, cached files). When every
// placed entry has a known size, placement progress counts bytes; otherwise
// it counts files.
type Sizer interface {
	// Size returns the size in bytes of the entry key, or false when it is
	// not known.
	Size(key string) (int64, bool)
}

type packageKey struct{}

// WithPackage returns ctx carrying the package ("id@version") a workspace
// operation works on; progress updates name it in Progress.Package.
func WithPackage(ctx context.Context, pkg string) context.Context {
	return context.WithValue(ctx, packageKey{}, pkg)
}

func packageOf(ctx context.Context) string {
	pkg, _ := ctx.Value(packageKey{}).(string)
	return pkg
}

// minByteStep is the smallest byte progress step reported between file
// boundaries, so copying does not report on every write.
const minByteStep = 64 << 10

// progressBar sends the updates of one bar. Every value it reports comes
// from counted work; done must be called exactly once (it is idempotent, so
// a deferred call can back up an explicit one).
type progressBar struct {
	rep    report.Reporter
	p      report.Progress
	closed bool
}

// newBar opens a bar and reports its first update (Current 0). Its ID is
// "<stage>:<package>:<n>", unique within this Workspace.
func (w *Workspace) newBar(ctx context.Context, stage report.Stage, unit report.Unit, message string, total int64) *progressBar {
	pkg := packageOf(ctx)
	b := &progressBar{
		rep: report.OrNop(w.Reporter),
		p: report.Progress{
			ID:      fmt.Sprintf("%s:%s:%d", stage, pkg, w.barSeq.Add(1)),
			Stage:   stage,
			Package: pkg,
			Message: message,
			Total:   total,
			Unit:    unit,
		},
	}
	b.send()
	return b
}

func (b *progressBar) send() { report.Report(b.rep, b.p) }

// update reports cur (and item, when not empty) as the amount done.
func (b *progressBar) update(cur int64, item string) {
	if b.closed {
		return
	}
	b.setCurrent(cur)
	if item != "" {
		b.p.Item = item
	}
	b.send()
}

// setTotal reports a total learned after the bar was opened.
func (b *progressBar) setTotal(total int64) {
	if b.closed {
		return
	}
	b.p.Total = total
	b.send()
}

// done reports the last update of the bar with the amount done so far.
func (b *progressBar) done() {
	if b.closed {
		return
	}
	b.closed = true
	b.p.Done = true
	b.send()
}

// doneAt is done with cur as the amount done (when it is more than the
// amount last reported), so a bar ended early still counts finished work.
func (b *progressBar) doneAt(cur int64) {
	if !b.closed && cur > b.p.Current {
		b.setCurrent(cur)
	}
	b.done()
}

// setCurrent sets the amount done. A known Total is only a declaration (a
// Sizer's size may understate the bytes a stream yields); when counted work
// passes it, Total is raised to the counted amount so Current never exceeds
// Total within a bar.
func (b *progressBar) setCurrent(cur int64) {
	b.p.Current = cur
	if b.p.Total > 0 && cur > b.p.Total {
		b.p.Total = cur
	}
}

// byteCounter counts bytes written to a placed file into a bar, reporting
// at most once per step bytes.
type byteCounter struct {
	bar       *progressBar
	step      int64
	cur, last int64
}

func newByteCounter(bar *progressBar, total int64) *byteCounter {
	step := total / 1000
	if step < minByteStep {
		step = minByteStep
	}
	return &byteCounter{bar: bar, step: step}
}

func (c *byteCounter) add(n int64) {
	c.cur += n
	if c.cur-c.last >= c.step {
		c.last = c.cur
		c.bar.update(c.cur, "")
	}
}

// startFile reports the bytes counted so far as a new file, item, starts.
func (c *byteCounter) startFile(item string) {
	c.last = c.cur
	c.bar.update(c.cur, item)
}

// countingWriter passes writes to w and counts them.
type countingWriter struct {
	w   io.Writer
	add func(int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		c.add(int64(n))
	}
	return n, err
}
