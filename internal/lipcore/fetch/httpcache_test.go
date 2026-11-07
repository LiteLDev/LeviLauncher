package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// recorder is a report.Reporter keeping Info messages and progress.
type recorder struct {
	report.Nop
	mu       sync.Mutex
	infos    []string
	progress []report.Progress
}

func (r *recorder) Info(m string) {
	r.mu.Lock()
	r.infos = append(r.infos, m)
	r.mu.Unlock()
}

func (r *recorder) ReportProgress(p report.Progress) {
	r.mu.Lock()
	r.progress = append(r.progress, p)
	r.mu.Unlock()
}

func newTestHTTPCache(t *testing.T) (*HTTPCache, *recorder) {
	rec := &recorder{}
	return NewHTTPCache(filepath.Join(t.TempDir(), "Cache"), rec), rec
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// Ported from tests/Lip.Core.Tests/Services/CacheServiceTests.cs.

func TestHTTPKeyIsSha256OfURL(t *testing.T) {
	u := "https://example.com/a.zip"
	sum := sha256.Sum256([]byte(u))
	if got, want := HTTPKey(u), "http-"+hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("HTTPKey = %q, want %q", got, want)
	}
	// Known value, lowercase hex.
	if got := HTTPKey("https://example.com/a.zip"); got != "http-"+strings.ToLower(got[5:]) || len(got) != 5+64 {
		t.Fatalf("HTTPKey = %q", got)
	}
}

func TestHTTPCacheGetOrCreateWritesTmpFileInCache(t *testing.T) {
	c, rec := newTestHTTPCache(t)
	key := HTTPKey("https://example.com/a.zip")
	var written string
	p, err := c.GetOrCreate(key, func(tmp string) error {
		written = tmp
		return os.WriteFile(tmp, []byte("content"), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(written) != c.Root || !strings.HasSuffix(written, ".tmp") || len(filepath.Base(written)) != 32+4 {
		t.Errorf("factory wrote %q, want <Root>/<N>.tmp", written)
	}
	if p != filepath.Join(c.Root, key) {
		t.Errorf("path = %q", p)
	}
	if names := dirNames(t, c.Root); len(names) != 1 || names[0] != key {
		t.Errorf("cache holds %v, want only %s", names, key)
	}
	if b, _ := os.ReadFile(p); string(b) != "content" {
		t.Errorf("content = %q", b)
	}
	// A second call is a hit and does not call the factory.
	p2, err := c.GetOrCreate(key, func(string) error { t.Error("factory called on a hit"); return nil })
	if err != nil || p2 != p {
		t.Fatalf("hit = %q, %v", p2, err)
	}
	want := []string{"Caching file for key '" + key + "'...", "Cache hit for key '" + key + "'."}
	if fmt.Sprint(rec.infos) != fmt.Sprint(want) {
		t.Errorf("infos = %q, want %q", rec.infos, want)
	}
}

func TestHTTPCacheGetOrCreateEscapesKey(t *testing.T) {
	c, _ := newTestHTTPCache(t)
	p, err := c.GetOrCreate("git-a/b c", func(tmp string) error { return os.WriteFile(tmp, nil, 0o644) })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "git-a%2Fb%20c" {
		t.Errorf("entry = %q", filepath.Base(p))
	}
}

func TestHTTPCacheGetOrCreateCreatedConcurrentlyReturnsExistingFile(t *testing.T) {
	c, _ := newTestHTTPCache(t)
	p, err := c.GetOrCreate("test-key", func(tmp string) error {
		if err := os.WriteFile(tmp, []byte("second"), 0o644); err != nil {
			return err
		}
		// Another writer stores the same key before this one moves its file into place.
		_, err := c.GetOrCreate("test-key", func(other string) error {
			return os.WriteFile(other, []byte("first"), 0o644)
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "first" {
		t.Errorf("content = %q, want first", b)
	}
	if names := dirNames(t, c.Root); len(names) != 1 {
		t.Errorf("cache holds %v, want the entry only", names)
	}
}

func TestHTTPCacheGetOrCreateFailureLeavesNothing(t *testing.T) {
	c, _ := newTestHTTPCache(t)
	boom := errors.New("boom")
	_, err := c.GetOrCreate("k", func(tmp string) error {
		os.WriteFile(tmp, []byte("partial"), 0o644)
		return boom
	})
	if err != boom {
		t.Fatalf("err = %v, want the factory's error as is", err)
	}
	if names := dirNames(t, c.Root); len(names) != 0 {
		t.Errorf("cache holds %v after a failure", names)
	}
}

func TestHTTPCacheTryGet(t *testing.T) {
	c, rec := newTestHTTPCache(t)
	if _, ok := c.TryGet("git-abc"); ok {
		t.Fatal("TryGet found a missing entry")
	}
	if _, err := os.Stat(c.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("TryGet created the cache: %v", err)
	}
	os.MkdirAll(c.Root, 0o755)
	want := filepath.Join(c.Root, EscapeKey("git-abc"))
	os.WriteFile(want, []byte("content"), 0o644)
	if p, ok := c.TryGet("git-abc"); !ok || p != want {
		t.Fatalf("TryGet = %q, %v", p, ok)
	}
	if len(rec.infos) != 0 {
		t.Errorf("TryGet reported %q", rec.infos)
	}
}

func TestHTTPCacheCleanDeletesCacheDirectoryOnly(t *testing.T) {
	base := t.TempDir()
	c := NewHTTPCache(filepath.Join(base, "lip", "Cache"), nil)
	if err := c.Clean(); err != nil {
		t.Fatalf("Clean of a missing cache: %v", err)
	}
	os.MkdirAll(filepath.Join(c.Root, "sub"), 0o755)
	ro := filepath.Join(c.Root, "sub", "ro")
	os.WriteFile(ro, []byte("x"), 0o444)
	os.WriteFile(filepath.Join(c.Root, "somefile.txt"), []byte("cached data"), 0o644)
	sibling := filepath.Join(base, "lip", "temp")
	os.MkdirAll(sibling, 0o755)
	if err := c.Clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.Root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cache still exists: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("Clean touched %s: %v", sibling, err)
	}
}

func TestDefaultHTTPCacheRoot(t *testing.T) {
	c := DefaultHTTPCache(nil)
	if filepath.Base(c.Root) != "Cache" && filepath.Base(c.Root) != "lip" {
		t.Errorf("DefaultHTTPCache root = %q", c.Root)
	}
}

func TestHTTPCacheCreateTemp(t *testing.T) {
	c, _ := newTestHTTPCache(t)
	f, err := c.CreateTemp()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(f.Name()) != c.Root || !strings.HasSuffix(f.Name(), ".tmp") {
		t.Errorf("temp file %q, want <Root>/<N>.tmp", f.Name())
	}
	if _, err := f.Write([]byte("pack")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(f); string(b) != "pack" {
		t.Errorf("read back %q", b)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if names := dirNames(t, c.Root); len(names) != 0 {
		t.Errorf("temp file left after close: %v", names)
	}
}

func TestRewriteGithub(t *testing.T) {
	type tc struct{ proxy, in, want string }
	for _, c := range []tc{
		{"https://github.bibk.top", "https://github.com/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip", "https://github.bibk.top/LiteLDev/LeviLamina/releases/download/v1.0.0/x.zip"},
		{"https://mirror.example/gh/?a=1", "https://github.com/a/b?q=1", "https://mirror.example/gh/a/b?q=1"},
		{"https://mirror.example/gh/?a=1", "https://github.com/a/b", "https://mirror.example/gh/a/b?a=1"},
		{"https://github.bibk.top", "https://GITHUB.com/a/b", "https://github.bibk.top/a/b"},
		// 0.38 no longer carries the fragment over.
		{"https://github.bibk.top", "https://github.com/a/b#frag", "https://github.bibk.top/a/b"},
	} {
		if got, ok := RewriteGithub(c.proxy, c.in); !ok || got != c.want {
			t.Errorf("RewriteGithub(%q, %q) = %q, %v; want %q", c.proxy, c.in, got, ok, c.want)
		}
	}
	for _, c := range []tc{
		{"", "https://github.com/a/b", ""},
		{"https://p", "https://codeload.github.com/a/b", ""},
		{"https://p", "https://raw.githubusercontent.com/a/b", ""},
		{"https://github.com", "https://github.com/a/b", ""},
	} {
		if got, ok := RewriteGithub(c.proxy, c.in); ok || got != c.in {
			t.Errorf("RewriteGithub(%q, %q) = %q, %v; want no proxy", c.proxy, c.in, got, ok)
		}
	}
}

// tlsClient returns a client for srv that sends every request, whatever
// its host, to srv.
func tlsClient(srv *httptest.Server) *http.Client {
	addr := srv.Listener.Addr().String()
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
}

func TestDownloadFileStatusMessages(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var code int
		fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &code)
		w.WriteHeader(code)
	}))
	defer srv.Close()
	client := tlsClient(srv)
	dest := filepath.Join(t.TempDir(), "f")
	cases := []struct {
		url, want string
		code      int
	}{
		{"https://github.com/s/404", "Could not download https://github.com/s/404: the server answered HTTP 404 (Not Found). GitHub answers this way when the repository or the file does not exist, or when the repository is private.", 404},
		{"https://codeload.github.com/s/401", "Could not download https://codeload.github.com/s/401: the server answered HTTP 401 (Unauthorized). GitHub answers this way when the repository or the file does not exist, or when the repository is private.", 401},
		{"https://objects.githubusercontent.com/s/404", "Could not download https://objects.githubusercontent.com/s/404: the server answered HTTP 404 (Not Found). GitHub answers this way when the repository or the file does not exist, or when the repository is private.", 404},
		{"https://proxy.example/https://github.com/s/404", "Could not download https://proxy.example/https://github.com/s/404: the server answered HTTP 404 (Not Found). GitHub answers this way when the repository or the file does not exist, or when the repository is private.", 404},
		{"https://example.com/s/401", "Could not download https://example.com/s/401: the server answered HTTP 401 (Unauthorized). The server does not allow lip to download this file.", 401},
		{"https://example.com/s/403", "Could not download https://example.com/s/403: the server answered HTTP 403 (Forbidden).", 403},
		{"https://example.com/s/404", "Could not download https://example.com/s/404: the server answered HTTP 404 (Not Found). The file does not exist there. Check the URL, or the package name and version that lead to it.", 404},
		{"https://github.com/s/410", "Could not download https://github.com/s/410: the server answered HTTP 410 (Gone). The file does not exist there. Check the URL, or the package name and version that lead to it.", 410},
		{"https://example.com/s/429", "Could not download https://example.com/s/429: the server answered HTTP 429 (Too Many Requests). Too many requests were sent. Wait a while and try again.", 429},
		{"https://example.com/s/503", "Could not download https://example.com/s/503: the server answered HTTP 503 (Service Unavailable). The server had a problem. Try again later.", 503},
		{"https://example.com/s/418", "Could not download https://example.com/s/418: the server answered HTTP 418 (I'm a teapot).", 418},
	}
	for _, c := range cases {
		err := DownloadFile(context.Background(), client, c.url, dest, nil)
		var de *DownloadError
		if !errors.As(err, &de) || de.Error() != c.want || de.StatusCode != c.code {
			t.Errorf("%s: err = %v, want %q (status %d)", c.url, err, c.want, c.code)
		}
	}
}

func TestDownloadFileAny2xxRedirectsAndProgress(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 300<<10)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redir" {
			http.Redirect(w, r, "/file", http.StatusFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusNonAuthoritativeInfo) // any 2xx is a success
		w.Write(body)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f")
	os.WriteFile(dest, []byte("old content longer than nothing"), 0o644)
	rec := &recorder{}
	if err := DownloadFile(context.Background(), tlsClient(srv), "https://example.com/redir", dest, rec); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); !bytes.Equal(b, body) {
		t.Fatalf("downloaded %d bytes, want %d", len(b), len(body))
	}
	ps := rec.progress
	if len(ps) < 2 || ps[0].Current != 0 || !ps[len(ps)-1].Done || ps[len(ps)-1].Current != int64(len(body)) {
		t.Fatalf("progress = %+v", ps)
	}
	for i, p := range ps {
		if p.Stage != report.StageDownload || p.Unit != report.UnitBytes || p.Total != int64(len(body)) ||
			p.Message != "Downloading https://example.com/redir" || p.ID != ps[0].ID || p.Done != (i == len(ps)-1) {
			t.Errorf("update %d = %+v", i, p)
		}
	}
}

func TestDownloadFileBrokeOff(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("only part"))
	}))
	defer srv.Close()
	rec := &recorder{}
	err := DownloadFile(context.Background(), tlsClient(srv), "https://example.com/f", filepath.Join(t.TempDir(), "f"), rec)
	want := "The download of https://example.com/f broke off after 9 bytes of 1000. Check your network connection and try again."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if ps := rec.progress; len(ps) == 0 || !ps[len(ps)-1].Done {
		t.Errorf("no Done update: %+v", ps)
	}
}

func TestDownloadFileConnectionFailed(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	u := "https://" + addr + "/f"
	err := DownloadFile(context.Background(), nil, u, filepath.Join(t.TempDir(), "f"), nil)
	if want := "Could not download " + u + ": the connection to the server failed."; err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func makeZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("a.txt")
	w.Write([]byte("hello"))
	zw.Close()
	return buf.Bytes()
}

// validZip is a test validator: an *UnavailableError unless path is a ZIP
// archive.
func validZip(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		if _, statErr := os.Stat(path); statErr != nil {
			return statErr
		}
		return &UnavailableError{Message: "The file " + path + " is not a valid ZIP archive.", Err: err}
	}
	return zr.Close()
}

func TestDownloadZipProxyErrorPageFallsThrough(t *testing.T) {
	zipBody := makeZip(t)
	var proxied, direct atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "proxy.example":
			proxied.Add(1)
			w.Write([]byte("<html>proxy error</html>"))
		case "github.com":
			direct.Add(1)
			w.Write(zipBody)
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c, rec := newTestHTTPCache(t)
	const asset = "https://github.com/o/r/releases/download/v1/x.zip"
	p, err := c.DownloadZip(context.Background(), []string{asset + "#frag"}, "https://proxy.example", tlsClient(srv), validZip)
	if err != nil {
		t.Fatal(err)
	}
	if proxied.Load() != 1 || direct.Load() != 1 {
		t.Errorf("requests: proxy %d, direct %d", proxied.Load(), direct.Load())
	}
	if want := filepath.Join(c.Root, HTTPKey(asset)); p != want {
		t.Errorf("path = %q, want %q (keyed by the URL without fragment)", p, want)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, zipBody) {
		t.Error("cached content is not the direct download")
	}
	if names := dirNames(t, c.Root); len(names) != 1 {
		t.Errorf("cache holds %v", names)
	}
	// A valid cached entry is used without downloading or reporting.
	rec.infos = nil
	if p2, err := c.DownloadZip(context.Background(), []string{asset}, "https://proxy.example", tlsClient(srv), validZip); err != nil || p2 != p {
		t.Fatalf("second DownloadZip = %q, %v", p2, err)
	}
	if proxied.Load() != 1 || direct.Load() != 1 || len(rec.infos) != 0 {
		t.Errorf("cache hit downloaded (proxy %d, direct %d) or reported %q", proxied.Load(), direct.Load(), rec.infos)
	}
}

func TestDownloadZipDamagedCacheEntryIsDownloadedAgain(t *testing.T) {
	zipBody := makeZip(t)
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Write(zipBody)
	}))
	defer srv.Close()
	c, _ := newTestHTTPCache(t)
	const asset = "https://example.com/x.zip"
	os.MkdirAll(c.Root, 0o755)
	os.WriteFile(c.Path(HTTPKey(asset)), []byte("damaged"), 0o644)
	p, err := c.DownloadZip(context.Background(), []string{asset}, "", tlsClient(srv), validZip)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, zipBody) || n.Load() != 1 {
		t.Errorf("damaged entry not replaced (requests %d)", n.Load())
	}
}

func TestDownloadZipErrors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "proxy.example":
			w.WriteHeader(502)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	client := tlsClient(srv)
	c, _ := newTestHTTPCache(t)
	ctx := context.Background()

	if _, err := c.DownloadZip(ctx, nil, "", client, validZip); err == nil ||
		err.Error() != "The asset lists no URLs to download it from, so the package is broken. Report this to the package author." {
		t.Errorf("no URLs: %v", err)
	}

	// One URL, no proxy: the download's error under "Could not download <url>."
	_, err := c.DownloadZip(ctx, []string{"https://example.com/a.zip"}, "", client, validZip)
	var ue *UnavailableError
	var de *DownloadError
	if !errors.As(err, &ue) || ue.Error() != "Could not download https://example.com/a.zip." || !errors.As(err, &de) || de.StatusCode != 404 {
		t.Errorf("one URL: %v", err)
	}

	// Through the proxy and directly.
	_, err = c.DownloadZip(ctx, []string{"https://github.com/o/r/a.zip"}, "https://proxy.example", client, validZip)
	var agg *AggregateError
	if err == nil || err.Error() != "Could not download https://github.com/o/r/a.zip, neither through the GitHub proxy (https://proxy.example/o/r/a.zip) nor directly." ||
		!errors.As(err, &agg) || agg.Message != "Every download address failed." || len(agg.Errs) != 2 {
		t.Errorf("proxy and direct: %v", err)
	}

	// A non-https URL fails as unavailable and the next one is tried; both failing are aggregated.
	_, err = c.DownloadZip(ctx, []string{"http://example.com/a.zip", "https://example.com/b.zip"}, "", client, validZip)
	if err == nil || err.Error() != "Could not download the asset from any of its 2 URLs (http://example.com/a.zip, https://example.com/b.zip)." ||
		!errors.As(err, &agg) || agg.Message != "Every URL of the asset failed." || len(agg.Errs) != 2 ||
		agg.Errs[0].Error() != "The asset URL http://example.com/a.zip is not an https URL: lip downloads assets only over https." {
		t.Errorf("non-https: %v", err)
	}
	if names, _ := os.ReadDir(c.Root); len(names) != 0 {
		t.Errorf("failed downloads left %v", names)
	}

	// An invalid proxy is named as such.
	_, err = c.DownloadZip(ctx, []string{"https://github.com/o/r/a.zip"}, "not a url", client, validZip)
	if !errors.As(err, &agg) || len(agg.Errs) != 2 || !strings.HasPrefix(agg.Errs[0].Error(), "The GitHub proxy set in github_proxy does not give a valid URL (not a url/o/r/a.zip).") {
		t.Errorf("invalid proxy: %v", err)
	}
}

func TestDownloadZipNextURLAfterFailure(t *testing.T) {
	zipBody := makeZip(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "good.example" {
			w.Write(zipBody)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c, _ := newTestHTTPCache(t)
	p, err := c.DownloadZip(context.Background(), []string{"https://bad.example/a.zip", "https://good.example/a.zip"}, "", tlsClient(srv), validZip)
	if err != nil || p != c.Path(HTTPKey("https://good.example/a.zip")) {
		t.Fatalf("DownloadZip = %q, %v", p, err)
	}
}

func TestHTTPCacheForget(t *testing.T) {
	c, _ := newTestHTTPCache(t)
	const u = "https://example.com/a.zip"
	if err := c.Forget(u); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(c.Root, 0o755)
	os.WriteFile(c.Path(HTTPKey(u)), nil, 0o644)
	if err := c.Forget(u); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.TryGet(HTTPKey(u)); ok {
		t.Error("Forget left the entry")
	}
}

func TestTransferReporterThrottle(t *testing.T) {
	var got []transfer
	r := &transferReporter{fn: func(t transfer) { got = append(got, t) }, t: transfer{Total: 10000}}
	for i := int64(0); i <= 10000; i++ {
		r.update(i)
	}
	r.finish(nil)
	if len(got) != 1002 || got[0].Current != 0 || got[1000].Current != 10000 || !got[1001].Done || got[1001].Current != 10000 {
		t.Fatalf("%d updates: first %+v last %+v", len(got), got[0], got[len(got)-1])
	}
}
