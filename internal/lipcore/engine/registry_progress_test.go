package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// progRec records structured progress (and the log lines, like recReporter).
type progRec struct {
	recReporter
	pmu sync.Mutex
	ps  []report.Progress
}

func (r *progRec) ReportProgress(p report.Progress) {
	r.pmu.Lock()
	r.ps = append(r.ps, p)
	r.pmu.Unlock()
}

// bars groups the updates by id, in order of first appearance.
func (r *progRec) bars() (ids []string, byID map[string][]report.Progress) {
	r.pmu.Lock()
	defer r.pmu.Unlock()
	byID = map[string][]report.Progress{}
	for _, p := range r.ps {
		if _, ok := byID[p.ID]; !ok {
			ids = append(ids, p.ID)
		}
		byID[p.ID] = append(byID[p.ID], p)
	}
	return ids, byID
}

// checkBars asserts the rules every bar follows: Current never decreases,
// and exactly one update, the last, is Done.
func checkBars(t *testing.T, r *progRec) {
	t.Helper()
	ids, byID := r.bars()
	for _, id := range ids {
		ups := byID[id]
		for i, p := range ups {
			if i > 0 && p.Current < ups[i-1].Current {
				t.Errorf("bar %s: Current went from %d to %d", id, ups[i-1].Current, p.Current)
			}
			if p.Done != (i == len(ups)-1) {
				t.Errorf("bar %s: update %d of %d has Done=%v", id, i+1, len(ups), p.Done)
			}
			if p.Total > 0 && p.Current > p.Total {
				t.Errorf("bar %s: Current %d exceeds Total %d", id, p.Current, p.Total)
			}
		}
	}
}

// barsOf returns the bars of stage, in order.
func recBarsOf(r *progRec, stage report.Stage) [][]report.Progress {
	ids, byID := r.bars()
	var out [][]report.Progress
	for _, id := range ids {
		if byID[id][0].Stage == stage {
			out = append(out, byID[id])
		}
	}
	return out
}

// unitBars returns the bars of stage counting unit, in order.
func unitBars(r *progRec, stage report.Stage, unit report.Unit) [][]report.Progress {
	var out [][]report.Progress
	for _, b := range recBarsOf(r, stage) {
		if b[0].Unit == unit {
			out = append(out, b)
		}
	}
	return out
}


// newProgressRegistry is newTestRegistry reporting to a progRec, with
// sequential progress ids.
func newProgressRegistry(t *testing.T) (*GitRegistry, *fakeGit, *fakeRepo, *progRec) {
	g, git, repo, _, _ := newTestRegistry(t)
	rec := &progRec{}
	g.Reporter = rec
	var mu sync.Mutex
	n := 0
	g.NewProgressID = func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("id-%d", n)
	}
	return g, git, repo, rec
}

func TestListTreeProgress(t *testing.T) {
	g, _, _, rec := newProgressRegistry(t)
	if _, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf}); err != nil {
		t.Fatal(err)
	}
	// Memoized: the second listing fetches nothing and reports nothing.
	if _, err := g.Manifest(ctx, specA("1.0.0")); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	// The tree listing, then the tooth.json blob and its request.
	if n := len(recBarsOf(rec, report.StageGitFetch)); n != 3 {
		t.Fatalf("%d git_fetch bars: %+v", n, rec.ps)
	}
	tree := unitBars(rec, report.StageGitFetch, report.UnitObjects)[0]
	if tree[0].Total != 0 || tree[0].Current != 0 {
		t.Errorf("first update %+v, want indeterminate", tree[0])
	}
	if l := last(tree); l.Unit != report.UnitObjects || l.Total != 1 || l.Current != 1 ||
		l.Package != "example.com/a@1.0.0" || l.Message != "Listing files of example.com/a@1.0.0" || l.Item != repoA {
		t.Errorf("last tree update %+v", l)
	}
	if !rec.has("info: Listing files of example.com/a@1.0.0 (commit c10000000000)...") {
		t.Errorf("Info line lost: %q", rec.msgs)
	}
}

// Each request of a fetch with proxy fallback is its own bar.
func TestListTreeProgressPerAttempt(t *testing.T) {
	g, _, repo, rec := newProgressRegistry(t)
	repo.proxy, repo.proxyNoTrees = true, true
	if _, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf}); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	trees := recBarsOf(rec, report.StageGitFetch)
	if len(trees) != 2 || trees[0][0].ID == trees[1][0].ID {
		t.Fatalf("bars %+v", trees)
	}
	for _, b := range trees {
		if l := last(b); !l.Done || l.Current != 1 || l.Total != 1 {
			t.Errorf("attempt bar ended %+v", l)
		}
	}
}

func TestFetchBlobsProgress(t *testing.T) {
	g, _, repo, rec := newProgressRegistry(t)
	src, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Prefetch(ctx, src.Keys()); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	bars := unitBars(rec, report.StageGitFetch, report.UnitBlobs)
	if len(bars) != 1 {
		t.Fatalf("bars %+v", bars)
	}
	blobs := bars[0]
	if f := blobs[0]; f.Unit != report.UnitBlobs || f.Total != 2 || f.Current != 0 || f.Message != "Fetching files of example.com/a@1.0.0" {
		t.Errorf("first blob update %+v", f)
	}
	if l := last(blobs); l.Current != 2 || l.Total != 2 {
		t.Errorf("last blob update %+v", l)
	}

	// Across batches of 256 wants: one bar, Total known before fetching.
	rec.ps = nil
	var oids []string
	for i := range 300 {
		data := fmt.Sprintf("blob %d", i)
		repo.blobs[blobOID(data)] = data
		oids = append(oids, blobOID(data))
	}
	if err := g.fetchBlobs(ctx, repoA, append(oids, oids[0]), specA("1.0.0")); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	bars = unitBars(rec, report.StageGitFetch, report.UnitBlobs)
	if len(bars) != 1 || bars[0][0].Total != 300 || last(bars[0]).Current != 300 {
		t.Fatalf("bars %+v", bars)
	}
	// Each request (256 wants, then 44) is its own objects bar.
	reqs := unitBars(rec, report.StageGitFetch, report.UnitObjects)
	if len(reqs) != 2 || last(reqs[0]).Total != 256 || last(reqs[1]).Total != 44 ||
		reqs[0][0].Message != "Fetching files of example.com/a@1.0.0" {
		t.Fatalf("request bars %+v", reqs)
	}
}

// The blob bar counts stored blobs only: entries received by a request
// that then fails are not counted, so a later batch moves the bar from
// the blobs actually stored.
func TestFetchBlobsProgressDiscardedPack(t *testing.T) {
	g, _, repo, rec := newProgressRegistry(t)
	var oids []string
	for i := range 266 {
		data := fmt.Sprintf("blob %d", i)
		repo.blobs[blobOID(data)] = data
		oids = append(oids, blobOID(data))
	}
	repo.cut = func(wanted []string) int {
		if len(wanted) == maxWantsPerRequest {
			return 250
		}
		return -1
	}
	if err := g.fetchBlobs(ctx, repoA, oids, specA("1.0.0")); err == nil {
		t.Fatal("expected the first batch's error")
	}
	checkBars(t, rec)
	bars := unitBars(rec, report.StageGitFetch, report.UnitBlobs)
	if len(bars) != 1 {
		t.Fatalf("bars %+v", bars)
	}
	b := bars[0]
	if b[0].Total != 266 || len(b) != 12 {
		t.Fatalf("blob bar %+v", b)
	}
	for i, p := range b[1:11] {
		if p.Current != int64(i+1) {
			t.Fatalf("update %d: Current %d, want %d", i+1, p.Current, i+1)
		}
	}
	if l := last(b); !l.Done || l.Current != 10 {
		t.Errorf("blob bar ended %+v", l)
	}
	// The failed request's own bar ended with the entries it received.
	reqs := unitBars(rec, report.StageGitFetch, report.UnitObjects)
	if len(reqs) != 2 || last(reqs[0]).Current != 250 || last(reqs[0]).Total != 256 || last(reqs[1]).Current != 10 {
		t.Fatalf("request bars %+v", reqs)
	}
}

// A blob bar ends with Done when the fetch fails, and the proxy fallback
// does not make it go backwards.
func TestFetchBlobsProgressErrors(t *testing.T) {
	g, _, repo, rec := newProgressRegistry(t)
	commit := "c100000000000000000000000000000000000000"
	tooth, dll := repo.trees[commit][0].OID, repo.trees[commit][1].OID
	repo.proxy = true
	repo.proxyWithold = map[string]bool{dll: true}
	if err := g.fetchBlobs(ctx, repoA, []string{tooth, dll}, specA("1.0.0")); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	if bars := unitBars(rec, report.StageGitFetch, report.UnitBlobs); len(bars) != 1 || last(bars[0]).Current != 2 {
		t.Fatalf("bars %+v", bars)
	}

	rec.ps = nil
	data := "never sent"
	repo.blobs[blobOID(data)] = data
	repo.withold = map[string]bool{blobOID(data): true}
	if err := g.fetchBlobs(ctx, repoA, []string{blobOID(data)}, specA("1.0.0")); err == nil {
		t.Fatal("expected an error")
	}
	checkBars(t, rec)
	if bars := unitBars(rec, report.StageGitFetch, report.UnitBlobs); len(bars) != 1 || !last(bars[0]).Done || last(bars[0]).Current != 0 {
		t.Fatalf("bars %+v", bars)
	}
}

func TestOpenURLDownloadProgress(t *testing.T) {
	g, _, _, rec := newProgressRegistry(t)
	g.OpenArchive = func(string, string) (fetch.AssetSource, error) { return fakeAsset{closed: new(bool)}, nil }
	const raw, proxied = "https://github.com/o/r/releases/download/v1/a.zip", "https://proxy.example/o/r/releases/download/v1/a.zip"
	g.Download = func(_ context.Context, u string, hooks fetch.FetchHooks) (string, error) {
		p := hooks.Attempt(proxied)
		p(fetch.Transfer{Total: 100})
		p(fetch.Transfer{Current: 40, Total: 100})
		p(fetch.Transfer{Current: 40, Total: 100, Done: true, Err: errors.New("reset")})
		d := hooks.Attempt(raw)
		d(fetch.Transfer{})
		d(fetch.Transfer{Current: 1 << 20})
		d(fetch.Transfer{Current: 2 << 20, Done: true})
		return "/cache/a.zip", nil
	}
	if _, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetZip, URLs: []string{raw}}); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	bars := recBarsOf(rec, report.StageDownload)
	if len(bars) != 2 || bars[0][0].ID == bars[1][0].ID {
		t.Fatalf("bars %+v", bars)
	}
	if l := last(bars[0]); l.Message != "Downloading "+proxied || l.Item != proxied || l.Current != 40 || l.Total != 100 {
		t.Errorf("proxy attempt ended %+v", l)
	}
	if l := last(bars[1]); l.Message != "Downloading "+raw || l.Total != 0 || l.Current != 2<<20 || l.Unit != report.UnitBytes || l.Package != "example.com/a@1.0.0" {
		t.Errorf("direct attempt ended %+v", l)
	}
}

// A legacy reporter (no ReportProgress) gets lip's percentages: every
// download opens at 0, a failed or abandoned bar never reports 100, and a
// successful download without Content-Length ends at 100.
func TestOpenURLLegacyProgress(t *testing.T) {
	g, _, _, _, rep := newTestRegistry(t)
	g.NewProgressID = func() string { return "id" }
	g.OpenArchive = func(string, string) (fetch.AssetSource, error) { return fakeAsset{closed: new(bool)}, nil }
	const raw, proxied = "https://github.com/o/r/releases/download/v1/a.zip", "https://proxy.example/o/r/releases/download/v1/a.zip"
	g.Download = func(_ context.Context, _ string, hooks fetch.FetchHooks) (string, error) {
		hooks.Waiting()
		p := hooks.Attempt(proxied)
		p(fetch.Transfer{})
		p(fetch.Transfer{Current: 1 << 20})
		p(fetch.Transfer{Current: 1 << 20, Done: true, Err: errors.New("reset")})
		d := hooks.Attempt(raw)
		d(fetch.Transfer{})
		d(fetch.Transfer{Current: 2 << 20})
		d(fetch.Transfer{Current: 2 << 20, Done: true})
		return "/cache/a.zip", nil
	}
	if _, err := g.openURL(ctx, specA("1.0.0"), "zip", raw); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"progress: Downloading " + proxied + " 0",
		"progress: Downloading " + raw + " 0",
		"progress: Downloading " + raw + " 100",
	}
	if fmt.Sprint(rep.msgs) != fmt.Sprint(want) {
		t.Errorf("lines: %q, want %q", rep.msgs, want)
	}
}

// A git request failing before the pack header (an indeterminate bar)
// does not report 100 to a legacy reporter.
func TestListTreeLegacyProgress(t *testing.T) {
	g, _, repo, _, rep := newTestRegistry(t)
	repo.cut = func([]string) int { return 0 }
	if _, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf}); err == nil {
		t.Fatal("expected an error")
	}
	for _, m := range rep.msgs {
		if strings.HasPrefix(m, "progress:") && strings.HasSuffix(m, " 100") {
			t.Errorf("unexpected %q", m)
		}
	}
}

func TestOpenURLCacheProgress(t *testing.T) {
	const raw = "https://example.com/a.zip"
	for name, tc := range map[string]struct {
		download func(hooks fetch.FetchHooks) error
		want     []string // "<message> <current>/<total> <done>" per bar
	}{
		"hit": {
			download: func(h fetch.FetchHooks) error { h.CacheHit(10); return nil },
			want:     []string{"Cache hit for key '" + raw + "'. 10/10 true"},
		},
		"wait then hit": {
			download: func(h fetch.FetchHooks) error { h.Waiting(); h.CacheHit(5); return nil },
			want:     []string{"Cache hit for key '" + raw + "'. 5/5 true"},
		},
		"wait then download": {
			download: func(h fetch.FetchHooks) error {
				h.Waiting()
				p := h.Attempt(raw)
				p(fetch.Transfer{Total: 3})
				p(fetch.Transfer{Current: 3, Total: 3, Done: true})
				return nil
			},
			want: []string{"Waiting for another download of " + raw + " 0/0 true", "Downloading " + raw + " 3/3 true"},
		},
		"wait then error": {
			download: func(h fetch.FetchHooks) error { h.Waiting(); return errors.New("boom") },
			want:     []string{"Waiting for another download of " + raw + " 0/0 true"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			g, _, _, rec := newProgressRegistry(t)
			g.OpenArchive = func(string, string) (fetch.AssetSource, error) { return fakeAsset{closed: new(bool)}, nil }
			g.Download = func(_ context.Context, _ string, hooks fetch.FetchHooks) (string, error) {
				return "/cache/a.zip", tc.download(hooks)
			}
			_, _ = g.openURL(ctx, specA("1.0.0"), "zip", raw)
			checkBars(t, rec)
			bars := recBarsOf(rec, report.StageDownload)
			var got []string
			for _, b := range bars {
				l := last(b)
				got = append(got, fmt.Sprintf("%s %d/%d %v", l.Message, l.Current, l.Total, l.Done))
				if l.Item != raw {
					t.Errorf("item %q", l.Item)
				}
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("bars:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestOpenURLExtractProgress(t *testing.T) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	payload := make([]byte, 300<<10)
	rand.Read(payload)
	tw.WriteHeader(&tar.Header{Name: "a.bin", Mode: 0o644, Size: int64(len(payload)), Typeflag: tar.TypeReg})
	tw.Write(payload)
	tw.Close()
	path := filepath.Join(t.TempDir(), "a.tgz")
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	zw.Write(tarBuf.Bytes())
	zw.Close()
	if err := os.WriteFile(path, gzBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	g, _, _, rec := newProgressRegistry(t)
	g.Download = func(context.Context, string, fetch.FetchHooks) (string, error) { return path, nil }
	src, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetTgz, URLs: []string{"https://example.com/a.tgz"}})
	if err != nil {
		t.Fatal(err)
	}
	defer src.(interface{ Close() error }).Close()
	checkBars(t, rec)
	bars := recBarsOf(rec, report.StageExtract)
	if len(bars) != 1 {
		t.Fatalf("bars %+v", rec.ps)
	}
	size := int64(gzBuf.Len())
	if f, l := bars[0][0], last(bars[0]); f.Current != 0 || l.Current != size || l.Total != size || len(bars[0]) < 4 {
		t.Errorf("updates %+v", bars[0])
	}

	// A corrupt tgz still ends its bar.
	os.WriteFile(path, gzBuf.Bytes()[:gzBuf.Len()/2], 0o644)
	g2, _, _, rec2 := newProgressRegistry(t)
	g2.Download = g.Download
	if _, err := g2.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetTgz, URLs: []string{"https://example.com/b.tgz"}}); err == nil {
		t.Fatal("expected an error")
	}
	checkBars(t, rec2)
	if bars := recBarsOf(rec2, report.StageExtract); len(bars) != 1 {
		t.Fatalf("bars %+v", rec2.ps)
	}
}

func TestCacheCleanProgress(t *testing.T) {
	rec := &progRec{}
	e := &Engine{Reporter: rec, CleanCache: func() error { t.Error("CleanCache called"); return nil }}
	e.CleanCacheProgress = func(_ context.Context, progress func(done, total int64)) error {
		for i := range int64(4) {
			progress(i, 3)
		}
		return nil
	}
	if err := e.CacheClean(ctx); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	bars := recBarsOf(rec, report.StageCacheClean)
	if len(bars) != 1 || len(bars[0]) != 5 || bars[0][0].ID != "cache_clean" || last(bars[0]).Current != 3 || last(bars[0]).Unit != report.UnitFiles {
		t.Fatalf("bars %+v", bars)
	}

	rec.ps = nil
	boom := errors.New("boom")
	e.CleanCacheProgress = func(_ context.Context, progress func(done, total int64)) error {
		progress(0, 3)
		progress(1, 3)
		return boom
	}
	if err := e.CacheClean(ctx); err != boom {
		t.Fatalf("err = %v", err)
	}
	checkBars(t, rec)
	if bars := recBarsOf(rec, report.StageCacheClean); len(bars) != 1 || last(bars[0]).Current != 1 {
		t.Fatalf("bars %+v", bars)
	}

	// With the real cache.
	rec.ps = nil
	cache := &fetch.Cache{Root: filepath.Join(t.TempDir(), "cache"), TempRoot: t.TempDir()}
	for _, k := range []string{"a", "b"} {
		if _, err := cache.Put(k, func(w io.Writer) error { _, err := w.Write([]byte(k)); return err }); err != nil {
			t.Fatal(err)
		}
	}
	e.CleanCacheProgress = cache.CleanProgress
	if err := e.CacheClean(ctx); err != nil {
		t.Fatal(err)
	}
	checkBars(t, rec)
	if bars := recBarsOf(rec, report.StageCacheClean); len(bars) != 1 || last(bars[0]).Total != 3 || last(bars[0]).Current != 3 {
		t.Fatalf("bars %+v", bars)
	}

	// New wires the progress variant.
	if e, err := New(t.TempDir(), "", nil); err != nil || e.CleanCacheProgress == nil {
		t.Fatalf("New: %v", err)
	}
}
