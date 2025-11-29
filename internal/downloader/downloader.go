package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Events struct {
	Status          string
	Progress        string
	Done            string
	Error           string
	ProgressFactory func(DownloadProgress) any
}

type Options struct {
	Throttle       time.Duration
	Resume         bool
	RemoveOnCancel bool
}

type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Dest       string
}

type Manager struct {
	mu     sync.Mutex
	st     *state
	events Events
	opts   Options
}

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

func NewManager(events Events, opts Options) *Manager {
	if opts.Throttle <= 0 {
		opts.Throttle = 250 * time.Millisecond
	}
	return &Manager{events: events, opts: opts}
}

func (m *Manager) Start(ctx context.Context, src string, dest string) string {
	dir := filepath.Dir(dest)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	m.mu.Lock()
	if m.st != nil && m.st.running {
		if m.st.cancelFn != nil {
			m.st.cancelFn()
		}
	}
	m.st = &state{ctx: ctx, url: src, dest: dest}
	local := m.st
	m.mu.Unlock()
	go m.run(local)
	application.Get().Event.Emit(m.events.Status, "started")
	return dest
}

func (m *Manager) Pause() {
	m.mu.Lock()
	if m.st != nil {
		m.st.paused = true
		if m.st.cancelFn != nil {
			m.st.cancelFn()
		}
		application.Get().Event.Emit(m.events.Status, "paused")
	}
	m.mu.Unlock()
}

func (m *Manager) Resume() {
	m.mu.Lock()
	local := m.st
	if local != nil {
		local.paused = false
		go m.run(local)
		application.Get().Event.Emit(m.events.Status, "resumed")
	}
	m.mu.Unlock()
}

func (m *Manager) Cancel() {
	m.mu.Lock()
	if m.st != nil {
		m.st.cancelled = true
		if m.st.cancelFn != nil {
			m.st.cancelFn()
		}
		application.Get().Event.Emit(m.events.Status, "cancelled")
	}
	m.mu.Unlock()
}

func (m *Manager) run(s *state) {
	m.mu.Lock()
	local := m.st
	m.mu.Unlock()
	if local != s {
		return
	}
	if local == nil || local.cancelled || local.paused {
		return
	}
	var cur int64
	if m.opts.Resume {
		if fi, err := os.Stat(local.dest); err == nil {
			cur = fi.Size()
		}
	}
	local.downloaded = cur
	ctx, cancel := context.WithCancel(local.ctx)
	m.mu.Lock()
	local.cancelFn = cancel
	local.running = true
	m.st = local
	m.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", local.url, nil)
	if err != nil {
		application.Get().Event.Emit(m.events.Error, err.Error())
		m.finishRunning(local)
		return
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	if m.opts.Resume && cur > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cur))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		application.Get().Event.Emit(m.events.Error, err.Error())
		m.finishRunning(local)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		application.Get().Event.Emit(m.events.Error, fmt.Sprintf("HTTP %s", resp.Status))
		m.finishRunning(local)
		return
	}

	if m.opts.Resume && cur > 0 && resp.StatusCode == http.StatusOK {
		_ = os.Remove(local.dest)
		cur = 0
		local.downloaded = 0
	}

	total := resp.ContentLength
	if m.opts.Resume && total > 0 && cur > 0 {
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
		application.Get().Event.Emit(m.events.Error, err.Error())
		m.finishRunning(local)
		return
	}
	if m.opts.Resume && cur > 0 {
		if _, err = f.Seek(cur, io.SeekStart); err != nil {
			_ = f.Close()
			application.Get().Event.Emit(m.events.Error, err.Error())
			m.finishRunning(local)
			return
		}
	}
	{
		payload := any(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
		if m.events.ProgressFactory != nil {
			payload = m.events.ProgressFactory(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
		}
		application.Get().Event.Emit(m.events.Progress, payload)
	}
	buf := make([]byte, 128*1024)
	lastEmit := time.Now()
	for {
		if local.cancelled || local.paused {
			_ = f.Close()
			if local.cancelled && m.opts.RemoveOnCancel && local.dest != "" {
				_ = os.Remove(local.dest)
			}
			m.finishRunning(local)
			if local.cancelled {
				application.Get().Event.Emit(m.events.Status, "cancelled")
			}
			return
		}
		n, er := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				application.Get().Event.Emit(m.events.Error, werr.Error())
				m.finishRunning(local)
				return
			}
			local.downloaded += int64(n)
			if time.Since(lastEmit) >= m.opts.Throttle {
				payload := any(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
				if m.events.ProgressFactory != nil {
					payload = m.events.ProgressFactory(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
				}
				application.Get().Event.Emit(m.events.Progress, payload)
				lastEmit = time.Now()
			}
		}
		if er != nil {
			if er == io.EOF {
				_ = f.Close()
				payload := any(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
				if m.events.ProgressFactory != nil {
					payload = m.events.ProgressFactory(DownloadProgress{Downloaded: local.downloaded, Total: local.total, Dest: local.dest})
				}
				application.Get().Event.Emit(m.events.Progress, payload)
				application.Get().Event.Emit(m.events.Done, local.dest)
			} else {
				if ctx.Err() == context.Canceled || local.cancelled {
					_ = f.Close()
					if m.opts.RemoveOnCancel && local.dest != "" {
						_ = os.Remove(local.dest)
					}
					application.Get().Event.Emit(m.events.Status, "cancelled")
				} else {
					_ = f.Close()
					application.Get().Event.Emit(m.events.Error, er.Error())
				}
			}
			m.finishRunning(local)
			return
		}
	}
}

func (m *Manager) finishRunning(s *state) {
	m.mu.Lock()
	s.running = false
	if m.st == s {
		m.st = nil
	}
	m.mu.Unlock()
}

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
