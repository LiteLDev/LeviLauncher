package doc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

// Ports of tests/Lip.Core.Tests/Json/YamlJsonTests.cs (lip v0.38.0-beta.1).

func readYAML(t *testing.T, yaml string) string {
	t.Helper()
	v, err := ParseYAML([]byte(yaml), "x.yaml")
	if err != nil {
		t.Fatalf("ParseYAML(%q): %v", yaml, err)
	}
	return string(MarshalJSONCompact(v))
}

func compact(t *testing.T, j string) string {
	t.Helper()
	v, err := ParseJSON([]byte(j), "x.json")
	if err != nil {
		t.Fatalf("ParseJSON(%q): %v", j, err)
	}
	return string(MarshalJSONCompact(v))
}

func TestParse_PlainScalar_ResolvesByCoreSchema(t *testing.T) {
	for _, tt := range []struct{ yaml, json string }{
		{"a: null", `{"a":null}`}, {"a: Null", `{"a":null}`}, {"a: NULL", `{"a":null}`}, {"a: ~", `{"a":null}`},
		{"a:", `{"a":null}`}, {"a: true", `{"a":true}`}, {"a: True", `{"a":true}`}, {"a: FALSE", `{"a":false}`},
		{"a: 4", `{"a":4}`}, {"a: -12", `{"a":-12}`}, {"a: +12", `{"a":12}`}, {"a: 007", `{"a":7}`},
		{"a: 0o17", `{"a":15}`}, {"a: 0x1F", `{"a":31}`},
		{"a: 123456789012345678901234567890", `{"a":123456789012345678901234567890}`},
		{"a: 1.5", `{"a":1.5}`}, {"a: .5", `{"a":0.5}`}, {"a: 1.", `{"a":1}`}, {"a: -1e3", `{"a":-1000}`},
	} {
		if got := readYAML(t, tt.yaml); got != tt.json {
			t.Errorf("ParseYAML(%q) = %s, want %s", tt.yaml, got, tt.json)
		}
	}
}

func TestParse_String_IsString(t *testing.T) {
	for _, tt := range []struct{ yaml, value string }{
		{"a: yes", "yes"}, {"a: on", "on"}, {"a: nULL", "nULL"}, {"a: tRUE", "tRUE"}, {"a: 1.0.0", "1.0.0"},
		{"a: 0x", "0x"}, {"a: 0b101", "0b101"}, {"a: 1_000", "1_000"}, {"a: nan", "nan"}, {"a: ^1.0.0", "^1.0.0"},
		{"a: https://example.com/a.zip", "https://example.com/a.zip"}, {"a: '1'", "1"}, {"a: \"true\"", "true"},
		{"a: ''", ""}, {"a: \"null\"", "null"}, {"a: !!str 5", "5"}, {"a: ! 5", "5"}, {"a: |\n  x\n  y\n", "x\ny\n"},
		{"a: >-\n  4\n", "4"},
	} {
		v, err := ParseYAML([]byte(tt.yaml), "x.yaml")
		if err != nil {
			t.Fatalf("ParseYAML(%q): %v", tt.yaml, err)
		}
		a, _ := v.(*Object).Get("a")
		if a != tt.value {
			t.Errorf("ParseYAML(%q).a = %#v, want %q", tt.yaml, a, tt.value)
		}
	}
}

func TestParse_CoreSchemaTag_IsResolved(t *testing.T) {
	for _, tt := range []struct{ yaml, json string }{
		{"a: !!int 5", `{"a":5}`}, {"a: !!int 0x10", `{"a":16}`}, {"a: !!float 5", `{"a":5}`},
		{"a: !!bool true", `{"a":true}`}, {"a: !!null ''", `{"a":null}`}, {"a: !!seq [1]", `{"a":[1]}`},
		{"a: !!map {b: 1}", `{"a":{"b":1}}`},
	} {
		if got := readYAML(t, tt.yaml); got != tt.json {
			t.Errorf("ParseYAML(%q) = %s, want %s", tt.yaml, got, tt.json)
		}
	}
}

func TestParse_BlockAndFlowCollections_ReadAsJson(t *testing.T) {
	yaml := `# comment
format: 4
id: example.com/a
variants:
  "":
    - platform: server
      runtime: win-x64
      assets: [ { type: self, place: [] } ]
  lua: []
info: {}`
	want := compact(t, `
		{ "format": 4, "id": "example.com/a",
		  "variants": {
		    "": [ { "platform": "server", "runtime": "win-x64",
		            "assets": [ { "type": "self", "place": [] } ] } ],
		    "lua": [] },
		  "info": {} }`)
	if got := readYAML(t, yaml); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParse_Json_ReadsAsTheSameValue(t *testing.T) {
	j := `{ "a": [1, -2.5, true, false, null, "x\tyé\\/"], "": {}, "b": "😀" }`
	if got, want := readYAML(t, j), compact(t, j); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParse_Alias_IsExpanded(t *testing.T) {
	if got := readYAML(t, "a: &x [1, 2]\nb: *x\n"); got != `{"a":[1,2],"b":[1,2]}` {
		t.Errorf("got %s", got)
	}
}

func TestParse_NotAJsonValue_Throws(t *testing.T) {
	for _, tt := range []struct{ yaml, reason string }{
		{"", "no YAML document"}, {"# only a comment\n", "no YAML document"},
		{"a: 1\n---\nb: 2\n", "more than one YAML document"}, {"a: [1\n", "not valid YAML"},
		{"a: 1\na: 2\n", "appears twice"}, {"1: a\n", "is not a string"}, {"null: a\n", "is not a string"},
		{"true: a\n", "is not a string"}, {"? [a]\n: b\n", "is not a string"}, {"a: *x\n", "names no earlier anchor"},
		{"a: .inf\n", "cannot be represented"}, {"a: -.Inf\n", "cannot be represented"},
		{"a: .NaN\n", "cannot be represented"}, {"a: 1e999\n", "out of range"}, {"a: !!int abc\n", "is not valid"},
		{"a: !!bool 1\n", "is not valid"}, {"a: !custom x\n", "is not valid"}, {"a: !!map [1]\n", "is not valid here"},
	} {
		_, err := ParseYAML([]byte(tt.yaml), "x.yaml")
		var ye *YAMLError
		if !errors.As(err, &ye) || !strings.Contains(ye.Message, tt.reason) {
			t.Errorf("ParseYAML(%q) = %v, want an error containing %q", tt.yaml, err, tt.reason)
		}
	}
}

func TestParse_ErrorMessages(t *testing.T) {
	for _, tt := range []struct{ yaml, msg string }{
		{"a: [1\n", "The file is not valid YAML at line 2, column 1."},
		{"a: 1\n---\nb: 2\n", "The file holds more than one YAML document; the second one starts at line 2, column 1. Keep only one document: remove the '---' or '...' line that separates them."},
		{"a: 1\nb:\n  c: 1\n  c: 2\n", "The mapping key 'c' at line 4, column 3 appears twice in the same mapping. Remove one of them."},
		{"[a]: 1", `The mapping key ["a"] at line 1, column 1 is not a string. Keys must be text; quote the key if it is meant as text.`},
		{"a: *x", "The alias *x at line 1, column 4 names no earlier anchor. Define the anchor &x before the alias uses it."},
		{"a: !!int x", "The value 'x' at line 1, column 4 tagged !!int is not valid. Remove the tag, or write a value the tag allows."},
		{"a: !!map [1]", "The tag !!map at line 1, column 4 is not valid here, on a sequence. Remove the tag."},
		{"a: .nan", `The value .nan cannot be represented in JSON. Quote it (".nan") if it is meant as text.`},
		{"a: 1e400", `The number 1e400 is out of range. Quote it ("1e400") if it is meant as text.`},
	} {
		_, err := ParseYAML([]byte(tt.yaml), "x.yaml")
		var ye *YAMLError
		if !errors.As(err, &ye) || ye.Message != tt.msg {
			t.Errorf("ParseYAML(%q) = %v\nwant %q", tt.yaml, err, tt.msg)
		}
	}
}

func TestParse_AliasExpansionBomb_Throws(t *testing.T) {
	var b strings.Builder
	b.WriteString("l0: &l0 [a, a, a, a, a, a, a, a, a, a]\n")
	for i := 1; i < 12; i++ {
		fmt.Fprintf(&b, "l%d: &l%d [*l%d, *l%d, *l%d, *l%d, *l%d]\n", i, i, i-1, i-1, i-1, i-1, i-1)
	}
	_, err := ParseYAML([]byte(b.String()), "x.yaml")
	if err == nil || !strings.Contains(err.Error(), "expands to more than") {
		t.Errorf("got %v", err)
	}
}

func TestParse_DeepNesting_Throws(t *testing.T) {
	if _, err := ParseYAML([]byte(strings.Repeat("[", 100)+strings.Repeat("]", 100)), "x.yaml"); err == nil {
		t.Error("no error")
	}
}

func TestParse_ByteOrderMarks_AreHonored(t *testing.T) {
	utf8 := append([]byte{0xEF, 0xBB, 0xBF}, "a: é"...)
	utf16le := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune("a: é")) {
		utf16le = append(utf16le, byte(u), byte(u>>8))
	}
	want := compact(t, `{"a":"é"}`)
	for _, in := range [][]byte{utf8, utf16le} {
		v, err := ParseYAML(in, "x.yaml")
		if err != nil || string(MarshalJSONCompact(v)) != want {
			t.Errorf("ParseYAML(%q) = %v, %v", in, v, err)
		}
	}
}

func TestParse_InvalidUtf8_Throws(t *testing.T) {
	_, err := ParseYAML([]byte("a: \xC3\x28"), "x.yaml")
	if err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("got %v", err)
	}
}

func TestSerialize_Document_WritesBlockYaml(t *testing.T) {
	v, err := ParseJSON([]byte(`{
	  "format": 4,
	  "id": "example.com/a",
	  "version": "1.0",
	  "variants": { "": [ { "platform": "server", "deps": {}, "purge": [] } ] },
	  "n": [ 1, -2.5, true, null, [ "x" ] ]
	}`), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `format: 4
id: example.com/a
version: "1.0"
variants:
  "":
  - platform: server
    deps: {}
    purge: []
"n":
- 1
- -2.5
- true
- null
- - x
`
	if got := string(MarshalYAML(v)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

var roundTripStrings = []string{
	"", " ", "1.0", "1", "-1", "0x10", "0o7", "1e3", ".5", ".inf", "-.Inf", ".nan", "true", "False", "null", "~",
	"NULL", "yes", "a: b", "a:b", "- a", "-", "? a", "#a", "a #b", "*a", "&a", "!a", "%a", "@a", "`a", "|", ">",
	"{a}", "[a]", "a, b", "'", "\"", "\\", " lead", "trail ", "multi\nline", "multi\nline\n", "\n", "tab\there",
	"\u0001\u007f", "中文 é", "😀", "---", "...", "https://example.com/a.zip#x", strings.Repeat("x", 500),
	"^1.0.0 || >=2.0.0",
}

func TestSerialize_String_ReadsBackAsTheSameString(t *testing.T) {
	for _, s := range roundTripStrings {
		o := NewObject()
		o.Set(s, s)
		o.Set("list", []any{s})
		yaml := MarshalYAML(o)
		v, err := ParseYAML(yaml, "x.yaml")
		if err != nil {
			t.Errorf("%q: ParseYAML(%q): %v", s, yaml, err)
			continue
		}
		r := v.(*Object)
		got, _ := r.Get(s)
		list, _ := r.Get("list")
		if got != s || list.([]any)[0] != s {
			t.Errorf("%q: read back %#v, %#v from %q", s, got, list, yaml)
		}
	}
}

func TestSerialize_Value_ReadsBackAsTheSameValue(t *testing.T) {
	for _, j := range []string{
		"null", "true", "4", "-0.5", "1E+20", "123456789012345678901234567890", `"x"`, "[]", "{}",
		`[[], {}, [[1]], {"a": {"b": []}}]`,
	} {
		v, err := ParseJSON([]byte(j), "x.json")
		if err != nil {
			t.Fatal(err)
		}
		yaml := MarshalYAML(v)
		r, err := ParseYAML(yaml, "x.yaml")
		if err != nil || !Equal(v, r) {
			t.Errorf("%s: %q read back as %v, %v", j, yaml, r, err)
		}
		if !strings.HasSuffix(string(yaml), "\n") {
			t.Errorf("%s: %q does not end with a line break", j, yaml)
		}
	}
}

func TestSerialize_Yaml11Boolean_IsQuoted(t *testing.T) {
	for _, s := range []string{"yes", "Off", "n"} {
		o := NewObject()
		o.Set("a", s)
		if got, want := string(MarshalYAML(o)), "a: \""+s+"\"\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestSerialize_NonAscii_IsWrittenUnescaped(t *testing.T) {
	o := NewObject()
	o.Set("name", "中文 é")
	if got := string(MarshalYAML(o)); got != "name: 中文 é\n" {
		t.Errorf("got %q", got)
	}
}

func TestSerialize_StringStyles(t *testing.T) {
	// Expected values were produced by lip's YamlJson.Serialize.
	for _, tt := range []struct{ in, out string }{
		{"*", "a: '*'\n"}, {"**/x", "a: '**/x'\n"}, {"[a]", "a: '[a]'\n"}, {"#x", "a: '#x'\n"},
		{"a: b", "a: 'a: b'\n"}, {"'a", "a: \"'a\"\n"}, {"a'b", "a: a'b\n"}, {"a\tb", "a: \"a\tb\"\n"},
		{"a\nb", "a: \"a\\nb\"\n"}, {"\x7f", "a: \"\\x7F\"\n"}, {"😀", "a: \"\\U0001F600\"\n"},
		{" a", "a: ' a'\n"}, {"\ufeff", "a: \ufeff\n"}, {"\uffff", "a: \"\\uFFFF\"\n"},
		{"a\u2028b", "a: >-\n  a\n  b\n"},
	} {
		o := NewObject()
		o.Set("a", tt.in)
		if got := string(MarshalYAML(o)); got != tt.out {
			t.Errorf("%q: got %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestSerialize_LongKey_IsComplex(t *testing.T) {
	k := strings.Repeat("k", 1025)
	o := NewObject()
	o.Set(k, "v")
	if got, want := string(MarshalYAML(o)), "? "+k+"\n: v\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSerializeToUtf8Bytes_WritesUtf8WithoutByteOrderMark(t *testing.T) {
	v, _ := ParseJSON([]byte(`{ "a": "é" }`), "x.json")
	if got := string(MarshalYAML(v)); got != "a: é\n" {
		t.Errorf("got %q", got)
	}
}

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
