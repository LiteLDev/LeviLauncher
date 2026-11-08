package doc

import (
	"math"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// The text decoding of .NET's StreamReader and the number formatting of
// System.Text.Json's Utf8JsonWriter, shared by the TOML and JSON file readers.

// decodeText decodes bytes as new StreamReader(stream, strictUtf8,
// detectEncodingFromByteOrderMarks: true).ReadToEnd() does, into UTF-16;
// the error is the DecoderFallbackException it throws for text without a
// byte order mark that is not UTF-8.
func decodeText(b []byte) ([]uint16, *DecoderError) {
	switch {
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return decodeUTF16(b[2:], true), nil
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		if len(b) < 4 || b[2] != 0 || b[3] != 0 {
			return decodeUTF16(b[2:], false), nil
		}
		return decodeUTF32(b[4:], false), nil
	case len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF:
		// Encoding.UTF8, which replaces invalid bytes.
		return decodeUTF8Replacing(b[3:]), nil
	case len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 0xFE && b[3] == 0xFF:
		return decodeUTF32(b[4:], true), nil
	}
	if err := streamReaderUTF8Error(b); err != nil {
		return nil, err
	}
	return utf16.Encode([]rune(string(b))), nil
}

// decodeUTF16 decodes as Encoding.Unicode / BigEndianUnicode: unpaired
// surrogates and a final odd byte become U+FFFD.
func decodeUTF16(b []byte, bigEndian bool) []uint16 {
	units := make([]uint16, 0, len(b)/2+1)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			units = append(units, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	out := make([]uint16, 0, len(units)+1)
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
				out = append(out, u, units[i+1])
				i++
			} else {
				out = append(out, 0xFFFD)
			}
		case u >= 0xDC00 && u <= 0xDFFF:
			out = append(out, 0xFFFD)
		default:
			out = append(out, u)
		}
	}
	if len(b)%2 != 0 {
		out = append(out, 0xFFFD)
	}
	return out
}

// decodeUTF32 decodes as Encoding.UTF32 / UTF32Encoding(bigEndian): invalid
// code points and trailing bytes become U+FFFD.
func decodeUTF32(b []byte, bigEndian bool) []uint16 {
	var out []uint16
	i := 0
	for ; i+3 < len(b); i += 4 {
		var c uint32
		if bigEndian {
			c = uint32(b[i])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3])
		} else {
			c = uint32(b[i+3])<<24 | uint32(b[i+2])<<16 | uint32(b[i+1])<<8 | uint32(b[i])
		}
		if c > 0x10FFFF || (c >= 0xD800 && c <= 0xDFFF) {
			out = append(out, 0xFFFD)
		} else {
			out = utf16.AppendRune(out, rune(c))
		}
	}
	if i < len(b) {
		out = append(out, 0xFFFD)
	}
	return out
}

// decodeUTF8Replacing decodes bytes as .NET's Encoding.UTF8 (replacement
// fallback) does, into UTF-16: each maximal invalid subsequence becomes one
// U+FFFD.
func decodeUTF8Replacing(b []byte) []uint16 {
	var out []uint16
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n <= 1 {
			n = invalidUTF8Length(b)
		}
		out = utf16.AppendRune(out, r)
		b = b[n:]
	}
	return out
}

// invalidUTF8Length is the length of the maximal subpart of an ill-formed
// UTF-8 sequence at the start of b (Unicode 3.9, "U+FFFD substitution of
// maximal subparts"), at least 1.
func invalidUTF8Length(b []byte) int {
	c := b[0]
	var need int
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
		return 1
	}
	n := 1
	for i := 0; i < need && n < len(b); i++ {
		if b[n] < lo || b[n] > hi {
			break
		}
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}

// formatDouble formats a finite double as Utf8JsonWriter.WriteNumberValue
// (double.TryFormat with the default format) does: the shortest text that
// round-trips, in fixed notation when the decimal exponent is from -4 to 16
// (0.0001, 10000000000000000), else as d.dddE+XX (1E-05, 1E+17).
func formatDouble(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // -d.ddde±XX
	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}
	ePos := 0
	for i := range len(s) {
		if s[i] == 'e' {
			ePos = i
			break
		}
	}
	mant, expText := s[:ePos], s[ePos+1:]
	exp, _ := strconv.Atoi(expText)
	digits := mant[:1]
	if len(mant) > 2 {
		digits += mant[2:]
	}
	scale := exp + 1 // value = 0.digits × 10^scale
	var out []byte
	if neg {
		out = append(out, '-')
	}
	if scale > 17 || scale < -3 {
		out = append(out, digits[0])
		if len(digits) > 1 {
			out = append(out, '.')
			out = append(out, digits[1:]...)
		}
		out = append(out, 'E')
		e := scale - 1
		if e < 0 {
			out = append(out, '-')
			e = -e
		} else {
			out = append(out, '+')
		}
		if e < 10 {
			out = append(out, '0')
		}
		out = strconv.AppendInt(out, int64(e), 10)
		return string(out)
	}
	if scale <= 0 {
		out = append(out, '0', '.')
		for range -scale {
			out = append(out, '0')
		}
		out = append(out, digits...)
		return string(out)
	}
	if len(digits) <= scale {
		out = append(out, digits...)
		for range scale - len(digits) {
			out = append(out, '0')
		}
		return string(out)
	}
	out = append(out, digits[:scale]...)
	out = append(out, '.')
	out = append(out, digits[scale:]...)
	return string(out)
}
