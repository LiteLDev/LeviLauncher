package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// progressRec records log lines like recReporter plus structured progress.
type progressRec struct {
	recReporter
	mu  sync.Mutex
	ups []report.Progress
}

func (r *progressRec) ReportProgress(p report.Progress) {
	r.mu.Lock()
	r.ups = append(r.ups, p)
	r.mu.Unlock()
}

// bars groups updates by bar id in first-update order, checking that every
// bar has a monotonic Current, a stable Stage, Unit and Package, a Total
// that only goes from unknown to known, and exactly one Done update, which
// is its last.
func (r *progressRec) bars(t *testing.T) [][]report.Progress {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	index := map[string]int{}
	var out [][]report.Progress
	for _, p := range r.ups {
		i, ok := index[p.ID]
		if !ok {
			i = len(out)
			index[p.ID] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], p)
	}
	for _, bar := range out {
		first := bar[0]
		for j, p := range bar {
			if p.Stage != first.Stage || p.Unit != first.Unit || p.Package != first.Package {
				t.Errorf("bar %s changed: %+v -> %+v", p.ID, first, p)
			}
			if j > 0 {
				prev := bar[j-1]
				if p.Current < prev.Current {
					t.Errorf("bar %s went back: %d -> %d", p.ID, prev.Current, p.Current)
				}
				if p.Total != prev.Total && prev.Total > 0 {
					t.Errorf("bar %s changed its total: %d -> %d", p.ID, prev.Total, p.Total)
				}
			}
			if p.Total > 0 && p.Current > p.Total {
				t.Errorf("bar %s: %d out of %d", p.ID, p.Current, p.Total)
			}
			if p.Done != (j == len(bar)-1) {
				t.Errorf("bar %s: update %d/%d has Done=%v", p.ID, j+1, len(bar), p.Done)
			}
		}
	}
	return out
}

func barsOf(bars [][]report.Progress, stage report.Stage) [][]report.Progress {
	var out [][]report.Progress
	for _, b := range bars {
		if b[0].Stage == stage {
			out = append(out, b)
		}
	}
	return out
}

func last(bar []report.Progress) report.Progress { return bar[len(bar)-1] }

// sizedSource is a memSource that knows its entry sizes, except unknown.
type sizedSource struct {
	*memSource
	unknown string
}

func (s sizedSource) Size(key string) (int64, bool) {
	v, ok := s.data[key]
	if !ok || key == s.unknown {
		return 0, false
	}
	return int64(len(v)), true
}

var _ Sizer = sizedSource{}

func placeVariant(assets ...manifest.AssetType) *manifest.Variant {
	v := &manifest.Variant{}
	for _, a := range assets {
		v.Assets = append(v.Assets, manifest.Asset{Type: a, Placements: []manifest.Placement{{Type: manifest.PlacementDir, Src: "", Dest: string(a)}}})
	}
	return v
}

func openerOf(srcs map[manifest.AssetType]Source) SourceOpener {
	return func(_ context.Context, a manifest.Asset) (Source, error) { return srcs[a.Type], nil }
}

func TestPlaceProgressCountsFiles(t *testing.T) {
	rec := &progressRec{}
	w := &Workspace{Dir: t.TempDir(), Reporter: rec, Glob: fakeCompile}
	open := openerOf(map[manifest.AssetType]Source{
		manifest.AssetSelf: newMemSource("a.txt", "A", "d/b.txt", "BB"),
		// Sizes are known for this asset only, so files are counted.
		manifest.AssetZip: sizedSource{memSource: newMemSource("c.txt", "CCC")},
	})
	ctx := WithPackage(context.Background(), "example.com/p@1.0.0")
	if _, err := w.InstallPackage(ctx, placeVariant(manifest.AssetSelf, manifest.AssetZip), open); err != nil {
		t.Fatal(err)
	}
	bars := rec.bars(t)
	if len(bars) != 1 || bars[0][0].Stage != report.StagePlace {
		t.Fatalf("bars %+v", bars)
	}
	bar := bars[0]
	if p := last(bar); p.Unit != report.UnitFiles || p.Total != 3 || p.Current != 3 || p.Package != "example.com/p@1.0.0" {
		t.Errorf("place bar ends with %+v", p)
	}
	if !strings.HasPrefix(bar[0].ID, "place:example.com/p@1.0.0:") {
		t.Errorf("id %q", bar[0].ID)
	}
	var items []string
	for _, p := range bar[1:4] {
		items = append(items, p.Item)
	}
	if want := []string{sep("self/a.txt"), sep("self/d/b.txt"), sep("zip/c.txt")}; strings.Join(items, "|") != strings.Join(want, "|") {
		t.Errorf("items %q, want %q", items, want)
	}
}

func TestPlaceProgressCountsBytes(t *testing.T) {
	rec := &progressRec{}
	w := &Workspace{Dir: t.TempDir(), Reporter: rec, Glob: fakeCompile}
	big := strings.Repeat("x", 1<<20)
	open := openerOf(map[manifest.AssetType]Source{
		manifest.AssetSelf: sizedSource{memSource: newMemSource("a.txt", "A", "big.bin", big)},
		manifest.AssetZip:  sizedSource{memSource: newMemSource("c.txt", "CCC")},
	})
	files, err := w.InstallPackage(context.Background(), placeVariant(manifest.AssetSelf, manifest.AssetZip), open)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, files[1]); got != big {
		t.Fatalf("big.bin has %d bytes", len(got))
	}
	bars := rec.bars(t)
	if len(bars) != 1 {
		t.Fatalf("bars = %d", len(bars))
	}
	bar := bars[0]
	total := int64(1 + len(big) + 3)
	if p := last(bar); p.Unit != report.UnitBytes || p.Total != total || p.Current != total {
		t.Errorf("place bar ends with %+v", p)
	}
	// Byte updates are throttled to 64 KiB steps: at most 16 inside big.bin,
	// plus one before and one after each file, the first and the Done.
	if n := len(bar); n > 16+2*3+2 {
		t.Errorf("%d updates for %d bytes", n, total)
	}
}

func TestPlaceProgressDoneOnError(t *testing.T) {
	rec := &progressRec{}
	w := &Workspace{Dir: t.TempDir(), Reporter: rec, Glob: fakeCompile}
	src := newMemSource("a.txt", "A", "b.txt", "B", "c.txt", "C")
	delete(src.data, "b.txt")
	open := openerOf(map[manifest.AssetType]Source{manifest.AssetSelf: src})
	if _, err := w.InstallPackage(context.Background(), placeVariant(manifest.AssetSelf), open); err == nil {
		t.Fatal("want error")
	}
	if p := last(rec.bars(t)[0]); !p.Done || p.Current != 1 || p.Total != 3 {
		t.Errorf("failed place bar ends with %+v", p)
	}

	// Cancelled between files.
	rec = &progressRec{}
	w.Reporter = rec
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelling := &cancelSource{memSource: newMemSource("a.txt", "A", "b.txt", "B"), cancel: cancel}
	open = openerOf(map[manifest.AssetType]Source{manifest.AssetSelf: cancelling})
	if _, err := w.InstallPackage(ctx, placeVariant(manifest.AssetSelf), open); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if p := last(rec.bars(t)[0]); !p.Done || p.Current != 1 || p.Total != 2 {
		t.Errorf("cancelled place bar ends with %+v", p)
	}
}

// cancelSource cancels the context when its first entry is opened.
type cancelSource struct {
	*memSource
	cancel func()
}

func (s *cancelSource) Open(key string) (io.ReadCloser, error) {
	s.cancel()
	return s.memSource.Open(key)
}

func TestScriptsProgress(t *testing.T) {
	rec := &progressRec{}
	scripts := &scriptLog{}
	w := &Workspace{Dir: t.TempDir(), Reporter: rec, Glob: fakeCompile, RunCommand: scripts.run}
	v := &manifest.Variant{}
	v.Scripts.PreInstall = []string{"p1", "p2"}
	v.Scripts.PostInstall = []string{"q1"}
	if _, err := w.InstallPackage(context.Background(), v, openerOf(nil)); err != nil {
		t.Fatal(err)
	}
	bars := barsOf(rec.bars(t), report.StageScripts)
	if len(bars) != 2 {
		t.Fatalf("script bars = %d, want one per hook with commands", len(bars))
	}
	if p := last(bars[0]); p.Message != "Running pre_install scripts" || p.Unit != report.UnitCommands || p.Total != 2 || p.Current != 2 {
		t.Errorf("pre_install bar ends with %+v", p)
	}
	if bars[0][1].Item != "p1" || bars[0][1].Current != 0 || bars[0][2].Item != "p2" || bars[0][2].Current != 1 {
		t.Errorf("pre_install updates %+v", bars[0])
	}
	if p := last(bars[1]); p.Message != "Running post_install scripts" || p.Total != 1 || p.Current != 1 {
		t.Errorf("post_install bar ends with %+v", p)
	}

	// A failing command ends the bar where it failed.
	rec.ups = nil
	scripts.fail = "p2"
	if _, err := w.InstallPackage(context.Background(), v, openerOf(nil)); err == nil {
		t.Fatal("want error")
	}
	bars = barsOf(rec.bars(t), report.StageScripts)
	if len(bars) != 1 {
		t.Fatalf("script bars = %d", len(bars))
	}
	if p := last(bars[0]); !p.Done || p.Total != 2 || p.Current != 1 {
		t.Errorf("failed script bar ends with %+v", p)
	}

	// No bar when scripts are ignored.
	rec.ups = nil
	w.IgnoreScripts = true
	if _, err := w.InstallPackage(context.Background(), v, openerOf(nil)); err != nil {
		t.Fatal(err)
	}
	if len(rec.ups) != 0 {
		t.Errorf("updates with IgnoreScripts: %+v", rec.ups)
	}
}

func TestUninstallProgress(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.txt", "keep.cfg", "logs/x.log", "logs/y.log", "z.log"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	rec := &progressRec{}
	w := &Workspace{Dir: dir, Reporter: rec, Glob: fakeCompile}
	v := &manifest.Variant{PreserveFiles: []string{"keep.cfg"}, RemoveFiles: []string{"logs/*.log"}}
	files := []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "keep.cfg"), filepath.Join(dir, "missing.txt")}
	ctx := WithPackage(context.Background(), "example.com/p@1.0.0")
	if err := w.UninstallPackage(ctx, v, files); err != nil {
		t.Fatal(err)
	}
	bars := barsOf(rec.bars(t), report.StageRemove)
	if len(bars) != 2 {
		t.Fatalf("remove bars = %d", len(bars))
	}
	// Removed, preserved and missing files all count as processed.
	recorded := bars[0]
	if p := last(recorded); p.Unit != report.UnitFiles || p.Total != 3 || p.Current != 3 || p.Package != "example.com/p@1.0.0" {
		t.Errorf("recorded files bar ends with %+v", p)
	}
	if recorded[1].Item != "a.txt" || recorded[2].Item != "keep.cfg" {
		t.Errorf("recorded files items %+v", recorded)
	}
	// The remove_files bar is indeterminate while walking, then counts the
	// matches.
	glob := bars[1]
	if glob[0].Total != 0 || glob[0].Message != "Removing files matching 'logs/*.log'" {
		t.Errorf("remove_files bar starts with %+v", glob[0])
	}
	if p := last(glob); p.Total != 2 || p.Current != 2 {
		t.Errorf("remove_files bar ends with %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "z.log")); err != nil {
		t.Errorf("unmatched file removed: %v", err)
	}
	// Log lines are unchanged.
	want := []string{"warning: File '" + filepath.Join(dir, "missing.txt") + "' does not exist, skipping removal."}
	if strings.Join(rec.lines, "|") != strings.Join(want, "|") {
		t.Errorf("lines %q, want %q", rec.lines, want)
	}
}

func TestUninstallProgressDoneOnCancel(t *testing.T) {
	dir := t.TempDir()
	rec := &progressRec{}
	w := &Workspace{Dir: dir, Reporter: rec, Glob: fakeCompile}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := &manifest.Variant{RemoveFiles: []string{"*.log"}}
	err := w.UninstallPackage(ctx, v, []string{filepath.Join(dir, "a.txt")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	bars := rec.bars(t)
	if len(bars) != 1 {
		t.Fatalf("bars = %d", len(bars))
	}
	if p := last(bars[0]); !p.Done || p.Total != 1 || p.Current != 0 {
		t.Errorf("cancelled remove bar ends with %+v", p)
	}

	// Cancelled during the remove_files walk.
	rec.ups = nil
	err = w.UninstallPackage(ctx, v, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	bars = rec.bars(t)
	if len(bars) != 1 {
		t.Fatalf("bars = %d", len(bars))
	}
	if p := last(bars[0]); !p.Done || p.Total != 0 || p.Current != 0 {
		t.Errorf("cancelled remove_files bar ends with %+v", p)
	}
}
