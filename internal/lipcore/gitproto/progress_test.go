package gitproto

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"runtime"
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

// A failed attempt's Done update counts every byte written before the
// failure: finish waits for the scanner to parse them, so the header total
// is never lost to a race with the scanner goroutine.
func TestPackTrackerFinishAfterHeader(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	pack := blobPack(3, 100)
	for i := range 200 {
		var got []PackProgress
		tr := newPackTracker(func(p PackProgress) { got = append(got, p) }, 1, "u")
		tr.Write(pack[:40])
		cut := errors.New("cut")
		tr.finish(cut)
		l := got[len(got)-1]
		if !l.Done || l.Total != 3 || l.Bytes != 40 || l.Err != cut {
			t.Fatalf("run %d: attempt ended %+v", i, l)
		}
		for j, p := range got {
			if p.Done != (j == len(got)-1) {
				t.Fatalf("run %d: update %d Done=%v", i, j, p.Done)
			}
		}
	}
}
