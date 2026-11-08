package gitproto

// Ports of lip 0.38.0-beta.1
// tests/Lip.Core.Tests/SourceProviders/GitTreeSourceTests.cs.

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

const (
	tsRepoURL = "https://example.com/owner/repo"
	tsCommit  = "0123456789abcdef0123456789abcdef01234567"
)

func tsOid(content string) string { return gitpack.HashObject(gitpack.ObjBlob, []byte(content)) }

var tsBlobs = func() map[string]string {
	m := map[string]string{}
	for _, c := range []string{"{}", "readme", "a", "b", "c", "exe", "a.dll"} {
		m[tsOid(c)] = c
	}
	return m
}()

var tsTree = []TreeEntry{
	{"lip.toml", 100644, tsOid("{}")},
	{"a.dll", 100644, tsOid("a")},
	{"sub/b.txt", 100644, tsOid("b")},
	{"sub/c.txt", 100644, tsOid("c")},
	{"run.sh", 100755, tsOid("exe")},
	{"link.dll", 120000, tsOid("a.dll")},
	{"group.txt", 100664, tsOid("readme")},
}

// fakeTreeClient is the Moq IGitRemoteClient of the C# tests.
type fakeTreeClient struct {
	mu         sync.Mutex
	listErrs   []error // returned by the first ListTree calls
	listCalls  int
	fetchCalls [][]string
	content    func(oid string) string // nil: the real content
	noBlobs    bool
}

func (f *fakeTreeClient) ListTree(ctx context.Context, repoURL, commitish string) ([]TreeEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if repoURL != tsRepoURL || commitish != tsCommit {
		panic("unexpected ListTree " + repoURL + " " + commitish)
	}
	f.listCalls++
	if len(f.listErrs) > 0 {
		err := f.listErrs[0]
		f.listErrs = f.listErrs[1:]
		return nil, err
	}
	return tsTree, nil
}

func (f *fakeTreeClient) FetchBlobs(ctx context.Context, repoURL string, ids []string, onBlob func(string, []byte) error) error {
	f.mu.Lock()
	f.fetchCalls = append(f.fetchCalls, slices.Clone(ids))
	f.mu.Unlock()
	if f.noBlobs {
		return nil
	}
	for _, oid := range ids {
		c := tsBlobs[oid]
		if f.content != nil {
			c = f.content(oid)
		}
		if err := onBlob(oid, []byte(c)); err != nil {
			return err
		}
	}
	return nil
}

func mustRead(t *testing.T) func(io.ReadCloser, error) string {
	return func(rc io.ReadCloser, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, _ := io.ReadAll(rc)
		return string(b)
	}
}

func TestTreeSourceKeysModes100644And100755Only(t *testing.T) {
	src := newTreeSource(&fakeTreeClient{}, context.Background(), tsRepoURL, tsCommit)
	// The symlink (120000) and the group-writable file (100664) are not
	// part of a git archive (protocol/package.md §1).
	if keys := src.Keys(); !reflect.DeepEqual(keys, []string{"lip.toml", "a.dll", "sub/b.txt", "sub/c.txt", "run.sh"}) {
		t.Fatalf("keys %v", keys)
	}
}

func TestTreeSourceMultipleAccessesListTreeOnce(t *testing.T) {
	f := &fakeTreeClient{}
	ctx := context.Background()
	src, err := createTreeSource(f, ctx, tsRepoURL, tsCommit)
	if err != nil {
		t.Fatal(err)
	}
	src.Keys()
	src.Keys()
	if err := nil2(src.Open("a.dll")); err != nil {
		t.Fatal(err)
	}
	if err := src.Prefetch(ctx, []string{"sub/b.txt"}); err != nil {
		t.Fatal(err)
	}
	if f.listCalls != 1 {
		t.Fatalf("ListTree %d", f.listCalls)
	}
}

// nil2 discards the reader of an Open result, keeping its error.
func nil2(rc io.ReadCloser, err error) error {
	if rc != nil {
		rc.Close()
	}
	return err
}

func TestTreeSourceListTreeFailedOnceRetries(t *testing.T) {
	f := &fakeTreeClient{listErrs: []error{errors.New("network")}}
	src := newTreeSource(f, context.Background(), tsRepoURL, tsCommit)
	if _, err := src.Open("a.dll"); err == nil || err.Error() != "network" {
		t.Fatalf("got %v", err)
	}
	if len(src.Keys()) != 5 || len(src.Keys()) != 5 || f.listCalls != 2 {
		t.Fatalf("ListTree %d", f.listCalls)
	}
}

func TestTreeSourceOpenSameKeyTwiceFetchesEachTime(t *testing.T) {
	f := &fakeTreeClient{}
	src := newTreeSource(f, context.Background(), tsRepoURL, tsCommit)
	first := mustRead(t)(src.Open("sub/b.txt"))
	second := mustRead(t)(src.Open("sub/b.txt"))
	if first != "b" || second != "b" {
		t.Fatalf("%q %q", first, second)
	}
	want := [][]string{{tsOid("b")}, {tsOid("b")}}
	if !reflect.DeepEqual(f.fetchCalls, want) || f.listCalls != 1 {
		t.Fatalf("fetches %v, lists %d", f.fetchCalls, f.listCalls)
	}
}

func TestTreeSourceAnotherInstanceFetchesAgain(t *testing.T) {
	f := &fakeTreeClient{}
	ctx := context.Background()
	first := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	if err := first.Prefetch(ctx, []string{"lip.toml"}); err != nil {
		t.Fatal(err)
	}
	second := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	if got := mustRead(t)(second.Open("lip.toml")); got != "{}" || len(f.fetchCalls) != 2 {
		t.Fatalf("%q, %d fetches", got, len(f.fetchCalls))
	}
}

func TestTreeSourcePrefetchMultipleKeysSingleCall(t *testing.T) {
	f := &fakeTreeClient{}
	ctx := context.Background()
	src := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	keys := []string{"a.dll", "sub/b.txt", "run.sh"}
	if err := src.Prefetch(ctx, keys); err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, k := range keys {
		contents = append(contents, mustRead(t)(src.Open(k)))
	}
	if err := src.Prefetch(ctx, keys); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(contents, []string{"a", "b", "exe"}) {
		t.Fatalf("contents %v", contents)
	}
	if len(f.fetchCalls) != 1 || !reflect.DeepEqual(f.fetchCalls[0], []string{tsOid("a"), tsOid("b"), tsOid("exe")}) {
		t.Fatalf("fetches %v", f.fetchCalls)
	}
	if n, ok := src.Size("run.sh"); !ok || n != 3 {
		t.Fatalf("size %d %v", n, ok)
	}
	if _, ok := src.Size("lip.toml"); ok {
		t.Fatal("size of a blob not prefetched")
	}
}

func TestTreeSourcePrefetchPartiallyPrefetched(t *testing.T) {
	f := &fakeTreeClient{}
	ctx := context.Background()
	src := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	if err := src.Prefetch(ctx, []string{"a.dll"}); err != nil {
		t.Fatal(err)
	}
	if err := src.Prefetch(ctx, []string{"a.dll", "sub/b.txt"}); err != nil {
		t.Fatal(err)
	}
	if len(f.fetchCalls) != 2 || !reflect.DeepEqual(f.fetchCalls[1], []string{tsOid("b")}) {
		t.Fatalf("fetches %v", f.fetchCalls)
	}
}

func TestTreeSourceHashMismatch(t *testing.T) {
	f := &fakeTreeClient{content: func(string) string { return "tampered" }}
	ctx := context.Background()
	src := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	var cb *CorruptBlobError
	if _, err := src.Open("a.dll"); !errors.As(err, &cb) {
		t.Fatalf("got %v", err)
	}
	want := "The content of 'a.dll' downloaded from commit " + tsCommit + " in " + tsRepoURL + " is corrupt: its git object id is " +
		tsOid("tampered") + ", but it should be " + tsOid("a") + ". The data was damaged on the way. Try again; if you use a GitHub" +
		" proxy, try without it ('lip config delete github_proxy'); if this keeps happening, report it as a lip bug."
	if cb.Message != want {
		t.Fatalf("got %q", cb.Message)
	}
	if err := src.Prefetch(ctx, []string{"a.dll"}); !errors.As(err, &cb) {
		t.Fatalf("got %v", err)
	}
	if _, err := src.Open("a.dll"); !errors.As(err, &cb) {
		t.Fatalf("got %v", err)
	}
}

func TestTreeSourceUnknownKey(t *testing.T) {
	for _, key := range []string{"missing.txt", "link.dll", "plugin/a.dll"} {
		f := &fakeTreeClient{}
		ctx := context.Background()
		src := newTreeSource(f, ctx, tsRepoURL, tsCommit)
		_, err := src.Open(key)
		if !errors.Is(err, fs.ErrNotExist) || err.Error() != "There is no file '"+key+"' in commit "+tsCommit+" of "+tsRepoURL+"." {
			t.Fatalf("%s: %v", key, err)
		}
		if err := src.Prefetch(ctx, []string{key}); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: %v", key, err)
		}
		if len(f.fetchCalls) != 0 {
			t.Fatalf("%s: fetched", key)
		}
	}
}

func TestTreeSourceBlobNotReturned(t *testing.T) {
	f := &fakeTreeClient{noBlobs: true}
	ctx := context.Background()
	src := newTreeSource(f, ctx, tsRepoURL, tsCommit)
	want := "The git server did not send 1 of the files lip asked for from commit " + tsCommit + " in " + tsRepoURL + ": 'a.dll'. Try again later."
	if _, err := src.Open("a.dll"); err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
	if err := src.Prefetch(ctx, []string{"a.dll"}); err == nil || !strings.HasPrefix(err.Error(), "The git server did not send") {
		t.Fatalf("got %v", err)
	}
}

// The archive.Source contract, structurally.
var _ interface {
	Keys() []string
	Open(key string) (io.ReadCloser, error)
	Size(key string) (int64, bool)
} = (*TreeSource)(nil)
