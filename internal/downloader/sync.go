package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// SyncOptions configure Download.
type SyncOptions struct {
	// MD5 is the expected digest of the finished file; empty skips the check.
	MD5 string
	// Verify, when set, checks the complete staging file before it is
	// published under dest (as Manager.StartVerified).
	Verify func(path string) error
	// Progress receives byte progress, at most once per Throttle.
	Progress func(DownloadProgress)
	// Status receives status changes: started, verifying, cancelled.
	Status func(status string)
	// Throttle is the minimum interval between Progress calls (default 250ms).
	Throttle time.Duration
}

// ErrBusy reports that another process is downloading the same file.
var ErrBusy = errors.New("ERR_DOWNLOAD_BUSY: another download of this file is running")

// Download fetches src into dest and blocks until the file is published, the
// download fails or ctx is cancelled. It resumes an existing partial file
// with an HTTP Range request, and keeps the partial file when ctx is
// cancelled, so running the same download again continues where it stopped.
//
// It returns nil once dest holds the verified file, ctx.Err() after a
// cancellation, ErrBusy when another process holds dest's lock, and the
// download error otherwise. Events go only to the callbacks in o, never to
// the GUI.
func Download(ctx context.Context, src, dest string, o SyncOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	unlock, err := lockDownload(dest)
	if err != nil {
		return err
	}
	defer unlock()

	var (
		mu      sync.Mutex
		done    bool
		lastErr string
	)
	m := &Manager{
		events: Events{Status: "status", Progress: "progress", Done: "done", Error: "error"},
		opts:   Options{Throttle: o.Throttle, Resume: true, RemoveOnCancel: false},
		tasks:  map[string]*state{},
	}
	if m.opts.Throttle <= 0 {
		m.opts.Throttle = 250 * time.Millisecond
	}
	m.emitEvent = func(name string, data ...any) bool {
		var payload any
		if len(data) > 0 {
			payload = data[0]
		}
		switch name {
		case "progress":
			if p, ok := payload.(DownloadProgress); ok && o.Progress != nil {
				o.Progress(p)
			}
		case "status":
			if s, ok := payload.(string); ok && o.Status != nil {
				o.Status(s)
			}
		case "done":
			mu.Lock()
			done = true
			mu.Unlock()
		case "error":
			if s, ok := payload.(string); ok {
				mu.Lock()
				lastErr = s
				mu.Unlock()
			}
		}
		return true
	}

	local := &state{ctx: ctx, url: src, dest: dest, expectedMD5: o.MD5, verify: o.Verify, running: true}
	m.tasks[dest] = local
	if o.Status != nil {
		o.Status("started")
	}
	// run returns only once the transfer reached a final state; MD5 retries
	// happen inside it.
	m.run(local)

	mu.Lock()
	defer mu.Unlock()
	switch {
	case done:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case lastErr != "":
		return errors.New(lastErr)
	default:
		return fmt.Errorf("download of %s stopped without a result", filepath.Base(dest))
	}
}

// lockDownload takes an exclusive lock file next to dest so two processes do
// not write the same partial file. The lock disappears with the process.
func lockDownload(dest string) (func(), error) {
	p, err := windows.UTF16PtrFromString(dest + ".lock")
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return func() { _ = windows.CloseHandle(h) }, nil
}
