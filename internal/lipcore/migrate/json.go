package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// maxDepth mirrors JsonDocumentOptions.MaxDepth's default.
const maxDepth = 64

type jkind uint8

const (
	jNull jkind = iota
	jBool
	jNumber
	jString
	jArray
	jObject
)

func (k jkind) String() string {
	switch k {
	case jNull:
		return "Null"
	case jBool:
		return "Boolean"
	case jNumber:
		return "Number"
	case jString:
		return "String"
	case jArray:
		return "Array"
	default:
		return "Object"
	}
}

// jvalue is an order-preserving JSON tree. Numbers keep their literal text so
// documents can be re-emitted without reformatting.
type jvalue struct {
	kind jkind
	b    bool
	str  string // string value, or number literal
	arr  []*jvalue
	obj  []jmember // document order, duplicates kept
}

type jmember struct {
	key string
	val *jvalue
}

// get returns the last member named key (System.Text.Json's behaviour for
// duplicate properties when binding to a type or calling GetProperty).
func (v *jvalue) get(key string) (*jvalue, bool) {
	for i := len(v.obj) - 1; i >= 0; i-- {
		if v.obj[i].key == key {
			return v.obj[i].val, true
		}
	}
	return nil, false
}

func (v *jvalue) hasDuplicateKeys() (string, bool) {
	seen := make(map[string]struct{}, len(v.obj))
	for _, m := range v.obj {
		if _, ok := seen[m.key]; ok {
			return m.key, true
		}
		seen[m.key] = struct{}{}
	}
	return "", false
}

// parseJSON parses a complete JSON document like JsonDocument.Parse with
// default options: strict RFC 8259 (no comments or trailing commas), valid
// UTF-8 and at most maxDepth nesting levels. A leading UTF-8 BOM is skipped,
// as JsonDocument.ParseAsync(Stream) does.
func parseJSON(data []byte) (*jvalue, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return nil, errors.New("invalid UTF-8 in JSON document")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := readJSONValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected data after JSON document")
		}
		return nil, err
	}
	if err := checkSurrogateEscapes(data); err != nil {
		return nil, err
	}
	return v, nil
}

// checkSurrogateEscapes rejects \u escapes that encode a lone UTF-16
// surrogate. encoding/json silently turns them into U+FFFD, whereas
// System.Text.Json fails when such a string is read. data must already be
// valid JSON.
func checkSurrogateEscapes(data []byte) error {
	hexAt := func(i int) (rune, bool) {
		if i+4 > len(data) {
			return 0, false
		}
		var r rune
		for _, c := range data[i : i+4] {
			switch {
			case c >= '0' && c <= '9':
				r = r<<4 | rune(c-'0')
			case c >= 'a' && c <= 'f':
				r = r<<4 | rune(c-'a'+10)
			case c >= 'A' && c <= 'F':
				r = r<<4 | rune(c-'A'+10)
			default:
				return 0, false
			}
		}
		return r, true
	}
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if !inString {
			inString = c == '"'
			continue
		}
		switch c {
		case '"':
			inString = false
		case '\\':
			if i+1 < len(data) && data[i+1] == 'u' {
				r, _ := hexAt(i + 2)
				switch {
				case r >= 0xd800 && r < 0xdc00:
					lo, ok := rune(0), i+7 < len(data) && data[i+6] == '\\' && data[i+7] == 'u'
					if ok {
						lo, ok = hexAt(i + 8)
					}
					if !ok || lo < 0xdc00 || lo > 0xdfff {
						return errors.New("JSON string contains an unpaired UTF-16 surrogate")
					}
					i += 11
				case r >= 0xdc00 && r <= 0xdfff:
					return errors.New("JSON string contains an unpaired UTF-16 surrogate")
				default:
					i += 5
				}
			} else {
				i++
			}
		}
	}
	return nil
}

func readJSONValue(dec *json.Decoder, depth int) (*jvalue, error) {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	switch t := tok.(type) {
	case nil:
		return &jvalue{kind: jNull}, nil
	case bool:
		return &jvalue{kind: jBool, b: t}, nil
	case json.Number:
		return &jvalue{kind: jNumber, str: t.String()}, nil
	case string:
		return &jvalue{kind: jString, str: t}, nil
	case json.Delim:
		if depth+1 > maxDepth {
			return nil, fmt.Errorf("JSON document exceeds the maximum depth of %d", maxDepth)
		}
		switch t {
		case '[':
			v := &jvalue{kind: jArray, arr: []*jvalue{}}
			for dec.More() {
				e, err := readJSONValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				v.arr = append(v.arr, e)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return v, nil
		case '{':
			v := &jvalue{kind: jObject, obj: []jmember{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected object key %v", kt)
				}
				e, err := readJSONValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				v.obj = append(v.obj, jmember{key: key, val: e})
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}

// encodeJSON writes v compactly, preserving member order and number literals.
func encodeJSON(buf *bytes.Buffer, v *jvalue) {
	switch v.kind {
	case jNull:
		buf.WriteString("null")
	case jBool:
		if v.b {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case jNumber:
		buf.WriteString(v.str)
	case jString:
		writeJSONString(buf, v.str)
	case jArray:
		buf.WriteByte('[')
		for i, e := range v.arr {
			if i > 0 {
				buf.WriteByte(',')
			}
			encodeJSON(buf, e)
		}
		buf.WriteByte(']')
	case jObject:
		buf.WriteByte('{')
		for i, m := range v.obj {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(buf, m.key)
			buf.WriteByte(':')
			encodeJSON(buf, m.val)
		}
		buf.WriteByte('}')
	}
}

func writeJSONString(buf *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			buf.WriteByte('\\')
			buf.WriteByte(c)
		case c == '\n':
			buf.WriteString(`\n`)
		case c == '\r':
			buf.WriteString(`\r`)
		case c == '\t':
			buf.WriteString(`\t`)
		case c < 0x20:
			buf.WriteString(`\u00`)
			buf.WriteByte(hex[c>>4])
			buf.WriteByte(hex[c&0xf])
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteByte('"')
}

// Builders for documents produced by the v1/v2 migration.

func jstr(s string) *jvalue { return &jvalue{kind: jString, str: s} }

// jnull is a JSON null.
func jnull() *jvalue { return &jvalue{kind: jNull} }

// jstrp encodes a nullable string.
func jstrp(s *string) *jvalue {
	if s == nil {
		return &jvalue{kind: jNull}
	}
	return jstr(*s)
}

func jnum(n int) *jvalue { return &jvalue{kind: jNumber, str: fmt.Sprint(n)} }

func jstrs(ss []string) *jvalue {
	v := &jvalue{kind: jArray, arr: make([]*jvalue, 0, len(ss))}
	for _, s := range ss {
		v.arr = append(v.arr, jstr(s))
	}
	return v
}

func jarr(items ...*jvalue) *jvalue {
	if items == nil {
		items = []*jvalue{}
	}
	return &jvalue{kind: jArray, arr: items}
}

type jobj struct{ v *jvalue }

func newObj() jobj { return jobj{&jvalue{kind: jObject, obj: []jmember{}}} }

func (o jobj) set(key string, val *jvalue) jobj {
	o.v.obj = append(o.v.obj, jmember{key: key, val: val})
	return o
}
