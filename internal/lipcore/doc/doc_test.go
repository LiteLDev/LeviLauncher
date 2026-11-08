package doc

import (
	"encoding/json"
	"strings"
	"testing"
)

// PackageFileParser.ParseJson / IntegerMember.

func TestParseJSON(t *testing.T) {
	for _, tt := range []struct{ in, out, err string }{
		{`{"a":[1,2.50,"x"]}`, `{"a":[1,2.50,"x"]}`, ""},
		{"\xEF\xBB\xBF{}", `{}`, ""},
		{"{\"a\":\"\xC3\x28\"}", "", "Invalid x.json: not valid UTF-8."},
		{`{"a":1,"a":2}`, "", "Invalid x.json: not strict JSON (no comments, trailing commas or duplicate keys)."},
		{`{"a":1,}`, "", "Invalid x.json: not strict JSON (no comments, trailing commas or duplicate keys)."},
		{`{"a":1}//`, "", "Invalid x.json: not strict JSON (no comments, trailing commas or duplicate keys)."},
		{`[01]`, "", "Invalid x.json: not strict JSON (no comments, trailing commas or duplicate keys)."},
		{strings.Repeat("[", 65) + strings.Repeat("]", 65), "", "Invalid x.json: not strict JSON (no comments, trailing commas or duplicate keys)."},
		{`["<>&'+` + "`" + `é😀"]`, `["\u003C\u003E\u0026\u0027\u002B\u0060\u00E9\uD83D\uDE00"]`, ""},
	} {
		v, err := ParseJSON([]byte(tt.in), "x.json")
		if tt.err != "" {
			if err == nil || err.Error() != tt.err {
				t.Errorf("ParseJSON(%q) = %v, want %q", tt.in, err, tt.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseJSON(%q): %v", tt.in, err)
			continue
		}
		if got := string(MarshalJSONCompact(v)); got != tt.out {
			t.Errorf("ParseJSON(%q) = %s, want %s", tt.in, got, tt.out)
		}
	}
}

func TestIntegerMember(t *testing.T) {
	for _, tt := range []struct {
		v    any
		want int64
		ok   bool
	}{
		{json.Number("4"), 4, true}, {json.Number("3.0"), 3, true}, {json.Number("4e0"), 4, true},
		{json.Number("1"), 1, true}, {json.Number("2.5"), 0, false}, {"3", 0, false}, {nil, 0, false},
		{json.Number("2147483647"), 2147483647, true}, {json.Number("2147483648"), 0, false},
		{json.Number("-2147483648"), -2147483648, true}, {json.Number("1e400"), 0, false},
		{json.Number("1e-400"), 0, true},
	} {
		got, ok := IntegerMember(tt.v)
		if got != tt.want || ok != tt.ok {
			t.Errorf("IntegerMember(%#v) = %d, %v; want %d, %v", tt.v, got, ok, tt.want, tt.ok)
		}
	}
}

func TestMarshalJSON(t *testing.T) {
	o := NewObject()
	o.Set("b", []any{json.Number("1"), true, nil, NewObject(), []any{}})
	o.Set("a", "é")
	if got := string(MarshalJSONCompact(o)); got != `{"b":[1,true,null,{},[]],"a":"\u00E9"}` {
		t.Errorf("compact: %s", got)
	}
	got := strings.ReplaceAll(string(MarshalJSONIndented(o)), "\r\n", "\n")
	want := "{\n  \"b\": [\n    1,\n    true,\n    null,\n    {},\n    []\n  ],\n  \"a\": \"\\u00E9\"\n}"
	if got != want {
		t.Errorf("indented:\n%s\nwant:\n%s", got, want)
	}
}

func TestObject(t *testing.T) {
	o := NewObject()
	o.Set("z", json.Number("1"))
	o.Set("a", []any{"x"})
	o.Set("z", json.Number("2"))
	if k := o.Keys(); len(k) != 2 || k[0] != "z" || k[1] != "a" || o.Len() != 2 {
		t.Fatalf("keys %v", k)
	}
	c := o.Clone()
	c.Get("a")
	if a, _ := c.Get("a"); true {
		a.([]any)[0] = "y"
	}
	if a, _ := o.Get("a"); a.([]any)[0] != "x" {
		t.Error("Clone is not deep")
	}
	if Equal(o, c) {
		t.Error("Equal after change")
	}
	o2 := NewObject()
	o2.Set("a", []any{"y"})
	o2.Set("z", json.Number("2"))
	if !Equal(c, o2) {
		t.Error("Equal ignores member order")
	}
	if !o.Delete("z") || o.Has("z") || o.Keys()[0] != "a" {
		t.Error("Delete")
	}
	var zero Object
	zero.Set("k", nil)
	if v, ok := zero.Get("k"); !ok || v != nil {
		t.Error("zero Object")
	}
}
