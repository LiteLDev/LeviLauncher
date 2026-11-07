package archive

import (
	"bytes"
	"compress/flate"
	"io"
	"math/rand"
	"strings"
	"testing"
)

// bitWriter writes a Deflate bit stream.
type bitWriter struct {
	out   []byte
	bits  uint64
	nbits uint
}

func (w *bitWriter) write(v uint64, n uint) {
	w.bits |= v << w.nbits
	w.nbits += n
	for w.nbits >= 8 {
		w.out = append(w.out, byte(w.bits))
		w.bits >>= 8
		w.nbits -= 8
	}
}

// code writes a Huffman code, most significant bit first.
func (w *bitWriter) code(v uint64, n uint) {
	for i := int(n) - 1; i >= 0; i-- {
		w.write(v>>uint(i)&1, 1)
	}
}

func (w *bitWriter) bytes() []byte {
	if w.nbits > 0 {
		w.out = append(w.out, byte(w.bits))
	}
	return w.out
}

// fixedLit writes a literal/length symbol with the fixed Huffman code.
func (w *bitWriter) fixedLit(sym int) {
	switch {
	case sym < 144:
		w.code(uint64(0x30+sym), 8)
	case sym < 256:
		w.code(uint64(0x190+sym-144), 9)
	case sym < 280:
		w.code(uint64(sym-256), 7)
	default:
		w.code(uint64(0xc0+sym-280), 8)
	}
}

// deflate64Fixed returns a Deflate64 stream (one fixed-Huffman block) that
// uses the Deflate64-only codes, and its decoded content: 40000 literals, a
// 65538-byte match (length code 285 with 16 extra bits) at distance 40000
// (distance code 30), and a 10-byte match at distance 60000 (code 31).
func deflate64Fixed() (stream, content []byte) {
	rnd := rand.New(rand.NewSource(1))
	var w bitWriter
	w.write(1, 1) // final
	w.write(1, 2) // fixed codes
	for range 40000 {
		b := byte(rnd.Intn(256))
		w.fixedLit(int(b))
		content = append(content, b)
	}
	match := func(lsym int, lextra uint64, lbits uint, length int, dsym int, dextra uint64, dist int) {
		w.fixedLit(lsym)
		w.write(lextra, lbits)
		w.code(uint64(dsym), 5)
		w.write(dextra, 14)
		for range length {
			content = append(content, content[len(content)-dist])
		}
	}
	match(285, 65535, 16, 65538, 30, 40000-32769, 40000)
	match(264, 0, 0, 10, 31, 60000-49153, 60000)
	w.fixedLit(256)
	return w.bytes(), content
}

func TestDeflate64FixedBlock(t *testing.T) {
	stream, want := deflate64Fixed()
	got, err := io.ReadAll(newDeflate64Reader(bytes.NewReader(stream)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded %d bytes, want %d (equal prefix: %v)", len(got), len(want), bytes.HasPrefix(want, got))
	}
}

// Deflate streams decode identically as Deflate64 when they never use
// length code 285 (Deflate's length 258), which these inputs cannot.
func TestDeflate64DynamicAndStoredBlocks(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	words := strings.Fields("lip tooth package manifest asset zip tar placement dependency version mirror source cache")
	var text bytes.Buffer
	for text.Len() < 300000 {
		text.WriteString(words[rnd.Intn(len(words))])
		text.WriteByte(" \n,."[rnd.Intn(4)])
	}
	for _, level := range []int{flate.NoCompression, flate.BestSpeed, flate.DefaultCompression, flate.HuffmanOnly} {
		var c bytes.Buffer
		fw, _ := flate.NewWriter(&c, level)
		fw.Write(text.Bytes())
		fw.Close()
		got, err := io.ReadAll(newDeflate64Reader(&c))
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if !bytes.Equal(got, text.Bytes()) {
			t.Fatalf("level %d: output differs", level)
		}
	}
}

func TestDeflate64Corrupt(t *testing.T) {
	var w bitWriter
	w.write(1, 1)
	w.write(1, 2)
	w.fixedLit('a')
	w.fixedLit(257) // length 3
	w.code(1, 5)    // distance 2, beyond the 1 byte decoded
	w.fixedLit(256)
	// .NET's Deflate64 decoder copies from its zero-filled window (checked
	// with lip v0.38.0-beta.1's ZipSource); zlib rejects the distance.
	if got, err := io.ReadAll(newDeflate64Reader(bytes.NewReader(w.bytes()))); err != nil || string(got) != "a\x00a\x00" {
		t.Errorf("Deflate64 distance too far: %q, %v", got, err)
	}
	if _, err := io.ReadAll(newDeflateReader(bytes.NewReader(w.bytes()))); err != errDeflate64 {
		t.Errorf("Deflate distance too far: err = %v", err)
	}
	stream, _ := deflate64Fixed()
	if _, err := io.ReadAll(newDeflate64Reader(bytes.NewReader(stream[:len(stream)/2]))); err != io.ErrUnexpectedEOF {
		t.Errorf("truncated: err = %v", err)
	}
	if _, err := io.ReadAll(newDeflate64Reader(bytes.NewReader([]byte{0x07}))); err != errDeflate64 {
		t.Errorf("reserved block type: err = %v", err)
	}
}
