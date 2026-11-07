package doc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
)

// JSONError is the error ParseJSON returns. Message is the message of the
// PackageFormatException lip throws, which names the file.
type JSONError struct {
	File    string
	Message string
	Err     error // the cause: a *DecoderError for text that is not UTF-8
}

func (e *JSONError) Error() string { return e.Message }
func (e *JSONError) Unwrap() error { return e.Err }

// maxJSONDepth is JsonDocumentOptions.MaxDepth's default (0 means 64).
const maxJSONDepth = 64

// ParseJSON parses strict JSON [RFC 8259] as PackageFileParser.ParseJson
// (src/Lip.Core/Mcmod/Parsing/PackageFileParser.cs) does: a UTF-8 byte order
// mark is skipped; the rest must be valid UTF-8 ("Invalid {file}: not valid
// UTF-8."); then JsonDocument.Parse with AllowDuplicateProperties = false,
// AllowTrailingCommas = false, CommentHandling = Disallow and the default
// maximum depth of 64, any failure being "Invalid {file}: not strict JSON (no
// comments, trailing commas or duplicate keys).".
//
// A \u escape of an unpaired surrogate in a string value, which JsonDocument
// accepts but cannot turn into a .NET string, reads as U+FFFD; in a property
// name it is an error, as ParseJson throws there.
func ParseJSON(data []byte, file string) (any, error) {
	data = trimUTF8BOM(data)
	if derr := utf8Error(data); derr != nil {
		return nil, &JSONError{File: file, Message: "Invalid " + file + ": not valid UTF-8.", Err: derr}
	}
	p := jsonParser{data: data}
	p.skipWS()
	v, err := p.value(0)
	if err == nil {
		p.skipWS()
		if p.pos != len(p.data) {
			err = p.errorf("unexpected data after the value")
		}
	}
	if err != nil {
		return nil, &JSONError{File: file,
			Message: "Invalid " + file + ": not strict JSON (no comments, trailing commas or duplicate keys).",
			Err:     err}
	}
	return v, nil
}

func trimUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

type jsonParser struct {
	data []byte
	pos  int
}

func (p *jsonParser) errorf(msg string) error {
	return errors.New(msg + " at byte " + strconv.Itoa(p.pos))
}

func (p *jsonParser) skipWS() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

// value parses a value; depth is the number of enclosing containers.
func (p *jsonParser) value(depth int) (any, error) {
	if p.pos >= len(p.data) {
		return nil, p.errorf("unexpected end of data")
	}
	switch c := p.data[p.pos]; {
	case c == '{':
		if depth >= maxJSONDepth {
			return nil, p.errorf("maximum depth exceeded")
		}
		p.pos++
		obj := NewObject()
		p.skipWS()
		if p.pos < len(p.data) && p.data[p.pos] == '}' {
			p.pos++
			return obj, nil
		}
		for {
			p.skipWS()
			if p.pos >= len(p.data) || p.data[p.pos] != '"' {
				return nil, p.errorf("expected a property name")
			}
			k, lone, err := p.str()
			if err != nil {
				return nil, err
			}
			if lone {
				// JsonDocument's duplicate check reads the name as a .NET
				// string, which throws (InvalidOperationException).
				return nil, p.errorf("unpaired surrogate escape in a property name")
			}
			if obj.Has(k) {
				return nil, p.errorf("duplicate property " + strconv.Quote(k))
			}
			p.skipWS()
			if p.pos >= len(p.data) || p.data[p.pos] != ':' {
				return nil, p.errorf("expected ':'")
			}
			p.pos++
			p.skipWS()
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			obj.Set(k, v)
			p.skipWS()
			if p.pos >= len(p.data) {
				return nil, p.errorf("unexpected end of data")
			}
			if p.data[p.pos] == ',' {
				p.pos++
				continue
			}
			if p.data[p.pos] == '}' {
				p.pos++
				return obj, nil
			}
			return nil, p.errorf("expected ',' or '}'")
		}
	case c == '[':
		if depth >= maxJSONDepth {
			return nil, p.errorf("maximum depth exceeded")
		}
		p.pos++
		arr := []any{}
		p.skipWS()
		if p.pos < len(p.data) && p.data[p.pos] == ']' {
			p.pos++
			return arr, nil
		}
		for {
			p.skipWS()
			v, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
			p.skipWS()
			if p.pos >= len(p.data) {
				return nil, p.errorf("unexpected end of data")
			}
			if p.data[p.pos] == ',' {
				p.pos++
				continue
			}
			if p.data[p.pos] == ']' {
				p.pos++
				return arr, nil
			}
			return nil, p.errorf("expected ',' or ']'")
		}
	case c == '"':
		s, _, err := p.str()
		return s, err
	case c == 't':
		return true, p.literal("true")
	case c == 'f':
		return false, p.literal("false")
	case c == 'n':
		return nil, p.literal("null")
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	default:
		return nil, p.errorf("unexpected character")
	}
}

func (p *jsonParser) literal(word string) error {
	if !strings.HasPrefix(string(p.data[p.pos:min(len(p.data), p.pos+len(word))]), word) {
		return p.errorf("invalid literal")
	}
	p.pos += len(word)
	return nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *jsonParser) number() (any, error) {
	start := p.pos
	d := p.data
	if d[p.pos] == '-' {
		p.pos++
	}
	if p.pos >= len(d) || !isDigit(d[p.pos]) {
		return nil, p.errorf("invalid number")
	}
	if d[p.pos] == '0' {
		p.pos++
	} else {
		for p.pos < len(d) && isDigit(d[p.pos]) {
			p.pos++
		}
	}
	if p.pos < len(d) && d[p.pos] == '.' {
		p.pos++
		if p.pos >= len(d) || !isDigit(d[p.pos]) {
			return nil, p.errorf("invalid number")
		}
		for p.pos < len(d) && isDigit(d[p.pos]) {
			p.pos++
		}
	}
	if p.pos < len(d) && (d[p.pos] == 'e' || d[p.pos] == 'E') {
		p.pos++
		if p.pos < len(d) && (d[p.pos] == '+' || d[p.pos] == '-') {
			p.pos++
		}
		if p.pos >= len(d) || !isDigit(d[p.pos]) {
			return nil, p.errorf("invalid number")
		}
		for p.pos < len(d) && isDigit(d[p.pos]) {
			p.pos++
		}
	}
	// A number must end at a delimiter.
	if p.pos < len(d) {
		switch d[p.pos] {
		case ' ', '\t', '\n', '\r', ',', ']', '}':
		default:
			return nil, p.errorf("invalid number")
		}
	}
	return json.Number(d[start:p.pos]), nil
}

func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

func (p *jsonParser) hex4() (rune, bool) {
	if p.pos+4 > len(p.data) {
		return 0, false
	}
	var r rune
	for i := 0; i < 4; i++ {
		h, ok := hexVal(p.data[p.pos+i])
		if !ok {
			return 0, false
		}
		r = r<<4 | rune(h)
	}
	p.pos += 4
	return r, true
}

// str parses a string starting at the opening quote. An unpaired surrogate
// escape reads as U+FFFD; lone reports whether there was one.
func (p *jsonParser) str() (s string, lone bool, err error) {
	p.pos++
	var b strings.Builder
	for {
		if p.pos >= len(p.data) {
			return "", false, p.errorf("unterminated string")
		}
		c := p.data[p.pos]
		switch {
		case c == '"':
			p.pos++
			return b.String(), lone, nil
		case c < 0x20:
			return "", false, p.errorf("control character in string")
		case c == '\\':
			p.pos++
			if p.pos >= len(p.data) {
				return "", false, p.errorf("unterminated string")
			}
			e := p.data[p.pos]
			p.pos++
			switch e {
			case '"', '\\', '/':
				b.WriteRune(rune(e))
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'u':
				r, ok := p.hex4()
				if !ok {
					return "", false, p.errorf("invalid \\u escape")
				}
				if !utf16.IsSurrogate(r) {
					b.WriteRune(r)
					continue
				}
				if r < 0xDC00 && p.pos+6 <= len(p.data) && p.data[p.pos] == '\\' && p.data[p.pos+1] == 'u' {
					save := p.pos
					p.pos += 2
					r2, ok := p.hex4()
					if !ok {
						return "", false, p.errorf("invalid \\u escape")
					}
					if dec := utf16.DecodeRune(r, r2); dec != utf8.RuneError {
						b.WriteRune(dec)
						continue
					}
					p.pos = save
				}
				b.WriteRune(utf8.RuneError)
				lone = true
			default:
				return "", false, p.errorf("invalid escape")
			}
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
}

// IntegerMember reports the value of a JSON number that is a whole number in
// the range of a 32-bit integer, as PackageFileParser.IntegerMember does
// (JsonElement.TryGetDouble, then value == Math.Floor(value) within
// int.MinValue..int.MaxValue): 4, 4.0 and 4e0 are all 4. Any other value
// reports false.
func IntegerMember(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		// Underflow reads as 0 or a subnormal in .NET; overflow is not finite.
		var ne *strconv.NumError
		if !errors.As(err, &ne) || ne.Err != strconv.ErrRange || math.IsInf(f, 0) {
			return 0, false
		}
	}
	if math.IsInf(f, 0) || math.IsNaN(f) || f != math.Floor(f) || f < math.MinInt32 || f > math.MaxInt32 {
		return 0, false
	}
	return int64(f), true
}

// MarshalJSONCompact writes v as System.Text.Json's JsonNode.ToJsonString()
// does with default options: no whitespace, members in order, strings with
// the default JavaScriptEncoder (non-ASCII, control characters and
// "&'+<>`\ escaped as \uXXXX or short escapes), numbers as their JSON text.
func MarshalJSONCompact(v any) []byte {
	return appendJSON(nil, v, "", 0)
}

// MarshalJSONIndented writes v as JsonNode.ToJsonString(new
// JsonSerializerOptions { WriteIndented = true }) does: two-space
// indentation, "key": value, empty containers as {} and [], and lines ended
// by Environment.NewLine (stjson.NewLine: "\r\n" on Windows), with no line
// break at the end.
func MarshalJSONIndented(v any) []byte {
	return appendJSON(nil, v, stjson.NewLine, 0)
}

func appendIndent(b []byte, nl string, level int) []byte {
	b = append(b, nl...)
	for range level {
		b = append(b, ' ', ' ')
	}
	return b
}

// appendJSON writes v; nl is "" for compact output.
func appendJSON(b []byte, v any, nl string, level int) []byte {
	switch t := v.(type) {
	case nil:
		return append(b, "null"...)
	case bool:
		return strconv.AppendBool(b, t)
	case json.Number:
		return append(b, t...)
	case string:
		return append(b, stjson.EscapeString(t)...)
	case []any:
		b = append(b, '[')
		for i, e := range t {
			if i > 0 {
				b = append(b, ',')
			}
			if nl != "" {
				b = appendIndent(b, nl, level+1)
			}
			b = appendJSON(b, e, nl, level+1)
		}
		if nl != "" && len(t) > 0 {
			b = appendIndent(b, nl, level)
		}
		return append(b, ']')
	case *Object:
		b = append(b, '{')
		for i, k := range t.Keys() {
			if i > 0 {
				b = append(b, ',')
			}
			if nl != "" {
				b = appendIndent(b, nl, level+1)
			}
			b = append(b, stjson.EscapeString(k)...)
			b = append(b, ':')
			if nl != "" {
				b = append(b, ' ')
			}
			b = appendJSON(b, t.vals[i], nl, level+1)
		}
		if nl != "" && t.Len() > 0 {
			b = appendIndent(b, nl, level)
		}
		return append(b, '}')
	default:
		panic(fmt.Sprintf("doc: %T is not a JSON value", v))
	}
}
