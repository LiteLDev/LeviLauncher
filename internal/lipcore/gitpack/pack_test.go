package gitpack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecodeUTF8MaximalSubparts(t *testing.T) {
	const r = "�"
	cases := map[string]string{
		"a\xe2\x82b.txt":       "a" + r + "b.txt",
		"\xe2\x82A":            r + "A",
		"\xf0\x9f\x98A":        r + "A",
		"\xed\xa0\x80":         r + r + r, // surrogate: ED takes 80..9F only
		"\xc0\x80":             r + r,
		"\xf4\x90\x80\x80":     r + r + r + r, // above U+10FFFF
		"\xe0\x80\x80":         r + r + r,     // overlong
		"\xe0\xa0":             r,
		"\x80\xbf":             r + r,
		"\xf5\x80":             r + r,
		"\xf0\x9f\x98":         r,
		"ok \xf0\x9f\x98\x80":  "ok \U0001F600",
		"\xe2\x82\xe2\x82\xac": r + "€",
	}
	for in, want := range cases {
		if got := decodeUTF8([]byte(in)); got != want {
			t.Errorf("decodeUTF8(%q) = %q, want %q", in, got, want)
		}
	}
	entries, err := ParseTree(rawTree(TreeEntry{Mode: "100644", Name: "a\xe2\x82b.txt", OID: emptyBlob}))
	if err != nil || entries[0].Name != "a"+r+"b.txt" {
		t.Fatalf("ParseTree name = %q, %v", entries[0].Name, err)
	}
}

// sealed appends the SHA-1 trailer to body.
func sealed(body []byte) []byte {
	sum := sha1.Sum(body)
	return append(append([]byte(nil), body...), sum[:]...)
}

func TestHeaderOverlappingTrailerIsCorrupt(t *testing.T) {
	head := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x01")
	cases := map[string][]byte{
		// REF_DELTA whose base id is read from the trailer.
		"ref base id": sealed(append(head, 0x70)),
		// Size varint running into the trailer.
		"size varint": sealed(append(head, 0xb0, 0x80)),
		// OFS_DELTA distance running into the trailer.
		"ofs distance": sealed(append(head, 0x60, 0x80)),
		// Entry whose zlib data needs the trailer bytes.
		"zlib data": sealed(append(head, 0x31, 0x78, 0x9c)),
	}
	for name, pack := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("panic: %v", p)
				}
			}()
			// Like lip, the size varint is read on into the trailer, so the
			// size it ends up with may exceed MaxObjectSize.
			var tooLarge *UnusableRepositoryError
			if err := NewStore().Decode(pack); !errors.Is(err, ErrCorrupt) && !errors.As(err, &tooLarge) {
				t.Fatalf("Decode: %v", err)
			}
			if _, err := OpenBytes(pack); !errors.Is(err, ErrCorrupt) && !errors.As(err, &tooLarge) {
				t.Fatalf("Open: %v", err)
			}
		})
	}

	// An entry that fully inflates but only by consuming trailer bytes.
	var w packWriter
	w.object(ObjBlob, []byte("x"))
	full := w.bytes()
	body := full[:len(full)-20]
	// Drop the last 4 (Adler-32) bytes of the zlib stream from the body; the
	// reader then takes them from the trailer.
	truncated := sealed(body[:len(body)-4])
	if _, err := OpenBytes(truncated); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("truncated: %v", err)
	}
}

type closeTracker struct {
	*bytes.Reader
	closed atomic.Int32
}

func (c *closeTracker) Close() error { c.closed.Add(1); return nil }

func TestPackLazy(t *testing.T) {
	base := bytes.Repeat([]byte("base content "), 100)
	baseID := HashObject(ObjBlob, base)
	w := &packWriter{}
	w.refDelta(baseID, insertDelta(len(base), "one"))
	off := w.object(ObjBlob, base)
	w.ofsDelta(off, insertDelta(len(base), "two"))
	w.object(ObjTree, nil)
	raw := w.bytes()

	src := &closeTracker{Reader: bytes.NewReader(raw)}
	p, err := Open(src, int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Len() != 4 {
		t.Fatalf("Len = %d", p.Len())
	}
	// Nothing is resolved by Open.
	if len(p.indexByOID) != 0 || len(p.cache) != 0 {
		t.Fatalf("Open resolved objects: %d indexed, %d cached", len(p.indexByOID), len(p.cache))
	}

	var got []string
	if err := p.ReadObjects(func(oid string, typ ObjectType, data []byte) error {
		if HashObject(typ, data) != oid {
			t.Errorf("oid %s does not match content", oid)
		}
		got = append(got, string(data[:min(len(data), 4)]))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The REF_DELTA is deferred until its base has been read.
	if want := []string{"base", "two", "", "one"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %q, want %q", got, want)
	}

	o, ok, err := p.FindObject(HashObject(ObjBlob, []byte("one")))
	if err != nil || !ok || string(o.Data) != "one" || o.Type != ObjBlob {
		t.Fatalf("FindObject = %v %v %v", o, ok, err)
	}
	if _, ok, err := p.FindObject(strings.Repeat("0", 40)); ok || err != nil {
		t.Fatalf("missing object: %v %v", ok, err)
	}
	stop := errors.New("stop")
	if err := p.ReadObjects(func(string, ObjectType, []byte) error { return stop }); err != stop {
		t.Fatalf("callback error = %v", err)
	}

	if err := p.Close(); err != nil || src.closed.Load() != 1 {
		t.Fatalf("Close: %v, closed %d times", err, src.closed.Load())
	}
	p.Close()
	if src.closed.Load() != 1 {
		t.Fatal("second Close closed the source again")
	}
	if _, _, err := p.FindObject(baseID); err == nil {
		t.Fatal("FindObject after Close succeeded")
	}

	// A failed Open closes the source.
	bad := append([]byte(nil), raw...)
	bad[len(bad)-1] ^= 1
	src = &closeTracker{Reader: bytes.NewReader(bad)}
	if _, err := Open(src, int64(len(bad))); !errors.Is(err, ErrCorrupt) || src.closed.Load() != 1 {
		t.Fatalf("Open = %v, closed %d", err, src.closed.Load())
	}
}

func TestPackFindObjectDetectsTampering(t *testing.T) {
	w := &packWriter{}
	w.object(ObjBlob, []byte("abc"))
	raw := w.bytes()
	p, err := OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	oid := HashObject(ObjBlob, []byte("abc"))
	if _, ok, err := p.FindObject(oid); !ok || err != nil {
		t.Fatal(ok, err)
	}
	// Index the object under an id its content does not hash to.
	p.indexByOID[emptyBlob] = p.indexByOID[oid]
	if _, _, err := p.FindObject(emptyBlob); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("tampered = %v", err)
	}
}

func TestPackCacheBudget(t *testing.T) {
	// Two 40 MiB OFS_DELTA bases cannot both stay cached within 64 MiB.
	w := &packWriter{}
	var want []string
	for i := byte(0); i < 2; i++ {
		base := bytes.Repeat([]byte{i}, 40<<20)
		off := w.object(ObjBlob, base)
		w.ofsDelta(off, insertDelta(len(base), fmt.Sprintf("delta %d", i)))
		want = append(want, fmt.Sprintf("delta %d", i))
	}
	p, err := OpenBytes(w.bytes())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.ReadObjects(nil); err != nil {
		t.Fatal(err)
	}
	if p.cacheBytes > cacheBudget || len(p.cache) != 1 {
		t.Fatalf("cache holds %d objects, %d bytes", len(p.cache), p.cacheBytes)
	}
	for _, s := range want {
		if o, ok, err := p.FindObject(HashObject(ObjBlob, []byte(s))); err != nil || !ok || string(o.Data) != s {
			t.Fatalf("%s: %v %v", s, ok, err)
		}
	}
}

// zeroBlob is n zero bytes followed by last.
func zeroBlob(n int64, last byte) io.Reader {
	return io.MultiReader(io.LimitReader(zeroReader{}, n-1), bytes.NewReader([]byte{last}))
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// TestPackMemoryBounded decodes a ~1 MB pack of 8 x 128 MiB blobs of zeros:
// only about one object is ever in memory, and nothing stays resident.
func TestPackMemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("inflates 1 GiB")
	}
	const blobSize = 128 << 20
	const blobs = 8

	w := &packWriter{}
	var oids []string
	for i := byte(0); i < blobs; i++ {
		w.entryHeader(int(ObjBlob), blobSize)
		zw, _ := zlib.NewWriterLevel(&w.buf, zlib.BestCompression)
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", blobSize)
		if _, err := io.Copy(io.MultiWriter(zw, h), zeroBlob(blobSize, i)); err != nil {
			t.Fatal(err)
		}
		zw.Close()
		oids = append(oids, fmt.Sprintf("%x", h.Sum(nil)))
	}
	raw := w.bytes()
	w = nil
	path := filepath.Join(t.TempDir(), "big.pack")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("pack: %d bytes for %d MiB of objects", len(raw), blobs*blobSize>>20)
	raw = nil

	old := debug.SetGCPercent(10)
	t.Cleanup(func() { debug.SetGCPercent(old) })
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	baseline := ms.HeapAlloc

	var peak atomic.Uint64
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var ms runtime.MemStats
		for {
			runtime.ReadMemStats(&ms)
			if ms.HeapAlloc > peak.Load() {
				peak.Store(ms.HeapAlloc)
			}
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := f.Stat()
	p, err := Open(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var seen []string
	if err := p.ReadObjects(func(oid string, typ ObjectType, data []byte) error {
		seen = append(seen, oid)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	o, ok, err := p.FindObject(oids[5])
	if err != nil || !ok || len(o.Data) != blobSize || o.Data[blobSize-1] != 5 {
		t.Fatalf("FindObject: %v %v", ok, err)
	}
	o = Object{}
	close(done)
	wg.Wait()

	runtime.GC()
	runtime.ReadMemStats(&ms)
	retained := int64(ms.HeapAlloc) - int64(baseline)
	t.Logf("peak heap %d MiB over baseline, retained %d MiB", (int64(peak.Load())-int64(baseline))>>20, retained>>20)
	if strings.Join(seen, ",") != strings.Join(oids, ",") {
		t.Fatalf("objects = %v, want %v", seen, oids)
	}
	if growth := int64(peak.Load()) - int64(baseline); growth > blobs*blobSize/2 {
		t.Errorf("peak heap grew by %d MiB while inflating %d MiB", growth>>20, blobs*blobSize>>20)
	}
	if retained > 16<<20 {
		t.Errorf("%d MiB stay on the heap after reading the pack", retained>>20)
	}
}
