package downloader

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Events struct {
	Status          string
	Progress        string
	Done            string
	Error           string
	ProgressFactory func(DownloadProgress) any
	StatusFactory   func(status string, dest string) any
	DoneFactory     func(dest string) any
	ErrorFactory    func(err string, dest string) any
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
	mu        sync.Mutex
	tasks     map[string]*state
	events    Events
	opts      Options
	emitEvent func(name string, data ...any) bool
}

type state struct {
	ctx         context.Context
	url         string
	dest        string
	expectedMD5 string
	retryCount  int
	total       int64
	downloaded  int64
	paused      bool
	cancelled   bool
	running     bool
	cancelFn    context.CancelFunc
}

func NewManager(events Events, opts Options) *Manager {
	if opts.Throttle <= 0 {
		opts.Throttle = 250 * time.Millisecond
	}
	return &Manager{
		events: events,
		opts:   opts,
		tasks:  map[string]*state{},
		emitEvent: func(name string, data ...any) bool {
			return application.Get().Event.Emit(name, data...)
		},
	}
}

func (m *Manager) Start(ctx context.Context, src string, dest string, md5sum string) string {
	dir := filepath.Dir(dest)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	m.mu.Lock()
	if m.tasks == nil {
		m.tasks = map[string]*state{}
	}
	if existing, ok := m.tasks[dest]; ok && existing != nil && existing.running {
		m.mu.Unlock()
		m.emitStatus("started", dest)
		return dest
	}
	local := &state{
		ctx:         ctx,
		url:         src,
		dest:        dest,
		expectedMD5: md5sum,
		running:     true,
	}
	m.tasks[dest] = local
	m.mu.Unlock()
	go m.run(local)
	m.emitStatus("started", dest)
	return dest
}

func (m *Manager) Pause() {
	m.mu.Lock()
	for _, st := range m.tasks {
		if st == nil {
			continue
		}
		st.paused = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		m.emitStatus("paused", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) Resume() {
	m.mu.Lock()
	for _, st := range m.tasks {
		if st == nil || st.cancelled {
			continue
		}
		if st.running {
			continue
		}
		st.paused = false
		st.running = true
		go m.run(st)
		m.emitStatus("resumed", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) Cancel() {
	m.mu.Lock()
	for dest, st := range m.tasks {
		if st == nil {
			continue
		}
		st.cancelled = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		if !st.running {
			delete(m.tasks, dest)
		}
		m.emitStatus("cancelled", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) PauseTask(dest string) {
	m.mu.Lock()
	if st, ok := m.tasks[dest]; ok && st != nil {
		st.paused = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		m.emitStatus("paused", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) ResumeTask(dest string) {
	m.mu.Lock()
	if st, ok := m.tasks[dest]; ok && st != nil && !st.cancelled && !st.running {
		st.paused = false
		st.running = true
		go m.run(st)
		m.emitStatus("resumed", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) CancelTask(dest string) {
	m.mu.Lock()
	if st, ok := m.tasks[dest]; ok && st != nil {
		st.cancelled = true
		if st.cancelFn != nil {
			st.cancelFn()
		}
		if !st.running {
			delete(m.tasks, dest)
		}
		m.emitStatus("cancelled", st.dest)
	}
	m.mu.Unlock()
}

func (m *Manager) run(s *state) {
	m.mu.Lock()
	local := m.tasks[s.dest]
	if local != s {
		m.mu.Unlock()
		return
	}
	if local == nil || local.cancelled || local.paused {
		if local != nil {
			local.running = false
			if local.cancelled {
				delete(m.tasks, local.dest)
			}
		}
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	for {
		cancelled, paused, current := m.taskState(local)
		if !current || cancelled || paused {
			m.finishRunning(local)
			return
		}

		var cur int64
		downloadDest := local.dest + ".download"
		if m.opts.Resume {
			if fi, err := os.Stat(downloadDest); err == nil {
				cur = fi.Size()
			}
		}
		m.mu.Lock()
		local.downloaded = cur
		m.mu.Unlock()
		ctx, cancel := context.WithCancel(local.ctx)
		m.mu.Lock()
		if m.tasks[local.dest] != local || local.cancelled || local.paused {
			local.running = false
			if local.cancelled && m.tasks[local.dest] == local {
				delete(m.tasks, local.dest)
			}
			m.mu.Unlock()
			cancel()
			return
		}
		local.cancelFn = cancel
		m.mu.Unlock()

		req, err := http.NewRequestWithContext(ctx, "GET", local.url, nil)
		if err != nil {
			m.emitError(err.Error(), local.dest)
			m.finishRunning(local)
			return
		}
		httpx.ApplyDefaultHeaders(req)
		if m.opts.Resume && cur > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cur))
		}
		resp, err := httpx.Do(req)
		if err != nil {
			cancelled, paused, _ := m.taskState(local)
			if paused || cancelled {
				if cancelled && m.opts.RemoveOnCancel && downloadDest != "" {
					_ = os.Remove(downloadDest)
				}
				m.finishRunning(local)
				return
			}
			m.emitError(err.Error(), local.dest)
			m.finishRunning(local)
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			m.emitError(fmt.Sprintf("HTTP %s", resp.Status), local.dest)
			m.finishRunning(local)
			return
		}

		if m.opts.Resume && cur > 0 && resp.StatusCode == http.StatusOK {
			_ = os.Remove(downloadDest)
			cur = 0
			m.mu.Lock()
			local.downloaded = 0
			m.mu.Unlock()
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
		m.mu.Lock()
		local.total = total
		m.mu.Unlock()

		flags := os.O_CREATE | os.O_WRONLY
		if cur == 0 {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(downloadDest, flags, 0o644)
		if err != nil {
			resp.Body.Close()
			m.emitError(err.Error(), local.dest)
			m.finishRunning(local)
			return
		}
		if m.opts.Resume && cur > 0 {
			if _, err = f.Seek(cur, io.SeekStart); err != nil {
				_ = f.Close()
				resp.Body.Close()
				m.emitError(err.Error(), local.dest)
				m.finishRunning(local)
				return
			}
		}
		m.emitProgress(m.progress(local))
		buf := make([]byte, 128*1024)
		lastEmit := time.Now()

		var loopErr error
		for {
			cancelled, paused, current := m.taskState(local)
			if !current || cancelled || paused {
				_ = f.Close()
				resp.Body.Close()
				cancel()
				if cancelled && m.opts.RemoveOnCancel && downloadDest != "" {
					_ = os.Remove(downloadDest)
				}
				m.finishRunning(local)
				if cancelled {
					m.emitStatus("cancelled", local.dest)
				}
				return
			}
			n, er := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					loopErr = werr
					break
				}
				progress := m.addDownloaded(local, int64(n))
				if time.Since(lastEmit) >= m.opts.Throttle {
					m.emitProgress(progress)
					lastEmit = time.Now()
				}
			}
			if er != nil {
				if er == io.EOF {
					loopErr = nil
				} else {
					loopErr = er
				}
				break
			}
		}
		_ = f.Close()
		resp.Body.Close()
		contextWasCancelled := ctx.Err() == context.Canceled
		cancel()
		m.mu.Lock()
		local.cancelFn = nil
		m.mu.Unlock()

		if loopErr != nil {
			cancelled, paused, _ := m.taskState(local)
			if !paused && (contextWasCancelled || cancelled) {
				if m.opts.RemoveOnCancel && downloadDest != "" {
					_ = os.Remove(downloadDest)
				}
				m.emitStatus("cancelled", local.dest)
			} else if !paused {
				m.emitError(loopErr.Error(), local.dest)
			}
			m.finishRunning(local)
			return
		}

		// Download complete, verify MD5
		if local.expectedMD5 != "" {
			m.emitStatus("verifying", local.dest)
			if hash, err := calculateMD5(downloadDest); err == nil {
				if !strings.EqualFold(hash, local.expectedMD5) {
					// Mismatch
					_ = os.Remove(downloadDest)
					retryCount := m.incrementRetryCount(local)
					if retryCount < 3 {
						m.emitError(fmt.Sprintf("MD5 mismatch (attempt %d/3), retrying...", retryCount), local.dest)
						m.emitStatus("started", local.dest)
						time.Sleep(1 * time.Second)
						continue
					} else {
						m.emitError("ERR_MD5_MISMATCH", local.dest)
						m.finishRunning(local)
						return
					}
				}
			} else {
				// Failed to calculate MD5, maybe treat as error?
				m.emitError(fmt.Sprintf("MD5 calculation failed: %v", err), local.dest)
				m.finishRunning(local)
				return
			}
		}

		if err := os.Rename(downloadDest, local.dest); err != nil {
			m.emitError(err.Error(), local.dest)
		} else {
			m.emitProgress(m.progress(local))
			m.emitDone(local.dest)
		}
		m.finishRunning(local)
		return
	}
}

func calculateMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Manager) finishRunning(s *state) {
	m.mu.Lock()
	s.running = false
	cancel := s.cancelFn
	s.cancelFn = nil
	if s.cancelled || !s.paused {
		if m.tasks != nil {
			if cur, ok := m.tasks[s.dest]; ok && cur == s {
				delete(m.tasks, s.dest)
			}
		}
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) taskState(s *state) (cancelled bool, paused bool, current bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return s.cancelled, s.paused, m.tasks[s.dest] == s
}

func (m *Manager) progress(s *state) DownloadProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	return DownloadProgress{
		Downloaded: s.downloaded,
		Total:      s.total,
		Dest:       s.dest,
	}
}

func (m *Manager) addDownloaded(s *state, count int64) DownloadProgress {
	m.mu.Lock()
	s.downloaded += count
	progress := DownloadProgress{
		Downloaded: s.downloaded,
		Total:      s.total,
		Dest:       s.dest,
	}
	m.mu.Unlock()
	return progress
}

func (m *Manager) incrementRetryCount(s *state) int {
	m.mu.Lock()
	s.retryCount++
	retryCount := s.retryCount
	m.mu.Unlock()
	return retryCount
}

func (m *Manager) emitProgress(progress DownloadProgress) {
	if m.events.Progress == "" {
		return
	}
	payload := any(progress)
	if m.events.ProgressFactory != nil {
		payload = m.events.ProgressFactory(progress)
	}
	m.emit(m.events.Progress, payload)
}

func (m *Manager) emitStatus(status string, dest string) {
	if m.events.Status == "" {
		return
	}
	payload := any(status)
	if m.events.StatusFactory != nil {
		payload = m.events.StatusFactory(status, dest)
	}
	m.emit(m.events.Status, payload)
}

func (m *Manager) emitDone(dest string) {
	if m.events.Done == "" {
		return
	}
	payload := any(dest)
	if m.events.DoneFactory != nil {
		payload = m.events.DoneFactory(dest)
	}
	m.emit(m.events.Done, payload)
}

func (m *Manager) emitError(err string, dest string) {
	if m.events.Error == "" {
		return
	}
	payload := any(err)
	if m.events.ErrorFactory != nil {
		payload = m.events.ErrorFactory(err, dest)
	}
	m.emit(m.events.Error, payload)
}

func (m *Manager) emit(name string, data ...any) {
	if m.emitEvent != nil {
		m.emitEvent(name, data...)
	}
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
