package gitproto

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempPacks redirects fetched packs to a test directory and returns it.
func tempPacks(t *testing.T) string {
	dir := t.TempDir()
	old := createTemp
	createTemp = func() (*os.File, bool, error) {
		f, err := os.CreateTemp(dir, "lip-*.pack")
		return f, true, err
	}
	t.Cleanup(func() { createTemp = old })
	return dir
}

func packCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func servePack(f *fakeRemote, pack []byte) {
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		w.Write(band(1, pack))
		w.Write([]byte("0000"))
	}
}

func TestFetchPackFile(t *testing.T) {
	dir := tempPacks(t)
	f, srv := newFake(t)
	servePack(f, fakePack())
	c := &Client{}

	var kept *PackFile
	err := c.Fetch(context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{BlobNone: true}, func(p *PackFile) error {
		kept = p
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !kept.Filtered || kept.Size() != int64(len(fakePack())) || packCount(t, dir) != 1 {
		t.Fatalf("filtered %v size %d files %d", kept.Filtered, kept.Size(), packCount(t, dir))
	}
	b, _ := io.ReadAll(io.NewSectionReader(kept, 0, kept.Size()))
	if string(b) != string(fakePack()) {
		t.Fatalf("pack %q", b)
	}
	if err := kept.Close(); err != nil || kept.Close() != nil {
		t.Fatalf("Close: %v", err)
	}
	if packCount(t, dir) != 0 {
		t.Fatal("Close did not delete the pack")
	}

	// Without filter support the pack is not filtered.
	f.caps = []string{"ls-refs", "fetch"}
	c = &Client{}
	if err := c.Fetch(context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{BlobNone: true}, func(p *PackFile) error {
		defer p.Close()
		if p.Filtered {
			t.Error("pack marked filtered")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A failed transfer deletes its file and never reaches the handler.
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		w.Write(band(1, []byte("PA")))
		w.Write(band(3, []byte("boom")))
	}
	err = c.Fetch(context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{}, func(p *PackFile) error {
		t.Error("handler called")
		return p.Close()
	})
	if err == nil || packCount(t, dir) != 0 {
		t.Fatalf("err %v, %d files left", err, packCount(t, dir))
	}

	p, err := NewPackFile(strings.NewReader("PACKdata"))
	if err != nil || p.Size() != 8 || packCount(t, dir) != 1 {
		t.Fatalf("NewPackFile: %v", err)
	}
	p.Close()
	if packCount(t, dir) != 0 {
		t.Fatal("NewPackFile pack not deleted")
	}
}

func TestDeleteOnCloseTemp(t *testing.T) {
	f, err := createDeleteOnCloseTemp()
	if err != nil {
		t.Fatal(err)
	}
	p := &PackFile{f: f}
	if _, err := f.Write([]byte("PACK")); err != nil {
		t.Fatal(err)
	}
	if err := p.finish(); err != nil || p.Size() != 4 {
		t.Fatalf("size %d: %v", p.Size(), err)
	}
	var b [4]byte
	if _, err := p.ReadAt(b[:], 0); err != nil || string(b[:]) != "PACK" {
		t.Fatalf("ReadAt %q: %v", b, err)
	}
	if !strings.HasPrefix(filepath.Base(f.Name()), "lip-") || filepath.Ext(f.Name()) != ".pack" {
		t.Errorf("name %s", f.Name())
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pack still exists: %v", err)
	}
}

// An incomplete pack from the proxy is rejected inside the fallback, so the
// direct URL is tried, asking only for what is still missing.
func TestFetchHandlerErrorFallsBack(t *testing.T) {
	dir := tempPacks(t)
	pf, psrv := newFake(t)
	servePack(pf, []byte("proxy"))
	df, dsrv := newFake(t)
	servePack(df, []byte("direct"))

	p := &fakeProxy{proxyBase: psrv.URL}
	c := &Client{Proxy: p}
	sleeps := recordingSleep(c)
	remaining := map[string]bool{id1: true, id2: true, id3: true}
	var got []string
	err := c.Fetch(context.Background(), dsrv.URL+"/repo.git", []string{id1, id2, id3}, FetchOptions{
		Want: func(oid string) bool { return remaining[oid] },
	}, func(pack *PackFile) error {
		defer pack.Close()
		b, _ := io.ReadAll(io.NewSectionReader(pack, 0, pack.Size()))
		got = append(got, string(b))
		if string(b) == "proxy" {
			// The proxy sent only id2.
			delete(remaining, id2)
			return errors.New("Remote did not send 2 requested blob(s)")
		}
		delete(remaining, id1)
		delete(remaining, id3)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "proxy,direct" || len(p.calls) != 2 {
		t.Fatalf("packs %q, proxy calls %q", got, p.calls)
	}
	if len(*sleeps) != 0 || len(pf.posts) != 1 {
		t.Fatalf("handler error retried: %d sleeps, %d proxy posts", len(*sleeps), len(pf.posts))
	}
	want := string(pkt("command=fetch", "0001", "no-progress", "ofs-delta", "want "+id1, "want "+id3, "done", "0000"))
	if len(df.bodies) != 1 || string(df.bodies[0]) != want {
		t.Fatalf("direct request %q\nwant %q", df.bodies, want)
	}
	if packCount(t, dir) != 0 {
		t.Fatal("packs left behind")
	}

	// Nothing left to want: the attempt succeeds without a request.
	err = c.Fetch(context.Background(), dsrv.URL+"/repo.git", []string{id1}, FetchOptions{
		Want: func(string) bool { return false },
	}, func(pack *PackFile) error {
		t.Error("handler called")
		return pack.Close()
	})
	if err != nil || len(df.bodies) != 1 {
		t.Fatalf("err %v, %d direct requests", err, len(df.bodies))
	}
}
