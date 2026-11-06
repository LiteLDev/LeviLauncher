// Package report defines how lipcore reports log messages and progress,
// mirroring lip's daemon callbacks (PrintInfo, PrintSuccess, PrintWarning,
// PrintError, ReportProgress).
package report

// Reporter receives user-facing messages and progress from lipcore.
// Implementations must be safe for concurrent use.
type Reporter interface {
	Info(message string)
	Success(message string)
	Warning(message string)
	Error(message string)
	// Progress reports progress of one operation. id identifies the operation
	// (lip uses a GUID per download), message describes it (lip uses
	// "Downloading <url>") and percentage is in [0, 100].
	Progress(id string, message string, percentage float64)
}

// Nop is a Reporter that discards everything.
type Nop struct{}

func (Nop) Info(string)                      {}
func (Nop) Success(string)                   {}
func (Nop) Warning(string)                   {}
func (Nop) Error(string)                     {}
func (Nop) Progress(string, string, float64) {}

// OrNop returns r, or Nop if r is nil.
func OrNop(r Reporter) Reporter {
	if r == nil {
		return Nop{}
	}
	return r
}
