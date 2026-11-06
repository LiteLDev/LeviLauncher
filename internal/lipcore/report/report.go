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

// ProgressReporter is implemented by reporters that accept structured
// progress. Report falls back to Reporter.Progress for those that don't.
type ProgressReporter interface {
	ReportProgress(p Progress)
}

// Stage names the phase of an operation a progress bar measures.
type Stage string

const (
	// StageTask is the whole operation: packages applied out of the plan
	// (Unit packages). It is reported once the plan is known, after solving.
	StageTask Stage = "task"
	// StageResolve covers resolving the requested packages and solving
	// dependencies. Its Total is the number of requested packages while they
	// are resolved, and unknown (0) while solving, with Current counting the
	// manifests read.
	StageResolve Stage = "resolve"
	// StageGitFetch is receiving one git pack: objects received out of the
	// object count in the pack header (Unit objects), or blobs received out of
	// the blobs to fetch for a package (Unit blobs).
	StageGitFetch Stage = "git_fetch"
	// StageDownload is downloading one asset URL: bytes received out of
	// Content-Length (Unit bytes; Total 0 when the server sends none).
	StageDownload Stage = "download"
	// StageExtract is decompressing an archive before it can be read: bytes
	// of the compressed file consumed out of its size (Unit bytes).
	StageExtract Stage = "extract"
	// StagePlace is copying a package's files into the instance: bytes copied
	// out of the total size of the files, or files copied when a size is
	// unknown (Unit bytes or files).
	StagePlace Stage = "place"
	// StageScripts is running a package's scripts: commands finished out of
	// the commands of the current script hook (Unit commands).
	StageScripts Stage = "scripts"
	// StageRemove is removing a package's files: entries removed out of the
	// entries to remove (Unit files).
	StageRemove Stage = "remove"
	// StageCacheClean is deleting the download cache: entries deleted out of
	// the entries found by a walk done before deleting (Unit files).
	StageCacheClean Stage = "cache_clean"
)

// Unit names what Progress.Current and Progress.Total count.
type Unit string

const (
	UnitBytes    Unit = "bytes"
	UnitFiles    Unit = "files"
	UnitObjects  Unit = "objects"
	UnitBlobs    Unit = "blobs"
	UnitPackages Unit = "packages"
	UnitCommands Unit = "commands"
)

// Progress is one measured progress update. Every value must come from
// counted work (bytes, objects, files, packages), never from elapsed time.
type Progress struct {
	// ID identifies one bar; updates with the same ID replace each other.
	ID string
	// Stage is the phase this bar measures.
	Stage Stage
	// Package is the package the bar belongs to ("id@version"), if any.
	Package string
	// Message describes the bar in English, e.g. "Downloading <url>".
	Message string
	// Current is the amount done so far, Total the amount known to exist.
	// Total <= 0 means the total is unknown: show an indeterminate bar.
	Current int64
	Total   int64
	Unit    Unit
	// Item is the item being worked on (a file path, a URL), if any.
	Item string
	// Done marks the last update of the bar, whether it completed or was
	// abandoned (e.g. a failed download attempt).
	Done bool
}

// Percentage returns Current out of Total in [0, 100], or 100 for a Done bar
// with an unknown total and 0 for any other bar with an unknown total.
func (p Progress) Percentage() float64 {
	if p.Total <= 0 {
		if p.Done {
			return 100
		}
		return 0
	}
	cur := p.Current
	if cur < 0 {
		cur = 0
	}
	if cur > p.Total {
		cur = p.Total
	}
	return float64(cur) * 100 / float64(p.Total)
}

// Report sends p to r. A reporter without ReportProgress gets
// Progress(p.ID, p.Message, p.Percentage()) for bars with a known total and
// for Done bars; updates of indeterminate bars are dropped for it.
func Report(r Reporter, p Progress) {
	r = OrNop(r)
	if pr, ok := r.(ProgressReporter); ok {
		pr.ReportProgress(p)
		return
	}
	if p.Total > 0 || p.Done {
		r.Progress(p.ID, p.Message, p.Percentage())
	}
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
