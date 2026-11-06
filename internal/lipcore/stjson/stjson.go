// Package stjson writes JSON byte-for-byte like .NET System.Text.Json with
// JsonSerializerOptions{WriteIndented = true} and the default encoder, which
// is how lip 0.37.0 writes tooth_lock.json.
//
// Output rules: 2-space indentation; "\r\n" line endings on Windows ("\n"
// elsewhere, see NewLine); `"key": value` with one space after the colon;
// empty arrays/objects written as "[]" / "{}"; no trailing newline; no BOM;
// the default JavaScriptEncoder escapes '"', '\\', control characters,
// '&', '<', '>', apostrophe, '+', backtick and every non-ASCII character as XXXX
// (uppercase hex, surrogate pairs for non-BMP); keys are written in the
// order provided.
package stjson

import (
	"errors"
	"runtime"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("stjson: not implemented")

// NewLine is the line terminator used by Marshal (Environment.NewLine).
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
	return nil, ErrNotImplemented
}

// Marshaler lets a type write itself with a Writer (for ordered maps etc.).
type Marshaler interface {
	MarshalSTJSON(w *Writer) error
}

// Writer is a low-level indented JSON writer with System.Text.Json
// formatting. Its methods track nesting and insert separators/indentation.
type Writer struct {
	buf []byte
	// Implementation fields may be added.
}

func (w *Writer) BeginObject()      { panic("stjson: not implemented") }
func (w *Writer) EndObject()        { panic("stjson: not implemented") }
func (w *Writer) BeginArray()       { panic("stjson: not implemented") }
func (w *Writer) EndArray()         { panic("stjson: not implemented") }
func (w *Writer) Key(name string)   { panic("stjson: not implemented") }
func (w *Writer) String(s string)   { panic("stjson: not implemented") }
func (w *Writer) Int(n int64)       { panic("stjson: not implemented") }
func (w *Writer) Bool(b bool)       { panic("stjson: not implemented") }
func (w *Writer) Null()             { panic("stjson: not implemented") }
func (w *Writer) Value(v any) error { return ErrNotImplemented }
func (w *Writer) Bytes() []byte     { return w.buf }

// EscapeString returns s quoted and escaped like the default
// System.Text.Json encoder.
func EscapeString(s string) string {
	panic("stjson: not implemented")
}
