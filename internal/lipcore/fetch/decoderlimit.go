package fetch

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math/bits"

	"github.com/klauspost/compress/zstd"
	"github.com/therootcompany/xz"
)

// maxDecoderWindow bounds the dictionary (LZMA, XZ) or window (Zstandard)
// allocated to decode one zip entry. A stream never refers back further
// than the output it has produced, so a dictionary is first shrunk to the
// entry's declared size, which keeps the decoded bytes identical; only an
// entry larger than this whose stream also asks for more is rejected.
const maxDecoderWindow = 256 << 20

// errDecoderWindow reports a dictionary or window above maxDecoderWindow.
func errDecoderWindow(what string, size uint64) error {
	return fmt.Errorf("%s of %d MiB exceeds the %d MiB limit", what, size>>20, maxDecoderWindow>>20)
}

// headerRewriter passes a compressed stream through while step parses its
// container structure, rewriting headers into out and setting skip for
// payload bytes copied as they are. Anything step cannot parse switches it
// to pass, copying the rest unchanged for the decoder to reject.
type headerRewriter struct {
	br   *bufio.Reader
	step func() error
	out  []byte
	skip int64
	pass bool
	err  error
}

func (h *headerRewriter) Read(p []byte) (int, error) {
	for {
		if len(h.out) > 0 {
			n := copy(p, h.out)
			h.out = h.out[n:]
			return n, nil
		}
		if h.err != nil {
			return 0, h.err
		}
		if len(p) == 0 {
			return 0, nil
		}
		if h.pass {
			return h.br.Read(p)
		}
		if h.skip > 0 {
			q := p[:min(int64(len(p)), h.skip)]
			n, err := h.br.Read(q)
			h.skip -= int64(n)
			h.err = err
			if n > 0 {
				return n, nil
			}
			continue
		}
		h.err = h.step()
	}
}

// read reads the next n bytes for step to parse and emit. At the end of the
// input (or a truncated structure) it emits what was read and passes the
// rest through, so the decoder sees the same input.
func (h *headerRewriter) read(n int) ([]byte, bool) {
	b := make([]byte, n)
	k, err := io.ReadFull(h.br, b)
	if err != nil {
		h.out = append(h.out, b[:k]...)
		h.pass = true
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			h.err = err
		}
		return nil, false
	}
	return b, true
}

// readByte is read(1), emitting the byte.
func (h *headerRewriter) readByte() (byte, bool) {
	c, err := h.br.ReadByte()
	if err != nil {
		h.pass = true
		if err != io.EOF {
			h.err = err
		}
		return 0, false
	}
	h.out = append(h.out, c)
	return c, true
}

func (h *headerRewriter) emit(b []byte) { h.out = append(h.out, b...) }

// --- XZ ---

// XZ filter IDs (xz file format 5.3); SharpCompress 0.50.5 decodes the
// branch/call/jump filters in front of LZMA2, but not Delta.
const (
	xzFilterDelta    = 0x03
	xzFilterX86      = 0x04
	xzFilterSPARC    = 0x09
	xzFilterLZMA2    = 0x21
	lzma2MaxDictProp = 39 // 3 GiB; 40 (4 GiB - 1) fails in SharpCompress
)

// xzCheckSizes are the sizes of the Check field by check ID.
var xzCheckSizes = [16]int64{0, 4, 4, 4, 8, 8, 8, 16, 16, 16, 32, 32, 32, 64, 64, 64}

// lzma2DictSize decodes an LZMA2 dictionary size property (0..39).
func lzma2DictSize(p byte) uint64 { return uint64(2|p&1) << (p/2 + 11) }

// lzma2DictProp returns the smallest property whose dictionary holds n
// bytes, or lzma2MaxDictProp+1 if none does.
func lzma2DictProp(n uint64) byte {
	for p := byte(0); p <= lzma2MaxDictProp; p++ {
		if lzma2DictSize(p) >= n {
			return p
		}
	}
	return lzma2MaxDictProp + 1
}

// xzDictLimiter rewrites the LZMA2 dictionary size of every XZ block down to
// what the entry's declared size needs (recomputing the block header CRC),
// and rejects the filters and dictionary sizes SharpCompress rejects.
type xzDictLimiter struct {
	headerRewriter
	need      uint64 // dictionary needed for the declared size
	state     int
	checkSize int64
	data      int64 // compressed bytes of the current block
}

const (
	xzStreamHeader = iota
	xzBlockOrIndex
	xzChunk
	xzStreamEnd
)

// newZipXZReader decodes a zip XZ entry (method 95) of the given declared
// uncompressed size.
func newZipXZReader(raw io.Reader, size uint64) (io.ReadCloser, error) {
	l := &xzDictLimiter{need: max(size, lzma2DictSize(0))}
	l.br = bufio.NewReader(raw)
	l.step = l.next
	// Any block header the limiter could not reach still cannot make the
	// decoder allocate more than the limiter would have allowed.
	dictMax := min(lzma2DictSize(min(lzma2DictProp(l.need), lzma2MaxDictProp)), maxDecoderWindow)
	xr, err := xz.NewReader(l, uint32(dictMax))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(xr), nil
}

func (l *xzDictLimiter) next() error {
	switch l.state {
	case xzStreamHeader:
		b, ok := l.read(12)
		if !ok {
			return nil
		}
		l.emit(b)
		if !bytes.Equal(b[:6], []byte("\xFD7zXZ\x00")) || b[6] != 0 || b[7] > 0x0F {
			l.pass = true
			return nil
		}
		l.checkSize = xzCheckSizes[b[7]]
		l.state = xzBlockOrIndex
	case xzBlockOrIndex:
		c, ok := l.readByte()
		if !ok {
			return nil
		}
		if c == 0 {
			l.readIndex()
			return nil
		}
		return l.blockHeader(c)
	case xzChunk:
		l.chunk()
	case xzStreamEnd:
		// Stream padding (zero bytes), then EOF or another stream.
		switch pk, err := l.br.Peek(1); {
		case err != nil:
			l.pass = true
		case pk[0] == 0:
			l.skip = 1
		default:
			l.state = xzStreamHeader
		}
	}
	return nil
}

// blockHeader parses and rewrites one block header (xz format 3.1) after its
// size byte, which is already emitted and never rewritten.
func (l *xzDictLimiter) blockHeader(size byte) error {
	n := (int(size) + 1) * 4
	rest, ok := l.read(n - 1)
	if !ok {
		return nil
	}
	h := append([]byte{size}, rest...)
	body := h[:n-4]
	if crc32.ChecksumIEEE(body) != binary.LittleEndian.Uint32(h[n-4:]) || h[1]&0x3C != 0 {
		l.emit(h[1:])
		l.pass = true
		return nil
	}
	pos := 2
	for _, present := range []bool{h[1]&0x40 != 0, h[1]&0x80 != 0} {
		if present {
			if pos = skipUvarint(body, pos); pos < 0 {
				l.emit(h[1:])
				l.pass = true
				return nil
			}
		}
	}
	filters := int(h[1]&3) + 1
	for i := 0; i < filters; i++ {
		if pos+2 > len(body) {
			break
		}
		id, props := body[pos], int(body[pos+1])
		pos += 2
		switch {
		case id == xzFilterDelta:
			return errors.New("xz: the Delta filter is not supported")
		case i < filters-1 && xzFilterX86 <= id && id <= xzFilterSPARC:
			pos += props // start offset, checked by the decoder
		case i == filters-1 && id == xzFilterLZMA2 && props == 1 && pos < len(body):
			p := body[pos]
			if p > lzma2MaxDictProp {
				return fmt.Errorf("xz: invalid LZMA2 dictionary size property %d", p)
			}
			if np := lzma2DictProp(l.need); np < p {
				p = np
				body[pos] = p
				binary.LittleEndian.PutUint32(h[n-4:], crc32.ChecksumIEEE(body))
			}
			if d := lzma2DictSize(p); d > maxDecoderWindow {
				return errDecoderWindow("xz: LZMA2 dictionary", d)
			}
		}
	}
	l.emit(h[1:])
	l.state, l.data = xzChunk, 0
	return nil
}

// chunk skips one LZMA2 chunk; after the end marker it skips the block
// padding and check.
func (l *xzDictLimiter) chunk() {
	c, ok := l.readByte()
	if !ok {
		return
	}
	l.data++
	switch {
	case c == 0:
		l.skip = (4-l.data%4)%4 + l.checkSize
		l.state = xzBlockOrIndex
	case c == 1 || c == 2: // uncompressed chunk
		if s, ok := l.read(2); ok {
			l.emit(s)
			l.skip = int64(binary.BigEndian.Uint16(s)) + 1
			l.data += 2 + l.skip
		}
	case c >= 0x80: // LZMA chunk, with properties from 0xC0
		hl := 4
		if c >= 0xC0 {
			hl = 5
		}
		if s, ok := l.read(hl); ok {
			l.emit(s)
			l.skip = int64(binary.BigEndian.Uint16(s[2:])) + 1
			l.data += int64(hl) + l.skip
		}
	default:
		l.pass = true
	}
}

// readIndex copies the index after its indicator byte (xz format 4), then
// the stream footer.
func (l *xzDictLimiter) readIndex() {
	size := int64(1)
	uvarint := func() (uint64, bool) {
		var v uint64
		for i := 0; i < 9; i++ {
			c, ok := l.readByte()
			if !ok {
				return 0, false
			}
			size++
			v |= uint64(c&0x7F) << (7 * i)
			if c&0x80 == 0 {
				return v, true
			}
		}
		l.pass = true
		return 0, false
	}
	records, ok := uvarint()
	for i := uint64(0); ok && i < 2*records; i++ {
		_, ok = uvarint()
	}
	if !ok {
		return
	}
	l.skip = (4-size%4)%4 + 4 + 12 // padding, CRC32, stream footer
	l.state = xzStreamEnd
}

// skipUvarint returns the position after the xz variable-length integer
// at b[pos:], or -1 if it is malformed.
func skipUvarint(b []byte, pos int) int {
	for i := 0; i < 9 && pos < len(b); i++ {
		pos++
		if b[pos-1]&0x80 == 0 {
			return pos
		}
	}
	return -1
}

// --- Zstandard ---

// zstdMinWindow is the smallest window a rewritten frame gets: the maximum
// block size, so that every block valid under the original window stays
// valid.
const zstdMinWindow = 128 << 10

// zstdWindowLimiter rewrites the window descriptor of every Zstandard frame
// down to what the entry's declared size needs.
type zstdWindowLimiter struct {
	headerRewriter
	need     uint64 // window needed for the declared size
	inFrame  bool
	checksum bool
}

// newZipZstdReader decodes a zip Zstandard entry (method 93) of the given
// declared uncompressed size.
func newZipZstdReader(raw io.Reader, size uint64) (io.ReadCloser, error) {
	// The upper clamp keeps pow2Ceil from overflowing; it is above any
	// window a frame can declare.
	l := &zstdWindowLimiter{need: min(max(size, zstdMinWindow), 1<<42)}
	l.br = bufio.NewReader(raw)
	l.step = l.next
	// The decoder rejects frames the limiter could not reach whose window
	// (or single-segment content size) is above what it would allow.
	limit := min(pow2Ceil(l.need), maxDecoderWindow)
	d, err := zstd.NewReader(l, zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(limit), zstd.WithDecoderMaxMemory(limit))
	if err != nil {
		return nil, err
	}
	return d.IOReadCloser(), nil
}

func pow2Ceil(n uint64) uint64 { return 1 << bits.Len64(n-1) }

func (l *zstdWindowLimiter) next() error {
	if l.inFrame {
		// Block header: Last_Block, Block_Type, Block_Size.
		b, ok := l.read(3)
		if !ok {
			return nil
		}
		l.emit(b)
		bh := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
		switch size := int64(bh >> 3); bh >> 1 & 3 {
		case 0, 2: // raw, compressed
			l.skip = size
		case 1: // RLE
			l.skip = 1
		default:
			l.pass = true
			return nil
		}
		if bh&1 != 0 {
			if l.checksum {
				l.skip += 4
			}
			l.inFrame = false
		}
		return nil
	}
	magic, ok := l.read(4)
	if !ok {
		return nil
	}
	l.emit(magic)
	switch {
	case bytes.Equal(magic, []byte{0x28, 0xB5, 0x2F, 0xFD}):
		return l.frameHeader()
	case magic[0]&0xF0 == 0x50 && bytes.Equal(magic[1:], []byte{0x2A, 0x4D, 0x18}):
		if s, ok := l.read(4); ok { // skippable frame
			l.emit(s)
			l.skip = int64(binary.LittleEndian.Uint32(s))
		}
	default:
		l.pass = true
	}
	return nil
}

// frameHeader parses and rewrites a frame header after its magic number.
func (l *zstdWindowLimiter) frameHeader() error {
	b, ok := l.read(1)
	if !ok {
		return nil
	}
	fhd := b[0]
	l.emit(b)
	single := fhd&0x20 != 0
	dictID, fcs := []int{0, 1, 2, 4}[fhd&3], []int{0, 2, 4, 8}[fhd>>6]
	if single && fcs == 0 {
		fcs = 1
	}
	if !single {
		w, ok := l.read(1)
		if !ok {
			return nil
		}
		exp, mant := w[0]>>3, uint64(w[0]&7)
		window := (uint64(1) << (10 + exp)) * (8 + mant) / 8
		if nw := pow2Ceil(l.need); nw < window {
			window = nw
			w[0] = byte(bits.Len64(nw-1)-10) << 3
		}
		if window > maxDecoderWindow {
			return errDecoderWindow("zstd: frame window", window)
		}
		l.emit(w)
	}
	rest, ok := l.read(dictID + fcs)
	if !ok {
		return nil
	}
	if single {
		// The window is the frame content size, which a stream cannot
		// shrink: the frame is that large.
		var b [8]byte
		copy(b[:], rest[dictID:])
		size := binary.LittleEndian.Uint64(b[:])
		if fcs == 2 {
			size += 256
		}
		if size > maxDecoderWindow {
			return errDecoderWindow("zstd: frame window", size)
		}
	}
	l.emit(rest)
	l.checksum = fhd&4 != 0
	l.inFrame = true
	return nil
}
