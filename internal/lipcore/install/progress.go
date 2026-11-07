package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// Measured progress, as the 0.37 engine/workspace reported it: every value
// comes from counted work (bytes, files, commands), never from elapsed time.

// minByteStep is the smallest byte progress step reported between file
// boundaries, so copying does not report on every write.
const minByteStep = 64 << 10

// progressBar sends the updates of one bar. done must be called exactly
// once (it is idempotent, so a deferred call can back up an explicit one).
type progressBar struct {
	rep    report.Reporter
	p      report.Progress
	closed bool
}

// newBar opens a bar and reports its first update (Current 0). Its ID is
// "<stage>:<package>:<n>", unique within this Installer.
func (in *Installer) newBar(pkg string, stage report.Stage, unit report.Unit, message string, total int64) *progressBar {
	b := &progressBar{
		rep: report.OrNop(in.Reporter),
		p: report.Progress{
			ID:      fmt.Sprintf("%s:%s:%d", stage, pkg, in.barSeq.Add(1)),
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
// source's size may understate the bytes a stream yields); when counted
// work passes it, Total is raised to the counted amount so Current never
// exceeds Total within a bar.
func (b *progressBar) setCurrent(cur int64) {
	b.p.Current = cur
	if b.p.Total > 0 && cur > b.p.Total {
		b.p.Total = cur
	}
}

// byteCounter counts bytes written into a bar, reporting at most once per
// step bytes.
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

// countingReader passes reads from r and counts them, failing with the
// context's error once ctx is cancelled.
type countingReader struct {
	ctx context.Context
	r   io.Reader
	add func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := c.r.Read(p)
	if n > 0 && c.add != nil {
		c.add(int64(n))
	}
	return n, err
}

// lineWriter splits hook output into lines for the reporter.
type lineWriter struct {
	emit func(string)
	buf  []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.line(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush emits a trailing partial line.
func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.line(w.buf)
		w.buf = nil
	}
}

func (w *lineWriter) line(b []byte) {
	s := strings.TrimRight(string(b), "\r")
	if strings.TrimSpace(s) == "" {
		return
	}
	w.emit(s)
}
