// Package stjson writes JSON byte-for-byte like .NET System.Text.Json with
// JsonSerializerOptions{WriteIndented = true} and the default encoder, which
// is how lip 0.37.0 writes tooth_lock.json.
//
// Output rules: 2-space indentation; "\r\n" line endings on Windows ("\n"
// elsewhere, see NewLine); `"key": value` with one space after the colon;
// empty arrays/objects written as "[]" / "{}"; no trailing newline; no BOM;
// the default JavaScriptEncoder escapes '"', '\\', control characters,
// '&', '<', '>', apostrophe, '+', backtick, DEL and every non-ASCII character
// as \uXXXX (uppercase hex, surrogate pairs for non-BMP; '\\' is written as
// "\\\\" and \b \t \n \f \r use their short forms; invalid UTF-8 becomes
// \uFFFD); keys are written in the order provided.
package stjson

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ErrNotImplemented is returned by skeleton stubs.
//
// Deprecated: kept for API compatibility; nothing returns it any more.
var ErrNotImplemented = errors.New("stjson: not implemented")

// NewLine is the line terminator used by Marshal (Environment.NewLine).
// Writer.NewLine overrides it per writer.
var NewLine = func() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}()

// Marshal encodes v. Structs are encoded field by field in declaration order
// using `json:"name"` tags ("-" skips; omitempty is NOT supported because
// System.Text.Json emits defaults); types implementing Marshaler control
// their own encoding; maps are encoded with sorted keys (callers needing
// insertion order must use an ordered type implementing Marshaler); nil
// slices are encoded as "[]" unless the field is tagged `stjson:"nullable"`,
// in which case nil encodes as null.
func Marshal(v any) ([]byte, error) {
	var w Writer
	if err := w.Value(v); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// Marshaler lets a type write itself with a Writer (for ordered maps etc.).
type Marshaler interface {
	MarshalSTJSON(w *Writer) error
}

type frame struct {
	count int
}

// Writer is a low-level indented JSON writer with System.Text.Json
// formatting. Its methods track nesting and insert separators/indentation.
// The zero value is ready to use.
type Writer struct {
	buf []byte
	// stack holds one frame per open object/array.
	stack []frame
	// afterKey is set after Key until the property value starts.
	afterKey bool
	// NewLine overrides the package-level NewLine when non-empty.
	NewLine string
}

func (w *Writer) newline() {
	nl := w.NewLine
	if nl == "" {
		nl = NewLine
	}
	w.buf = append(w.buf, nl...)
	for range w.stack {
		w.buf = append(w.buf, ' ', ' ')
	}
}

// beforeValue writes the separator and indentation preceding a value.
func (w *Writer) beforeValue() {
	if w.afterKey {
		w.afterKey = false
		return
	}
	if len(w.stack) == 0 {
		return
	}
	top := &w.stack[len(w.stack)-1]
	if top.count > 0 {
		w.buf = append(w.buf, ',')
	}
	top.count++
	w.newline()
}

func (w *Writer) begin(c byte) {
	w.beforeValue()
	w.buf = append(w.buf, c)
	w.stack = append(w.stack, frame{})
}

func (w *Writer) end(c byte) {
	if len(w.stack) == 0 {
		panic("stjson: unbalanced end")
	}
	n := w.stack[len(w.stack)-1].count
	w.stack = w.stack[:len(w.stack)-1]
	if n > 0 {
		w.newline()
	}
	w.buf = append(w.buf, c)
}

func (w *Writer) BeginObject() { w.begin('{') }
func (w *Writer) EndObject()   { w.end('}') }
func (w *Writer) BeginArray()  { w.begin('[') }
func (w *Writer) EndArray()    { w.end(']') }

// Key writes a property name; the next call must write its value.
func (w *Writer) Key(name string) {
	w.beforeValue()
	w.buf = appendEscaped(w.buf, name)
	w.buf = append(w.buf, ':', ' ')
	w.afterKey = true
}

func (w *Writer) String(s string) {
	w.beforeValue()
	w.buf = appendEscaped(w.buf, s)
}

func (w *Writer) Int(n int64) {
	w.beforeValue()
	w.buf = strconv.AppendInt(w.buf, n, 10)
}

func (w *Writer) Uint(n uint64) {
	w.beforeValue()
	w.buf = strconv.AppendUint(w.buf, n, 10)
}

func (w *Writer) Bool(b bool) {
	w.beforeValue()
	w.buf = strconv.AppendBool(w.buf, b)
}

func (w *Writer) Null() {
	w.beforeValue()
	w.buf = append(w.buf, "null"...)
}

// Value encodes v with the rules described on Marshal.
func (w *Writer) Value(v any) error {
	if v == nil {
		w.Null()
		return nil
	}
	return w.value(reflect.ValueOf(v), false)
}

func (w *Writer) Bytes() []byte { return w.buf }

var marshalerType = reflect.TypeFor[Marshaler]()

func (w *Writer) value(v reflect.Value, nullable bool) error {
	if !v.IsValid() {
		w.Null()
		return nil
	}
	if v.Type().Implements(marshalerType) {
		if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
			w.Null()
			return nil
		}
		return v.Interface().(Marshaler).MarshalSTJSON(w)
	}
	if v.Kind() != reflect.Pointer && reflect.PointerTo(v.Type()).Implements(marshalerType) {
		if !v.CanAddr() {
			p := reflect.New(v.Type())
			p.Elem().Set(v)
			v = p.Elem()
		}
		return v.Addr().Interface().(Marshaler).MarshalSTJSON(w)
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			w.Null()
			return nil
		}
		return w.value(v.Elem(), false)
	case reflect.String:
		w.String(v.String())
	case reflect.Bool:
		w.Bool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.Int(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.Uint(v.Uint())
	case reflect.Slice:
		if v.IsNil() && nullable {
			w.Null()
			return nil
		}
		fallthrough
	case reflect.Array:
		w.BeginArray()
		for i := range v.Len() {
			if err := w.value(v.Index(i), false); err != nil {
				return err
			}
		}
		w.EndArray()
	case reflect.Map:
		if v.IsNil() && nullable {
			w.Null()
			return nil
		}
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("stjson: unsupported map key type %s", v.Type().Key())
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		w.BeginObject()
		for _, k := range keys {
			w.Key(k.String())
			if err := w.value(v.MapIndex(k), false); err != nil {
				return err
			}
		}
		w.EndObject()
	case reflect.Struct:
		t := v.Type()
		w.BeginObject()
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			w.Key(name)
			if err := w.value(v.Field(i), f.Tag.Get("stjson") == "nullable"); err != nil {
				return err
			}
		}
		w.EndObject()
	default:
		return fmt.Errorf("stjson: unsupported type %s", v.Type())
	}
	return nil
}

// EscapeString returns s quoted and escaped like the default
// System.Text.Json encoder.
func EscapeString(s string) string {
	return string(appendEscaped(nil, s))
}

const hexDigits = "0123456789ABCDEF"

func appendU(b []byte, r uint16) []byte {
	return append(b, '\\', 'u', hexDigits[r>>12&0xF], hexDigits[r>>8&0xF], hexDigits[r>>4&0xF], hexDigits[r&0xF])
}

func appendEscaped(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			switch c {
			case '\\':
				b = append(b, '\\', '\\')
			case '\b':
				b = append(b, '\\', 'b')
			case '\t':
				b = append(b, '\\', 't')
			case '\n':
				b = append(b, '\\', 'n')
			case '\f':
				b = append(b, '\\', 'f')
			case '\r':
				b = append(b, '\\', 'r')
			case '"', '&', '\'', '+', '<', '>', '`', 0x7F:
				b = appendU(b, uint16(c))
			default:
				if c < 0x20 {
					b = appendU(b, uint16(c))
				} else {
					b = append(b, c)
				}
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		// utf8 decoding maps invalid sequences and surrogate code points to
		// U+FFFD, matching how STJ writes lone UTF-16 surrogates.
		if r >= 0x10000 {
			r1, r2 := utf16.EncodeRune(r)
			b = appendU(b, uint16(r1))
			b = appendU(b, uint16(r2))
		} else {
			b = appendU(b, uint16(r))
		}
	}
	return append(b, '"')
}
