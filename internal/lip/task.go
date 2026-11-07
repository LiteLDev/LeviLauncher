package lip

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

const (
	EventLipTaskStarted  = "lip_task_started"
	EventLipTaskLog      = "lip_task_log"
	EventLipTaskProgress = "lip_task_progress"
	EventLipTaskFinished = "lip_task_finished"
)

var lipTaskCounter uint64

type LipTaskStartedEvent struct {
	TaskID    string   `json:"taskId"`
	Method    string   `json:"method"`
	Target    string   `json:"target"`
	Packages  []string `json:"packages"`
	Timestamp int64    `json:"timestamp"`
}

type LipTaskLogEvent struct {
	TaskID    string   `json:"taskId"`
	Method    string   `json:"method"`
	Target    string   `json:"target"`
	Packages  []string `json:"packages"`
	Timestamp int64    `json:"timestamp"`
	Level     string   `json:"level"`
	Message   string   `json:"message"`
	Raw       string   `json:"raw"`
}

// LipTaskProgressEvent is one update of a progress bar of a task; updates
// with the same ProgressID replace each other. Structured updates carry
// Stage (a report.Stage), Current and Total counted in Unit, and Done on the
// bar's last update. Legacy updates (Reporter.Progress) carry only Message
// and Percentage, with an empty Stage.
type LipTaskProgressEvent struct {
	TaskID     string   `json:"taskId"`
	Method     string   `json:"method"`
	Target     string   `json:"target"`
	Packages   []string `json:"packages"`
	Timestamp  int64    `json:"timestamp"`
	ProgressID string   `json:"progressId"`
	Message    string   `json:"message"`
	Percentage float64  `json:"percentage"`
	Stage      string   `json:"stage"`
	Package    string   `json:"package"`
	Current    int64    `json:"current"`
	Total      int64    `json:"total"`
	Unit       string   `json:"unit"`
	Item       string   `json:"item"`
	Done       bool     `json:"done"`
	// Indeterminate is set on structured updates of a running bar whose
	// total is unknown (Total <= 0).
	Indeterminate bool `json:"indeterminate"`
}

type LipTaskFinishedEvent struct {
	TaskID     string   `json:"taskId"`
	Method     string   `json:"method"`
	Target     string   `json:"target"`
	Packages   []string `json:"packages"`
	Timestamp  int64    `json:"timestamp"`
	Success    bool     `json:"success"`
	Error      string   `json:"error"`
	Stderr     string   `json:"stderr"`
	DurationMs int64    `json:"durationMs"`
}

type lipTaskMeta struct {
	TaskID    string
	Method    string
	Target    string
	Packages  []string
	StartedAt time.Time
}

func nextLipTaskID() string {
	seq := atomic.AddUint64(&lipTaskCounter, 1)
	return fmt.Sprintf("lip-%d-%d", time.Now().UnixNano(), seq)
}

func currentUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func normalizeTaskTarget(workDir string) string {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	base := strings.TrimSpace(filepath.Base(cleaned))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return cleaned
	}
	return base
}

func normalizePackages(packages []string) []string {
	out := make([]string, 0, len(packages))
	for _, item := range packages {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func newTaskMeta(method string, workDir string, packages []string) lipTaskMeta {
	return lipTaskMeta{
		TaskID:    nextLipTaskID(),
		Method:    strings.TrimSpace(method),
		Target:    normalizeTaskTarget(workDir),
		Packages:  append([]string{}, packages...),
		StartedAt: time.Now(),
	}
}

func emitTaskStarted(meta lipTaskMeta) {
	emitEvent(EventLipTaskStarted, LipTaskStartedEvent{
		TaskID:    meta.TaskID,
		Method:    meta.Method,
		Target:    meta.Target,
		Packages:  append([]string{}, meta.Packages...),
		Timestamp: currentUnixMilli(),
	})
}

func emitTaskLog(meta lipTaskMeta, level string, message string, raw string) {
	emitEvent(EventLipTaskLog, LipTaskLogEvent{
		TaskID:    meta.TaskID,
		Method:    meta.Method,
		Target:    meta.Target,
		Packages:  append([]string{}, meta.Packages...),
		Timestamp: currentUnixMilli(),
		Level:     strings.TrimSpace(level),
		Message:   strings.TrimSpace(message),
		Raw:       strings.TrimSpace(raw),
	})
}

func emitTaskProgress(meta lipTaskMeta, progressID string, message string, percentage float64) {
	emitEvent(EventLipTaskProgress, LipTaskProgressEvent{
		TaskID:     meta.TaskID,
		Method:     meta.Method,
		Target:     meta.Target,
		Packages:   append([]string{}, meta.Packages...),
		Timestamp:  currentUnixMilli(),
		ProgressID: strings.TrimSpace(progressID),
		Message:    strings.TrimSpace(message),
		Percentage: percentage,
	})
}

func emitTaskStructuredProgress(meta lipTaskMeta, p report.Progress) {
	emitEvent(EventLipTaskProgress, LipTaskProgressEvent{
		TaskID:        meta.TaskID,
		Method:        meta.Method,
		Target:        meta.Target,
		Packages:      append([]string{}, meta.Packages...),
		Timestamp:     currentUnixMilli(),
		ProgressID:    strings.TrimSpace(p.ID),
		Message:       strings.TrimSpace(p.Message),
		Percentage:    p.Percentage(),
		Stage:         string(p.Stage),
		Package:       p.Package,
		Current:       p.Current,
		Total:         p.Total,
		Unit:          string(p.Unit),
		Item:          p.Item,
		Done:          p.Done,
		Indeterminate: p.Total <= 0 && !p.Done && p.Stage != "",
	})
}

func emitTaskFinished(meta lipTaskMeta, success bool, taskError string) {
	duration := time.Since(meta.StartedAt).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	emitEvent(EventLipTaskFinished, LipTaskFinishedEvent{
		TaskID:     meta.TaskID,
		Method:     meta.Method,
		Target:     meta.Target,
		Packages:   append([]string{}, meta.Packages...),
		Timestamp:  currentUnixMilli(),
		Success:    success,
		Error:      strings.TrimSpace(taskError),
		DurationMs: duration,
	})
}

// progressInterval is the minimum time between two forwarded updates of one
// bar that only advance Current or change Item. It limits how often a bar is
// redrawn, never what it shows: every forwarded value is a measured one, and
// an update held back goes out at most progressInterval after the last one
// forwarded.
const progressInterval = 100 * time.Millisecond

// progressNow is the clock of the progress throttle; replaced in tests.
var progressNow = time.Now

// progressAfterFunc runs f after d unless the returned stop is called first;
// it schedules the trailing update of a throttled bar. Replaced in tests.
var progressAfterFunc = func(d time.Duration, f func()) (stop func() bool) {
	return time.AfterFunc(d, f).Stop
}

// barState is what a taskReporter remembers of one open bar.
type barState struct {
	last    report.Progress  // last forwarded update
	sentAt  time.Time        // when last was forwarded
	pending *report.Progress // latest update held back since, if any
	stop    func() bool      // cancels the scheduled send of pending, if any
	seq     uint64           // numbers scheduled sends, so a stale one is ignored
}

// taskReporter forwards engine reports of one task as lip_task_* events.
// It throttles structured progress per bar; mu guards that state, so a
// taskReporter is safe for concurrent use.
type taskReporter struct {
	meta lipTaskMeta

	mu     sync.Mutex
	bars   map[string]*barState // open bars by ID
	closed bool                 // set by close; later structured updates are dropped
}

func newTaskReporter(meta lipTaskMeta) *taskReporter {
	return &taskReporter{meta: meta, bars: make(map[string]*barState)}
}

func (r *taskReporter) logMessage(level string, message string) {
	log.Printf("lip %s: %s", level, message)
	emitTaskLog(r.meta, level, message, message)
}

func (r *taskReporter) Info(message string) { r.logMessage("info", message) }

// Success is reported as info: lip prints step-level successes, which the UI
// would otherwise mistake for task completion.
func (r *taskReporter) Success(message string) { r.logMessage("info", message) }

func (r *taskReporter) Warning(message string) { r.logMessage("warning", message) }

func (r *taskReporter) Error(message string) { r.logMessage("error", message) }

// Progress forwards a legacy update as is; its sources throttle themselves
// (lip's download progress moves in steps of at least 0.1%).
func (r *taskReporter) Progress(id string, message string, percentage float64) {
	emitTaskProgress(r.meta, id, message, percentage)
}

// ReportProgress forwards structured progress. It always forwards the first
// update of a bar, its Done update, an update that reaches a known total and
// one that changes the bar's Stage or Total; other updates of a bar (a new
// Current or Item, as per-file bars send for every file) go out at most once
// per progressInterval. The latest update held back is sent when that
// interval ends, unless a later update of its bar went out first, so a bar
// never shows a value older than progressInterval.
func (r *taskReporter) ReportProgress(p report.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	now := progressNow()
	bar, open := r.bars[p.ID]
	send := !open || p.Done || (p.Total > 0 && p.Current >= p.Total) ||
		p.Stage != bar.last.Stage || p.Total != bar.last.Total ||
		now.Sub(bar.sentAt) >= progressInterval
	if !send {
		held := p
		bar.pending = &held
		if bar.stop == nil {
			bar.seq++
			seq := bar.seq
			bar.stop = progressAfterFunc(progressInterval-now.Sub(bar.sentAt), func() { r.sendPending(p.ID, bar, seq) })
		}
		return
	}
	switch {
	case p.Done:
		bar.cancel()
		delete(r.bars, p.ID)
	case open:
		bar.cancel()
		bar.last, bar.sentAt, bar.pending = p, now, nil
	default:
		r.bars[p.ID] = &barState{last: p, sentAt: now}
	}
	// Emitting under mu keeps the updates of a bar in order.
	emitTaskStructuredProgress(r.meta, p)
}

// cancel stops the scheduled send of bar's held-back update. A nil bar (one
// not open yet) has nothing scheduled.
func (bar *barState) cancel() {
	if bar != nil && bar.stop != nil {
		bar.stop()
		bar.stop = nil
	}
}

// sendPending is the scheduled send of bar's held-back update. It does
// nothing if the reporter was closed, bar finished, or the send numbered seq
// was cancelled because bar forwarded an update since.
func (r *taskReporter) sendPending(id string, bar *barState, seq uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.bars[id] != bar || bar.stop == nil || bar.seq != seq {
		return
	}
	bar.stop = nil
	if bar.pending == nil {
		return
	}
	p := *bar.pending
	bar.last, bar.sentAt, bar.pending = p, progressNow(), nil
	emitTaskStructuredProgress(r.meta, p)
}

// close ends every bar still open with one Done update carrying its latest
// measured values (the one held back, if any), so a bar the engine left open
// still ends Done, and drops structured updates from then on. finishTask
// calls it before lip_task_finished.
func (r *taskReporter) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	ids := make([]string, 0, len(r.bars))
	for id := range r.bars {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		bar := r.bars[id]
		bar.cancel()
		p := bar.last
		if bar.pending != nil {
			p = *bar.pending
		}
		p.Done = true
		emitTaskStructuredProgress(r.meta, p)
	}
	r.bars = nil
}

// logReporter writes engine reports of untracked (quiet) calls to the log.
type logReporter struct{}

func (logReporter) Info(message string)              { log.Printf("lip info: %s", message) }
func (logReporter) Success(message string)           { log.Printf("lip success: %s", message) }
func (logReporter) Warning(message string)           { log.Printf("lip warning: %s", message) }
func (logReporter) Error(message string)             { log.Printf("lip error: %s", message) }
func (logReporter) Progress(string, string, float64) {}
func (logReporter) ReportProgress(report.Progress)   {}

var (
	_ report.ProgressReporter = (*taskReporter)(nil)
	_ report.ProgressReporter = logReporter{}
)
