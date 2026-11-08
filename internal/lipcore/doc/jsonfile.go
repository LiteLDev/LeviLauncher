package doc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf16"
)

// JSONFileError is the error ParseJSONFile returns: the JsonException of
// lip's JsonFile.Parse (src/Lip.Core/Json/JsonFile.cs).
type JSONFileError struct {
	Message string
	Err     error
}

func (e *JSONFileError) Error() string { return e.Message }
func (e *JSONFileError) Unwrap() error { return e.Err }

// ParseJSONFile reads a JSON file as lip's JsonFile.Parse does
// (protocol/README.md "JSON file"): the bytes are decoded as .NET's
// StreamReader does with a strict UTF-8 encoding and byte order mark
// detection (decodeText), then read as strict JSON (JsonDocument.Parse with
// AllowDuplicateProperties = false: no comments, trailing commas or
// duplicate member names, at most 64 levels of nesting). Errors are
// *JSONFileError.
func ParseJSONFile(data []byte) (any, error) {
	text, derr := decodeText(data)
	if derr != nil {
		return nil, &JSONFileError{Message: "The file is not valid UTF-8 text.", Err: derr}
	}
	p := jsonParser{data: []byte(string(utf16.Decode(text)))}
	p.skipWS()
	v, err := p.value(0)
	if err == nil {
		p.skipWS()
		if p.pos != len(p.data) {
			err = p.errorf("unexpected data after the value")
		}
	}
	if err != nil {
		return nil, &JSONFileError{Message: "The file is not valid JSON: " + err.Error() + ".", Err: err}
	}
	return v, nil
}

// MarshalJSONFile writes a JSON value as lip writes JSON files
// (JsonFile.SerializeToUtf8Bytes): indented by two spaces, lines ended by
// "\n", members in order, empty containers as {} and [], strings escaped by
// JavaScriptEncoder.UnsafeRelaxedJsonEscaping, numbers as their JSON text,
// and a line break at the end; UTF-8 without a byte order mark.
func MarshalJSONFile(v any) []byte {
	b := appendJSONFile(nil, v, 0)
	return append(b, '\n')
}

func appendJSONFile(b []byte, v any, level int) []byte {
	switch t := v.(type) {
	case nil:
		return append(b, "null"...)
	case bool:
		return strconv.AppendBool(b, t)
	case json.Number:
		return append(b, t...)
	case string:
		return appendRelaxedString(b, t)
	case []any:
		b = append(b, '[')
		for i, e := range t {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendIndent(b, "\n", level+1)
			b = appendJSONFile(b, e, level+1)
		}
		if len(t) > 0 {
			b = appendIndent(b, "\n", level)
		}
		return append(b, ']')
	case *Object:
		b = append(b, '{')
		for i, k := range t.Keys() {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendIndent(b, "\n", level+1)
			b = appendRelaxedString(b, k)
			b = append(b, ':', ' ')
			b = appendJSONFile(b, t.vals[i], level+1)
		}
		if t.Len() > 0 {
			b = appendIndent(b, "\n", level)
		}
		return append(b, '}')
	default:
		panic(fmt.Sprintf("doc: %T is not a JSON value", v))
	}
}

// relaxedEscapes reports whether UnsafeRelaxedJsonEscaping escapes c.
func relaxedEscapes(c rune) bool {
	if c > 0xFFFF {
		return true
	}
	u := uint16(c)
	i := sort.Search(len(relaxedEscapedBMP), func(i int) bool { return relaxedEscapedBMP[i][1] >= u })
	return i < len(relaxedEscapedBMP) && relaxedEscapedBMP[i][0] <= u
}

func appendUEscape(b []byte, u uint16) []byte {
	const hex = "0123456789ABCDEF"
	return append(b, '\\', 'u', hex[u>>12], hex[u>>8&0xF], hex[u>>4&0xF], hex[u&0xF])
}

// appendRelaxedString writes s as a JSON string as Utf8JsonWriter does with
// JavaScriptEncoder.UnsafeRelaxedJsonEscaping.
func appendRelaxedString(b []byte, s string) []byte {
	b = append(b, '"')
	for _, c := range s {
		if !relaxedEscapes(c) {
			b = append(b, string(c)...)
			continue
		}
		switch c {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\b':
			b = append(b, '\\', 'b')
		case '\f':
			b = append(b, '\\', 'f')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if c > 0xFFFF {
				r1, r2 := utf16.EncodeRune(c)
				b = appendUEscape(b, uint16(r1))
				b = appendUEscape(b, uint16(r2))
			} else {
				b = appendUEscape(b, uint16(c))
			}
		}
	}
	return append(b, '"')
}
