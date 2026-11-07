package gitpack

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// streamPack is a pack with every entry kind: a REF_DELTA whose base comes
// later, plain objects, an OFS_DELTA and an incompressible blob.
func streamPack(t *testing.T) []byte {
	t.Helper()
	base := []byte("base content")
	w := &packWriter{}
	w.refDelta(HashObject(ObjBlob, base), insertDelta(len(base), "one"))
	w.object(ObjBlob, base)
	off := w.object(ObjBlob, []byte("x"))
	w.ofsDelta(off, insertDelta(1, "two"))
	big := make([]byte, 200<<10)
	rand.Read(big)
	w.object(ObjBlob, big)
	w.object(ObjBlob, nil)
	return w.bytes()
}

func scanInChunks(pack []byte, chunk int) ([]StreamProgress, error) {
	var got []StreamProgress
	s := NewStreamScanner(func(p StreamProgress) { got = append(got, p) })
	for i := 0; i < len(pack); i += chunk {
		s.Write(pack[i:min(i+chunk, len(pack))])
	}
	err := s.Close()
	return got, err
}

func TestStreamScannerCounts(t *testing.T) {
	pack := streamPack(t)
	if _, err := OpenBytes(pack); err != nil {
		t.Fatalf("test pack is invalid: %v", err)
	}
	for _, chunk := range []int{1, 7, 4096, len(pack)} {
		got, err := scanInChunks(pack, chunk)
		if err != nil {
			t.Fatalf("chunk %d: %v", chunk, err)
		}
		if len(got) != 7 || got[0] != (StreamProgress{Total: 6}) {
			t.Fatalf("chunk %d: updates %+v", chunk, got)
		}
		for i, p := range got {
			if p.Objects != int64(i) || p.Total != 6 || p.Err != nil {
				t.Fatalf("chunk %d: updates %+v", chunk, got)
			}
		}
	}
}

func TestStreamScannerFailure(t *testing.T) {
	pack := streamPack(t)
	cases := map[string][]byte{
		"truncated":     pack[:len(pack)/2],
		"bad signature": append([]byte("KCAP"), pack[4:]...),
		"bad zlib":      append(append([]byte(nil), pack[:13]...), bytes.Repeat([]byte{0xff}, 300<<10)...),
	}
	for name, data := range cases {
		got, err := scanInChunks(data, 1000)
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		if len(got) == 0 || got[len(got)-1].Err == nil {
			t.Errorf("%s: updates %+v", name, got)
		}
		for i := 1; i < len(got)-1; i++ {
			if got[i].Objects < got[i-1].Objects {
				t.Errorf("%s: updates %+v", name, got)
			}
		}
	}
}

// Abort stops the scanner; no callback runs afterwards.
func TestStreamScannerAbort(t *testing.T) {
	pack := streamPack(t)
	calls := 0
	s := NewStreamScanner(func(StreamProgress) { calls++ })
	s.Write(pack[:20])
	s.Abort(bytes.ErrTooLarge)
	n := calls
	s.Write(pack[20:])
	if calls != n || n == 0 {
		t.Errorf("calls %d after abort, %d before", calls, n)
	}
}
