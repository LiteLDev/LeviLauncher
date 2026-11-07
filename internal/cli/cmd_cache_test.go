package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cacheFixture struct {
	lip, downloads, imports, installers string
	cleaned                             int
}

func newCacheFixture(t *testing.T) *cacheFixture {
	t.Helper()
	root := t.TempDir()
	f := &cacheFixture{
		lip: filepath.Join(root, "lip"), downloads: filepath.Join(root, "Downloads"),
		imports: filepath.Join(root, "Imports"), installers: filepath.Join(root, "Installers"),
	}
	saved := []any{cacheLipDir, cacheLipClean, cacheDownloadDirs, cachePartialDirs}
	t.Cleanup(func() {
		cacheLipDir = saved[0].(func() string)
		cacheLipClean = saved[1].(func(context.Context) error)
		cacheDownloadDirs = saved[2].(func() []string)
		cachePartialDirs = saved[3].(func() []string)
	})
	cacheLipDir = func() string { return f.lip }
	cacheLipClean = func(context.Context) error {
		f.cleaned++
		return os.RemoveAll(f.lip)
	}
	cacheDownloadDirs = func() []string { return []string{f.downloads, f.imports} }
	cachePartialDirs = func() []string { return []string{f.downloads, f.installers} }
	return f
}

func writeSized(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestCacheClean(t *testing.T) {
	h := newHarness(t)
	f := newCacheFixture(t)
	writeSized(t, filepath.Join(f.lip, "a", "pkg.zip"), 100)
	writeSized(t, filepath.Join(f.downloads, "map.mcworld"), 10)
	writeSized(t, filepath.Join(f.imports, "pack.mcpack"), 5)
	writeSized(t, filepath.Join(f.downloads, "big.zip.partial.tmp"), 7)
	writeSized(t, filepath.Join(f.installers, "Release 1.21.msixvc.partial.tmp"), 1000)
	// A download that is running holds "<final>.lock": its partial is kept.
	writeSized(t, filepath.Join(f.installers, "Preview 1.22.msixvc.partial.tmp"), 3)
	writeSized(t, filepath.Join(f.installers, "Preview 1.22.msixvc.lock"), 0)

	code, out, errOut := h.run("cache", "clean", "--dry-run", "--json")
	var res []cacheResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || len(res) != 3 {
		t.Fatalf("dry run: %d %q %q", code, out, errOut)
	}
	want := map[string][3]int64{"packages": {1, 100, 0}, "downloads": {2, 15, 0}, "partial": {2, 1007, 1}}
	for _, r := range res {
		w := want[r.Kind]
		if int64(r.Items) != w[0] || r.Bytes != w[1] || int64(r.Skipped) != w[2] || !r.DryRun {
			t.Errorf("%s: %+v, want %v", r.Kind, r, w)
		}
	}
	if f.cleaned != 0 || !exists(filepath.Join(f.downloads, "map.mcworld")) {
		t.Fatal("--dry-run deleted something")
	}

	if code, _, errOut := h.run("cache", "clean"); code != ExitUsage || !strings.Contains(errOut, "ERR_CONFIRMATION_REQUIRED") {
		t.Fatalf("no --yes without a console: %d %q", code, errOut)
	}

	code, out, errOut = h.run("cache", "clean", "partial", "downloads", "-y")
	if code != 0 || !strings.Contains(out, "Freed 1022 B") {
		t.Fatalf("clean: %d %q %q", code, out, errOut)
	}
	if f.cleaned != 0 {
		t.Fatal("packages cleaned although not asked")
	}
	if exists(filepath.Join(f.downloads, "map.mcworld")) || exists(filepath.Join(f.installers, "Release 1.21.msixvc.partial.tmp")) {
		t.Fatal("files not deleted")
	}
	if !exists(filepath.Join(f.installers, "Preview 1.22.msixvc.partial.tmp")) {
		t.Fatal("the partial of a running download was deleted")
	}

	if code, _, _ := h.run("cache", "clean", "packages", "--yes"); code != 0 || f.cleaned != 1 {
		t.Fatalf("packages: %d cleaned=%d", code, f.cleaned)
	}
	// Nothing left: no confirmation needed, exit 0.
	if code, out, _ := h.run("cache", "clean", "packages", "downloads"); code != 0 || !strings.Contains(out, "Nothing to clean") {
		t.Fatalf("nothing to clean: %d %q", code, out)
	}
}

func TestCacheCleanErrors(t *testing.T) {
	h := newHarness(t)
	f := newCacheFixture(t)
	writeSized(t, filepath.Join(f.lip, "x"), 1)
	cacheLipClean = func(context.Context) error { return errors.New("ERR_LIP_CACHE_CLEAN_FAILED: locked") }
	if code, _, errOut := h.run("cache", "clean", "packages", "--yes"); code != ExitFailure || !strings.Contains(errOut, "[ERR_LIP_CACHE_CLEAN_FAILED]") {
		t.Fatalf("lip failure: %d %q", code, errOut)
	}
	if code, _, errOut := h.run("cache", "clean", "packges"); code != ExitUsage || !strings.Contains(errOut, `did you mean "packages"?`) {
		t.Fatalf("unknown kind: %d %q", code, errOut)
	}
}
