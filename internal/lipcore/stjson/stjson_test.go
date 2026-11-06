package stjson

import (
	"strings"
	"testing"
)

func setNewLine(t *testing.T, nl string) {
	t.Helper()
	old := NewLine
	NewLine = nl
	t.Cleanup(func() { NewLine = old })
}

// Expected values were produced with System.Text.Json on .NET 10
// (JsonSerializer.Serialize(string) with the default encoder).
func TestEscapeString(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", `""`},
		{"plain ASCII text 123 /:;=?@[]^_{|}~!#$%()*,-.", `"plain ASCII text 123 /:;=?@[]^_{|}~!#$%()*,-."`},
		{`a"b`, `"a\u0022b"`},
		{`a\b`, `"a\\b"`},
		{"&&", `"` + `\u` + "0026" + `\u` + "0026" + `"`},
		{"<>'+`", `"` + `\u` + "003C" + `\u` + "003E" + `\u` + "0027" + `\u` + "002B" + `\u` + "0060" + `"`},
		{"\x00\x01\x1f\x7f", `"\u0000\u0001\u001F\u007F"`},
		{"\b\t\n\f\r\x0b", `"\b\t\n\f\r\u000B"`},
		{"\u00e9", `"\u00E9"`},
		{"\u0080\u00a0", `"\u0080\u00A0"`},
		{"\u4e2d\u6587", `"\u4E2D\u6587"`},
		{"\u2028\ufeff\ufffd\uffff", `"\u2028\uFEFF\uFFFD\uFFFF"`},
		{"\U0001F600", `"\uD83D\uDE00"`},
		{"a\xffb", `"a\uFFFDb"`},
		{"\xed\xa0\x80", `"\uFFFD\uFFFD\uFFFD"`},
	}
	for _, tt := range tests {
		if got := EscapeString(tt.in); got != tt.want {
			t.Errorf("EscapeString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestWriterIndented(t *testing.T) {
	setNewLine(t, "\r\n")
	var w Writer
	w.BeginObject()
	w.Key("a&b\u00e9")
	w.BeginArray()
	w.EndArray()
	w.Key("o")
	w.BeginObject()
	w.EndObject()
	w.Key("n")
	w.BeginArray()
	w.Int(1)
	w.EndArray()
	w.Key("x")
	w.Null()
	w.Key("neg")
	w.Int(-5)
	w.Key("b")
	w.Bool(true)
	w.EndObject()
	// Output of JsonSerializer.Serialize(dictionary, WriteIndented=true).
	want := "{\r\n" +
		"  \"a\\u0026b\\u00E9\": [],\r\n" +
		"  \"o\": {},\r\n" +
		"  \"n\": [\r\n" +
		"    1\r\n" +
		"  ],\r\n" +
		"  \"x\": null,\r\n" +
		"  \"neg\": -5,\r\n" +
		"  \"b\": true\r\n" +
		"}"
	if got := string(w.Bytes()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWriterNestedArrays(t *testing.T) {
	setNewLine(t, "\n")
	got, err := Marshal([]any{[]int{}, map[string]int{}, []int{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	want := "[\n  [],\n  {},\n  [\n    1,\n    2\n  ]\n]"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWriterNewLineOverride(t *testing.T) {
	setNewLine(t, "\n")
	w := Writer{NewLine: "\r\n"}
	w.BeginArray()
	w.String("x")
	w.EndArray()
	if got := string(w.Bytes()); got != "[\r\n  \"x\"\r\n]" {
		t.Errorf("got %q", got)
	}
}

type ordered []string

func (o ordered) MarshalSTJSON(w *Writer) error {
	w.BeginObject()
	for _, k := range o {
		w.Key(k)
		w.String(strings.ToUpper(k))
	}
	w.EndObject()
	return nil
}

type ptrMarshaler struct{ n int }

func (p *ptrMarshaler) MarshalSTJSON(w *Writer) error {
	w.Int(int64(p.n * 10))
	return nil
}

type sample struct {
	Name     string            `json:"name"`
	Skip     string            `json:"-"`
	List     []string          `json:"list"`
	Nullable []string          `json:"nullable" stjson:"nullable"`
	Map      map[string]int    `json:"map"`
	Order    ordered           `json:"order"`
	Ptr      *int              `json:"ptr"`
	PM       ptrMarshaler      `json:"pm"`
	Untagged bool              //nolint
	unexp    int               //nolint:unused
	Empty    map[string]string `json:"empty,omitempty"`
}

func TestMarshalStruct(t *testing.T) {
	setNewLine(t, "\n")
	got, err := Marshal(sample{
		Name:  "n",
		Skip:  "skipped",
		Map:   map[string]int{"b": 2, "a": 1},
		Order: ordered{"z", "a"},
		PM:    ptrMarshaler{n: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "name": "n",
  "list": [],
  "nullable": null,
  "map": {
    "a": 1,
    "b": 2
  },
  "order": {
    "z": "Z",
    "a": "A"
  },
  "ptr": null,
  "pm": 40,
  "Untagged": false,
  "empty": {}
}`
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMarshalScalarsAndErrors(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{"s", `"s"`},
		{uint8(7), "7"},
		{int64(-1), "-1"},
		{false, "false"},
		{[0]int{}, "[]"},
	}
	for _, tt := range tests {
		got, err := Marshal(tt.in)
		if err != nil || string(got) != tt.want {
			t.Errorf("Marshal(%#v) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	// Pointer-receiver Marshalers also apply to non-addressable values.
	got, err := Marshal(map[string]ptrMarshaler{"k": {n: 2}})
	if err != nil || string(got) != "{"+NewLine+"  \"k\": 20"+NewLine+"}" {
		t.Errorf("map of pointer-receiver Marshalers = %q, %v", got, err)
	}
	for _, bad := range []any{1.5, map[int]int{1: 1}, make(chan int)} {
		if _, err := Marshal(bad); err == nil {
			t.Errorf("Marshal(%T) succeeded, want error", bad)
		}
	}
}
