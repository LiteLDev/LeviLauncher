package catalog

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultProbeTimeout is the per-mirror timeout of the window's mirror test.
const DefaultProbeTimeout = 7 * time.Second

// MirrorResult is the outcome of probing one download mirror.
type MirrorResult struct {
	URL       string
	Host      string
	LatencyMs int64 // elapsed time, measured on failure too
	OK        bool  // HEAD answered 2xx or 3xx
	Measured  bool  // false when the probe did not run (cancelled)
}

// MirrorHost is the host name of a mirror URL, or the URL itself.
func MirrorHost(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return raw
}

// BestMirror picks the mirror to download from: the lowest latency among
// mirrors that answered, else the lowest among all measured ones. ok is
// false when nothing was measured.
func BestMirror(results []MirrorResult) (MirrorResult, bool) {
	var measured, good []MirrorResult
	for _, r := range results {
		if !r.Measured {
			continue
		}
		measured = append(measured, r)
		if r.OK {
			good = append(good, r)
		}
	}
	list := good
	if len(list) == 0 {
		list = measured
	}
	if len(list) == 0 {
		return MirrorResult{}, false
	}
	best := list[0]
	for _, r := range list[1:] {
		if r.LatencyMs < best.LatencyMs {
			best = r
		}
	}
	return best, true
}

// RankMirrors orders results best first: answering mirrors by latency, then
// failed ones by latency, then unmeasured ones.
func RankMirrors(results []MirrorResult) []MirrorResult {
	out := append([]MirrorResult(nil), results...)
	rank := func(r MirrorResult) int {
		switch {
		case r.Measured && r.OK:
			return 0
		case r.Measured:
			return 1
		}
		return 2
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := rank(out[i]), rank(out[j]); a != b {
			return a < b
		}
		return out[i].LatencyMs < out[j].LatencyMs
	})
	return out
}

// ProbeMirrors sends a HEAD request to every URL, concurrently, each with
// its own timeout (DefaultProbeTimeout when timeout <= 0), and returns the
// results in the order of urls. prepare, when set, adjusts each request
// (default headers). A cancelled ctx leaves the remaining results
// unmeasured.
func ProbeMirrors(ctx context.Context, client *http.Client, urls []string, timeout time.Duration, prepare func(*http.Request)) []MirrorResult {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	if client == nil {
		client = http.DefaultClient
	}
	out := make([]MirrorResult, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		out[i] = MirrorResult{URL: u, Host: MirrorHost(u)}
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			rctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			start := time.Now()
			ok := false
			req, err := http.NewRequestWithContext(rctx, http.MethodHead, strings.TrimSpace(u), nil)
			if err == nil {
				if prepare != nil {
					prepare(req)
				}
				if resp, err := client.Do(req); err == nil {
					_ = resp.Body.Close()
					ok = resp.StatusCode >= 200 && resp.StatusCode < 400
				}
			}
			if ctx.Err() != nil {
				return
			}
			out[i].LatencyMs = time.Since(start).Milliseconds()
			out[i].OK = ok
			out[i].Measured = true
		}(i, u)
	}
	wg.Wait()
	return out
}
