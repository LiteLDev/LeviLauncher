package engine

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/fetch"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

func blobOID(data string) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// fakeRepo is one remote repository.
type fakeRepo struct {
	tags     []gitproto.TagRef
	trees    map[string][]gitpack.File // commit -> files
	blobs    map[string]string         // oid -> content
	withold  map[string]bool           // blobs the server "forgets" to send
	noFilter bool                      // tree fetch includes blobs

	// proxy makes Fetch try a GitHub proxy first, like gitproto.Client
	// with fetch.ProxyRewriter; the proxy omits proxyWithold blobs and,
	// with proxyNoTrees, every tree.
	proxy        bool
	proxyFailed  bool
	proxyWithold map[string]bool
	proxyNoTrees bool
}

type fetchCall struct {
	Repo  string
	Wants []string
	Opts  gitproto.FetchOptions
	Proxy bool
}

type fakeGit struct {
	mu      sync.Mutex
	repos   map[string]*fakeRepo
	fetches []fetchCall
	lists   int
	closed  int // fakePackStore.Close calls
}

func (g *fakeGit) ListTags(_ context.Context, repoURL string) ([]gitproto.TagRef, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lists++
	r, ok := g.repos[repoURL]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoURL)
	}
	return r.tags, nil
}

// Fetch serves JSON "packs" describing the call; with repo.proxy it tries
// the proxy first and falls back like gitproto.Client.Fetch with
// fetch.ProxyRewriter.
func (g *fakeGit) Fetch(_ context.Context, repoURL string, wants []string, opts gitproto.FetchOptions, handle gitproto.PackHandler) error {
	if len(wants) > maxWantsPerRequest {
		return fmt.Errorf("too many wants: %d", len(wants))
	}
	attempts := 0
	attempt := func(proxy bool) error {
		var wanted []string
		for _, w := range wants {
			if opts.Want == nil || opts.Want(w) {
				wanted = append(wanted, w)
			}
		}
		if len(wanted) == 0 {
			return nil
		}
		if opts.Progress != nil {
			// Like gitproto: indeterminate until the header, one update per
			// object of a pack holding one entry per want, then Done.
			attempts++
			p := gitproto.PackProgress{Attempt: attempts, URL: repoURL}
			opts.Progress(p)
			p.Total = int64(len(wanted))
			for i := range len(wanted) + 1 {
				p.Objects = int64(i)
				opts.Progress(p)
			}
			p.Done = true
			opts.Progress(p)
		}
		c := fetchCall{Repo: repoURL, Wants: wanted, Opts: gitproto.FetchOptions{Shallow: opts.Shallow, BlobNone: opts.BlobNone}, Proxy: proxy}
		g.mu.Lock()
		g.fetches = append(g.fetches, c)
		r := g.repos[repoURL]
		g.mu.Unlock()
		b, err := json.Marshal(struct {
			Repo  string
			Wants []string
			Proxy bool
		}{c.Repo, c.Wants, c.Proxy})
		if err != nil {
			return err
		}
		pack, err := gitproto.NewPackFile(bytes.NewReader(b))
		if err != nil {
			return err
		}
		pack.Filtered = opts.BlobNone && !r.noFilter
		return handle(pack)
	}
	g.mu.Lock()
	r := g.repos[repoURL]
	useProxy := r != nil && r.proxy && !r.proxyFailed
	g.mu.Unlock()
	if !useProxy {
		return fetch.StripNoFallback(attempt(false))
	}
	err := attempt(true)
	if err == nil {
		return nil
	}
	if nf, ok := err.(*fetch.NoFallbackError); ok {
		return nf.Err
	}
	g.mu.Lock()
	r.proxyFailed = true
	g.mu.Unlock()
	return fetch.StripNoFallback(attempt(false))
}

// fakePackStore decodes the "packs" produced by fakeGit.Fetch.
type fakePackStore struct {
	files  map[string][]gitpack.File
	blobs  map[string][]byte
	closed *int
}

func (s *fakePackStore) Objects(fn func(oid string, t gitpack.ObjectType, data []byte) error) error {
	for oid, b := range s.blobs {
		if err := fn(oid, gitpack.ObjBlob, b); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakePackStore) Close() error {
	if s.closed != nil {
		*s.closed++
	}
	return nil
}

func (s *fakePackStore) Files(commit string) ([]gitpack.File, error) {
	f, ok := s.files[commit]
	if !ok {
		return nil, fmt.Errorf("Remote did not send object %s.", commit)
	}
	return f, nil
}

func (s *fakePackStore) Blob(oid string) ([]byte, bool, error) {
	b, ok := s.blobs[oid]
	return b, ok, nil
}

func (g *fakeGit) decode(pack *gitproto.PackFile) (PackStore, error) {
	defer pack.Close()
	raw, err := io.ReadAll(io.NewSectionReader(pack, 0, pack.Size()))
	if err != nil {
		return nil, err
	}
	var c fetchCall
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.repos[c.Repo]
	st := &fakePackStore{files: map[string][]gitpack.File{}, blobs: map[string][]byte{}, closed: &g.closed}
	for _, w := range c.Wants {
		if files, ok := r.trees[w]; ok {
			if c.Proxy && r.proxyNoTrees {
				continue
			}
			st.files[w] = files
			if r.noFilter {
				for _, f := range files {
					st.blobs[f.OID] = []byte(r.blobs[f.OID])
				}
			}
			continue
		}
		if b, ok := r.blobs[w]; ok && !r.withold[w] && !(c.Proxy && r.proxyWithold[w]) {
			st.blobs[w] = []byte(b)
		}
	}
	return st, nil
}

// dirCache is a BlobCache in a temp dir.
type dirCache struct {
	dir  string
	puts int
}

func (c *dirCache) path(key string) string {
	return filepath.Join(c.dir, strings.ReplaceAll(key, ":", "_"))
}

func (c *dirCache) Get(key string) (string, bool) {
	p := c.path(key)
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

func (c *dirCache) Put(key string, write func(io.Writer) error) (string, error) {
	c.puts++
	f, err := os.Create(c.path(key))
	if err != nil {
		return "", err
	}
	defer f.Close()
	return c.path(key), write(f)
}

const repoA = "https://example.com/a.git"

func newTestRegistry(t *testing.T) (*GitRegistry, *fakeGit, *fakeRepo, *dirCache, *recReporter) {
	t.Helper()
	tooth := `{"tooth":"example.com/a","version":"1.0.0"}`
	repo := &fakeRepo{
		tags: []gitproto.TagRef{
			{Name: "v1.0.0", Commit: "c100000000000000000000000000000000000000"},
			{Name: "latest", Commit: "dead"},
			{Name: "v1.2.0", Commit: "c120000000000000000000000000000000000000"},
			{Name: "v1.2", Commit: "c12short0000000000000000000000000000000"},
			{Name: "v2.0.0-rc.1", Commit: "c200000000000000000000000000000000000000"},
		},
		trees: map[string][]gitpack.File{
			"c100000000000000000000000000000000000000": {
				{Path: "tooth.json", OID: blobOID(tooth), Mode: "100644"},
				{Path: "lib/x.dll", OID: blobOID("x"), Mode: "100755"},
				{Path: "link", OID: blobOID("target"), Mode: "120000"},
			},
			"c120000000000000000000000000000000000000": {
				{Path: "README.md", OID: blobOID("readme"), Mode: "100644"},
			},
			"c200000000000000000000000000000000000000": {
				{Path: "tooth.json", OID: blobOID("lnk"), Mode: "120000"},
			},
		},
		blobs: map[string]string{
			blobOID(tooth):    tooth,
			blobOID("x"):      "x",
			blobOID("readme"): "readme",
		},
	}
	git := &fakeGit{repos: map[string]*fakeRepo{repoA: repo}}
	cache := &dirCache{dir: t.TempDir()}
	rep := &recReporter{}
	g := &GitRegistry{
		Git:        git,
		Reporter:   rep,
		RID:        "test-rid",
		DecodePack: git.decode,
		ParseManifest: func(raw []byte, rid string) (*manifest.Manifest, error) {
			if rid != "test-rid" {
				return nil, fmt.Errorf("rid = %s", rid)
			}
			var m manifest.Manifest
			if err := json.Unmarshal(raw, &struct {
				Tooth   *string `json:"tooth"`
				Version *string `json:"version"`
			}{&m.Tooth, &m.Version}); err != nil {
				return nil, err
			}
			return &m, nil
		},
		TagVersion: func(tag string) (string, bool) {
			if !strings.HasPrefix(tag, "v") {
				return "", false
			}
			v := tag[1:]
			if strings.Count(strings.SplitN(v, "-", 2)[0], ".") == 1 {
				parts := strings.SplitN(v, "-", 2)
				parts[0] += ".0"
				v = strings.Join(parts, "-")
			}
			return v, true
		},
		CompareVersions: func(a, b string) int {
			va, _ := parseFakeVersion(a)
			vb, _ := parseFakeVersion(b)
			return va.Compare(vb)
		},
		Blobs: cache,
	}
	return g, git, repo, cache, rep
}

func specA(v string) manifest.PackageSpec {
	return manifest.PackageSpec{ID: manifest.PackageID{Path: "example.com/a"}, Version: v}
}

func TestGitRegistryVersions(t *testing.T) {
	g, git, _, _, _ := newTestRegistry(t)
	got, err := g.Versions(ctx, manifest.PackageID{Path: "example.com/a", Variant: "client"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1.0.0", "1.2.0", "2.0.0-rc.1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("versions = %q, want %q", got, want)
	}
	// Canonical tag spelling wins over "v1.2".
	commit, err := g.resolveCommit(ctx, specA("1.2.0+meta"))
	if err != nil || commit != "c120000000000000000000000000000000000000" {
		t.Errorf("commit = %s, %v", commit, err)
	}
	// Variants share the repository's tag list.
	if _, err := g.Versions(ctx, manifest.PackageID{Path: "example.com/a"}); err != nil {
		t.Fatal(err)
	}
	if git.lists != 1 {
		t.Errorf("ListTags called %d times, want 1", git.lists)
	}
	_, err = g.resolveCommit(ctx, specA("9.0.0"))
	if want := "Package example.com/a@9.0.0 not found: no tag for version 9.0.0 in https://example.com/a.git."; err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestGitRegistryVersionsErrorNotCached(t *testing.T) {
	g, git, _, _, _ := newTestRegistry(t)
	id := manifest.PackageID{Path: "example.com/b"}
	if _, err := g.Versions(ctx, id); err == nil {
		t.Fatal("expected error")
	}
	git.repos["https://example.com/b.git"] = &fakeRepo{tags: []gitproto.TagRef{{Name: "v0.1.0", Commit: "x"}}}
	got, err := g.Versions(ctx, id)
	if err != nil || !reflect.DeepEqual(got, []string{"0.1.0"}) {
		t.Fatalf("versions = %v, %v", got, err)
	}
}

func TestGitRegistryManifest(t *testing.T) {
	g, git, repo, cache, _ := newTestRegistry(t)
	m, err := g.Manifest(ctx, specA("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Tooth != "example.com/a" || m.Version != "1.0.0" {
		t.Errorf("manifest = %+v", m)
	}
	want := []fetchCall{
		{Repo: repoA, Wants: []string{"c100000000000000000000000000000000000000"}, Opts: gitproto.FetchOptions{Shallow: true, BlobNone: true}},
		{Repo: repoA, Wants: []string{repo.trees["c100000000000000000000000000000000000000"][0].OID}},
	}
	if !reflect.DeepEqual(git.fetches, want) {
		t.Errorf("fetches = %+v\nwant %+v", git.fetches, want)
	}
	if _, ok := cache.Get(fetch.BlobKey(want[1].Wants[0])); !ok {
		t.Errorf("tooth.json blob not cached")
	}
	// Memoized.
	if _, err := g.Manifest(ctx, specA("1.0.0")); err != nil || len(git.fetches) != 2 {
		t.Errorf("second Manifest: %v, %d fetches", err, len(git.fetches))
	}
}

func TestGitRegistryManifestErrors(t *testing.T) {
	tests := []struct {
		name    string
		version string
		mutate  func(r *fakeRepo)
		want    string
	}{
		{
			name:    "no tooth.json",
			version: "1.2.0",
			want:    "Package example.com/a@1.2.0 has no tooth.json at the root of commit c120000000000000000000000000000000000000.",
		},
		{
			name:    "symlink tooth.json",
			version: "2.0.0-rc.1",
			want:    "tooth.json of package example.com/a@2.0.0-rc.1 is not a regular file (mode 120000).",
		},
		{
			name:    "corrupt blob",
			version: "1.0.0",
			mutate: func(r *fakeRepo) {
				r.blobs[r.trees["c100000000000000000000000000000000000000"][0].OID] = "tampered"
			},
			want: "Blob content hash " + blobOID("tampered") + " does not match object id ",
		},
		{
			name:    "missing blob",
			version: "1.0.0",
			mutate: func(r *fakeRepo) {
				r.withold = map[string]bool{r.trees["c100000000000000000000000000000000000000"][0].OID: true}
			},
			want: "Remote https://example.com/a.git did not send 1 requested blob(s), e.g. ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _, repo, cache, _ := newTestRegistry(t)
			if tt.mutate != nil {
				tt.mutate(repo)
			}
			_, err := g.Manifest(ctx, specA(tt.version))
			if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Fatalf("err = %v, want prefix %q", err, tt.want)
			}
			if cache.puts != 0 {
				t.Errorf("failed blob was cached")
			}
		})
	}
}

func TestGitTreeSource(t *testing.T) {
	g, git, repo, cache, rep := newTestRegistry(t)
	commit := "c100000000000000000000000000000000000000"
	// 300 extra files force two blob batches.
	for i := 0; i < 300; i++ {
		content := fmt.Sprintf("file %d", i)
		repo.trees[commit] = append(repo.trees[commit], gitpack.File{Path: fmt.Sprintf("data/%03d.txt", i), OID: blobOID(content), Mode: "100644"})
		repo.blobs[blobOID(content)] = content
	}
	src, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.has("info: Listing files of example.com/a@1.0.0 (commit c10000000000)...") {
		t.Errorf("messages = %q", rep.msgs)
	}
	keys := src.Keys()
	if len(keys) != 302 || keys[0] != "tooth.json" || keys[1] != "lib/x.dll" || keys[2] != "data/000.txt" {
		t.Fatalf("keys = %d %q", len(keys), keys[:3])
	}
	for _, k := range keys {
		if k == "link" {
			t.Errorf("symlink exposed")
		}
	}
	git.fetches = nil
	if err := src.Prefetch(ctx, keys); err != nil {
		t.Fatal(err)
	}
	if len(git.fetches) != 2 || len(git.fetches[0].Wants) != 256 || len(git.fetches[1].Wants) != 46 {
		t.Fatalf("fetch batches = %d", len(git.fetches))
	}
	if cache.puts != 302 {
		t.Errorf("puts = %d", cache.puts)
	}
	if err := src.Prefetch(ctx, keys); err != nil || len(git.fetches) != 2 {
		t.Errorf("second prefetch refetched: %v", err)
	}
	rc, err := src.Open("data/007.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "file 7" {
		t.Errorf("content = %q", b)
	}
	if _, err := src.Open("nope"); err == nil {
		t.Errorf("expected missing key error")
	}
	if err := src.Prefetch(ctx, []string{"nope"}); err == nil {
		t.Errorf("expected missing key error")
	}
}

func TestGitTreeSourceRetainedPack(t *testing.T) {
	g, git, repo, _, _ := newTestRegistry(t)
	repo.noFilter = true
	src, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Prefetch(ctx, src.Keys()); err != nil {
		t.Fatal(err)
	}
	if len(git.fetches) != 1 {
		t.Errorf("fetches = %d, want only the tree fetch", len(git.fetches))
	}
}

type fakeAsset struct {
	keys   []string
	closed *bool
}

func (a fakeAsset) Keys() []string { return a.keys }
func (a fakeAsset) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (a fakeAsset) Prefetch(context.Context, []string) error { return nil }
func (a fakeAsset) Close() error                                 { *a.closed = true; return nil }

func TestGitRegistryURLAssets(t *testing.T) {
	g, _, _, _, rep := newTestRegistry(t)
	var tried []string
	g.NewProgressID = func() string { return "id" }
	g.Download = func(_ context.Context, u string, hooks fetch.FetchHooks) (string, error) {
		tried = append(tried, u)
		if strings.Contains(u, "bad") {
			return "", errors.New("HTTP 404")
		}
		hooks.Attempt(u)(fetch.Transfer{Current: 50, Total: 100})
		return "/cache/" + u[strings.LastIndex(u, "/")+1:], nil
	}
	closed := false
	g.OpenArchive = func(assetType, path string) (fetch.AssetSource, error) {
		if assetType != "zip" || path != "/cache/ok.zip" {
			return nil, fmt.Errorf("unexpected %s %s", assetType, path)
		}
		return fakeAsset{keys: []string{"a.txt"}, closed: &closed}, nil
	}
	asset := manifest.Asset{Type: manifest.AssetZip, URLs: []string{"https://bad.example/x.zip", "https://ok.example/ok.zip"}}
	src, err := g.OpenAsset(ctx, specA("1.0.0"), asset)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tried, asset.URLs) || !reflect.DeepEqual(src.Keys(), []string{"a.txt"}) {
		t.Errorf("tried %q, keys %q", tried, src.Keys())
	}
	if err := src.Prefetch(ctx, src.Keys()); err != nil {
		t.Error(err)
	}
	if c, ok := src.(io.Closer); !ok || c.Close() != nil || !closed {
		t.Errorf("source not closable")
	}
	if !rep.has("progress: Downloading https://ok.example/ok.zip 50") {
		t.Errorf("messages = %q", rep.msgs)
	}

	asset.URLs = []string{"https://bad.example/1.zip", "https://bad.example/2.zip"}
	_, err = g.OpenAsset(ctx, specA("1.0.0"), asset)
	want := "Failed to get file source from all provided URLs (Failed to get file source from https://bad.example/1.zip) (Failed to get file source from https://bad.example/2.zip)"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v\nwant %s", err, want)
	}
	if err != nil && !strings.Contains(fmt.Sprint(errors.Unwrap(err.(*aggregateError).errs[0])), "HTTP 404") {
		t.Errorf("cause lost")
	}
}

func TestRepoKey(t *testing.T) {
	tests := map[string]string{
		"https://github.com/LiteLDev/LeviLamina.git": "https://github.com/liteldev/levilamina",
		"https://example.com/A/B.git":                "https://example.com/A/B.git",
	}
	for in, want := range tests {
		if got := repoKey(in); got != want {
			t.Errorf("repoKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerifyBlob(t *testing.T) {
	if err := verifyBlob(blobOID("hello"), []byte("hello")); err != nil {
		t.Error(err)
	}
	// Known git object id of the empty blob.
	if err := verifyBlob("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", nil); err != nil {
		t.Error(err)
	}
	if err := verifyBlob(blobOID("hello"), []byte("world")); err == nil {
		t.Error("expected mismatch")
	}
}

func TestNewGUID(t *testing.T) {
	g := newGUID()
	if len(g) != 36 || g[14] != '4' || strings.Count(g, "-") != 4 || g == newGUID() {
		t.Errorf("guid = %s", g)
	}
}
