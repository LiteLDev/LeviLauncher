package archive

import (
	"errors"
	"io"
	"iter"
)

// .NET reads Deflate64 entries with DeflateManagedStream (InflaterManaged,
// OutputWindow), which hands out decoded data only between decoding steps:
// when the compressed data ends inside a block, what was decoded since the
// last hand-out may never be served. lip reads an entry with
// Stream.CopyToAsync, so what a truncated Deflate64 entry yields depends on
// how those classes cut the work. dotnetDeflate64Length replays that
// control flow, pulling the steps from a decoder as it needs them.

type inflateEventKind uint8

const (
	evHeader        inflateEventKind = iota // BFINAL and BTYPE; n = the 3 bits
	evDynamicHeader                         // the code lengths of a dynamic block
	evStoredHeader                          // LEN and NLEN; n = LEN
	evStoredByte
	evLiteral
	evMatch // n = length
	evEOB
)

// inflateEvent is a step of decoding; end is the number of input bits it
// needs.
type inflateEvent struct {
	end  int64
	kind inflateEventKind
	n    int
}

// errStopped ends decoding when no more steps are wanted.
var errStopped = errors.New("inflate: stopped")

func (d *deflate64Reader) event(kind inflateEventKind, n int) {
	if d.emit != nil && !d.stopped {
		d.stopped = !d.emit(inflateEvent{end: d.consumed*8 - int64(d.nbits), kind: kind, n: n})
	}
}

const (
	dotnetWindowSize      = 262144 // OutputWindow.WindowSize
	dotnetInputBufferSize = 8192   // DeflateManagedStream.DefaultBufferSize
	// dotnetCopyBufferSize is the buffer Stream.CopyToAsync reads into:
	// ArrayPool<byte>.Shared.Rent(81920).
	dotnetCopyBufferSize = 131072
)

// dotnetDeflate64Length returns how many bytes .NET serves of a Deflate64
// entry whose compressed data (compressedSize bytes, read from r) ends
// before its final block does, given its uncompressed size; ok is false
// when the data does not end early (it is complete or not valid), where
// .NET serves what the decoder yields. It decodes the entry once, keeping
// only the decoder's window.
func dotnetDeflate64Length(r io.Reader, compressedSize int64, uncompressedSize uint64) (n int64, ok bool) {
	var decodeErr error
	next, stop := iter.Pull(func(yield func(inflateEvent) bool) {
		d := newDeflate64Reader(r)
		d.emit = yield
		for decodeErr == nil {
			d.buf, d.off = d.buf[:0], 0
			decodeErr = d.decode()
		}
	})
	defer stop()
	limit := int64(uncompressedSize)
	if uncompressedSize > 1<<62 {
		limit = 1 << 62
	}
	s := &dotnetInflater{next: next, total: compressedSize, limit: limit}
	for s.readAsync(dotnetCopyBufferSize) != 0 {
	}
	switch {
	case decodeErr == nil:
		// .NET stopped before the end of the data (at the uncompressed
		// size), and so does the reader.
		return s.copied, true
	case errors.Is(decodeErr, io.ErrUnexpectedEOF):
		return s.copied, true
	}
	return 0, false
}

type dotnetBlockState uint8

const (
	stBlockHeader dotnetBlockState = iota
	stDynamicHeader
	stHuffman
	stStoredHeader
	stStored
)

// dotnetInflater is DeflateManagedStream and InflaterManaged, counting
// bytes instead of moving them.
type dotnetInflater struct {
	next    func() (inflateEvent, bool)
	pending inflateEvent
	has     bool

	total int64 // compressed bytes
	given int64 // compressed bytes handed to the inflater

	used   int64 // OutputWindow bytes not yet copied out
	copied int64 // _currentInflatedCount
	limit  int64 // _uncompressedSize
	done   bool  // InflaterState.Done

	state      dotnetBlockState
	final      bool
	storedLeft int
}

// peek returns the next event if its bits are available.
func (s *dotnetInflater) peek() (inflateEvent, bool) {
	if !s.has {
		if s.pending, s.has = s.next(); !s.has {
			return inflateEvent{}, false
		}
	}
	return s.pending, s.pending.end <= s.given*8
}

// consume takes the event peek returned.
func (s *dotnetInflater) consume() { s.has = false }

// readAsync is DeflateManagedStream.ReadAsync(Memory<byte>).
func (s *dotnetInflater) readAsync(size int) int64 {
	if n := s.inflate(size); n != 0 {
		return n
	}
	if s.done {
		return 0
	}
	for {
		if s.given == s.total {
			return 0
		}
		s.given += min(dotnetInputBufferSize, s.total-s.given)
		n := s.inflate(size)
		if n == 0 && !s.done {
			continue
		}
		return n
	}
}

// inflate is InflaterManaged.Inflate.
func (s *dotnetInflater) inflate(size int) int64 {
	var count int64
	rest := int64(size)
	for {
		if s.limit > s.copied {
			rest = min(rest, s.limit-s.copied)
			k := min(rest, s.used)
			s.used -= k
			s.copied += k
			count += k
			rest -= k
		} else {
			s.done = true
			s.used = 0
		}
		if rest == 0 || s.done || !s.decode() {
			return count
		}
	}
}

// decode is InflaterManaged.Decode.
func (s *dotnetInflater) decode() bool {
	if s.done {
		return true
	}
	if s.state == stBlockHeader {
		e, ok := s.peek()
		if !ok {
			return false
		}
		s.consume()
		s.final = e.n&1 != 0
		switch e.n >> 1 {
		case 0:
			s.state = stStoredHeader
		case 1:
			s.state = stHuffman
		default:
			s.state = stDynamicHeader
		}
	}
	var result, eob bool
	switch s.state {
	case stDynamicHeader:
		if _, ok := s.peek(); !ok {
			return false
		}
		s.consume()
		s.state = stHuffman
		result = true
	case stHuffman:
		result, eob = s.decodeBlock()
	default:
		result, eob = s.decodeStored()
	}
	if eob && s.final {
		s.done = true
	}
	return result
}

// decodeBlock is InflaterManaged.DecodeBlock.
func (s *dotnetInflater) decodeBlock() (result, eob bool) {
	free := dotnetWindowSize - s.used
	for free > 65536 {
		e, ok := s.peek()
		if !ok {
			return false, false
		}
		s.consume()
		switch e.kind {
		case evLiteral:
			s.used++
			free--
		case evMatch:
			s.used += int64(e.n)
			free -= int64(e.n)
		default: // evEOB
			s.state = stBlockHeader
			return true, true
		}
	}
	return true, false
}

// decodeUncompressedBlock is InflaterManaged.DecodeUncompressedBlock.
func (s *dotnetInflater) decodeStored() (result, eob bool) {
	if s.state == stStoredHeader {
		e, ok := s.peek()
		if !ok {
			return false, false
		}
		s.consume()
		s.storedLeft = e.n
		s.state = stStored
	}
	for s.storedLeft > 0 && s.used < dotnetWindowSize {
		if _, ok := s.peek(); !ok {
			break
		}
		s.consume()
		s.used++
		s.storedLeft--
	}
	if s.storedLeft == 0 {
		s.state = stBlockHeader
		return true, true
	}
	return s.used == dotnetWindowSize, false
}

// dotnetHuffman is HuffmanTree of .NET's Deflate64 decoder, with its
// handling of code lengths that do not form a valid code: unused bit
// strings decode as symbol 0, and codes that overlap overwrite each other.
type dotnetHuffman struct {
	tableBits   int
	table       []int16
	left, right []int16
	lengths     []uint8
}

// newDotnetHuffman is the HuffmanTree constructor (CreateTable); lengths
// has 288, 32 or 19 elements.
func newDotnetHuffman(lengths []uint8) (*dotnetHuffman, error) {
	h := &dotnetHuffman{tableBits: 7, lengths: lengths}
	if len(lengths) == 288 {
		h.tableBits = 9
	}
	h.table = make([]int16, 1<<h.tableBits)
	h.left = make([]int16, 2*len(lengths))
	h.right = make([]int16, 2*len(lengths))

	// CalculateHuffmanCode.
	var count, next [17]uint32
	for _, l := range lengths {
		count[l]++
	}
	count[0] = 0
	var code uint32
	for bits := 1; bits <= 16; bits++ {
		code = (code + count[bits-1]) << 1
		next[bits] = code
	}
	codes := make([]uint32, len(lengths))
	for i, l := range lengths {
		if l > 0 {
			var rev uint32
			c := next[l]
			for n := l; n > 0; n-- {
				rev = rev<<1 | c&1
				c >>= 1
			}
			codes[i] = rev
			next[l]++
		}
	}

	avail := int16(len(lengths))
	for ch, l := range lengths {
		if l == 0 {
			continue
		}
		start := int(codes[ch])
		if int(l) <= h.tableBits {
			increment := 1 << l
			if start >= increment {
				return nil, errDeflate64
			}
			for j := 0; j < 1<<(h.tableBits-int(l)); j++ {
				h.table[start] = int16(ch)
				start += increment
			}
			continue
		}
		overflowBits := int(l) - h.tableBits
		codeBitMask := 1 << h.tableBits
		index := start & (1<<h.tableBits - 1)
		array := h.table
		for {
			value := array[index]
			if value == 0 {
				array[index] = -avail
				value = -avail
				avail++
			}
			if value > 0 {
				return nil, errDeflate64
			}
			if start&codeBitMask == 0 {
				array = h.left
			} else {
				array = h.right
			}
			index = int(-value)
			if index >= len(array) {
				return nil, errDeflate64
			}
			codeBitMask <<= 1
			overflowBits--
			if overflowBits == 0 {
				break
			}
		}
		array[index] = int16(ch)
	}
	return h, nil
}

// decode is HuffmanTree.GetNextSymbol: it looks at the next 16 bits (or
// fewer, zero-padded, at the end of the input).
func (h *dotnetHuffman) decode(d *deflate64Reader) (int, error) {
	for d.nbits < 16 {
		b, err := d.r.ReadByte()
		if err != nil {
			break
		}
		d.consumed++
		d.bits |= uint64(b) << d.nbits
		d.nbits += 8
	}
	if d.nbits == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	// .NET receives the compressed data in 8 KiB buffers and looks the
	// symbol up with the bits it has, zero-padded, before it asks for the
	// next buffer; a code that does not fit then is looked up again. With a
	// valid code both lookups agree; with one that is not, the first may
	// already give a (wrong) symbol or fail.
	pos := d.consumed*8 - int64(d.nbits)
	if before := uint((pos/(dotnetInputBufferSize*8)+1)*dotnetInputBufferSize*8 - pos); before < d.nbits && before < 16 {
		symbol, l := h.lookup(d.bits & (1<<before - 1))
		if l == 0 {
			return 0, errDeflate64 // InvalidHuffmanData
		}
		if l <= before {
			d.bits >>= l
			d.nbits -= l
			return symbol, nil
		}
	}
	symbol, l := h.lookup(d.bits & (1<<min(d.nbits, 63) - 1))
	if l == 0 {
		return 0, errDeflate64 // InvalidHuffmanData
	}
	if l > d.nbits {
		return 0, io.ErrUnexpectedEOF
	}
	d.bits >>= l
	d.nbits -= l
	return symbol, nil
}

// lookup finds the symbol the bits start with and its code length.
func (h *dotnetHuffman) lookup(bits uint64) (int, uint) {
	symbol := int(h.table[bits&uint64(len(h.table)-1)])
	if symbol < 0 {
		mask := uint64(1) << h.tableBits
		for symbol < 0 {
			symbol = -symbol
			if bits&mask == 0 {
				symbol = int(h.left[symbol])
			} else {
				symbol = int(h.right[symbol])
			}
			mask <<= 1
		}
	}
	return symbol, uint(h.lengths[symbol])
}
