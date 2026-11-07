package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/update"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
)

type progressMode int

const (
	progressOff   progressMode = iota // -q, --json, or not a console without -v
	progressLine                      // stderr is a console: one redrawn line
	progressPlain                     // -v without a console: one line per phase
)

// Progress reports the progress of a long operation on stderr:
//
//   - stderr is a console: a single line, redrawn in place;
//   - not a console, with -v: one plain line per phase;
//   - -q, --json, or not a console without -v: nothing.
//
// Get it with Ctx.Progress. Writes through Notice, Warn and errors clear the
// line first, so they never mix with it.
type Progress struct {
	c        *Ctx
	mode     progressMode
	label    string
	shown    int // runes on screen
	lastDraw time.Time
	width    func() int
	mu       sync.Mutex
	listen   []func(name string, data any)
}

func newProgress(c *Ctx) *Progress {
	p := &Progress{c: c, width: consoleWidth}
	switch {
	case c.G.Quiet || c.G.JSON:
		p.mode = progressOff
	case c.streams.StderrConsole:
		p.mode = progressLine
	case c.G.Verbose:
		p.mode = progressPlain
	}
	return p
}

// Progress returns the command's progress reporter.
func (c *Ctx) Progress() *Progress {
	if c.prog == nil {
		c.prog = newProgress(c)
	}
	return c.prog
}

// Phase starts a new step: "Downloading Minecraft 1.21.50.07".
func (p *Progress) Phase(format string, a ...any) {
	label := strings.TrimRight(fmt.Sprintf(format, a...), "\n")
	p.c.errMu.Lock()
	defer p.c.errMu.Unlock()
	if label == p.label {
		return
	}
	p.label = label
	switch p.mode {
	case progressLine:
		p.drawLocked(label)
	case progressPlain:
		fmt.Fprintf(p.c.errw, "%s\n", label)
	}
}

// Bytes shows byte progress of the current phase.
func (p *Progress) Bytes(done, total int64) {
	if total > 0 {
		p.status(fmt.Sprintf("%3d%%  %s / %s", percent(done, total), Size(done), Size(total)), done >= total)
	} else {
		p.status(Size(done), false)
	}
}

// Count shows item progress of the current phase: "3/10 files".
func (p *Progress) Count(done, total int64, unit string) {
	s := fmt.Sprintf("%d", done)
	if total > 0 {
		s = fmt.Sprintf("%d/%d", done, total)
	}
	if unit != "" {
		s += " " + unit
	}
	p.status(s, total > 0 && done >= total)
}

// Percent shows a percentage of the current phase.
func (p *Progress) Percent(pct float64) {
	p.status(fmt.Sprintf("%3.0f%%", pct), pct >= 100)
}

func percent(done, total int64) int {
	if total <= 0 {
		return 0
	}
	v := int(done * 100 / total)
	return min(max(v, 0), 100)
}

func (p *Progress) status(s string, force bool) {
	if p.mode != progressLine {
		return
	}
	p.c.errMu.Lock()
	defer p.c.errMu.Unlock()
	if !force && time.Since(p.lastDraw) < 100*time.Millisecond {
		return
	}
	text := s
	if p.label != "" {
		text = p.label + "  " + s
	}
	p.drawLocked(text)
}

// Done clears the progress line.
func (p *Progress) Done() {
	p.c.errMu.Lock()
	defer p.c.errMu.Unlock()
	p.clearLocked()
	p.label = ""
}

func (p *Progress) drawLocked(text string) {
	if w := p.width(); w > 1 {
		if r := []rune(text); len(r) > w-1 {
			text = string(r[:w-2]) + "…"
		}
	}
	n := len([]rune(text))
	pad := ""
	if p.shown > n {
		pad = strings.Repeat(" ", p.shown-n)
	}
	fmt.Fprintf(p.c.errw, "\r%s%s", text, pad)
	if pad != "" {
		fmt.Fprintf(p.c.errw, "\r%s", text)
	}
	p.shown = n
	p.lastDraw = time.Now()
}

// clearLocked erases the progress line; the caller holds errMu.
func (p *Progress) clearLocked() {
	if p == nil || p.shown == 0 {
		return
	}
	fmt.Fprintf(p.c.errw, "\r%s\r", strings.Repeat(" ", p.shown))
	p.shown = 0
	p.lastDraw = time.Time{}
}

// Listen registers fn for every backend event of this command (download
// done/error, launch outcome...). It runs on the emitting goroutine; keep it
// short.
func (p *Progress) Listen(fn func(name string, data any)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listen = append(p.listen, fn)
}

// Sink returns an event sink that renders the backend's known progress
// events (downloads, extraction, LIP tasks, backups, self-update, runtime
// installers, loader migration, launch) and passes every event to the
// Listen functions. Run installs it as the process-wide default, so most
// commands never need to call it.
func (p *Progress) Sink() eventsink.Sink {
	return eventsink.Func(p.handle)
}

func (p *Progress) handle(name string, data ...any) {
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	p.mu.Lock()
	listeners := append([]func(string, any){}, p.listen...)
	p.mu.Unlock()
	for _, fn := range listeners {
		fn(name, payload)
	}
	p.render(name, payload)
}

// ProgressFunc returns a downloader progress callback drawing byte progress.
func (p *Progress) ProgressFunc() func(downloader.DownloadProgress) {
	return func(d downloader.DownloadProgress) { p.Bytes(d.Downloaded, d.Total) }
}

func (p *Progress) render(name string, payload any) {
	switch v := payload.(type) {
	case msixvc.DownloadProgress:
		p.Bytes(v.Downloaded, v.Total)
	case msixvc.DownloadStatus:
		if v.Status == "verifying" {
			p.Phase("Verifying download")
		}
	case types.ExtractProgress:
		p.c.errMu.Lock()
		unnamed := p.label == ""
		p.c.errMu.Unlock()
		if unnamed {
			p.Phase("Extracting")
		}
		p.Bytes(v.GlobalCurrent, v.GlobalTotal)
	case update.AppUpdateProgress:
		p.Bytes(v.Downloaded, v.Total)
	case vcruntime.EnsureProgress:
		p.Bytes(v.Downloaded, v.Total)
	case leviloader.MigrateProgress:
		p.Count(int64(v.Done), int64(v.Total), "instances")
	case types.InstanceBackupCreateProgress:
		if v.TotalBytes > 0 {
			p.Bytes(v.Bytes, v.TotalBytes)
		} else {
			p.Count(int64(v.Files), int64(v.TotalFiles), "files")
		}
	case types.InstanceBackupRestoreProgress:
		label := v.Phase
		if v.ScopeLabel != "" {
			label += " " + v.ScopeLabel
		}
		p.Phase("Restoring: %s (%d/%d)", label, v.CurrentStep, v.TotalSteps)
		if v.SubTotal > 0 {
			if v.SubUnit == "bytes" {
				p.Bytes(v.SubCurrent, v.SubTotal)
			} else {
				p.Count(v.SubCurrent, v.SubTotal, v.SubUnit)
			}
		}
	case lip.LipTaskProgressEvent:
		if v.Message != "" {
			p.Phase("%s", v.Message)
		}
		switch {
		case v.Total > 0 && v.Unit == "bytes":
			p.Bytes(v.Current, v.Total)
		case v.Total > 0:
			p.Count(v.Current, v.Total, v.Unit)
		case v.Percentage > 0:
			p.Percent(v.Percentage)
		}
	case lip.LipTaskLogEvent:
		if v.Message != "" {
			p.c.Verbosef("lip: %s", v.Message)
		}
	}
	switch name {
	case update.EventAppUpdateStatus:
		if s, ok := payload.(string); ok {
			p.Phase("Self-update: %s", s)
		}
	case launch.EventMcLaunchStart:
		p.Phase("Starting Minecraft")
	case leviloader.EventMigrateStart:
		p.Phase("Migrating the loader")
	case vcruntime.EventEnsureStart:
		p.Phase("Installing the Visual C++ runtime")
	case "gameinput.ensure.start":
		p.Phase("Installing GameInput")
	}
}
