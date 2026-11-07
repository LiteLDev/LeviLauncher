package gitproto

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

// PktLineType is the kind of a pkt-line read by PktLineReader.
type PktLineType int

const (
	// PktData is a data line; its payload is in PktLineReader.Payload.
	PktData PktLineType = iota
	// PktFlush is "0000".
	PktFlush
	// PktDelimiter is "0001".
	PktDelimiter
	// PktResponseEnd is "0002".
	PktResponseEnd
	// PktEndOfStream means the stream ended cleanly before a header.
	PktEndOfStream
)

func (t PktLineType) String() string {
	switch t {
	case PktData:
		return "Data"
	case PktFlush:
		return "Flush"
	case PktDelimiter:
		return "Delimiter"
	case PktResponseEnd:
		return "ResponseEnd"
	case PktEndOfStream:
		return "EndOfStream"
	}
	return "PktLineType(" + strconv.Itoa(int(t)) + ")"
}

// MaxPktLineLength is the largest pkt-line length (header included).
const MaxPktLineLength = 65520

// MaxPktPayloadLength is the largest pkt-line payload.
const MaxPktPayloadLength = MaxPktLineLength - 4

// PktLineReader reads git pkt-lines. The payload of the last data line is
// valid until the next Read.
type PktLineReader struct {
	r       io.Reader
	header  [4]byte
	buf     []byte
	Payload []byte
}

// NewPktLineReader returns a reader over r.
func NewPktLineReader(r io.Reader) *PktLineReader {
	return &PktLineReader{r: r, buf: make([]byte, MaxPktLineLength)}
}

// PayloadText is the last data payload decoded as UTF-8, without the
// trailing LF.
func (p *PktLineReader) PayloadText() string {
	return gitpack.DecodeUTF8(bytes.TrimSuffix(p.Payload, []byte{'\n'}))
}

// Read reads the next pkt-line. A clean end of stream before a header
// returns PktEndOfStream; a truncated header or payload is a ProtocolError.
// Other read errors are returned as they are.
func (p *PktLineReader) Read() (PktLineType, error) {
	p.Payload = nil

	n, err := io.ReadFull(p.r, p.header[:])
	if err != nil {
		// Only a clean end of the underlying stream is a protocol error;
		// transport failures (wrapped by the caller) pass through.
		if n == 0 && err == io.EOF {
			return PktEndOfStream, nil
		}
		if err == io.ErrUnexpectedEOF {
			return 0, protocolErrorf("The response ended in the middle of a pkt-line header.")
		}
		return 0, err
	}

	length, ok := parseHex4(p.header[:])
	if !ok {
		return 0, protocolErrorf("Invalid pkt-line length '%s'; the response is not in git pkt-line format.", gitpack.ASCIIString(p.header[:]))
	}
	switch {
	case length == 0:
		return PktFlush, nil
	case length == 1:
		return PktDelimiter, nil
	case length == 2:
		return PktResponseEnd, nil
	case length == 3 || length > MaxPktLineLength:
		return 0, protocolErrorf("Invalid pkt-line length %d; lengths must be 0 to 2 or 4 to %d.", length, MaxPktLineLength)
	}

	payload := p.buf[:length-4]
	if _, err := io.ReadFull(p.r, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return 0, &ProtocolError{Message: "The response ended in the middle of a pkt-line payload.", Err: err}
		}
		return 0, err
	}
	p.Payload = payload
	return PktData, nil
}

// parseHex4 parses four hex digits (either case), like .NET
// int.TryParse(AllowHexSpecifier).
func parseHex4(b []byte) (int, bool) {
	v := 0
	for _, c := range b {
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}

// PktLineWriter builds a request body made of pkt-lines.
type PktLineWriter struct {
	buf bytes.Buffer
}

// WriteLine writes text followed by LF as a data line.
func (w *PktLineWriter) WriteLine(text string) *PktLineWriter {
	return w.WriteData([]byte(text + "\n"))
}

// WriteData writes a data line. It panics if data exceeds
// MaxPktPayloadLength, mirroring lip's ArgumentException.
func (w *PktLineWriter) WriteData(data []byte) *PktLineWriter {
	if len(data) > MaxPktPayloadLength {
		panic(fmt.Sprintf("A git request line of %d bytes is longer than the %d bytes git allows.", len(data), MaxPktPayloadLength))
	}
	fmt.Fprintf(&w.buf, "%04x", len(data)+4)
	w.buf.Write(data)
	return w
}

// WriteFlush writes "0000".
func (w *PktLineWriter) WriteFlush() *PktLineWriter {
	w.buf.WriteString("0000")
	return w
}

// WriteDelimiter writes "0001".
func (w *PktLineWriter) WriteDelimiter() *PktLineWriter {
	w.buf.WriteString("0001")
	return w
}

// Bytes returns the written bytes.
func (w *PktLineWriter) Bytes() []byte {
	return bytes.Clone(w.buf.Bytes())
}
