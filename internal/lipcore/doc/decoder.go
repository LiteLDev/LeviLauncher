package doc

import (
	"fmt"
	"strings"
)

// DecoderError is the DecoderFallbackException a strict UTF8Encoding
// (throwOnInvalidBytes: true) throws for bytes that are not UTF-8: lip
// keeps it as the inner exception of its own message, so its message is
// the "caused by" line lip shows.
type DecoderError struct {
	// Bytes is the ill-formed subsequence (its maximal subpart, as the
	// decoder reports it).
	Bytes []byte
	// Index is where the subsequence starts, relative to the start of the
	// buffer the decoder was decoding when it found the problem; it is
	// negative when the subsequence began in an earlier buffer.
	Index int
}

func (e *DecoderError) Error() string {
	var b strings.Builder
	for _, c := range e.Bytes {
		fmt.Fprintf(&b, "[%02X]", c)
	}
	return fmt.Sprintf("Unable to translate bytes %s at index %d from specified code page to Unicode.",
		b.String(), e.Index)
}

// streamReaderBufferSize is the byte buffer size of a StreamReader created
// without one (StreamReader.DefaultBufferSize).
const streamReaderBufferSize = 1024

// firstInvalidUTF8 finds the first ill-formed subsequence of b as .NET's
// UTF-8 decoder does: start and n delimit its maximal subpart (Unicode
// Table 3-7), and detect is the offset of the byte that shows it is ill
// formed: the byte itself for one that cannot start a sequence, the byte
// after the subpart, or len(b) when b ends inside a sequence (found when
// the decoder is flushed).
func firstInvalidUTF8(b []byte) (start, n, detect int, found bool) {
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			i++
			continue
		}
		need := 0
		lo, hi := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			need = 2
		case c == 0xED:
			need, hi = 2, 0x9F
		case c == 0xF0:
			need, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, hi = 3, 0x8F
		default:
			return i, 1, i, true
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) {
				return i, j - i, len(b), true
			}
			if b[j] < lo || b[j] > hi {
				return i, j - i, j, true
			}
			lo, hi = 0x80, 0xBF
			j++
		}
		i = j
	}
	return 0, 0, 0, false
}

// utf8Error is the DecoderFallbackException of decoding all of b in one
// call (UTF8Encoding.GetCharCount / GetString), or nil when b is UTF-8.
func utf8Error(b []byte) *DecoderError {
	start, n, _, found := firstInvalidUTF8(b)
	if !found {
		return nil
	}
	return &DecoderError{Bytes: b[start : start+n], Index: start}
}

// streamReaderUTF8Error is the DecoderFallbackException of reading b to the
// end with a StreamReader over a strict UTF8Encoding (b being what follows
// any byte order mark, which starts at the beginning of the first buffer),
// or nil when b is UTF-8. The reader decodes buffers of 1024 bytes and
// flushes the decoder at the end, so the index is relative to the buffer in
// which the subsequence was found to be ill formed, or to the end of b.
func streamReaderUTF8Error(b []byte) *DecoderError {
	start, n, detect, found := firstInvalidUTF8(b)
	if !found {
		return nil
	}
	base := len(b)
	if detect < len(b) {
		base = detect / streamReaderBufferSize * streamReaderBufferSize
	}
	return &DecoderError{Bytes: b[start : start+n], Index: start - base}
}
