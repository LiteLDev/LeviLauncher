package msixvc

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/downloader"
)

const (
	EventDownloadStatus     = "msixvc_download_status"
	EventDownloadProgress   = "msixvc_download_progress"
	EventDownloadDone       = "msixvc_download_done"
	EventDownloadError      = "msixvc_download_error"
	EventAppxInstallLoading = "appx_install_loading"
)

var msixMgr = downloader.NewManager(
	downloader.Events{
		Status:   EventDownloadStatus,
		Progress: EventDownloadProgress,
		Done:     EventDownloadDone,
		Error:    EventDownloadError,
		ProgressFactory: func(p downloader.DownloadProgress) any {
			return DownloadProgress{Downloaded: p.Downloaded, Total: p.Total, Dest: p.Dest}
		},
		StatusFactory: func(status string, dest string) any {
			return DownloadStatus{Status: status, Dest: dest}
		},
		DoneFactory: func(dest string) any {
			return DownloadDone{Dest: dest}
		},
		ErrorFactory: func(err string, dest string) any {
			return DownloadError{Error: err, Dest: dest}
		},
	},
	downloader.Options{Throttle: 250 * time.Millisecond, Resume: true, RemoveOnCancel: true},
)

type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Dest       string
}

type DownloadStatus struct {
	Status string
	Dest   string
}

type DownloadDone struct {
	Dest string
}

type DownloadError struct {
	Error string
	Dest  string
}

// StartDownload starts downloading an installer. It returns an error when the
// download cannot start, for example downloader.ErrBusy when another process
// is downloading the same file.
func StartDownload(ctx context.Context, rawurl string, md5sum string) (string, error) {
	dir, err := apppath.InstallersDir()
	if err != nil {
		return "", err
	}
	fname := deriveFilename(rawurl)
	dest := filepath.Join(dir, fname)
	return msixMgr.StartChecked(ctx, stripFilenameParam(rawurl), dest, md5sum)
}

// StartNamedDownload shares task controls/events with MSIXVC downloads while
// preserving the UWP archive extension and the signed CDN URL verbatim.
func StartNamedDownload(ctx context.Context, rawurl, filename string, verify func(string) error) (string, error) {
	if filename == "" || filepath.Base(filename) != filename || strings.ContainsAny(filename, `<>:"/\|?*`) {
		return "", fmt.Errorf("ERR_INVALID_NAME")
	}
	dir, err := apppath.InstallersDir()
	if err != nil {
		return "", err
	}
	return msixMgr.StartVerified(ctx, rawurl, filepath.Join(dir, filename), verify)
}

// InstallerPath returns where StartDownload and DownloadSync store the
// installer downloaded from rawurl.
func InstallerPath(rawurl string) (string, error) {
	dir, err := apppath.InstallersDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, deriveFilename(rawurl)), nil
}

// DownloadSync downloads an MSIXVC installer like StartDownload, but blocks
// until it is published, failed or ctx is cancelled, and reports progress
// only to o. A cancelled download keeps its partial file; calling DownloadSync
// again resumes it. It does not share tasks or events with the GUI downloads.
func DownloadSync(ctx context.Context, rawurl, md5sum string, o downloader.SyncOptions) (string, error) {
	dest, err := InstallerPath(rawurl)
	if err != nil {
		return "", err
	}
	o.MD5 = md5sum
	return dest, downloader.Download(ctx, stripFilenameParam(rawurl), dest, o)
}

// DownloadNamedSync is the blocking form of StartNamedDownload (UWP
// archives): the signed URL is used verbatim and verify checks the complete
// file before it is published.
func DownloadNamedSync(ctx context.Context, rawurl, filename string, verify func(string) error, o downloader.SyncOptions) (string, error) {
	if filename == "" || filepath.Base(filename) != filename || strings.ContainsAny(filename, `<>:"/\|?*`) {
		return "", fmt.Errorf("ERR_INVALID_NAME")
	}
	dir, err := apppath.InstallersDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, filename)
	o.Verify = verify
	return dest, downloader.Download(ctx, rawurl, dest, o)
}

func Pause() { msixMgr.Pause() }

func Resume() { msixMgr.Resume() }

func Cancel() { msixMgr.Cancel() }

func PauseTask(dest string) { msixMgr.PauseTask(dest) }

func ResumeTask(dest string) { msixMgr.ResumeTask(dest) }

func CancelTask(dest string) { msixMgr.CancelTask(dest) }

func run() {}

func deriveFilename(raw string) string {
	fname := "download.msixvc"
	if u, e := url.Parse(raw); e == nil {
		if v := u.Query().Get("filename"); v != "" {
			fname = ensureMsixvcFilename(v)
		} else {
			parts := strings.Split(u.Path, "/")
			if len(parts) > 0 && parts[len(parts)-1] != "" {
				fname = parts[len(parts)-1]
			}
		}
	}
	return fname
}

func stripFilenameParam(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Del("filename")
	u.RawQuery = q.Encode()
	return u.String()
}

func ensureMsixvcFilename(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "download.msixvc"
	}
	n = strings.ReplaceAll(n, "/", "_")
	n = strings.ReplaceAll(n, "\\", "_")
	lower := strings.ToLower(n)
	if !strings.HasSuffix(lower, ".msixvc") {
		n += ".msixvc"
	}
	return n
}
