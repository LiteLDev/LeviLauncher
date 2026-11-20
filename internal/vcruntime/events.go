package vcruntime

const (
	EventEnsureStart    = "vcruntime.ensure.start"
	EventEnsureProgress = "vcruntime.ensure.progress"
	EventEnsureDone     = "vcruntime.ensure.done"

	EventDownloadStart    = "vcruntime.download.start"
	EventDownloadProgress = "vcruntime.download.progress"
	EventDownloadDone     = "vcruntime.download.done"
	EventDownloadError    = "vcruntime.download.error"
	EventMissing          = "vcruntime.missing"
)

type EnsureProgress struct {
	Downloaded int64
	Total      int64
}
