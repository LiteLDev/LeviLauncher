package msixvc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

const (
	EventDownloadStatus     = "msixvc_download_status"
	EventDownloadProgress   = "msixvc_download_progress"
	EventDownloadDone       = "msixvc_download_done"
	EventDownloadError      = "msixvc_download_error"
	EventAppxInstallLoading = "appx_install_loading"
)

var (
	mu sync.Mutex
	st *state
)

type state struct {
	ctx        context.Context
	url        string
	dest       string
	total      int64
	downloaded int64
	paused     bool
	cancelled  bool
	running    bool
	cancelFn   context.CancelFunc
}

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
	mu.Lock()
	if st != nil && st.running {
		if st.cancelFn != nil {
			st.cancelFn()
		}
	}
	st = &state{ctx: ctx, url: rawurl, dest: dest}
	mu.Unlock()
	go run()
	application.Get().Event.Emit(EventDownloadStatus, "started")
	return dest
}

func Pause() {
	mu.Lock()
	if st != nil {
		st.paused = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		application.Get().Event.Emit(EventDownloadStatus, "paused")
	}
	mu.Unlock()
}

func Resume() {
	mu.Lock()
	if st != nil {
		st.paused = false
		go run()
		application.Get().Event.Emit(EventDownloadStatus, "resumed")
	}
	mu.Unlock()
}

func Cancel() {
	mu.Lock()
	if st != nil {
		st.cancelled = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		application.Get().Event.Emit(EventDownloadStatus, "cancelled")
	}
	mu.Unlock()
}

func run() {
	mu.Lock()
	local := st
	mu.Unlock()
	if local == nil || local.cancelled || local.paused {
		return
	}
	var cur int64
	if fi, err := os.Stat(local.dest); err == nil {
		cur = fi.Size()
	}
	local.downloaded = cur
	ctx, cancel := context.WithCancel(local.ctx)
	mu.Lock()
	local.cancelFn = cancel
	local.running = true
	st = local
	mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", stripFilenameParam(local.url), nil)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		finishRunning(local)
		return
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	if cur > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cur))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		finishRunning(local)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		application.Get().Event.Emit(EventDownloadError, fmt.Sprintf("HTTP %s", resp.Status))
		finishRunning(local)
		return
	}

	if cur > 0 && resp.StatusCode == http.StatusOK {
		_ = os.Remove(local.dest)
		cur = 0
		local.downloaded = 0
	}

	total := resp.ContentLength
	if total > 0 && cur > 0 {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if idx := strings.LastIndex(cr, "/"); idx != -1 {
				if all := cr[idx+1:]; all != "*" {
					if v, e := parseInt64(all); e == nil {
						total = v
					}
				}
			}
		} else {
			total = cur + total
		}
	}
	local.total = total

	flags := os.O_CREATE | os.O_WRONLY
	if cur == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(local.dest, flags, 0o644)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		finishRunning(local)
		return
	}
	if cur > 0 {
		if _, err = f.Seek(cur, io.SeekStart); err != nil {
			_ = f.Close()
			application.Get().Event.Emit(EventDownloadError, err.Error())
			finishRunning(local)
			return
		}
	}
	buf := make([]byte, 128*1024)
	lastEmit := timeNow()
	for {
		if local.cancelled || local.paused {
			_ = f.Close()
			if local.cancelled && local.dest != "" {
				_ = os.Remove(local.dest)
			}
			finishRunning(local)
			if local.cancelled {
				application.Get().Event.Emit(EventDownloadStatus, "cancelled")
			}
			return
		}
		n, er := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				application.Get().Event.Emit(EventDownloadError, werr.Error())
				finishRunning(local)
				return
			}
			local.downloaded += int64(n)
			if since(lastEmit) >= 250 {
				application.Get().Event.Emit(EventDownloadProgress, DownloadProgress{
					local.downloaded,
					local.total,
					local.dest,
				})
				lastEmit = timeNow()
			}
		}
		if er != nil {
			if er == io.EOF {
				_ = f.Close()
				application.Get().Event.Emit(EventDownloadProgress, DownloadProgress{
					local.downloaded,
					local.total,
					local.dest,
				})
				application.Get().Event.Emit(EventDownloadDone, local.dest)
			} else {
				if ctx.Err() == context.Canceled || local.cancelled {
					_ = f.Close()
					if local.dest != "" {
						_ = os.Remove(local.dest)
					}
					application.Get().Event.Emit(EventDownloadStatus, "cancelled")
				} else {
					_ = f.Close()
					application.Get().Event.Emit(EventDownloadError, er.Error())
				}
			}
			finishRunning(local)
			return
		}
	}
}

func finishRunning(s *state) {
	mu.Lock()
	s.running = false
	if st == s {
		st = nil
	}
	mu.Unlock()
}

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

var timeNow = func() int64 { return timeNowMS() }
var since = func(last int64) int64 { return timeNowMS() - last }

func timeNowMS() int64 { return time.Now().UnixNano() / int64(time.Millisecond) }

func parseInt64(s string) (int64, error) {
	var v int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not int")
		}
		v = v*10 + int64(c-'0')
	}
	return v, nil
}
