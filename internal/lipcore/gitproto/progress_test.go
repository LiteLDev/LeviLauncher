package gitproto

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// blobPack returns a valid pack of n blobs of size bytes of random data.
func blobPack(n, size int) []byte {
	var b bytes.Buffer
	b.WriteString("PACK")
	binary.Write(&b, binary.BigEndian, uint32(2))
	binary.Write(&b, binary.BigEndian, uint32(n))
	for range n {
		data := make([]byte, size)
		rand.Read(data)
		s := size
		c := byte(3<<4) | byte(s&0x0f)
		for s >>= 4; s > 0; s >>= 7 {
			b.WriteByte(c | 0x80)
			c = byte(s & 0x7f)
		}
		b.WriteByte(c)
		zw := zlib.NewWriter(&b)
		zw.Write(data)
		zw.Close()
	}
	sum := sha1.Sum(b.Bytes())
	b.Write(sum[:])
	return b.Bytes()
}

// servePackChunks serves pack in side-band packets of at most chunk bytes.
func servePackChunks(f *fakeRemote, pack []byte, chunk int) {
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		for i := 0; i < len(pack); i += chunk {
			w.Write(band(1, pack[i:min(i+chunk, len(pack))]))
		}
		w.Write([]byte("0000"))
	}
}

// checkAttempts asserts the per-attempt rules and returns the updates of
// each attempt.
func checkAttempts(t *testing.T, got []PackProgress) map[int][]PackProgress {
	t.Helper()
	by := map[int][]PackProgress{}
	for _, p := range got {
		by[p.Attempt] = append(by[p.Attempt], p)
	}
	for a, ups := range by {
		for i, p := range ups {
			if i > 0 && (p.Objects < ups[i-1].Objects || p.Bytes < ups[i-1].Bytes) {
				t.Errorf("attempt %d went backwards: %+v", a, ups)
			}
			if p.Done != (i == len(ups)-1) {
				t.Errorf("attempt %d: update %d of %d Done=%v", a, i+1, len(ups), p.Done)
			}
		}
	}
	return by
}

func TestFetchProgress(t *testing.T) {
	tempPacks(t)
	f, srv := newFake(t)
	pack := blobPack(50, 8<<10)
	servePackChunks(f, pack, 1000)
	var got []PackProgress
	_, err := fetchBytes(&Client{}, context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{
		Progress: func(p PackProgress) { got = append(got, p) },
	})
	if err != nil {
		t.Fatal(err)
	}
	by := checkAttempts(t, got)
	ups := by[1]
	if len(by) != 1 || len(ups) < 3 {
		t.Fatalf("updates %+v", got)
	}
	if f := ups[0]; f.Total != 0 || f.Objects != 0 || f.Bytes != 0 || f.URL != srv.URL+"/repo.git" {
		t.Errorf("first update %+v", f)
	}
	if l := ups[len(ups)-1]; l.Objects != 50 || l.Total != 50 || l.Bytes != int64(len(pack)) {
		t.Errorf("last update %+v", l)
	}
	if ups[1].Total != 50 {
		t.Errorf("header not reported first: %+v", ups[1])
	}
}

// A stream that cannot be scanned does not fail the fetch: the object total
// is dropped and bytes are still counted.
func TestFetchProgressUnscannable(t *testing.T) {
	tempPacks(t)
	f, srv := newFake(t)
	junk := append([]byte("PACK\x00\x00\x00\x02\x00\x00\x00\x05"), bytes.Repeat([]byte{0xff}, 200<<10)...)
	servePackChunks(f, junk, 4096)
	var got []PackProgress
	b, err := fetchBytes(&Client{}, context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{
		Progress: func(p PackProgress) { got = append(got, p) },
	})
	if err != nil || !bytes.Equal(b, junk) {
		t.Fatalf("err %v, %d bytes", err, len(b))
	}
	ups := checkAttempts(t, got)[1]
	l := ups[len(ups)-1]
	if l.Total != 0 || l.Objects != 0 || l.Bytes != int64(len(junk)) {
		t.Errorf("last update %+v", l)
	}
	if len(ups) < 4 {
		t.Errorf("no byte updates: %+v", ups)
	}
}

// Every attempt (proxy, then direct) ends with exactly one Done update,
// also when it fails.
func TestFetchProgressAttempts(t *testing.T) {
	tempPacks(t)
	pf, psrv := newFake(t)
	pack := blobPack(3, 100)
	pf.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		w.Write(band(1, pack[:40]))
		w.Write(band(3, []byte("boom")))
	}
	df, dsrv := newFake(t)
	servePackChunks(df, pack, 10)
	c := &Client{Proxy: &fakeProxy{proxyBase: psrv.URL}}
	recordingSleep(c)
	var got []PackProgress
	if _, err := fetchBytes(c, context.Background(), dsrv.URL+"/repo.git", []string{id1}, FetchOptions{
		Progress: func(p PackProgress) { got = append(got, p) },
	}); err != nil {
		t.Fatal(err)
	}
	by := checkAttempts(t, got)
	if len(by) != 2 {
		t.Fatalf("attempts %+v", got)
	}
	if l := by[1][len(by[1])-1]; l.URL != psrv.URL+"/repo.git" || l.Bytes != 40 || l.Total != 3 || l.Objects == 3 {
		t.Errorf("proxy attempt ended %+v", l)
	}
	if l := by[2][len(by[2])-1]; l.URL != dsrv.URL+"/repo.git" || l.Objects != 3 || l.Total != 3 {
		t.Errorf("direct attempt ended %+v", l)
	}

	// The handler runs after the attempt's Done update.
	got = nil
	err := c.Fetch(context.Background(), dsrv.URL+"/repo.git", []string{id1}, FetchOptions{
		Progress: func(p PackProgress) { got = append(got, p) },
	}, func(p *PackFile) error {
		defer p.Close()
		if len(got) == 0 || !got[len(got)-1].Done {
			return fmt.Errorf("handler before Done: %+v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A canceled fetch still ends its bar.
func TestFetchProgressCanceled(t *testing.T) {
	tempPacks(t)
	f, srv := newFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	pack := blobPack(3, 100)
	hold := make(chan struct{})
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		w.Write(band(1, pack[:30]))
		w.(http.Flusher).Flush()
		cancel()
		<-hold // keep the response open until the client gave up
	}
	var got []PackProgress
	_, err := fetchBytes(&Client{}, ctx, srv.URL+"/repo.git", []string{id1}, FetchOptions{
		Progress: func(p PackProgress) { got = append(got, p) },
	})
	close(hold)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	ups := checkAttempts(t, got)[1]
	if len(ups) == 0 || !ups[len(ups)-1].Done {
		t.Fatalf("updates %+v", got)
	}
}
