package cli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lipdirs"
)

// Cache kinds of `cache clean`.
const (
	cacheKindPackages  = "packages"  // lip's package download cache
	cacheKindDownloads = "downloads" // files downloaded or handed over for an import
	cacheKindPartial   = "partial"   // unfinished downloads (*.partial.tmp)
)

var cacheKinds = []string{cacheKindPackages, cacheKindDownloads, cacheKindPartial}

// cacheResult is one kind in `cache clean --json`.
type cacheResult struct {
	Kind    string `json:"kind"`
	Items   int    `json:"items"`
	Bytes   int64  `json:"bytes"`
	Skipped int    `json:"skipped"` // in use (an active download), kept
	DryRun  bool   `json:"dryRun"`
}

// Seams replaced by tests.
var (
	cacheLipDir   = lipdirs.CacheDir
	cacheLipClean = lip.CacheClean
	// cacheDownloadDirs hold files downloaded or copied for an import.
	cacheDownloadDirs = func() []string { return []string{apppath.DownloadsDir(), apppath.ImportsDir()} }
	// cachePartialDirs hold resumable downloads.
	cachePartialDirs = func() []string {
		dirs := []string{apppath.DownloadsDir(), apppath.UpdatesDir()}
		if d, err := apppath.InstallersDir(); err == nil {
			dirs = append(dirs, d)
		}
		return dirs
	}
)

func init() {
	Register(&Command{
		Path:    "cache clean",
		Summary: "free disk space used by caches",
		Args:    "[packages|downloads|partial]...",
		Description: `Kinds (default: all three):
  packages   the mod package cache (downloaded LIP packages)
  downloads  CurseForge and other files downloaded for an import
  partial    unfinished downloads; the next download starts over

Files of a download that is running are kept. Game installers are managed with
'installer list' and 'installer remove'.`,
		Examples: []string{"cache clean --dry-run", "cache clean packages --yes", "cache clean partial downloads"},
		Flags:    []*Flag{BoolFlag("dry-run", "n", "show what would be deleted and change nothing")},
		MaxArgs:  Unlimited,
		Run:      runCacheClean,
	})
	RegisterBackendErrors(map[string]BackendError{
		"ERR_LIP_CACHE_CLEAN_FAILED": {Exit: ExitFailure, Message: "cleaning the mod package cache failed"},
	})
}

// cacheFile is a file a kind would delete.
type cacheFile struct {
	path string
	size int64
}

// cachePlan is what cleaning one kind deletes.
type cachePlan struct {
	kind    string
	files   []cacheFile // downloads and partial: the files to remove
	bytes   int64
	items   int
	skipped int
}

func parseCacheKinds(args []string) ([]string, error) {
	if len(args) == 0 {
		return append([]string{}, cacheKinds...), nil
	}
	want := map[string]bool{}
	for _, a := range args {
		k := strings.ToLower(strings.TrimSpace(a))
		switch k {
		case "package", "packages":
			k = cacheKindPackages
		case "download", "downloads":
			k = cacheKindDownloads
		case "partial", "partials":
			k = cacheKindPartial
		default:
			e := UsageError("unknown cache kind %q: use packages, downloads or partial", a)
			if s := suggest(k, cacheKinds); s != "" {
				e = e.WithHint("did you mean %q?", s)
			}
			return nil, e
		}
		want[k] = true
	}
	var out []string
	for _, k := range cacheKinds {
		if want[k] {
			out = append(out, k)
		}
	}
	return out, nil
}

// downloadLocked reports whether a download of path (or of the file path is
// the partial of) is running: it holds "<final>.lock".
func downloadLocked(path string) bool {
	final := strings.TrimSuffix(path, ".partial.tmp")
	if _, err := os.Stat(final + ".lock"); err == nil {
		return true
	}
	return false
}

func planCacheKind(kind string) cachePlan {
	p := cachePlan{kind: kind}
	switch kind {
	case cacheKindPackages:
		root := cacheLipDir()
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type().IsRegular() {
				if info, err := d.Info(); err == nil {
					p.items++
					p.bytes += info.Size()
				}
			}
			return nil
		})
	case cacheKindDownloads:
		for _, dir := range cacheDownloadDirs() {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if !e.Type().IsRegular() || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".partial.tmp") {
					continue
				}
				p.add(filepath.Join(dir, name), e)
			}
		}
	case cacheKindPartial:
		seen := map[string]bool{}
		for _, dir := range cachePartialDirs() {
			_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".partial.tmp") {
					return nil
				}
				key := strings.ToLower(filepath.Clean(path))
				if seen[key] {
					return nil
				}
				seen[key] = true
				p.add(path, d)
				return nil
			})
		}
	}
	return p
}

func (p *cachePlan) add(path string, d fs.DirEntry) {
	if downloadLocked(path) {
		p.skipped++
		return
	}
	info, err := d.Info()
	if err != nil {
		return
	}
	p.files = append(p.files, cacheFile{path: path, size: info.Size()})
	p.items++
	p.bytes += info.Size()
}

// clean deletes the plan's files and reports what was actually freed.
func (p *cachePlan) clean(ctx context.Context) (cacheResult, error) {
	res := cacheResult{Kind: p.kind, Skipped: p.skipped}
	if p.kind == cacheKindPackages {
		if p.items == 0 {
			return res, nil
		}
		if err := cacheLipClean(ctx); err != nil {
			return res, err
		}
		res.Items, res.Bytes = p.items, p.bytes
		return res, nil
	}
	for _, f := range p.files {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := os.Remove(f.path); err != nil {
			if !os.IsNotExist(err) {
				res.Skipped++ // in use by another process
			}
			continue
		}
		res.Items++
		res.Bytes += f.size
	}
	return res, nil
}

func runCacheClean(c *Ctx) error {
	kinds, err := parseCacheKinds(c.Args)
	if err != nil {
		return err
	}
	dry := c.Bool("dry-run")
	plans := make([]cachePlan, 0, len(kinds))
	var items int
	var bytes int64
	for _, k := range kinds {
		p := planCacheKind(k)
		plans = append(plans, p)
		items += p.items
		bytes += p.bytes
	}

	results := make([]cacheResult, 0, len(plans))
	switch {
	case dry || items == 0:
		for _, p := range plans {
			results = append(results, cacheResult{Kind: p.kind, Items: p.items, Bytes: p.bytes, Skipped: p.skipped, DryRun: dry})
		}
	default:
		if err := c.Confirm(fmt.Sprintf("Delete %d cached files (%s) from %s?", items, Size(bytes), strings.Join(kinds, ", "))); err != nil {
			return err
		}
		prog := c.Progress()
		var firstErr error
		for i := range plans {
			prog.Phase("Cleaning %s", plans[i].kind)
			r, err := plans[i].clean(c.Context())
			results = append(results, r)
			if err != nil {
				if c.Context().Err() != nil {
					return err
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		prog.Done()
		if firstErr != nil {
			cacheRender(c, results, dry)
			return FromError(firstErr)
		}
	}
	return cacheRender(c, results, dry)
}

func cacheRender(c *Ctx, results []cacheResult, dry bool) error {
	ids := make([]string, 0, len(results))
	var total int64
	var skipped int
	for _, r := range results {
		ids = append(ids, r.Kind)
		total += r.Bytes
		skipped += r.Skipped
	}
	return c.Render(results, ids, func(w io.Writer) error {
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			rows = append(rows, []string{r.Kind, fmt.Sprint(r.Items), Size(r.Bytes)})
		}
		if err := Table(w, []string{"KIND", "FILES", "SIZE"}, rows); err != nil {
			return err
		}
		switch {
		case dry:
			fmt.Fprintf(w, "Would free %s\n", Size(total))
		case total == 0 && skipped == 0:
			fmt.Fprintln(w, "Nothing to clean")
		default:
			fmt.Fprintf(w, "Freed %s\n", Size(total))
		}
		if skipped > 0 {
			c.Notice("Kept %d file(s) of downloads that are running", skipped)
		}
		return nil
	})
}
