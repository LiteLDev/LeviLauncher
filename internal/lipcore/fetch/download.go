package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
)

// ProgressFunc receives download progress in [0, 100].
type ProgressFunc func(percentage float64)

// Transfer is measured progress of one download attempt: bytes received out
// of the response's Content-Length.
type Transfer struct {
	// Current is the number of bytes received so far.
	Current int64
	// Total is the Content-Length, or 0 when the server sent none.
	Total int64
	// Done marks the attempt's last update; Err is its error, if it failed.
	Done bool
	Err  error
}

// FetchHooks receives the progress of one Downloader.FetchWith call. Hooks
// are per call (unlike Cache fields, shared by every task using the
// process-wide cache); every field may be nil.
type FetchHooks struct {
	// Waiting is called before the call blocks because another download of
	// the same key in this process is under way (it then usually ends with
	// a cache hit).
	Waiting func()
	// CacheHit is called with the cached file's size when the URL is served
	// from the cache without downloading.
	CacheHit func(size int64)
	// Attempt is called once per URL requested, with that URL (the proxied
	// URL first when the proxy applies, then the original one, as lip's
	// SourceService does). The returned func, if not nil, receives the
	// attempt's progress: once the server answered HTTP 200 an update with
	// Current 0, then throttled updates (at most one per 0.1% of the total,
	// or per 64 KiB without a total) and exactly one Done update, whether
	// the attempt succeeded or failed. An attempt that fails before the
	// HTTP 200 sends nothing.
	Attempt func(downloadURL string) func(Transfer)
}

// Downloader downloads asset URLs into the cache, keyed by the original
// (unproxied) URL. The GET must return 200.
//
// Deprecated: removed with the 0.37 engine; use HTTPCache.DownloadZip.
type Downloader struct {
	// HTTP is the client to use; nil means httpclient.Default. Its redirect
	// policy defaults to httpclient.CheckRedirect.
	HTTP  *http.Client
	Proxy *ProxyRewriter
	Cache *Cache
	// ProgressFor, if set, is called once per download attempt with the URL
	// actually requested (lip reports progress per attempt as
	// "Downloading <downloadUrl>"); the returned func receives that
	// attempt's progress instead of Fetch's progress argument. FetchWith
	// takes per-call hooks instead.
	ProgressFor func(downloadURL string) ProgressFunc
	// IdleTimeout aborts an attempt (moving on to the next URL) when no
	// response or body data arrives for this long. Zero means 100 seconds,
	// Flurl's default request timeout.
	IdleTimeout time.Duration
}

// errIdle cancels a download attempt that stalled.
var errIdle = errors.New("download stalled")

// Fetch returns the cached file for rawURL, downloading it first if needed
// (lip SourceService.Get(url, isArchive)). The proxied URL, when the proxy
// applies, is tried first and then rawURL; the first successful download is
// cached. progress may be nil; for each attempt it receives values from 0
// to 100, non-decreasing, ending at 100 on success.
func (d *Downloader) Fetch(ctx context.Context, rawURL string, progress ProgressFunc) (string, error) {
	return d.FetchWith(ctx, rawURL, FetchHooks{
		Attempt: func(u string) func(Transfer) {
			p := progress
			if d.ProgressFor != nil {
				p = d.ProgressFor(u)
			}
			if p == nil {
				return nil
			}
			return percentages(p)
		},
	})
}

// FetchWith is Fetch reporting measured progress to hooks.
func (d *Downloader) FetchWith(ctx context.Context, rawURL string, hooks FetchHooks) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cache := d.Cache
	if cache == nil {
		cache = DefaultCache()
	}
	return cache.getOrCreateHooks(rawURL, hooks.Waiting, hooks.CacheHit, func(f *os.File) error {
		var errs []error
		for _, u := range d.Proxy.downloadURLs(rawURL) {
			if err := ctx.Err(); err != nil {
				errs = append(errs, err)
				break
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			if err := f.Truncate(0); err != nil {
				return err
			}
			var report func(Transfer)
			if hooks.Attempt != nil {
				report = hooks.Attempt(u)
			}
			err := d.download(ctx, u, f, report)
			if err == nil {
				return nil
			}
			errs = append(errs, fmt.Errorf("failed to download from %s: %w", u, err))
		}
		return fmt.Errorf("failed to download file source from all URLs for %s: %w", rawURL, errors.Join(errs...))
	})
}

// percentages adapts a ProgressFunc to Transfer updates: 0 when the
// attempt starts, the percentage of Content-Length when it is known, and
// 100 on success.
func percentages(fn ProgressFunc) func(Transfer) {
	pr := &progressReporter{fn: fn, last: -1}
	return func(t Transfer) {
		switch {
		case t.Done && t.Err == nil:
			pr.report(100)
		case t.Done:
		case t.Total > 0:
			pr.report(float64(t.Current) * 100 / float64(t.Total))
		case t.Current == 0:
			pr.report(0)
		}
	}
}

// download is lip's FileDownloader.DownloadFile: GET url, require HTTP 200
// and stream the body to w, reporting progress.
func (d *Downloader) download(parent context.Context, url string, w io.Writer, report func(Transfer)) (err error) {
	idle := d.IdleTimeout
	if idle <= 0 {
		idle = 100 * time.Second
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	timer := time.AfterFunc(idle, func() { cancel(errIdle) })
	defer timer.Stop()
	defer func() {
		if err != nil && errors.Is(context.Cause(ctx), errIdle) {
			err = fmt.Errorf("no data received from %s for %s: %w", url, idle, errIdle)
		}
	}()

	// Like lip's Flurl status check, never follow an https -> http redirect.
	client := httpclient.WithLipRedirects(d.HTTP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: HTTP %d", url, resp.StatusCode)
	}

	total := resp.ContentLength
	tr := &transferReporter{fn: report, t: Transfer{Total: max(total, 0)}}
	tr.update(0)
	defer func() { tr.finish(err) }()
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		timer.Reset(idle)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
			done += int64(n)
			tr.update(done)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if total >= 0 && done != total {
		return fmt.Errorf("failed to download %s: got %d of %d bytes", url, done, total)
	}
	return nil
}

// transferReporter throttles the Transfer updates of one attempt: the first
// update, then at most one per 0.1% of the total (or per 64 KiB without a
// total), and the Done update.
type transferReporter struct {
	fn   func(Transfer)
	t    Transfer
	last int64 // Current of the last update sent
	sent bool
}

// transferByteStep is the byte step of updates without a total.
const transferByteStep = 64 << 10

func (r *transferReporter) update(current int64) {
	r.t.Current = current
	if r.fn == nil {
		return
	}
	if r.sent {
		if r.t.Total > 0 && current*1000/r.t.Total <= r.last*1000/r.t.Total {
			return
		}
		if r.t.Total <= 0 && current/transferByteStep <= r.last/transferByteStep {
			return
		}
	}
	r.sent = true
	r.last = current
	r.fn(r.t)
}

func (r *transferReporter) finish(err error) {
	if r.fn == nil {
		return
	}
	r.t.Done, r.t.Err = true, err
	r.fn(r.t)
}

// progressReporter forwards non-decreasing progress values, at most one per
// 0.1 percentage point, always forwarding 0 and 100.
type progressReporter struct {
	fn   ProgressFunc
	last float64
}

func (p *progressReporter) report(v float64) {
	if p.fn == nil {
		return
	}
	if v > 100 {
		v = 100
	}
	if v < p.last || (v-p.last < 0.1 && v != 100 && p.last >= 0) || v == p.last {
		return
	}
	p.last = v
	p.fn(v)
}
