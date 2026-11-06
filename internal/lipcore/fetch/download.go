package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ProgressFunc receives download progress in [0, 100].
type ProgressFunc func(percentage float64)

// Downloader downloads asset URLs into the cache, keyed by the original
// (unproxied) URL. The GET must return 200.
type Downloader struct {
	HTTP  *http.Client
	Proxy *ProxyRewriter
	Cache *Cache
	// ProgressFor, if set, is called once per download attempt with the URL
	// actually requested (lip reports progress per attempt as
	// "Downloading <downloadUrl>"); the returned func receives that
	// attempt's progress instead of Fetch's progress argument.
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
	if ctx == nil {
		ctx = context.Background()
	}
	cache := d.Cache
	if cache == nil {
		cache = DefaultCache()
	}
	return cache.getOrCreate(rawURL, func(f *os.File) error {
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
			p := progress
			if d.ProgressFor != nil {
				p = d.ProgressFor(u)
			}
			err := d.download(ctx, u, f, p)
			if err == nil {
				return nil
			}
			errs = append(errs, fmt.Errorf("failed to download from %s: %w", u, err))
		}
		return fmt.Errorf("failed to download file source from all URLs for %s: %w", rawURL, errors.Join(errs...))
	})
}

// download is lip's FileDownloader.DownloadFile: GET url, require HTTP 200
// and stream the body to w, reporting progress.
func (d *Downloader) download(parent context.Context, url string, w io.Writer, progress ProgressFunc) (err error) {
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

	client := d.HTTP
	if client == nil {
		client = http.DefaultClient
	}
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

	pr := &progressReporter{fn: progress, last: -1}
	pr.report(0)
	total := resp.ContentLength
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
			if total > 0 {
				pr.report(float64(done) * 100 / float64(total))
			}
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
	pr.report(100)
	return nil
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
