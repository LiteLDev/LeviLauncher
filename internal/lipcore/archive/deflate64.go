package archive

import (
	"bufio"
	"errors"
	"io"
)

// deflate64Reader decodes Deflate (zip method 8) and Deflate64 ("enhanced
// deflating", zip method 9), which SharpCompress reads and Windows Explorer
// writes for large files: Deflate with a 64 KiB window, length code 285
// carrying 16 extra bits (lengths 3 to 65538) and the distance codes 30 and
// 31.
//
// It decodes every symbol whose bits are all in the input, so data that
// ends early yields everything before the incomplete symbol, then
// io.ErrUnexpectedEOF. In Deflate mode it rejects what zlib (which .NET's
// DeflateStream uses) rejects; compress/flate cannot serve here, as it
// stops decoding a block up to a few bits before where zlib does.
type deflate64Reader struct {
	r     io.ByteReader
	bits  uint64
	nbits uint

	deflate bool // Deflate instead of Deflate64

	win   [1 << 16]byte // history; win[total&winMask] is the next byte
	total int64         // bytes decoded so far
	buf   []byte        // decoded bytes not yet returned
	off   int

	inBlock bool
	final   bool
	stored  int // bytes left in a stored block; -1 in a Huffman block
	lit     symbolDecoder
	dist    symbolDecoder
	err     error

	consumed int64 // bytes read from r
	// emit, when not nil, receives each step of decoding; decoding stops
	// (errStopped) once it returns false.
	emit    func(inflateEvent) bool
	stopped bool
}

const deflate64WinMask = 1<<16 - 1

var errDeflate64 = errors.New("inflate: invalid compressed data")

var (
	deflate64LenBase  = [29]int{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 3}
	deflate64LenExtra = [29]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 16}
	deflate64DistBase = [32]int{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537, 2049, 3073,
		4097, 6145, 8193, 12289, 16385, 24577, 32769, 49153}
	deflate64DistExtra = [32]uint{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14}
	codeLengthOrder    = [19]int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}
)

func newDeflate64Reader(r io.Reader) *deflate64Reader {
	br, ok := r.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &deflate64Reader{r: br}
}

// newDeflateReader decodes Deflate as zlib does.
func newDeflateReader(r io.Reader) *deflate64Reader {
	d := newDeflate64Reader(r)
	d.deflate = true
	return d
}

func (d *deflate64Reader) Read(p []byte) (int, error) {
	for d.off == len(d.buf) {
		if d.err != nil {
			return 0, d.err
		}
		d.buf, d.off = d.buf[:0], 0
		d.err = d.decode()
	}
	n := copy(p, d.buf[d.off:])
	d.off += n
	return n, nil
}

// decode decodes up to about 32 KiB into d.buf; it returns io.EOF after the
// final block.
func (d *deflate64Reader) decode() error {
	for len(d.buf) < 1<<15 {
		if d.stopped {
			return errStopped
		}
		if !d.inBlock {
			if d.final {
				return io.EOF
			}
			if err := d.blockHeader(); err != nil {
				return err
			}
			continue
		}
		if d.stored >= 0 {
			if d.stored == 0 {
				d.inBlock = false
				continue
			}
			b, err := d.getBits(8)
			if err != nil {
				return err
			}
			d.put(byte(b))
			d.stored--
			d.event(evStoredByte, 1)
			continue
		}
		sym, err := d.lit.decode(d)
		if err != nil {
			return err
		}
		switch {
		case sym < 256:
			d.put(byte(sym))
			d.event(evLiteral, 1)
		case sym == 256:
			d.inBlock = false
			d.event(evEOB, 0)
		default:
			sym -= 257
			if sym >= len(deflate64LenBase) {
				return errDeflate64
			}
			base, extraBits := deflate64LenBase[sym], deflate64LenExtra[sym]
			if d.deflate && sym == 28 {
				base, extraBits = 258, 0
			}
			extra, err := d.getBits(extraBits)
			if err != nil {
				return err
			}
			length := base + extra
			dsym, err := d.dist.decode(d)
			if err != nil {
				return err
			}
			if dsym >= len(deflate64DistBase) || d.deflate && dsym >= 30 {
				return errDeflate64
			}
			if extra, err = d.getBits(deflate64DistExtra[dsym]); err != nil {
				return err
			}
			dist := deflate64DistBase[dsym] + extra
			if d.deflate && int64(dist) > d.total {
				return errDeflate64 // zlib: "invalid distance too far back"
			}
			// .NET's Deflate64 decoder copies from its zero-filled window
			// what precedes the data; so does win.
			d.event(evMatch, length)
			for ; length > 0; length-- {
				d.put(d.win[(d.total-int64(dist))&deflate64WinMask])
			}
		}
	}
	return nil
}

func (d *deflate64Reader) put(b byte) {
	d.win[d.total&deflate64WinMask] = b
	d.total++
	d.buf = append(d.buf, b)
}

func (d *deflate64Reader) getBits(n uint) (int, error) {
	for d.nbits < n {
		b, err := d.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		d.consumed++
		d.bits |= uint64(b) << d.nbits
		d.nbits += 8
	}
	v := int(d.bits & (1<<n - 1))
	d.bits >>= n
	d.nbits -= n
	return v, nil
}

func (d *deflate64Reader) blockHeader() error {
	h, err := d.getBits(3)
	if err != nil {
		return err
	}
	d.final = h&1 != 0
	d.event(evHeader, h)
	switch h >> 1 {
	case 0: // stored
		d.bits >>= d.nbits % 8
		d.nbits -= d.nbits % 8
		n, err := d.getBits(16)
		if err != nil {
			return err
		}
		nn, err := d.getBits(16)
		if err != nil {
			return err
		}
		if n != ^nn&0xffff {
			return errDeflate64
		}
		d.stored = n
		d.event(evStoredHeader, n)
	case 1: // fixed Huffman codes
		var lengths [288 + 32]uint8
		for i := range 288 {
			switch {
			case i < 144:
				lengths[i] = 8
			case i < 256:
				lengths[i] = 9
			case i < 280:
				lengths[i] = 7
			default:
				lengths[i] = 8
			}
		}
		for i := 288; i < len(lengths); i++ {
			lengths[i] = 5
		}
		if err := d.setCodes(lengths[:288], lengths[288:]); err != nil {
			return err
		}
	case 2: // dynamic Huffman codes
		if err := d.dynamicCodes(); err != nil {
			return err
		}
		d.event(evDynamicHeader, 0)
	default:
		return errDeflate64
	}
	d.inBlock = true
	return nil
}

func (d *deflate64Reader) setCodes(lit, dist []uint8) error {
	d.stored = -1
	if !d.deflate {
		// HuffmanTree of .NET: 288 literal/length and 32 distance lengths.
		var l [288]uint8
		var ds [32]uint8
		copy(l[:], lit)
		copy(ds[:], dist)
		lt, err := newDotnetHuffman(l[:])
		if err != nil {
			return err
		}
		dt, err := newDotnetHuffman(ds[:])
		if err != nil {
			return err
		}
		d.lit, d.dist = lt, dt
		return nil
	}
	lt, dt := &huffman{}, &huffman{}
	if err := lt.build(lit, true); err != nil {
		return err
	}
	if err := dt.build(dist, true); err != nil {
		return err
	}
	d.lit, d.dist = lt, dt
	return nil
}

func (d *deflate64Reader) dynamicCodes() error {
	hlit, err := d.getBits(5)
	if err != nil {
		return err
	}
	hdist, err := d.getBits(5)
	if err != nil {
		return err
	}
	hclen, err := d.getBits(4)
	if err != nil {
		return err
	}
	nlit, ndist := hlit+257, hdist+1
	// zlib: "too many length or distance symbols"; .NET's Deflate64
	// decoder takes up to 288 and 32.
	if d.deflate && (nlit > 286 || ndist > 30) {
		return errDeflate64
	}
	var clen [19]uint8
	for i := range hclen + 4 {
		v, err := d.getBits(3)
		if err != nil {
			return err
		}
		clen[codeLengthOrder[i]] = uint8(v)
	}
	var cl symbolDecoder
	var zlibCL huffman
	if d.deflate {
		if err := zlibCL.buildCodeLengths(clen[:]); err != nil {
			return err
		}
		cl = &zlibCL
	} else if cl, err = newDotnetHuffman(clen[:]); err != nil {
		return err
	}
	lengths := make([]uint8, nlit+ndist)
	for i := 0; i < len(lengths); {
		var sym int
		var err error
		if d.deflate && zlibCL.max == 0 {
			// zlib accepts a code length code without codes; each of
			// its one-bit table entries reads as the length 0.
			_, err = d.getBits(1)
		} else {
			sym, err = cl.decode(d)
		}
		if err != nil {
			return err
		}
		if sym < 16 {
			lengths[i] = uint8(sym)
			i++
			continue
		}
		var val uint8
		var rep int
		switch sym {
		case 16:
			if i == 0 {
				return errDeflate64
			}
			val = lengths[i-1]
			rep, err = d.getBits(2)
			rep += 3
		case 17:
			rep, err = d.getBits(3)
			rep += 3
		default:
			rep, err = d.getBits(7)
			rep += 11
		}
		if err != nil {
			return err
		}
		if i+rep > len(lengths) {
			return errDeflate64
		}
		for ; rep > 0; rep-- {
			lengths[i] = val
			i++
		}
	}
	if lengths[256] == 0 {
		return errDeflate64
	}
	return d.setCodes(lengths[:nlit], lengths[nlit:])
}

// huffman is a canonical Huffman code. Codes of up to huffmanFastBits bits
// are looked up in a table; longer ones are decoded one bit at a time (as
// in zlib's puff.c).
type huffman struct {
	count  [16]int // codes per length
	max    int     // the longest code length; 0 when there are no codes
	symbol []int   // symbols ordered by code
	// fast maps the next huffmanFastBits stream bits to symbol<<4 | length,
	// or 0 for codes longer than that.
	fast [1 << huffmanFastBits]uint16
}

const huffmanFastBits = 9

// symbolDecoder decodes one symbol of a Huffman code.
type symbolDecoder interface {
	decode(d *deflate64Reader) (int, error)
}

// buildCodeLengths builds the code length code as zlib does, which accepts
// only a complete one (or one without codes).
func (h *huffman) buildCodeLengths(lengths []uint8) error {
	if err := h.build(lengths, false); err != nil {
		return err
	}
	if h.max > 0 && h.incomplete() {
		return errDeflate64 // zlib: "invalid code lengths set"
	}
	return nil
}

// build builds the code of a literal/length or distance alphabet. strict
// is zlib's inflate_table check: an incomplete code is accepted only when
// it is a single code of one bit (or has no codes at all).
func (h *huffman) build(lengths []uint8, strict bool) error {
	h.count = [16]int{}
	h.fast = [1 << huffmanFastBits]uint16{}
	h.max = 0
	for _, l := range lengths {
		h.count[l]++
		if int(l) > h.max {
			h.max = int(l)
		}
	}
	h.count[0] = 0
	left := 1
	for l := 1; l < 16; l++ {
		left = left<<1 - h.count[l]
		if left < 0 {
			return errDeflate64 // over-subscribed
		}
	}
	if strict && h.max > 1 && h.incomplete() {
		return errDeflate64 // zlib: "invalid literal/lengths set" / "invalid distances set"
	}
	var offs [16]int
	for l := 1; l < 15; l++ {
		offs[l+1] = offs[l] + h.count[l]
	}
	h.symbol = make([]int, offs[15]+h.count[15])
	for sym, l := range lengths {
		if l != 0 {
			h.symbol[offs[l]] = sym
			offs[l]++
		}
	}
	// Canonical codes in symbol order; the stream holds them bit-reversed.
	code := 0
	for l := 1; l <= huffmanFastBits; l++ {
		start := offs[l] - h.count[l]
		for i := start; i < offs[l]; i++ {
			rev := 0
			for b := range l {
				rev |= (code >> b & 1) << (l - 1 - b)
			}
			for fill := rev; fill < len(h.fast); fill += 1 << l {
				h.fast[fill] = uint16(h.symbol[i]<<4 | l)
			}
			code++
		}
		code <<= 1
	}
	return nil
}

// incomplete reports whether some bit strings are not codes.
func (h *huffman) incomplete() bool {
	left := 1
	for l := 1; l < 16; l++ {
		left = left<<1 - h.count[l]
	}
	return left > 0
}

func (h *huffman) decode(d *deflate64Reader) (int, error) {
	for d.nbits < huffmanFastBits {
		b, err := d.r.ReadByte()
		if err != nil {
			break // near the end of the stream: decode bit by bit
		}
		d.consumed++
		d.bits |= uint64(b) << d.nbits
		d.nbits += 8
	}
	if d.nbits >= huffmanFastBits {
		if e := h.fast[d.bits&(1<<huffmanFastBits-1)]; e != 0 {
			l := uint(e & 15)
			d.bits >>= l
			d.nbits -= l
			return int(e >> 4), nil
		}
	}
	// An incomplete code (at most one code of one bit, or none) is known to
	// be invalid after one bit, as in zlib's tables.
	maxLen := max(h.max, 1)
	code, first, index := 0, 0, 0
	for l := 1; l <= maxLen; l++ {
		b, err := d.getBits(1)
		if err != nil {
			return 0, err
		}
		code |= b
		count := h.count[l]
		if code-count < first {
			return h.symbol[index+code-first], nil
		}
		index += count
		first = (first + count) << 1
		code <<= 1
	}
	return 0, errDeflate64 // not a code
}
