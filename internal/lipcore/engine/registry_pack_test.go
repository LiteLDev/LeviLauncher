package engine

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
	"github.com/liteldev/LeviLauncher/internal/lipcore/gitproto"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// An incomplete blob pack from the proxy is retried directly, asking only for
// the blobs it did not send, and the proxy is not used again.
func TestGitRegistryProxyIncompleteBlobPack(t *testing.T) {
	g, git, repo, cache, _ := newTestRegistry(t)
	repo.proxy = true
	commit := "c100000000000000000000000000000000000000"
	tooth, dll := repo.trees[commit][0].OID, repo.trees[commit][1].OID
	repo.proxyWithold = map[string]bool{dll: true}

	src, err := g.OpenAsset(ctx, specA("1.0.0"), manifest.Asset{Type: manifest.AssetSelf})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Prefetch(ctx, src.Keys()); err != nil {
		t.Fatal(err)
	}
	want := []fetchCall{
		{Repo: repoA, Wants: []string{commit}, Opts: gitproto.FetchOptions{Shallow: true, BlobNone: true}, Proxy: true},
		{Repo: repoA, Wants: []string{tooth, dll}, Proxy: true},
		{Repo: repoA, Wants: []string{dll}},
	}
	if !reflect.DeepEqual(git.fetches, want) {
		t.Fatalf("fetches = %+v\nwant %+v", git.fetches, want)
	}
	if !repo.proxyFailed || cache.puts != 2 {
		t.Fatalf("proxy failed %v, %d blobs stored", repo.proxyFailed, cache.puts)
	}
	if git.closed != 3 {
		t.Errorf("%d of 3 packs closed", git.closed)
	}

	// Later requests go directly.
	if _, err := g.Manifest(ctx, specA("1.2.0")); err == nil {
		t.Fatal("expected no tooth.json")
	}
	if last := git.fetches[len(git.fetches)-1]; last.Proxy {
		t.Errorf("proxy used after failing: %+v", last)
	}
}

// A tree pack from the proxy that lacks the tree is retried directly.
func TestGitRegistryProxyIncompleteTreePack(t *testing.T) {
	g, git, repo, _, _ := newTestRegistry(t)
	repo.proxy = true
	repo.proxyNoTrees = true
	commit := "c100000000000000000000000000000000000000"
	m, err := g.Manifest(ctx, specA("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Tooth != "example.com/a" {
		t.Errorf("manifest = %+v", m)
	}
	want := []fetchCall{
		{Repo: repoA, Wants: []string{commit}, Opts: gitproto.FetchOptions{Shallow: true, BlobNone: true}, Proxy: true},
		{Repo: repoA, Wants: []string{commit}, Opts: gitproto.FetchOptions{Shallow: true, BlobNone: true}},
		{Repo: repoA, Wants: []string{repo.trees[commit][0].OID}},
	}
	if !reflect.DeepEqual(git.fetches, want) || !repo.proxyFailed {
		t.Fatalf("fetches = %+v\nwant %+v", git.fetches, want)
	}
}

func TestGitRegistryRetainedPacksClosed(t *testing.T) {
	g, git, repo, _, _ := newTestRegistry(t)
	// Filtered tree packs are closed once listed.
	if _, err := g.listTree(ctx, repoA, "c100000000000000000000000000000000000000"); err != nil {
		t.Fatal(err)
	}
	if git.closed != 1 || len(g.retained) != 0 {
		t.Fatalf("closed %d, retained %d", git.closed, len(g.retained))
	}

	// Unfiltered ones are retained; the oldest beyond 8 is closed.
	repo.noFilter = true
	git.closed = 0
	for i := range 10 {
		commit := fmt.Sprintf("d%039d", i)
		repo.trees[commit] = []gitpack.File{{Path: "f", OID: blobOID("x"), Mode: "100644"}}
		if _, err := g.listTree(ctx, repoA, commit); err != nil {
			t.Fatal(err)
		}
	}
	if git.closed != 2 || len(g.retained) != maxRetainedPacks {
		t.Fatalf("closed %d, retained %d", git.closed, len(g.retained))
	}
	if err := g.Close(); err != nil || git.closed != 10 || len(g.retained) != 0 {
		t.Fatalf("Close: %v, closed %d", err, git.closed)
	}
}

// packGit serves one real packfile for every fetch.
type packGit struct {
	pack     []byte
	filtered bool
	fetches  int
}

func (g *packGit) ListTags(context.Context, string) ([]gitproto.TagRef, error) { return nil, nil }

func (g *packGit) Fetch(_ context.Context, _ string, _ []string, _ gitproto.FetchOptions, handle gitproto.PackHandler) error {
	g.fetches++
	p, err := gitproto.NewPackFile(bytes.NewReader(g.pack))
	if err != nil {
		return err
	}
	p.Filtered = g.filtered
	return handle(p)
}

// buildPack writes a version 2 pack of undeltified objects; contents are
// read from readers so large objects are never held in memory.
func buildPack(t *testing.T, objs []packObj) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("PACK")
	binary.Write(&b, binary.BigEndian, uint32(2))
	binary.Write(&b, binary.BigEndian, uint32(len(objs)))
	for _, o := range objs {
		size := o.size
		c := byte(o.typ<<4) | byte(size&0x0f)
		for size >>= 4; size > 0; size >>= 7 {
			b.WriteByte(c | 0x80)
			c = byte(size & 0x7f)
		}
		b.WriteByte(c)
		zw, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
		if _, err := io.Copy(zw, o.data()); err != nil {
			t.Fatal(err)
		}
		zw.Close()
	}
	sum := sha1.Sum(b.Bytes())
	return append(b.Bytes(), sum[:]...)
}

type packObj struct {
	typ  gitpack.ObjectType
	size int64
	data func() io.Reader
}

func smallObj(t gitpack.ObjectType, data []byte) (packObj, string) {
	return packObj{typ: t, size: int64(len(data)), data: func() io.Reader { return bytes.NewReader(data) }}, gitpack.HashObject(t, data)
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// TestGitRegistryLargeUnfilteredPack lists a tree from a server without
// filter support whose pack holds 4 x 128 MiB blobs: the retained pack stays
// on disk, only the requested blob is inflated, and the heap stays far below
// the inflated size.
func TestGitRegistryLargeUnfilteredPack(t *testing.T) {
	if testing.Short() {
		t.Skip("inflates 512 MiB")
	}
	const blobSize = 128 << 20
	var objs []packObj
	var tree bytes.Buffer
	for i := range 4 {
		last := []byte{byte(i)}
		o := packObj{typ: gitpack.ObjBlob, size: blobSize, data: func() io.Reader {
			return io.MultiReader(io.LimitReader(zeros{}, blobSize-1), bytes.NewReader(last))
		}}
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", blobSize)
		io.Copy(h, o.data())
		objs = append(objs, o)
		fmt.Fprintf(&tree, "100644 f%d\x00", i)
		tree.Write(h.Sum(nil))
	}
	treeObj, treeID := smallObj(gitpack.ObjTree, tree.Bytes())
	commitObj, commitID := smallObj(gitpack.ObjCommit, []byte("tree "+treeID+"\n\nmsg\n"))
	pack := buildPack(t, append([]packObj{commitObj, treeObj}, objs...))

	git := &packGit{pack: pack}
	g := &GitRegistry{Git: git, Blobs: &dirCache{dir: t.TempDir()}}
	t.Cleanup(func() { g.Close() })

	old := debug.SetGCPercent(10)
	t.Cleanup(func() { debug.SetGCPercent(old) })
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	baseline := ms.HeapAlloc

	files, err := g.listTree(ctx, "https://example.com/big.git", commitID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 || files[2].Path != "f2" || len(g.retained) != 1 {
		t.Fatalf("files %+v, retained %d", files, len(g.retained))
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	if retained := int64(ms.HeapAlloc) - int64(baseline); retained > 32<<20 {
		t.Errorf("%d MiB stay on the heap after listing the tree", retained>>20)
	}

	rc, err := g.openBlob(ctx, "https://example.com/big.git", files[2].OID)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	n, _ := io.Copy(io.Discard, rc)
	if n != blobSize || git.fetches != 1 {
		t.Fatalf("blob of %d bytes after %d fetches", n, git.fetches)
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	if retained := int64(ms.HeapAlloc) - int64(baseline); retained > 32<<20 {
		t.Errorf("%d MiB stay on the heap after reading a blob", retained>>20)
	}
}
