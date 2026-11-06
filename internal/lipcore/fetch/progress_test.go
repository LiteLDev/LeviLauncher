package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// transfers records the Transfer updates of each attempt.
type transfers struct {
	mu       sync.Mutex
	attempts []string
	updates  map[string][]Transfer
}

func (r *transfers) hook(u string) func(Transfer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updates == nil {
		r.updates = map[string][]Transfer{}
	}
	r.attempts = append(r.attempts, u)
	return func(t Transfer) {
		r.mu.Lock()
		r.updates[u] = append(r.updates[u], t)
		r.mu.Unlock()
	}
}

// checkTransfers asserts monotonic Current and exactly one Done, the last.
func checkTransfers(t *testing.T, ups []Transfer) {
	t.Helper()
	for i, u := range ups {
		if i > 0 && u.Current < ups[i-1].Current {
			t.Errorf("Current went back: %+v", ups)
		}
		if u.Done != (i == len(ups)-1) {
			t.Errorf("update %d of %d Done=%v: %+v", i+1, len(ups), u.Done, ups)
		}
	}
}

func TestFetchWithAttempts(t *testing.T) {
	body := bytes.Repeat([]byte("0123456789"), 100*1024)
	// The proxy answers 200 but drops the connection after part of the body.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024000")
		w.Write(body[:300<<10])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	defer proxy.Close()
	var gh hits
	github := serve(body, http.StatusOK, &gh)
	defer github.Close()
	ghURL, _ := url.Parse(github.URL)

	d := &Downloader{
		HTTP:  &http.Client{Transport: githubTransport{ghURL}},
		Proxy: NewProxyRewriter(proxy.URL),
		Cache: testCache(t),
	}
	raw := "https://github.com/o/r/releases/download/v1/x.zip"
	var rec transfers
	hits := 0
	p, err := d.FetchWith(context.Background(), raw, FetchHooks{Attempt: rec.hook, CacheHit: func(int64) { hits++ }})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{proxy.URL + "/o/r/releases/download/v1/x.zip", raw}
	if len(rec.attempts) != 2 || rec.attempts[0] != want[0] || rec.attempts[1] != want[1] || hits != 0 {
		t.Fatalf("attempts %q, hits %d", rec.attempts, hits)
	}
	failed := rec.updates[want[0]]
	checkTransfers(t, failed)
	if l := failed[len(failed)-1]; l.Err == nil || l.Total != int64(len(body)) || l.Current == 0 {
		t.Errorf("proxy attempt ended %+v", l)
	}
	ok := rec.updates[raw]
	checkTransfers(t, ok)
	if f, l := ok[0], ok[len(ok)-1]; f.Current != 0 || l.Err != nil || l.Current != int64(len(body)) || l.Total != int64(len(body)) {
		t.Errorf("direct attempt %+v ... %+v", f, l)
	}
	if len(ok) > 1003 {
		t.Errorf("%d updates, want at most one per 0.1%%", len(ok))
	}

	// A cache hit reports the cached file's size and requests nothing.
	rec = transfers{}
	var size int64 = -1
	p2, err := d.FetchWith(context.Background(), raw, FetchHooks{Attempt: rec.hook, CacheHit: func(n int64) { size = n }})
	if err != nil || p2 != p || size != int64(len(body)) || len(rec.attempts) != 0 {
		t.Fatalf("hit: %v %q size %d attempts %q", err, p2, size, rec.attempts)
	}
}

// Without a Content-Length, updates count bytes in 64 KiB steps.
func TestFetchWithUnknownLength(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < len(body); i += 32 << 10 {
			w.Write(body[i : i+32<<10])
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()
	d := &Downloader{HTTP: srv.Client(), Cache: testCache(t)}
	var rec transfers
	if _, err := d.FetchWith(context.Background(), srv.URL+"/a", FetchHooks{Attempt: rec.hook}); err != nil {
		t.Fatal(err)
	}
	ups := rec.updates[srv.URL+"/a"]
	checkTransfers(t, ups)
	for _, u := range ups {
		if u.Total != 0 {
			t.Fatalf("Total %d, want unknown", u.Total)
		}
	}
	if l := ups[len(ups)-1]; l.Current != int64(len(body)) || len(ups) < 3 || len(ups) > 1<<20/transferByteStep+2 {
		t.Errorf("%d updates, last %+v", len(ups), l)
	}

	// The legacy percentage callback still gets 0 and 100.
	var pcts []float64
	if _, err := (&Downloader{HTTP: srv.Client(), Cache: testCache(t)}).Fetch(context.Background(), srv.URL+"/b", func(v float64) { pcts = append(pcts, v) }); err != nil {
		t.Fatal(err)
	}
	if len(pcts) != 2 || pcts[0] != 0 || pcts[1] != 100 {
		t.Errorf("percentages %v", pcts)
	}
}

// A call that has to wait for another download of the same key says so
// before blocking, then gets the cache hit.
func TestFetchWithWaiting(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write([]byte("data"))
	}))
	defer srv.Close()
	d := &Downloader{HTTP: srv.Client(), Cache: testCache(t)}
	raw := srv.URL + "/a"

	first := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		_, err := d.FetchWith(context.Background(), raw, FetchHooks{Attempt: func(string) func(Transfer) {
			close(started)
			return nil
		}})
		first <- err
	}()
	<-started

	waited := make(chan struct{})
	var size int64 = -1
	second := make(chan error, 1)
	go func() {
		_, err := d.FetchWith(context.Background(), raw, FetchHooks{
			Waiting:  func() { close(waited) },
			CacheHit: func(n int64) { size = n },
			Attempt:  func(string) func(Transfer) { t.Error("second call downloaded"); return nil },
		})
		second <- err
	}()
	select {
	case <-waited:
	case <-time.After(10 * time.Second):
		t.Fatal("Waiting not called")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil || size != 4 {
		t.Fatalf("second: %v, size %d", err, size)
	}
}

func TestTransferReporterThrottle(t *testing.T) {
	var got []Transfer
	r := &transferReporter{fn: func(t Transfer) { got = append(got, t) }, t: Transfer{Total: 10000}}
	for i := int64(0); i <= 10000; i++ {
		r.update(i)
	}
	r.finish(nil)
	if len(got) != 1002 || got[0].Current != 0 || got[1000].Current != 10000 || !got[1001].Done || got[1001].Current != 10000 {
		t.Fatalf("%d updates: first %+v last %+v", len(got), got[0], got[len(got)-1])
	}
}

func TestOpenArchiveProgressTgz(t *testing.T) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	payload := make([]byte, 1<<20)
	rand.Read(payload)
	tw.WriteHeader(&tar.Header{Name: "a.bin", Mode: 0o644, Size: int64(len(payload)), Typeflag: tar.TypeReg})
	tw.Write(payload)
	tw.Close()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write(tarBuf.Bytes())
	zw.Close()
	path := filepath.Join(t.TempDir(), "a.tgz")
	if err := os.WriteFile(path, gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	type upd struct{ done, total int64 }
	var got []upd
	src, err := OpenArchiveProgress("tgz", path, func(done, total int64) { got = append(got, upd{done, total}) })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	size := int64(gz.Len())
	if len(got) < 3 || got[0] != (upd{0, size}) || got[len(got)-1] != (upd{size, size}) {
		t.Fatalf("updates %v (size %d)", got, size)
	}
	for i := 1; i < len(got); i++ {
		if got[i].done <= got[i-1].done || got[i].total != size {
			t.Fatalf("updates %v", got)
		}
	}
	if keys := src.Keys(); len(keys) != 1 || keys[0] != "a.bin" {
		t.Errorf("keys %q", keys)
	}

	// A zip is indexed without decompressing: no updates.
	got = nil
	zipPath := filepath.Join(t.TempDir(), "a.zip")
	writeTestZip(t, zipPath)
	zsrc, err := OpenArchiveProgress("zip", zipPath, func(done, total int64) { got = append(got, upd{done, total}) })
	if err != nil {
		t.Fatal(err)
	}
	zsrc.Close()
	if len(got) != 0 {
		t.Errorf("zip updates %v", got)
	}

	// A truncated tgz fails after reporting what it read.
	got = nil
	if err := os.WriteFile(path, gz.Bytes()[:size/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenArchiveProgress("tgz", path, func(done, total int64) { got = append(got, upd{done, total}) }); err == nil {
		t.Fatal("expected an error")
	}
	if len(got) < 2 || got[len(got)-1].done != size/2 {
		t.Errorf("updates %v", got)
	}
}

func writeTestZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, _ := zw.Create("a.txt")
	io.WriteString(w, "a")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanProgress(t *testing.T) {
	c := testCache(t)
	var got [][2]int64
	record := func(done, total int64) { got = append(got, [2]int64{done, total}) }
	if err := c.CleanProgress(context.Background(), record); err != nil || len(got) != 0 {
		t.Fatalf("missing dir: %v %v", err, got)
	}
	for _, k := range []string{"a", "b"} {
		if _, err := c.Put(k, func(w io.Writer) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(c.Root, "d", "e")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "ro"), []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	// root, a, b, d, d/e, d/e/ro
	const total = 6

	// Canceled: nothing is deleted after the cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	got = nil
	err := c.CleanProgress(ctx, func(done, total int64) {
		record(done, total)
		if done == 2 {
			cancel()
		}
	})
	if err != context.Canceled || len(got) != 3 || got[2] != [2]int64{2, total} {
		t.Fatalf("canceled: %v %v", err, got)
	}
	if _, err := os.Stat(c.Root); err != nil {
		t.Fatalf("root removed by a canceled clean: %v", err)
	}

	// The remaining entries are counted again.
	got = nil
	if err := c.CleanProgress(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if len(got) != total-2+1 || got[0] != [2]int64{0, total - 2} || got[len(got)-1] != [2]int64{total - 2, total - 2} {
		t.Fatalf("updates %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i][0] != got[i-1][0]+1 {
			t.Fatalf("updates %v", got)
		}
	}
	if _, err := os.Stat(c.Root); !os.IsNotExist(err) {
		t.Errorf("cache dir still exists: %v", err)
	}
}
