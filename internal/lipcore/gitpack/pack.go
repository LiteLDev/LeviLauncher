package gitpack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"sync"
)

// cacheBudget bounds the resolved delta bases a Pack keeps in memory (lip
// GitPackFile.CacheBudget).
const cacheBudget = 64 << 20

// ReaderAtCloser is the storage behind a Pack, typically a temporary file.
type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

// Pack is a version 2/3 packfile read lazily from r, porting lip's
// GitPackFile: Open verifies the checksum and scans the entry headers
// (inflating each entry into nothing to find where it ends); objects are
// inflated on demand and deltas are resolved against objects in the same
// pack, keeping resolved delta bases in a cache of at most 64 MiB. A Pack
// is safe for concurrent use.
type Pack struct {
	mu     sync.Mutex
	src    ReaderAtCloser
	r      *byteReader
	z      inflater
	closed bool

	entries       []packEntry
	indexByOffset map[int64]int
	indexByOID    map[string]int
	ofsDeltaBases map[int]bool
	refDeltaBases map[string]bool
	cache         map[int]Object
	cacheBytes    int64
	fullyIndexed  bool

	// external supplies REF_DELTA bases from outside the pack (Store.Decode
	// of thin packs); nil for lip's in-pack-only resolution.
	external func(oid string) (Object, bool)
}

type packEntry struct {
	typ        int
	size       int64
	dataOffset int64
	baseOffset int64  // OFS_DELTA base entry offset
	baseOID    string // REF_DELTA base id
}

// Open scans the packfile of the given size in src, verifying its trailing
// SHA-1 checksum and every entry header; every entry must inflate to exactly
// its declared size, OFS_DELTA bases must start exactly at an earlier entry
// and the last entry must end exactly at the trailer. Open takes ownership of
// src: it is closed on error and by Pack.Close.
func Open(src ReaderAtCloser, size int64) (*Pack, error) {
	p := &Pack{
		src:           src,
		r:             newByteReader(src, size),
		indexByOffset: map[int64]int{},
		indexByOID:    map[string]int{},
		ofsDeltaBases: map[int]bool{},
		refDeltaBases: map[string]bool{},
		cache:         map[int]Object{},
	}
	if err := p.scan(); err != nil {
		src.Close()
		return nil, err
	}
	return p, nil
}

type bytesCloser struct{ *bytes.Reader }

func (bytesCloser) Close() error { return nil }

// OpenBytes opens an in-memory packfile.
func OpenBytes(pack []byte) (*Pack, error) {
	return Open(bytesCloser{bytes.NewReader(pack)}, int64(len(pack)))
}

// Len returns the number of entries in the pack.
func (p *Pack) Len() int { return len(p.entries) }

// Close releases the pack's storage. Later reads fail.
func (p *Pack) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	p.cache = nil
	return p.src.Close()
}

var errClosed = errors.New("gitpack: pack is closed")

func (p *Pack) scan() error {
	size := p.r.size
	if size < 32 {
		return corrupt("too short")
	}
	var header [12]byte
	if _, err := p.src.ReadAt(header[:], 0); err != nil {
		return corrupt("unexpected end")
	}
	if !bytes.Equal(header[:4], []byte("PACK")) {
		return corrupt("bad signature")
	}
	version := binary.BigEndian.Uint32(header[4:8])
	if version != 2 && version != 3 {
		return corrupt("unsupported version %d", version)
	}
	count := binary.BigEndian.Uint32(header[8:12])
	if err := p.verifyChecksum(); err != nil {
		return err
	}

	end := size - 20
	r := p.r
	r.seek(12)
	readByte := func() (int64, error) {
		b, err := r.ReadByte()
		if err != nil {
			return 0, corrupt("unexpected end")
		}
		return int64(b), nil
	}
	for i := uint32(0); i < count; i++ {
		offset := r.pos()
		if offset >= end {
			return corrupt("object count exceeds data")
		}
		c, err := readByte()
		if err != nil {
			return err
		}
		typ := int((c >> 4) & 7)
		objSize := c & 0x0f
		shift := 4
		for c&0x80 != 0 {
			if shift > 56 {
				return corrupt("object size too large")
			}
			if c, err = readByte(); err != nil {
				return err
			}
			objSize |= (c & 0x7f) << shift
			shift += 7
		}

		e := packEntry{typ: typ, size: objSize, baseOffset: -1}
		switch {
		case typ == objOfsDelta:
			if c, err = readByte(); err != nil {
				return err
			}
			distance := c & 0x7f
			for c&0x80 != 0 {
				if distance > (1<<63-1)>>8 {
					return corrupt("delta offset too large")
				}
				if c, err = readByte(); err != nil {
					return err
				}
				distance = ((distance + 1) << 7) | (c & 0x7f)
			}
			e.baseOffset = offset - distance
			baseIdx, ok := p.indexByOffset[e.baseOffset]
			if distance == 0 || !ok {
				return corrupt("invalid delta base offset")
			}
			p.ofsDeltaBases[baseIdx] = true
		case typ == objRefDelta:
			var id [20]byte
			if _, err := io.ReadFull(r, id[:]); err != nil {
				return corrupt("unexpected end")
			}
			e.baseOID = hex.EncodeToString(id[:])
			p.refDeltaBases[e.baseOID] = true
		case typ < 1 || typ > 4:
			return corrupt("invalid object type %d", typ)
		}

		if objSize > MaxObjectSize {
			return tooLarge(objSize)
		}

		e.dataOffset = r.pos()
		if err := p.z.skip(r, objSize); err != nil {
			return err
		}
		if r.pos() > end {
			return corrupt("object data overlaps trailer")
		}

		p.indexByOffset[offset] = len(p.entries)
		p.entries = append(p.entries, e)
	}
	if r.pos() != end {
		return corrupt("unexpected data after last object")
	}
	return nil
}

func (p *Pack) verifyChecksum() error {
	end := p.r.size - 20
	h := sha1.New()
	if _, err := io.Copy(h, io.NewSectionReader(p.src, 0, end)); err != nil {
		return corrupt("unexpected end")
	}
	var trailer [20]byte
	if _, err := p.src.ReadAt(trailer[:], end); err != nil {
		return corrupt("unexpected end")
	}
	if !bytes.Equal(h.Sum(nil), trailer[:]) {
		return corrupt("checksum mismatch")
	}
	return nil
}

// ReadObjects resolves every object in pack order and calls fn with its id,
// computed from the content (so objects are SHA-1 verified), its type and its
// content. data may be kept but must not be modified (it can be shared with
// the delta-base cache). REF_DELTA entries whose base
// comes later are deferred until it has been read. ReadObjects stops at the
// first error fn returns. The pack is locked during the calls, so fn must
// not use p.
func (p *Pack) ReadObjects(fn func(oid string, t ObjectType, data []byte) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readObjects(fn)
}

func (p *Pack) readObjects(fn func(oid string, t ObjectType, data []byte) error) error {
	if p.closed {
		return errClosed
	}
	emit := func(i int) error {
		t, data, err := p.resolve(i)
		if err != nil {
			return err
		}
		oid := HashObject(t, data)
		p.indexByOID[oid] = i
		if p.refDeltaBases[oid] {
			p.addToCache(i, Object{Type: t, Data: data})
		}
		if fn != nil {
			return fn(oid, t, data)
		}
		return nil
	}

	var deferred []int
	for i := range p.entries {
		ok, err := p.canResolve(i)
		if err != nil {
			return err
		}
		if !ok {
			deferred = append(deferred, i)
			continue
		}
		if err := emit(i); err != nil {
			return err
		}
	}
	for len(deferred) > 0 {
		var still []int
		for _, i := range deferred {
			ok, err := p.canResolve(i)
			if err != nil {
				return err
			}
			if !ok {
				still = append(still, i)
				continue
			}
			if err := emit(i); err != nil {
				return err
			}
		}
		if len(still) == len(deferred) {
			return corrupt("missing delta base")
		}
		deferred = still
	}
	p.fullyIndexed = true
	return nil
}

// FindObject returns the object with id oid; ok is false when the pack does
// not contain it. The first call indexes the whole pack. The content is
// re-verified against oid.
func (p *Pack) FindObject(oid string) (Object, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return Object{}, false, errClosed
	}
	if !p.fullyIndexed {
		if err := p.readObjects(nil); err != nil {
			return Object{}, false, err
		}
	}
	i, ok := p.indexByOID[oid]
	if !ok {
		return Object{}, false, nil
	}
	t, data, err := p.resolve(i)
	if err != nil {
		return Object{}, false, err
	}
	if actual := HashObject(t, data); actual != oid {
		return Object{}, false, corrupt("object %s hashes to %s", oid, actual)
	}
	return Object{Type: t, Data: data}, true, nil
}

// canResolve reports whether every REF_DELTA base in entry i's delta chain
// has already been read (or is available externally).
func (p *Pack) canResolve(i int) (bool, error) {
	for range len(p.entries) + 1 {
		e := p.entries[i]
		switch e.typ {
		case objOfsDelta:
			i = p.indexByOffset[e.baseOffset]
		case objRefDelta:
			if _, ok := p.indexByOID[e.baseOID]; ok {
				return true, nil
			}
			if p.external != nil {
				_, ok := p.external(e.baseOID)
				return ok, nil
			}
			return false, nil
		default:
			return true, nil
		}
	}
	return false, corrupt("delta cycle")
}

// resolve undeltifies entry i. Delta bases (and intermediate chain results)
// are cached as lip does.
func (p *Pack) resolve(i int) (ObjectType, []byte, error) {
	var chain []int
	cur := i
	var resolved Object
	for {
		if o, ok := p.cache[cur]; ok {
			resolved = o
			break
		}
		e := p.entries[cur]
		if e.typ != objOfsDelta && e.typ != objRefDelta {
			data, err := p.inflate(e)
			if err != nil {
				return 0, nil, err
			}
			resolved = Object{Type: ObjectType(e.typ), Data: data}
			if len(chain) > 0 || p.ofsDeltaBases[cur] {
				p.addToCache(cur, resolved)
			}
			break
		}
		chain = append(chain, cur)
		if len(chain) > len(p.entries) {
			return 0, nil, corrupt("delta cycle")
		}
		if e.typ == objOfsDelta {
			cur = p.indexByOffset[e.baseOffset]
			continue
		}
		if idx, ok := p.indexByOID[e.baseOID]; ok {
			cur = idx
			continue
		}
		if p.external != nil {
			if o, ok := p.external(e.baseOID); ok {
				resolved = o
				break
			}
		}
		return 0, nil, corrupt("missing delta base %s", e.baseOID)
	}

	for k := len(chain) - 1; k >= 0; k-- {
		idx := chain[k]
		delta, err := p.inflate(p.entries[idx])
		if err != nil {
			return 0, nil, err
		}
		out, err := ApplyDelta(resolved.Data, delta)
		if err != nil {
			return 0, nil, err
		}
		resolved = Object{Type: resolved.Type, Data: out}
		if k > 0 || p.ofsDeltaBases[idx] {
			p.addToCache(idx, resolved)
		}
	}
	return resolved.Type, resolved.Data, nil
}

// addToCache ports GitPackFile.AddToCache: objects larger than the budget
// are not cached, and the whole cache is dropped when it would overflow.
func (p *Pack) addToCache(i int, o Object) {
	n := int64(len(o.Data))
	if n > cacheBudget {
		return
	}
	if p.cacheBytes+n > cacheBudget {
		clear(p.cache)
		p.cacheBytes = 0
	}
	if _, ok := p.cache[i]; !ok {
		p.cache[i] = o
		p.cacheBytes += n
	}
}

func (p *Pack) inflate(e packEntry) ([]byte, error) {
	out := make([]byte, e.size)
	p.r.seek(e.dataOffset)
	if err := p.z.inflate(p.r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// byteReader is a seekable buffered reader over an io.ReaderAt (lip's
// BufferedByteReader). It implements io.ByteReader, so compress/flate reads
// exactly the bytes of a zlib stream and no more, keeping pos exact.
type byteReader struct {
	src   io.ReaderAt
	size  int64
	buf   []byte
	start int64 // file offset of buf[0]
	n     int   // valid bytes in buf
	off   int   // read position in buf
}

func newByteReader(src io.ReaderAt, size int64) *byteReader {
	return &byteReader{src: src, size: size, buf: make([]byte, 64<<10)}
}

func (r *byteReader) pos() int64 { return r.start + int64(r.off) }

func (r *byteReader) seek(pos int64) {
	if pos >= r.start && pos <= r.start+int64(r.n) {
		r.off = int(pos - r.start)
		return
	}
	r.start, r.n, r.off = pos, 0, 0
}

func (r *byteReader) fill() error {
	r.start += int64(r.n)
	r.n, r.off = 0, 0
	if r.start >= r.size {
		return io.EOF
	}
	want := min(int64(len(r.buf)), r.size-r.start)
	n, err := r.src.ReadAt(r.buf[:want], r.start)
	r.n = n
	if n > 0 {
		return nil
	}
	if err == nil || err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func (r *byteReader) ReadByte() (byte, error) {
	if r.off >= r.n {
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
	b := r.buf[r.off]
	r.off++
	return b, nil
}

func (r *byteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.off >= r.n {
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf[r.off:r.n])
	r.off += n
	return n, nil
}

// inflater decompresses the zlib stream of one pack entry, reusing its
// decompressor to avoid reallocating the window.
type inflater struct {
	zr io.ReadCloser
}

func (z *inflater) reset(r *byteReader) (io.Reader, error) {
	var err error
	if z.zr == nil {
		z.zr, err = zlib.NewReader(r)
	} else {
		err = z.zr.(zlib.Resetter).Reset(r, nil)
	}
	if err != nil {
		z.zr = nil
		return nil, corrupt("invalid zlib header: %v", err)
	}
	return z.zr, nil
}

// finish checks that the stream ends (with a valid Adler-32 trailer) right
// after the declared size.
func (z *inflater) finish(size int64) error {
	var one [1]byte
	n, err := z.zr.Read(one[:])
	for n == 0 && err == nil {
		n, err = z.zr.Read(one[:])
	}
	if n > 0 {
		return corrupt("zlib stream: output exceeds declared size %d", size)
	}
	if err != io.EOF {
		return corrupt("zlib stream: %v", err)
	}
	return nil
}

// inflate fills out with one zlib stream starting at r's position, which
// must expand to exactly len(out) bytes.
func (z *inflater) inflate(r *byteReader, out []byte) error {
	zr, err := z.reset(r)
	if err != nil {
		return err
	}
	if _, err := io.ReadFull(zr, out); err != nil {
		return corrupt("zlib stream: expected %d bytes: %v", len(out), err)
	}
	return z.finish(int64(len(out)))
}

// skip consumes one zlib stream starting at r's position, which must expand
// to exactly size bytes, without keeping the output.
func (z *inflater) skip(r *byteReader, size int64) error {
	zr, err := z.reset(r)
	if err != nil {
		return err
	}
	n, err := io.CopyN(io.Discard, zr, size)
	if n != size {
		if err == io.EOF {
			return corrupt("zlib stream: expected %d bytes but got %d", size, n)
		}
		return corrupt("zlib stream: %v", err)
	}
	return z.finish(size)
}
