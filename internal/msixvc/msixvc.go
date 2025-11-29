package msixvc

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/utils"
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
	},
	downloader.Options{Throttle: 250 * time.Millisecond, Resume: true, RemoveOnCancel: true},
)

type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Dest       string
}

func StartDownload(ctx context.Context, rawurl string) string {
	dir, err := utils.GetInstallerDir()
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		return ""
	}
	fname := deriveFilename(rawurl)
	dest := filepath.Join(dir, fname)
	return msixMgr.Start(ctx, stripFilenameParam(rawurl), dest)
}

func Pause() { msixMgr.Pause() }

func Resume() { msixMgr.Resume() }

func Cancel() { msixMgr.Cancel() }

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
