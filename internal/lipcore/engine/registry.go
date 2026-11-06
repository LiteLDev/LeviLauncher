package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/lipcore/engine/workspace"
	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/migrate"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// GitClient is the subset of gitproto.Client the registry uses.
type GitClient interface {
	ListTags(ctx context.Context, repoURL string) ([]gitproto.TagRef, error)
	Fetch(ctx context.Context, repoURL string, wants []string, opts gitproto.FetchOptions, handle gitproto.PackHandler) error
}

// PackStore is an opened packfile (a lazy, file-backed gitpack.Pack in
// production).
type PackStore interface {
	// Files lists the regular files of commit's tree (paths with "/", in
	// tree order). Root entries that are neither regular files, trees nor
	// gitlinks (e.g. symlinks) are listed too, so a non-regular root
	// tooth.json can be reported; callers filter by mode.
	Files(commit string) ([]gitpack.File, error)
	// Blob returns the content of blob oid; ok is false when the pack does
	// not contain it; an error is returned when oid is not a blob.
	Blob(oid string) (data []byte, ok bool, err error)
	// Objects calls fn with every object of the pack in pack order; data
	// must not be modified.
	Objects(fn func(oid string, t gitpack.ObjectType, data []byte) error) error
	// Close releases the pack (deleting its temporary file).
	Close() error
}

// BlobCache stores verified blobs ("git-blob:<oid>"); *fetch.Cache
// implements it.
type BlobCache interface {
	Get(key string) (path string, ok bool)
	Put(key string, write func(w io.Writer) error) (string, error)
}

// GitRegistry is lip's GitRemotePackageRegistry + SourceService:
// versions come from tags of https://<path>.git; a version's manifest is the
// root tooth.json of the tag commit (migrated with migrate.ToV3 and parsed;
// the engine overwrites tooth/version with a warning when they differ from
// the request); "self" assets are the tag's git tree (blobs fetched lazily in
// batches and cached as git-blob:<oid>, SHA-1 verified); other assets are
// downloaded through Downloader (tried URL by URL) and opened with
// fetch.OpenArchive. Results are memoized per process.
type GitRegistry struct {
	Git        GitClient
	Cache      *fetch.Cache
	Downloader *fetch.Downloader
	Reporter   report.Reporter
	RID        string

	// Seams; nil means the real implementation.

	// DecodePack opens a fetched packfile (gitpack.Open), taking ownership
	// of it: it is closed on error and by PackStore.Close.
	DecodePack func(pack *gitproto.PackFile) (PackStore, error)
	// ParseManifest migrates and parses raw tooth.json bytes
	// (migrate.ToV3 + manifest.Parse).
	ParseManifest func(raw []byte, rid string) (*manifest.Manifest, error)
	// TagVersion converts a tag name to normalized version text
	// (semver.ParseTag).
	TagVersion func(tag string) (string, bool)
	// CompareVersions compares two version texts by precedence.
	CompareVersions func(a, b string) int
	// Blobs overrides Cache for blob storage.
	Blobs BlobCache
	// Download fetches an asset URL into the cache (Downloader.Fetch).
	Download func(ctx context.Context, rawURL string, progress fetch.ProgressFunc) (string, error)
	// OpenArchive opens a downloaded asset (fetch.OpenArchive).
	OpenArchive func(assetType, path string) (fetch.AssetSource, error)
	// NewProgressID returns a fresh progress id (a random GUID).
	NewProgressID func() string

	tags      memo[map[string]gitproto.TagRef]
	manifests memo[*manifest.Manifest]
	trees     memo[[]gitpack.File]

	mu        sync.Mutex
	treeOrder []string
	memBlobs  map[string][]byte

	// packMu guards retained and is held while a retained pack is read, so
	// an evicted pack is never closed under a reader (lip's
	// _retainedPackLock).
	packMu   sync.Mutex
	retained []retainedPack
}

type retainedPack struct {
	repoKey string
	store   PackStore
}

const (
	maxWantsPerRequest = 256
	maxRetainedPacks   = 8
	maxCachedTrees     = 64
)

// NewGitRegistry wires a registry with the real gitproto client, the shared
// lip cache and the given GitHub proxy prefix (may be empty).
func NewGitRegistry(githubProxy string, r report.Reporter, opts ...RegistryOption) *GitRegistry {
	hc := registryHTTPClient(opts)
	rw := fetch.NewProxyRewriter(githubProxy)
	cache := fetch.DefaultCache()
	return &GitRegistry{
		Git:        &gitproto.Client{Proxy: rw, HTTP: hc},
		Cache:      cache,
		Downloader: &fetch.Downloader{Proxy: rw, Cache: cache, HTTP: hc},
		Reporter:   r,
		RID:        CurrentRID(),
	}
}

func (g *GitRegistry) rep() report.Reporter { return report.OrNop(g.Reporter) }

func (g *GitRegistry) rid() string {
	if g.RID != "" {
		return g.RID
	}
	return CurrentRID()
}

func (g *GitRegistry) tagVersion(tag string) (string, bool) {
	if g.TagVersion != nil {
		return g.TagVersion(tag)
	}
	v, ok := semver.ParseTag(tag)
	if !ok {
		return "", false
	}
	return v.String(), true
}

func (g *GitRegistry) compareVersions(a, b string) int {
	if g.CompareVersions != nil {
		return g.CompareVersions(a, b)
	}
	va, errA := semver.Parse(a)
	vb, errB := semver.Parse(b)
	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}
	return va.Compare(vb)
}

func (g *GitRegistry) decodePack(pack *gitproto.PackFile) (PackStore, error) {
	if g.DecodePack != nil {
		return g.DecodePack(pack)
	}
	p, err := gitpack.Open(pack, pack.Size())
	if err != nil {
		return nil, err
	}
	return gitpackStore{p}, nil
}

// Close releases the retained packs (lip's GitRemoteClient.Dispose).
func (g *GitRegistry) Close() error {
	g.packMu.Lock()
	defer g.packMu.Unlock()
	var errs []error
	for _, p := range g.retained {
		errs = append(errs, p.store.Close())
	}
	g.retained = nil
	return errors.Join(errs...)
}

func (g *GitRegistry) parseManifest(raw []byte) (*manifest.Manifest, error) {
	if g.ParseManifest != nil {
		return g.ParseManifest(raw, g.rid())
	}
	v3, err := migrate.ToV3(raw, g.rid())
	if err != nil {
		return nil, err
	}
	return manifest.Parse(v3)
}

func (g *GitRegistry) blobCache() BlobCache {
	if g.Blobs != nil {
		return g.Blobs
	}
	if g.Cache != nil {
		return g.Cache
	}
	return nil
}

// repoKey ports GitRemoteClient.GetRepoKey.
func repoKey(repoURL string) string {
	key := strings.TrimRight(repoURL, "/")
	u, err := url.Parse(repoURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return key
	}
	key = strings.ToLower(key)
	return strings.TrimSuffix(key, ".git")
}

// Versions returns the versions of id's version tags, oldest first.
func (g *GitRegistry) Versions(ctx context.Context, id manifest.PackageID) ([]string, error) {
	tags, err := g.versionTags(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(tags))
	for v := range tags {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if c := g.compareVersions(out[i], out[j]); c != 0 {
			return c < 0
		}
		return out[i] < out[j]
	})
	return out, nil
}

// versionTags ports GetVersionTags/ListVersionTags: version -> tag, where
// the canonical tag spelling "v<version>" wins, else the ordinally smallest.
func (g *GitRegistry) versionTags(ctx context.Context, id manifest.PackageID) (map[string]gitproto.TagRef, error) {
	repo := gitproto.RepoURL(id.Path)
	return g.tags.do(repoKey(repo), func() (map[string]gitproto.TagRef, error) {
		refs, err := g.Git.ListTags(ctx, repo)
		if err != nil {
			return nil, err
		}
		sorted := append([]gitproto.TagRef(nil), refs...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
		tags := map[string]gitproto.TagRef{}
		for _, r := range sorted {
			v, ok := g.tagVersion(r.Name)
			if !ok {
				continue
			}
			existing, have := tags[v]
			if !have || (r.Name == "v"+v && existing.Name != r.Name) {
				tags[v] = r
			}
		}
		return tags, nil
	})
}

// resolveCommit ports ResolveTag/ResolveCommit.
func (g *GitRegistry) resolveCommit(ctx context.Context, spec manifest.PackageSpec) (string, error) {
	tags, err := g.versionTags(ctx, spec.ID)
	if err != nil {
		return "", err
	}
	v, _, _ := strings.Cut(spec.Version, "+")
	tag, ok := tags[v]
	if !ok {
		return "", fmt.Errorf("Package %s not found: no tag for version %s in %s.", spec, spec.Version, gitproto.RepoURL(spec.ID.Path))
	}
	return tag.Commit, nil
}

// Manifest returns the migrated, parsed root tooth.json of the tag commit.
func (g *GitRegistry) Manifest(ctx context.Context, spec manifest.PackageSpec) (*manifest.Manifest, error) {
	repo := gitproto.RepoURL(spec.ID.Path)
	commit, err := g.resolveCommit(ctx, spec)
	if err != nil {
		return nil, err
	}
	return g.manifests.do(repoKey(repo)+"@"+commit, func() (*manifest.Manifest, error) {
		tree, err := g.listTree(ctx, repo, commit)
		if err != nil {
			return nil, err
		}
		var entry *gitpack.File
		for i := range tree {
			if tree[i].Path == "tooth.json" {
				entry = &tree[i]
				break
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("Package %s has no tooth.json at the root of commit %s.", spec, commit)
		}
		if !isRegularMode(entry.Mode) {
			return nil, fmt.Errorf("tooth.json of package %s is not a regular file (mode %s).", spec, entry.Mode)
		}
		rc, err := g.openBlob(ctx, repo, entry.OID)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		raw, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		return g.parseManifest(raw)
	})
}

// OpenAsset opens one asset of spec. A "self" asset is the lazy git tree
// of the tag commit (lip SourceService.GetPackageViaGit); other types are
// downloaded from their URLs, tried in order (PackageInstaller.GetSource).
func (g *GitRegistry) OpenAsset(ctx context.Context, spec manifest.PackageSpec, asset manifest.Asset) (workspace.Source, error) {
	switch asset.Type {
	case manifest.AssetSelf:
		return g.openTree(ctx, spec)
	case manifest.AssetUncompressed, manifest.AssetTar, manifest.AssetTgz, manifest.AssetZip:
		return g.openURLs(ctx, asset)
	default:
		return nil, fmt.Errorf("Unsupported asset type '%s'.", asset.Type)
	}
}

func (g *GitRegistry) openTree(ctx context.Context, spec manifest.PackageSpec) (workspace.Source, error) {
	repo := gitproto.RepoURL(spec.ID.Path)
	commit, err := g.resolveCommit(ctx, spec)
	if err != nil {
		return nil, err
	}
	short := commit
	if len(short) > 12 {
		short = short[:12]
	}
	g.rep().Info(fmt.Sprintf("Listing files of %s@%s (commit %s)...", spec.ID.Path, spec.Version, short))
	tree, err := g.listTree(ctx, repo, commit)
	if err != nil {
		return nil, err
	}
	src := &gitTreeSource{reg: g, ctx: ctx, repo: repo, oids: map[string]string{}, cached: map[string]bool{}}
	for _, f := range tree {
		if !isRegularMode(f.Mode) {
			continue
		}
		if _, dup := src.oids[f.Path]; dup {
			continue
		}
		src.keys = append(src.keys, f.Path)
		src.oids[f.Path] = strings.ToLower(f.OID)
	}
	return src, nil
}

func (g *GitRegistry) openURLs(ctx context.Context, asset manifest.Asset) (workspace.Source, error) {
	var errs []error
	for _, u := range asset.URLs {
		src, err := g.openURL(ctx, string(asset.Type), u)
		if err == nil {
			return src, nil
		}
		errs = append(errs, &wrappedError{msg: fmt.Sprintf("Failed to get file source from %s", u), cause: err})
	}
	return nil, &aggregateError{msg: "Failed to get file source from all provided URLs", errs: errs}
}

func (g *GitRegistry) openURL(ctx context.Context, assetType, rawURL string) (workspace.Source, error) {
	progressID := g.newProgressID()
	message := "Downloading " + rawURL
	if g.Downloader != nil && g.Downloader.Proxy != nil {
		if proxied, ok := g.Downloader.Proxy.Rewrite(rawURL); ok {
			message = "Downloading " + proxied
		}
	}
	progress := func(pct float64) { g.rep().Progress(progressID, message, pct) }

	download := g.Download
	if download == nil {
		if g.Downloader == nil {
			return nil, fmt.Errorf("no downloader configured")
		}
		download = g.Downloader.Fetch
	}
	path, err := download(ctx, rawURL, progress)
	if err != nil {
		return nil, err
	}
	open := g.OpenArchive
	if open == nil {
		open = fetch.OpenArchive
	}
	src, err := open(assetType, path)
	if err != nil {
		return nil, err
	}
	return archiveSource{src}, nil
}

func (g *GitRegistry) newProgressID() string {
	if g.NewProgressID != nil {
		return g.NewProgressID()
	}
	return newGUID()
}

// newGUID returns a random version 4 GUID in .NET "D" format.
func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// archiveSource adapts fetch.AssetSource to workspace.Source.
type archiveSource struct{ fetch.AssetSource }

func (archiveSource) Prefetch(context.Context, []string) error { return nil }

// listTree ports GitRemoteClient.ListTree/FetchTree: a shallow, blob-less
// fetch of commit, walked into its file list. The walk runs inside the
// per-URL proxy fallback, so an incomplete pack from the proxy is retried
// directly. Trees are memoized (at most 64). When the server does not
// support the blob filter, the pack holds every blob and is retained to
// serve them.
func (g *GitRegistry) listTree(ctx context.Context, repo, commit string) ([]gitpack.File, error) {
	commit = strings.ToLower(commit)
	key := repoKey(repo) + "@" + commit
	files, err := g.trees.do(key, func() ([]gitpack.File, error) {
		var files []gitpack.File
		err := g.Git.Fetch(ctx, repo, []string{commit}, gitproto.FetchOptions{Shallow: true, BlobNone: true}, func(pack *gitproto.PackFile) error {
			filtered := pack.Filtered
			store, err := g.decodePack(pack)
			if err != nil {
				return err
			}
			if files, err = store.Files(commit); err != nil {
				store.Close()
				return err
			}
			if filtered {
				store.Close()
				return nil
			}
			g.retain(repoKey(repo), store)
			return nil
		})
		return files, err
	})
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	if !containsString(g.treeOrder, key) {
		g.treeOrder = append(g.treeOrder, key)
		for len(g.treeOrder) > maxCachedTrees {
			g.trees.forget(g.treeOrder[0])
			g.treeOrder = g.treeOrder[1:]
		}
	}
	g.mu.Unlock()
	return files, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// retain keeps store to serve blobs; the oldest of more than 8 retained
// packs is closed (lip's RetainPack).
func (g *GitRegistry) retain(key string, store PackStore) {
	g.packMu.Lock()
	defer g.packMu.Unlock()
	if len(g.retained) >= maxRetainedPacks {
		g.retained[0].store.Close()
		g.retained = append([]retainedPack(nil), g.retained[1:]...)
	}
	g.retained = append(g.retained, retainedPack{repoKey: key, store: store})
}

func (g *GitRegistry) retainedBlob(key, oid string) ([]byte, bool) {
	g.packMu.Lock()
	defer g.packMu.Unlock()
	for _, p := range g.retained {
		if p.repoKey != key {
			continue
		}
		if data, ok, err := p.store.Blob(oid); ok && err == nil {
			return data, true
		}
	}
	return nil, false
}

// isCached reports whether blob oid is already stored.
func (g *GitRegistry) isCached(oid string) bool {
	if c := g.blobCache(); c != nil {
		_, ok := c.Get(fetch.BlobKey(oid))
		return ok
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.memBlobs[oid]
	return ok
}

// storeBlob verifies data against oid and stores it.
func (g *GitRegistry) storeBlob(oid string, data []byte) error {
	if err := verifyBlob(oid, data); err != nil {
		return err
	}
	if c := g.blobCache(); c != nil {
		if _, ok := c.Get(fetch.BlobKey(oid)); ok {
			return nil
		}
		_, err := c.Put(fetch.BlobKey(oid), func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		})
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.memBlobs == nil {
		g.memBlobs = map[string][]byte{}
	}
	g.memBlobs[oid] = data
	return nil
}

// readStoredBlob opens a stored blob.
func (g *GitRegistry) readStoredBlob(oid string) (io.ReadCloser, bool) {
	if c := g.blobCache(); c != nil {
		path, ok := c.Get(fetch.BlobKey(oid))
		if !ok {
			return nil, false
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, false
		}
		return f, true
	}
	g.mu.Lock()
	data, ok := g.memBlobs[oid]
	g.mu.Unlock()
	if !ok {
		return nil, false
	}
	return io.NopCloser(bytes.NewReader(data)), true
}

// openBlob returns blob oid, fetching and caching it when needed.
func (g *GitRegistry) openBlob(ctx context.Context, repo, oid string) (io.ReadCloser, error) {
	oid = strings.ToLower(oid)
	if rc, ok := g.readStoredBlob(oid); ok {
		return rc, nil
	}
	if err := g.fetchBlobs(ctx, repo, []string{oid}); err != nil {
		return nil, err
	}
	if rc, ok := g.readStoredBlob(oid); ok {
		return rc, nil
	}
	return nil, fmt.Errorf("Blob %s was not returned by %s.", oid, repo)
}

// fetchBlobs ports GitRemoteClient.FetchBlobs + GitTreeSource's verified
// caching: blobs come from a retained pack when possible, otherwise from
// fetches of at most 256 wants each; every blob is verified and stored.
func (g *GitRegistry) fetchBlobs(ctx context.Context, repo string, oids []string) error {
	key := repoKey(repo)
	var remaining []string
	seen := map[string]bool{}
	for _, oid := range oids {
		oid = strings.ToLower(oid)
		if seen[oid] {
			continue
		}
		seen[oid] = true
		if data, ok := g.retainedBlob(key, oid); ok {
			if err := g.storeBlob(oid, data); err != nil {
				return err
			}
			continue
		}
		remaining = append(remaining, oid)
	}

	var errs []error
	for start := 0; start < len(remaining); start += maxWantsPerRequest {
		end := start + maxWantsPerRequest
		if end > len(remaining) {
			end = len(remaining)
		}
		if err := g.fetchBlobBatch(ctx, repo, remaining[start:end]); err != nil {
			// lip throws a handler error out of FetchBlobs at once, leaving
			// later batches unfetched.
			if nf, ok := err.(*fetch.NoFallbackError); ok {
				return nf.Err
			}
			errs = append(errs, err)
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return &aggregateError{msg: fmt.Sprintf("Failed to fetch blobs from %s.", repo), errs: errs}
	}
}

// fetchBlobBatch ports GitRemoteClient.FetchBlobBatch: the blobs of each
// pack are stored as they are read, and a pack missing some of them fails
// that attempt, so after a proxy attempt the original URL is asked for only
// the blobs still missing (and the proxy is not used for the repository
// again). As in lip, only fetching and opening the pack falls back: errors
// while reading its objects or storing blobs fail the batch as they are,
// and are returned marked by fetch.NoFallback so fetchBlobs stops too.
func (g *GitRegistry) fetchBlobBatch(ctx context.Context, repo string, batch []string) error {
	remaining := make(map[string]bool, len(batch))
	for _, oid := range batch {
		remaining[oid] = true
	}
	opts := gitproto.FetchOptions{Want: func(oid string) bool { return remaining[oid] }}
	var handlerErr error
	err := g.Git.Fetch(ctx, repo, batch, opts, func(pack *gitproto.PackFile) error {
		store, err := g.decodePack(pack)
		if err != nil {
			return err
		}
		defer store.Close()
		if err := store.Objects(func(oid string, t gitpack.ObjectType, data []byte) error {
			if !remaining[oid] {
				return nil
			}
			if t != gitpack.ObjBlob {
				return fmt.Errorf("Object %s is a %s, not a blob.", oid, t)
			}
			if err := g.storeBlob(oid, data); err != nil {
				return err
			}
			delete(remaining, oid)
			return nil
		}); err != nil {
			handlerErr = err
			return fetch.NoFallback(err)
		}
		var missing []string
		for _, oid := range batch {
			if remaining[oid] {
				missing = append(missing, oid)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("Remote %s did not send %d requested blob(s), e.g. %s.", repo, len(missing), missing[0])
		}
		return nil
	})
	if handlerErr != nil {
		// Fetch returned it at once, unchanged (no retry or fallback).
		return fetch.NoFallback(handlerErr)
	}
	return err
}

// verifyBlob checks that data hashes to oid as a git blob (SHA-1, or
// SHA-256 for 64-character ids).
func verifyBlob(oid string, data []byte) error {
	var h hash.Hash
	if len(oid) == 64 {
		h = sha256.New()
	} else {
		h = sha1.New()
	}
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, oid) {
		return fmt.Errorf("Blob content hash %s does not match object id %s.", actual, oid)
	}
	return nil
}

func isRegularMode(mode string) bool {
	switch strings.TrimLeft(mode, "0") {
	case "100644", "100755", "100664":
		return true
	}
	return false
}

// gitTreeSource ports GitTreeSource: keys are the regular files of the
// tree; contents are fetched on demand (Prefetch batches them).
type gitTreeSource struct {
	reg *GitRegistry
	// ctx is the context of the operation that opened the source; Open
	// (which has no context parameter) uses it for on-demand fetches.
	ctx  context.Context
	repo string
	keys []string
	oids map[string]string

	mu     sync.Mutex
	cached map[string]bool
}

func (s *gitTreeSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *gitTreeSource) Open(key string) (io.ReadCloser, error) {
	oid, ok := s.oids[key]
	if !ok {
		return nil, fmt.Errorf("Key not found: %s (Parameter 'key')", key)
	}
	rc, err := s.reg.openBlob(s.ctx, s.repo, oid)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cached[oid] = true
	s.mu.Unlock()
	return rc, nil
}

func (s *gitTreeSource) Prefetch(ctx context.Context, keys []string) error {
	var pending []string
	seen := map[string]bool{}
	for _, k := range keys {
		oid, ok := s.oids[k]
		if !ok {
			return fmt.Errorf("Key not found: %s (Parameter 'keys')", k)
		}
		s.mu.Lock()
		known := s.cached[oid]
		s.mu.Unlock()
		if known || seen[oid] {
			continue
		}
		seen[oid] = true
		if s.reg.isCached(oid) {
			s.mu.Lock()
			s.cached[oid] = true
			s.mu.Unlock()
			continue
		}
		pending = append(pending, oid)
	}
	if len(pending) == 0 {
		return nil
	}
	if err := s.reg.fetchBlobs(ctx, s.repo, pending); err != nil {
		return err
	}
	s.mu.Lock()
	for _, oid := range pending {
		s.cached[oid] = true
	}
	s.mu.Unlock()
	return nil
}

// gitpackStore adapts gitpack.Pack to PackStore.
type gitpackStore struct{ p *gitpack.Pack }

// Files reads the pack keeping only its non-blob objects in memory, as
// lip's FetchTree does, and walks commit's tree.
func (p gitpackStore) Files(commit string) ([]gitpack.File, error) {
	s := gitpack.NewStore()
	if err := p.p.ReadObjects(func(oid string, t gitpack.ObjectType, data []byte) error {
		if t != gitpack.ObjBlob {
			s.Add(oid, gitpack.Object{Type: t, Data: data})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	tree, err := s.CommitTree(commit)
	if err != nil {
		return nil, err
	}
	files, err := s.WalkFiles(tree)
	if err != nil {
		return nil, err
	}
	if root, ok := s.Get(tree); ok {
		entries, err := gitpack.ParseTree(root.Data)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			mode := strings.TrimLeft(e.Mode, "0")
			if isRegularMode(mode) || mode == "40000" || mode == "160000" {
				continue
			}
			files = append(files, gitpack.File{Path: e.Name, OID: e.OID, Mode: mode})
		}
	}
	return files, nil
}

func (p gitpackStore) Blob(oid string) ([]byte, bool, error) {
	o, ok, err := p.p.FindObject(oid)
	if err != nil || !ok {
		return nil, false, err
	}
	if o.Type != gitpack.ObjBlob {
		return nil, false, fmt.Errorf("Object %s is a %s, not a blob.", oid, o.Type)
	}
	return o.Data, true, nil
}

func (p gitpackStore) Objects(fn func(oid string, t gitpack.ObjectType, data []byte) error) error {
	return p.p.ReadObjects(fn)
}

func (p gitpackStore) Close() error { return p.p.Close() }

// memo memoizes successful results per key; concurrent callers share one
// computation and failures are not kept (lip's Lazy<Task> caches).
type memo[T any] struct {
	mu sync.Mutex
	m  map[string]*memoCall[T]
}

type memoCall[T any] struct {
	done chan struct{}
	v    T
	err  error
}

func (c *memo[T]) do(key string, fn func() (T, error)) (T, error) {
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]*memoCall[T]{}
	}
	if call, ok := c.m[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.v, call.err
	}
	call := &memoCall[T]{done: make(chan struct{})}
	c.m[key] = call
	c.mu.Unlock()

	ok := false
	defer func() {
		if !ok || call.err != nil {
			if !ok {
				call.err = fmt.Errorf("engine: memoized call for %s panicked", key)
			}
			c.mu.Lock()
			if c.m[key] == call {
				delete(c.m, key)
			}
			c.mu.Unlock()
		}
		close(call.done)
	}()
	call.v, call.err = fn()
	ok = true
	return call.v, call.err
}

func (c *memo[T]) forget(key string) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}
