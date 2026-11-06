package lip

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
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

type LipTaskProgressEvent struct {
	TaskID     string   `json:"taskId"`
	Method     string   `json:"method"`
	Target     string   `json:"target"`
	Packages   []string `json:"packages"`
	Timestamp  int64    `json:"timestamp"`
	ProgressID string   `json:"progressId"`
	Message    string   `json:"message"`
	Percentage float64  `json:"percentage"`
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

// taskReporter forwards engine reports of one task as lip_task_* events.
// It holds no mutable state, so it is safe for concurrent use.
type taskReporter struct {
	meta lipTaskMeta
}

func (r taskReporter) logMessage(level string, message string) {
	log.Printf("lip %s: %s", level, message)
	emitTaskLog(r.meta, level, message, message)
}

func (r taskReporter) Info(message string) { r.logMessage("info", message) }

// Success is reported as info: lip prints step-level successes, which the UI
// would otherwise mistake for task completion.
func (r taskReporter) Success(message string) { r.logMessage("info", message) }

func (r taskReporter) Warning(message string) { r.logMessage("warning", message) }

func (r taskReporter) Error(message string) { r.logMessage("error", message) }

func (r taskReporter) Progress(id string, message string, percentage float64) {
	emitTaskProgress(r.meta, id, message, percentage)
}

// logReporter writes engine reports of untracked (quiet) calls to the log.
type logReporter struct{}

func (logReporter) Info(message string)              { log.Printf("lip info: %s", message) }
func (logReporter) Success(message string)           { log.Printf("lip success: %s", message) }
func (logReporter) Warning(message string)           { log.Printf("lip warning: %s", message) }
func (logReporter) Error(message string)             { log.Printf("lip error: %s", message) }
func (logReporter) Progress(string, string, float64) {}
