package doc

import (
	"encoding/json"
	"strings"
	"testing"
)

// Ports of tests/Lip.Core.Tests/Json/TomlJsonTests.cs (lip main), and cases
// checked against lip's TomlJson (Tomlyn 2.10.2) by hand.

func readTOML(t *testing.T, toml string) string {
	t.Helper()
	v, err := ParseTOML([]byte(toml))
	if err != nil {
		t.Fatalf("ParseTOML(%q): %v", toml, err)
	}
	return string(MarshalJSONCompact(v))
}

func assertRoundTrips(t *testing.T, j string) {
	t.Helper()
	v, err := ParseJSON([]byte(j), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	toml, err := MarshalTOML(v)
	if err != nil {
		t.Fatalf("MarshalTOML(%s): %v", j, err)
	}
	back, err := ParseTOML(toml)
	if err != nil {
		t.Fatalf("ParseTOML(%q): %v", toml, err)
	}
	if !Equal(normalizeNumbers(v), normalizeNumbers(back)) {
		t.Errorf("%s does not read back:\n%s", j, toml)
	}
}

// normalizeNumbers writes every number as formatDouble does, so that values
// compare as JsonNode.DeepEquals compares them.
func normalizeNumbers(v any) any {
	switch t := v.(type) {
	case *Object:
		o := NewObject()
		for _, k := range t.Keys() {
			m, _ := t.Get(k)
			o.Set(k, normalizeNumbers(m))
		}
		return o
	case []any:
		a := make([]any, len(t))
		for i, e := range t {
			a[i] = normalizeNumbers(e)
		}
		return a
	case json.Number:
		f, _ := t.Float64()
		return json.Number(formatDouble(f))
	default:
		return v
	}
}

func TestParseTOML_Values_ReadsJsonValue(t *testing.T) {
	for _, tt := range []struct{ toml, json string }{
		{`a = true`, `{"a":true}`},
		{`a = false`, `{"a":false}`},
		{`a = 4`, `{"a":4}`},
		{`a = -12`, `{"a":-12}`},
		{`a = 0x1F`, `{"a":31}`},
		{`a = 1_000`, `{"a":1000}`},
		{`a = 1.5`, `{"a":1.5}`},
		{`a = "1.0.0"`, `{"a":"1.0.0"}`},
		{`a = 'C:\x'`, `{"a":"C:\\x"}`},
		{`a = []`, `{"a":[]}`},
		{`a = {}`, `{"a":{}}`},
		{`a.b = 1`, `{"a":{"b":1}}`},
		{`"" = 1`, `{"":1}`},
		{"[[a]]\nb = 1\n[[a]]\nb = 2", `{"a":[{"b":1},{"b":2}]}`},
		{`a = [1, "x", { b = 2 }]`, `{"a":[1,"x",{"b":2}]}`},
		{``, `{}`},
		{"a = 1.0\nb = 1e5\nc = -0.0\nd = 1.5e-7\ne = -0\nf = 0o17\ng = 0b101", `{"a":1,"b":100000,"c":-0,"d":1.5E-07,"e":0,"f":15,"g":5}`},
		{`a = "\e\x41\u00e9\U0001F600"`, `{"a":"\u001BA\u00E9\uD83D\uDE00"}`},
		{"a = {\n  b = 1, # c\n  c.d = 2,\n}", `{"a":{"b":1,"c":{"d":2}}}`},
		{"a.b = 1\na.c = 2\nd = 3\na.e = 4", `{"a":{"b":1,"c":2,"e":4},"d":3}`},
		{"[a.b.c]\n[a]\nb.d = 1", `{"a":{"b":{"c":{},"d":1}}}`},
		{"[a]\nb.c = 1\n[a.b.d]\ne = 1", `{"a":{"b":{"c":1,"d":{"e":1}}}}`},
		{"[[ a ]]\n[a.b]\nc = 1\n[[a]]\n[ a . b ]\nc = 2", `{"a":[{"b":{"c":1}},{"b":{"c":2}}]}`},
		{"a = \"\"\"\n  x\"\"\"", `{"a":"  x"}`},
		{"a = \"\"\"x\\  \n\n  y\"\"\"", `{"a":"xy"}`},
		{"a = \"\"\"x\\\n  y\"\"\"", `{"a":"xy"}`},
		{"a = \"\"\"x\\\r\ny\\\n\"\"\"", `{"a":"xy"}`},
		{"a = \"\"\"x\\ \n y\"\"\"", `{"a":"xy"}`},
		{"a = \"\"\"x\\\t\ny\"\"\"", `{"a":"xy"}`},
		{"[[a]]\nb.c = 1\n[a.b]", `{"a":[{"b":{"c":1}}]}`},
		{"a = \"\"\"x\r\ny\"\"\"", `{"a":"x\r\ny"}`},
		{`a = """x"""""`, `{"a":"x\u0022\u0022"}`},
		{`a = '''x''''`, `{"a":"x\u0027"}`},
		{"a = [\n  1, # c\n  2,\n]", `{"a":[1,2]}`},
		{"a = 1\r\nb = 2", `{"a":1,"b":2}`},
		{"[a] #c\nx=1", `{"a":{"x":1}}`},
	} {
		if got := readTOML(t, tt.toml); got != tt.json {
			t.Errorf("ParseTOML(%q) = %s, want %s", tt.toml, got, tt.json)
		}
	}
}

func TestParseTOML_NotAJsonValueOrNotToml_Throws(t *testing.T) {
	const date = "is a date or a time, which lip does not read. Quote it if it is meant as text."
	const inf = "is inf or nan, which JSON cannot represent. Quote it if it is meant as text."
	for _, tt := range []struct{ toml, msg string }{
		{"a = 1979-05-27T07:32:00Z", "The value of 'a' " + date},
		{"a = 1979-05-27", "The value of 'a' " + date},
		{"a = 1979-05-27 07:32:00", "The value of 'a' " + date},
		{"a = 07:32", "The value of 'a' " + date},
		{"[x]\na = [07:32:00]", "The value of 'x.a[0]' " + date},
		{"a = inf", "The value of 'a' " + inf},
		{"a = -nan", "The value of 'a' " + inf},
		{"a = 1e400", "The value of 'a' " + inf},
		{"a = 1\na = 2", ""},
		{"a: 1", ""},
		{`{ "a": 1 }`, ""},
		{"a = ", ""},
		{"a = 9223372036854775808", ""},
		{"a = 01", ""},
		{"a = 1979-13-27", ""},
		{"a = .5", ""},
		{"a = 5.", ""},
		{"a = 1__0", ""},
		{"a = 0X1", ""},
		{"a = True", ""},
		{"a.b = 1\n[a]", ""},
		{"[[a]]\n[a]", ""},
		{"[a]\nb=1\n[a]", ""},
		{"a = {b = 1}\n[a.c]", ""},
		{"a = {b = {c = 1}, b.d = 2}", ""},
		{"a = [{b = 1}]\n[[a]]", ""},
		{"[a.b]\nx = 1\n[a]\nb.y = 2", ""},
		{"a = 1\rb = 2", ""},
		{`a = "\uD800"`, ""},
		{`a = "\q"`, ""},
		{"a = \"x\x7f\"", ""},
		{"a = 1 # \x7f", ""},
		{"é = 1", ""},
		{"a = { b = 1,, }", ""},
		{"a = { b = 1 } x", ""},
		{"a = [,]", ""},
		{"a=1 b=2", ""},
		{`"\u0061" = 1` + "\na = 2", ""},
		{`a = """x""""""`, ""},
		// Tomlyn's limit on nesting, checked before the values.
		{"a = " + strings.Repeat("[", 64) + strings.Repeat("]", 64), ""},
		{"a = 1979-05-27\nb = " + strings.Repeat("[", 64) + strings.Repeat("]", 64), ""},
		{"[" + strings.Repeat("k.", 63) + "k]", ""},
		// Tomlyn: a backslash and a space need more than one line break.
		{"a = \"\"\"x\\ \ny\"\"\"", ""},
		{"a = \"\"\"x\\ y\"\"\"", ""},
	} {
		_, err := ParseTOML([]byte(tt.toml))
		te, ok := err.(*TOMLError)
		if !ok {
			t.Errorf("ParseTOML(%q) = %v, want a *TOMLError", tt.toml, err)
			continue
		}
		if tt.msg != "" && te.Message != tt.msg {
			t.Errorf("ParseTOML(%q) = %q, want %q", tt.toml, te.Message, tt.msg)
		}
		if tt.msg == "" && !strings.HasPrefix(te.Message, "The file is not valid TOML: ") {
			t.Errorf("ParseTOML(%q) = %q, want a syntax error", tt.toml, te.Message)
		}
	}
	readTOML(t, "a = "+strings.Repeat("[", 63)+strings.Repeat("]", 63))
}

func TestParseTOML_InvalidUtf8_Throws(t *testing.T) {
	_, err := ParseTOML([]byte{0x61, 0x3D, 0x22, 0xFF, 0x22})
	if err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("err = %v", err)
	}
}

func TestParseTOML_ByteOrderMark_IsSkipped(t *testing.T) {
	if got := readTOML(t, "\xEF\xBB\xBFa = 1"); got != `{"a":1}` {
		t.Errorf("got %s", got)
	}
}

func TestMarshalTOML_ReadsBackAsSameValue(t *testing.T) {
	for _, j := range []string{
		`{"format":4,"id":"github.com/a/b","version":"1.0.0"}`,
		`{"a":"","b":"true","c":"1","d":"x\"y\\z\n\t\u0001\u007f","e":"日本"}`,
		`{"a b":1,"":2,"a.b":3,"ü":4,"#":5}`,
		`{"a":[],"b":{},"c":[[1,2],["x"]],"d":[1,"x",true,{"e":1}]}`,
		`{"a":1.5,"b":-0.25,"c":1e300,"d":-9223372036854775808}`,
		`{"a":{"b":{"c":1}},"d":2}`,
		`{"a":[{"b":1},{"c":[{"d":{"e":1}}]}]}`,
		`{"a":[{"b":{}},{"c":[]}]}`,
		`{"a":{"b":[{"c":{"d":1}}],"e":{"f":2}}}`,
		`{
		  "format": 4, "id": "github.com/user/repo", "version": "1.0.0",
		  "info": { "name": "x", "tags": ["a", "b"] },
		  "variants": {
		    "": [
		      {
		        "platform": "server", "runtime": "win-x64",
		        "deps": { "github.com/a/b#c/d": ">=1.0.0" },
		        "assets": [
		          { "type": "self", "place": [ { "src": "a/", "dest": "b/" } ] },
		          { "type": "mcmod", "urls": [ "https://example.com/a.zip" ] }
		        ],
		        "preserve": [ "**/config.json" ],
		        "hooks": { "post_install": [ "echo hi" ] }
		      },
		      { "platform": "server", "runtime": "linux-x64" }
		    ],
		    "x/y": [ { "platform": "client", "runtime": "win-x64" } ]
		  }
		}`,
	} {
		assertRoundTrips(t, j)
	}
}

func TestMarshalTOML_PackageFile_WritesTablesAndArraysOfTables(t *testing.T) {
	v, err := ParseJSON([]byte(`{
	  "format": 4, "id": "github.com/user/repo", "version": "0.1.0",
	  "variants": { "": [ { "platform": "server", "runtime": "win-x64",
	    "assets": [ { "type": "self", "place": [ { "src": "a", "dest": "b" } ] } ] } ] }
	}`), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := MarshalTOML(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `format = 4
id = "github.com/user/repo"
version = "0.1.0"

[[variants.""]]
platform = "server"
runtime = "win-x64"

[[variants."".assets]]
type = "self"
place = [{ src = "a", dest = "b" }]
`
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMarshalTOML_NullOrNotAnObject_Fails(t *testing.T) {
	o := NewObject()
	o.Set("a", nil)
	if _, err := MarshalTOML(o); err == nil {
		t.Error("null")
	}
	if _, err := MarshalTOML([]any{json.Number("1")}); err == nil {
		t.Error("array")
	}
}

func TestMarshalTOML_Numbers(t *testing.T) {
	v, err := ParseJSON([]byte(`{"a":1.0,"b":1e2,"c":-0,"d":9223372036854775808,"f":100000000000000000000000,"g":1E400}`), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := MarshalTOML(v)
	want := "a = 1.0\nb = 1e2\nc = 0\nd = 9.223372036854776E+18\nf = 1E+23\ng = 1E400\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMarshalTOML_Layout(t *testing.T) {
	v, err := ParseJSON([]byte(`{"x":{"y":{"z":1}},"w":[{"a":{"b":1}},{"c":2}],"v":[{"a":1}],"u":{},"t":[],`+
		`"s":{"p":{}},"r":[[{"a":1}]],"q":{"m":{"n":{"o":1}}}}`), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := MarshalTOML(v)
	want := "v = [{ a = 1 }]\nu = {}\nt = []\nr = [[{ a = 1 }]]\n\n[x.y]\nz = 1\n\n[s]\np = {}\n\n[q.m.n]\no = 1\n\n" +
		"[[w]]\n\n[w.a]\nb = 1\n\n[[w]]\nc = 2\n"
	if string(got) != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestMarshalTOML_NoByteOrderMark(t *testing.T) {
	o := NewObject()
	o.Set("a", "日本")
	if got, _ := MarshalTOML(o); string(got) != "a = \"日本\"\n" {
		t.Errorf("got %q", got)
	}
}

// JsonFile.

func TestJSONFile(t *testing.T) {
	v, err := ParseJSONFile([]byte("\xEF\xBB\xBF{\"b\":[1,true,null,{},[]],\"a\":\"é<>&'\\u0001😀\\u2028\"}"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"b\": [\n    1,\n    true,\n    null,\n    {},\n    []\n  ],\n" +
		"  \"a\": \"é<>&'\\u0001\\uD83D\\uDE00\\u2028\"\n}\n"
	if got := string(MarshalJSONFile(v)); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	for _, in := range []string{`{"a":1,"a":2}`, `{"a":1,}`, `{} //`, "{\"a\":\"\xFF\"}",
		strings.Repeat("[", 65) + strings.Repeat("]", 65)} {
		if _, err := ParseJSONFile([]byte(in)); err == nil {
			t.Errorf("ParseJSONFile(%q) succeeded", in)
		}
	}
}
