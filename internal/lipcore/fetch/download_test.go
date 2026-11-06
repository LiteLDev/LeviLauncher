package fetch

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// githubTransport routes requests for github.com to the test server.
type githubTransport struct{ github *url.URL }

func (g githubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "github.com" {
		r = r.Clone(r.Context())
		r.URL.Scheme = g.github.Scheme
		r.URL.Host = g.github.Host
		r.Host = g.github.Host
	}
	return http.DefaultTransport.RoundTrip(r)
}

type hits struct {
	mu   sync.Mutex
	urls []string
}

func (h *hits) add(s string) { h.mu.Lock(); h.urls = append(h.urls, s); h.mu.Unlock() }

func serve(body []byte, status int, h *hits) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.add(r.URL.String())
		if status != http.StatusOK {
			http.Error(w, "nope", status)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		// Write in pieces so progress has intermediate values.
		for i := 0; i < len(body); i += 100 * 1024 {
			end := min(i+100*1024, len(body))
			w.Write(body[i:end])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
}

func checkProgress(t *testing.T, got []float64) {
	t.Helper()
	if len(got) < 2 || got[0] != 0 || got[len(got)-1] != 100 {
		t.Errorf("progress = %v, want 0 ... 100", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] || got[i] > 100 {
			t.Errorf("progress not monotonic in [0,100]: %v", got)
			break
		}
	}
}

func TestDownloaderFetch(t *testing.T) {
	body := bytes.Repeat([]byte("0123456789"), 100*1024)
	var h hits
	srv := serve(body, http.StatusOK, &h)
	defer srv.Close()

	d := &Downloader{HTTP: srv.Client(), Cache: testCache(t)}
	var prog []float64
	raw := srv.URL + "/a/b.zip"
	p, err := d.Fetch(context.Background(), raw, func(v float64) { prog = append(prog, v) })
	if err != nil {
		t.Fatal(err)
	}
	if p != d.Cache.Path(raw) {
		t.Errorf("path = %q", p)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, body) {
		t.Error("content mismatch")
	}
	checkProgress(t, prog)
	if len(prog) < 3 {
		t.Errorf("no intermediate progress: %v", prog)
	}

	// Second fetch is a cache hit.
	if _, err := d.Fetch(context.Background(), raw, nil); err != nil {
		t.Fatal(err)
	}
	if len(h.urls) != 1 {
		t.Errorf("server hit %d times", len(h.urls))
	}
}

func TestDownloaderNon200(t *testing.T) {
	var h hits
	srv := serve(nil, http.StatusNotFound, &h)
	defer srv.Close()
	d := &Downloader{HTTP: srv.Client(), Cache: testCache(t)}
	raw := srv.URL + "/missing.zip"
	_, err := d.Fetch(context.Background(), raw, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := d.Cache.Get(raw); ok {
		t.Error("failed download was cached")
	}
	if left, _ := os.ReadDir(d.Cache.TempRoot); len(left) != 0 {
		t.Errorf("temp dir not empty: %v", left)
	}
}

func TestDownloaderProxyFallback(t *testing.T) {
	body := []byte("release asset")
	var proxyHits, githubHits hits
	proxy := serve(nil, http.StatusBadGateway, &proxyHits)
	defer proxy.Close()
	github := serve(body, http.StatusOK, &githubHits)
	defer github.Close()
	gh, _ := url.Parse(github.URL)

	cache := testCache(t)
	d := &Downloader{
		HTTP:  &http.Client{Transport: githubTransport{gh}},
		Proxy: NewProxyRewriter(proxy.URL),
		Cache: cache,
	}
	var attempts []string
	d.ProgressFor = func(u string) ProgressFunc {
		attempts = append(attempts, u)
		return nil
	}
	raw := "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"
	p, err := d.Fetch(context.Background(), raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, body) {
		t.Errorf("content = %q", b)
	}
	if p != cache.Path(raw) {
		t.Errorf("cached under %q, want key of original URL", p)
	}
	wantAttempts := []string{proxy.URL + "/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", raw}
	if !equal(attempts, wantAttempts) {
		t.Errorf("attempts = %q, want %q", attempts, wantAttempts)
	}
	if len(proxyHits.urls) != 1 || proxyHits.urls[0] != "/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip" {
		t.Errorf("proxy hits = %q", proxyHits.urls)
	}
	if len(githubHits.urls) != 1 {
		t.Errorf("github hits = %q", githubHits.urls)
	}

	// Asset downloads do not remember proxy failures (lip SourceService):
	// another asset of the same repository tries the proxy again.
	attempts = nil
	if _, err := d.Fetch(context.Background(), "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.1/x.zip", nil); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Errorf("attempts = %q", attempts)
	}

	// Proxy success: direct URL not used.
	okProxy := serve(body, http.StatusOK, &proxyHits)
	defer okProxy.Close()
	d.Proxy = NewProxyRewriter(okProxy.URL)
	githubHits.urls = nil
	attempts = nil
	if _, err := d.Fetch(context.Background(), "https://github.com/a/b/releases/download/v1/y.zip", nil); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || len(githubHits.urls) != 0 {
		t.Errorf("attempts = %q, github hits = %q", attempts, githubHits.urls)
	}
}

func TestDownloaderAllFail(t *testing.T) {
	var h hits
	proxy := serve(nil, http.StatusInternalServerError, &h)
	defer proxy.Close()
	github := serve(nil, http.StatusForbidden, &h)
	defer github.Close()
	gh, _ := url.Parse(github.URL)
	d := &Downloader{HTTP: &http.Client{Transport: githubTransport{gh}}, Proxy: NewProxyRewriter(proxy.URL), Cache: testCache(t)}
	_, err := d.Fetch(context.Background(), "https://github.com/a/b/c.zip", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err = %v", err)
	}
}

func TestDownloaderConcurrentSameURL(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 512*1024)
	var h hits
	srv := serve(body, http.StatusOK, &h)
	defer srv.Close()
	d := &Downloader{HTTP: srv.Client(), Cache: testCache(t)}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = d.Fetch(context.Background(), srv.URL+"/same.zip", nil)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(h.urls) != 1 {
		t.Errorf("server hit %d times, want 1", len(h.urls))
	}
}

func TestDownloaderStalledProxyFallsBack(t *testing.T) {
	release := make(chan struct{})
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer stalled.Close()
	defer close(release)
	var gh hits
	github := serve([]byte("ok"), http.StatusOK, &gh)
	defer github.Close()
	ghURL, _ := url.Parse(github.URL)
	d := &Downloader{
		HTTP:        &http.Client{Transport: githubTransport{ghURL}},
		Proxy:       NewProxyRewriter(stalled.URL),
		Cache:       testCache(t),
		IdleTimeout: 200 * time.Millisecond,
	}
	p, err := d.Fetch(context.Background(), "https://github.com/a/b/releases/download/v1/z.zip", nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "ok" {
		t.Errorf("content = %q (partial proxy data must be discarded)", b)
	}
}

func TestProgressReporter(t *testing.T) {
	var got []float64
	p := &progressReporter{fn: func(v float64) { got = append(got, v) }, last: -1}
	for _, v := range []float64{0, 0, 0.05, 0.2, 0.1, 50, 50.01, 120, 100} {
		p.report(v)
	}
	want := []float64{0, 0.2, 50, 100}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
