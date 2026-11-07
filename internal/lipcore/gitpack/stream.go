package gitpack

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
)

// StreamProgress is the state of a StreamScanner.
type StreamProgress struct {
	// Objects is the number of entries received completely so far: the
	// entry header, any delta base reference and the whole zlib stream.
	Objects int64
	// Total is the object count of the pack header; 0 until the header has
	// been received.
	Total int64
	// Err is set on the last update when the stream cannot be scanned (it
	// is corrupt, or ended early); no updates follow.
	Err error
}

// StreamScanner counts the entries of a packfile while its bytes are being
// received, so a fetch can report objects received out of the count in the
// pack header. Bytes written to it are scanned by a goroutine: each entry
// header is parsed (including OFS_DELTA and REF_DELTA base references) and
// its zlib stream inflated into nothing to find where the entry ends. The
// scanner validates far less than Open (no checksum, no delta base or size
// checks beyond the object size limit); it only measures progress. Once it
// fails it keeps accepting and discarding writes, so it never fails or
// blocks the writer for longer than the scan of the written bytes.
type StreamScanner struct {
	pw   *io.PipeWriter
	done chan struct{}
	err  error
}

// NewStreamScanner starts a scanner calling fn from its goroutine: once
// with Total set when the header has been read, then after every entry, and
// once with Err set if scanning fails. Calls are sequential, and all of them
// happen before Close returns.
func NewStreamScanner(fn func(StreamProgress)) *StreamScanner {
	pr, pw := io.Pipe()
	s := &StreamScanner{pw: pw, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		s.err = scanStream(bufio.NewReaderSize(pr, 64<<10), fn)
		if s.err != nil && fn != nil {
			fn(StreamProgress{Err: s.err})
		}
		// Keep accepting writes (the trailer, or everything after a
		// failure) until the writer closes the pipe.
		_, _ = io.Copy(io.Discard, pr)
	}()
	return s
}

// Write passes p to the scanner. It never fails.
func (s *StreamScanner) Write(p []byte) (int, error) {
	_, _ = s.pw.Write(p)
	return len(p), nil
}

// Close ends the stream, waits for the scanner and returns its error (nil
// when every entry of the header's count was received).
func (s *StreamScanner) Close() error {
	_ = s.pw.Close()
	<-s.done
	return s.err
}

// Abort ends the stream with err (e.g. the fetch failed) and waits for the
// scanner, so no callback runs after Abort returns.
func (s *StreamScanner) Abort(err error) {
	_ = s.pw.CloseWithError(err)
	<-s.done
}

// scanStream reads the pack header and the entries it announces from r.
func scanStream(r *bufio.Reader, fn func(StreamProgress)) error {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return corrupt("unexpected end")
	}
	if !bytes.Equal(header[:4], []byte("PACK")) {
		return corrupt("bad signature")
	}
	if v := binary.BigEndian.Uint32(header[4:8]); v != 2 && v != 3 {
		return corrupt("unsupported version %d", v)
	}
	total := int64(binary.BigEndian.Uint32(header[8:12]))
	if fn != nil {
		fn(StreamProgress{Total: total})
	}

	readByte := func() (int64, error) {
		b, err := r.ReadByte()
		if err != nil {
			return 0, corrupt("unexpected end")
		}
		return int64(b), nil
	}
	var zr io.ReadCloser
	for i := int64(0); i < total; i++ {
		c, err := readByte()
		if err != nil {
			return err
		}
		typ := int((c >> 4) & 7)
		size := c & 0x0f
		shift := 4
		for c&0x80 != 0 {
			if shift > 56 {
				return corrupt("object size too large")
			}
			if c, err = readByte(); err != nil {
				return err
			}
			size |= (c & 0x7f) << shift
			shift += 7
		}
		switch {
		case typ == objOfsDelta:
			for {
				if c, err = readByte(); err != nil {
					return err
				}
				if c&0x80 == 0 {
					break
				}
			}
		case typ == objRefDelta:
			if _, err := io.CopyN(io.Discard, r, 20); err != nil {
				return corrupt("unexpected end")
			}
		case typ < 1 || typ > 4:
			return corrupt("invalid object type %d", typ)
		}
		if size > MaxObjectSize {
			return corrupt("object of %d bytes exceeds the %d byte limit", size, MaxObjectSize)
		}

		if zr == nil {
			zr, err = zlib.NewReader(r)
		} else {
			err = zr.(zlib.Resetter).Reset(r, nil)
		}
		if err != nil {
			return corrupt("invalid zlib header: %v", err)
		}
		// Reading one byte past size makes the zlib reader reach its end
		// (consuming the Adler-32 trailer) when the size is right.
		n, err := io.Copy(io.Discard, io.LimitReader(zr, size+1))
		if err != nil {
			return corrupt("zlib stream: %v", err)
		}
		if n != size {
			return corrupt("zlib stream: expected %d bytes but got %d", size, n)
		}
		if fn != nil {
			fn(StreamProgress{Objects: i + 1, Total: total})
		}
	}
	return nil
}
