package doc

// Differential golden generator: runs the corpus through lip's own C# code
// and writes testdata/*.jsonl.gz. It only runs when LIPCORE_DOC_HARNESS names
// a harness executable: a .NET console program referencing Lip.Core.dll of lip
// v0.38.0-beta.1 that reads base64 inputs from stdin, one per line, and per
// input writes "OK <result>", "ERR <base64 YamlFormatException or
// PackageFormatException message>" or "CRASH <base64 other exception>", with
// <result> for each mode (args[0]):
//
//	serialize  base64 YamlJson.Serialize(JsonDocument.Parse(input).RootElement)
//	parse      base64 YamlJson.Parse(input).RootElement.GetRawText()
//	parsejson  base64 compact and indented JsonNode text of
//	           PackageFileParser.ParseJson(input, "x.json"), then
//	           PackageFileParser.IntegerMember(root, "n") ("none" if null,
//	           "-" for a root that is not an object)

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runHarness(t *testing.T, exe, mode string, inputs [][]byte) []string {
	t.Helper()
	var in bytes.Buffer
	for _, b := range inputs {
		in.WriteString(base64.StdEncoding.EncodeToString(b))
		in.WriteByte('\n')
	}
	cmd := exec.Command(exe, mode)
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("harness %s: %v", mode, err)
	}
	var res []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		res = append(res, sc.Text())
	}
	if len(res) != len(inputs) {
		t.Fatalf("harness %s: %d results for %d inputs", mode, len(res), len(inputs))
	}
	return res
}

func b64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func writeJSONL(t *testing.T, name string, recs []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	zw.Write(buf.Bytes())
	zw.Close()
	if err := os.WriteFile("testdata/"+name+".gz", gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateGolden(t *testing.T) {
	exe := os.Getenv("LIPCORE_DOC_HARNESS")
	if exe == "" {
		t.Skip("LIPCORE_DOC_HARNESS not set")
	}

	// --- serialize ---
	var serIn [][]byte
	for _, s := range writerStrings() {
		o := NewObject()
		o.Set(s, s)
		o.Set("list", []any{s})
		inner := NewObject()
		inner.Set("k", s)
		o.Set("nested", []any{inner, []any{s}})
		serIn = append(serIn, MarshalJSONCompact(o))
		serIn = append(serIn, MarshalJSONCompact(s))
	}
	serIn = append(serIn, writerDocs()...)
	// Random trees, to mix scalar styles with nesting and indentation.
	rng := rand.New(rand.NewSource(380))
	strs := writerStrings()
	var gen func(depth int) any
	gen = func(depth int) any {
		switch k := rng.Intn(10); {
		case depth > 4 || k < 4:
			switch rng.Intn(6) {
			case 0:
				return json.Number(fmt.Sprint(rng.Intn(2000) - 1000))
			case 1:
				return rng.Intn(2) == 0
			case 2:
				return nil
			default:
				return strs[rng.Intn(len(strs))]
			}
		case k < 7:
			n := rng.Intn(4)
			a := []any{}
			for range n {
				a = append(a, gen(depth+1))
			}
			return a
		default:
			n := rng.Intn(4)
			o := NewObject()
			for range n {
				o.Set(strs[rng.Intn(len(strs))], gen(depth+1))
			}
			return o
		}
	}
	for range 600 {
		serIn = append(serIn, MarshalJSONCompact(gen(0)))
	}
	res := runHarness(t, exe, "serialize", serIn)
	var recs []map[string]any
	for i, r := range res {
		if !strings.HasPrefix(r, "OK ") {
			t.Fatalf("serialize %s: %s", serIn[i], r)
		}
		recs = append(recs, map[string]any{"in": string(serIn[i]), "out": b64(r[3:])})
	}
	writeJSONL(t, "serialize.jsonl", recs)

	// --- parse YAML ---
	var parseIn [][]byte
	for _, r := range recs {
		parseIn = append(parseIn, []byte(r["out"].(string)))
	}
	parseIn = append(parseIn, yamlDocs()...)
	recs = nil
	res = runHarness(t, exe, "parse", parseIn)
	for i, r := range res {
		kind, payload, _ := strings.Cut(r, " ")
		rec := map[string]any{"in": base64.StdEncoding.EncodeToString(parseIn[i]), "kind": kind}
		if payload != "" {
			rec["out"] = b64(payload)
		}
		recs = append(recs, rec)
	}
	writeJSONL(t, "parse.jsonl", recs)

	// --- parse JSON ---
	jin := jsonDocs()
	recs = nil
	res = runHarness(t, exe, "parsejson", jin)
	for i, r := range res {
		f := strings.Split(r, " ")
		rec := map[string]any{"in": base64.StdEncoding.EncodeToString(jin[i]), "kind": f[0]}
		switch f[0] {
		case "OK":
			rec["compact"] = b64(f[1])
			rec["indented"] = strings.ReplaceAll(b64(f[2]), "\r\n", "\n")
			rec["int"] = f[3]
		default:
			rec["out"] = b64(f[1])
		}
		recs = append(recs, rec)
	}
	writeJSONL(t, "parsejson.jsonl", recs)
}

// writerStrings is the corpus of strings for the YAML writer.
func writerStrings() []string {
	base := []string{
		"", " ", "  ", "1.0", "1", "-1", "+1", "0x10", "0X10", "0o7", "0O7", "0b1", "1e3", "1E3", ".5", "5.", "-.5",
		".inf", "-.Inf", "+.INF", ".nan", ".NaN", "true", "True", "TRUE", "tRue", "False", "null", "Null", "NULL",
		"nULL", "~", "~a", "yes", "Yes", "YES", "yEs", "no", "No", "NO", "on", "On", "ON", "off", "Off", "OFF", "y", "Y",
		"n", "N", "a: b", "a:b", "a:", ":a", ":", "::", "- a", "-", "--", "---", "---a", "--- a", "...", "....", "... a",
		"? a", "?", "?a", "#a", "a #b", "a#b", "a# b", "*a", "&a", "!a", "%a", "@a", "`a", "|", ">", "|a", ">a",
		"{a}", "[a]", "a, b", "a,b", "'", "\"", "\\", "a'b", "'a", "a'", "''", "a\"b", "\"a", " lead", "trail ",
		" both ", "a  b", "multi\nline", "multi\nline\n", "\n", "\n\n", "a\n\nb", "a \nb", "a\n b", "\r", "a\r\nb",
		"tab\there", "\t", "\ta", "a\t", "\u0001\u007f", "\u0000", "\u001b", "\u0085", "\u0080", "\u009f", "\u00a0",
		"a\u00a0b", "\u00a0a", "中文 é", "😀", "a😀b", "\U0001F600 x", "\ufeff", "a\ufeffb", "\ufffe", "\uffff",
		"\ufffd", "\u2028", "a\u2028b", "\u2028a", "a\u2028", "a\u2028\u2028", "a\u2028\u2028b", " a\u2028b",
		"a \u2028b", "a\u2028 b", "a\u2029b", "a\u2028\u2029b", "\u2029", "a\u2028b\u2028", "x\u2028\u2028\u2028",
		"a\u2028\tb", "\u2028 ", "a\u2028b ", "https://example.com/a.zip#x", strings.Repeat("x", 500),
		strings.Repeat("y", 1024), strings.Repeat("z", 1025), strings.Repeat("é", 1024), strings.Repeat("😀", 512),
		strings.Repeat("😀", 513), "^1.0.0 || >=2.0.0", ">=1.0.0", "<1.0.0", "1.0.0", "v1.0.0", "1.2.3-beta.1+build",
		"github.com/LiteLDev/LeviLamina", "a/b/c", "*", "**", "**/x", "*.dll", "a*", "x: y: z", "a - b", "a -b",
		"-a", "- ", " -", "-\ta", "? ", "?\ta", ":\ta", "a:\tb", "a: ", "a :b", "a # b", "a\t#b", "#", "# a", "&", "!",
		"!!str", "! a", "%", "@", "`", "=", "<<", "=a", "a=b", "0", "00", "007", "-0", "+0", "0.0", "-0.0", "1_000",
		"1,000", "0x", "0o", "0xG", "0o8", "1e", "1e+", "e1", ".", "..", "+", "+.", ".e1", "1.e1", "12:30",
		"2001-12-14", "2001-12-14t21:59:43.10-05:00", "a\u3000b", "\u3000", "\u200b", "\u0300", "a\u0300",
		"[", "]", "{", "}", ",", "a]", "a}", "a[", "a{", "a,", "a?", "?a?", "a!", "a&", "a*", "a|", "a>", "a%", "a@",
		"a`", "a'", "\"\"", "'x'", "\"x\"", "\\n", "C:\\Program Files\\x", "a\\", "a\vb", "a\fb", "a\bb",
		"Ⅻ", "ǅ", "٣", "１２３", "\u0661\u0662", "ı", "İ",
	}
	for c := 0x20; c < 0x7F; c++ {
		ch := string(rune(c))
		base = append(base, ch, "a"+ch, ch+"a", "a"+ch+"b", "a "+ch, ch+" ", ch+" a", "a "+ch+" b", ch+ch)
	}
	return base
}

// writerDocs are whole documents for the writer.
func writerDocs() [][]byte {
	return [][]byte{
		[]byte(`{}`), []byte(`[]`), []byte(`null`), []byte(`true`), []byte(`4`), []byte(`-0.5`), []byte(`1E+20`),
		[]byte(`123456789012345678901234567890`), []byte(`"x"`), []byte(`""`), []byte(`[[], {}, [[1]], {"a": {"b": []}}]`),
		[]byte(`{"format":4,"id":"example.com/a","version":"1.0","variants":{"":[{"platform":"server","deps":{},"purge":[]}]},"n":[1,-2.5,true,null,["x"]]}`),
		[]byte(`[[[["a"]]],[{"a":[{"b":{}}]}]]`), []byte(`{"a":[[1,2],[3]],"b":[{"c":[]}],"d":{"e":{"f":[[]]}}}`),
		[]byte(`[{"a":1,"b":[1,{"c":2}]},[{"d":3}]]`), []byte(`{"a\u2028b":{"x\u2028":[1]},"k":"v\u2028"}`),
		[]byte(`{"` + strings.Repeat("k", 1100) + `":[1,{"a":2}],"z":{"` + strings.Repeat("q", 1030) + `":"v"}}`),
		[]byte(`[1e400, 1.0, 1e2, -0, 0.10]`), []byte(`{"a":"\u2028x","b":["\u2028"],"c":{"d":"a\u2028b\u2028"}}`),
		[]byte(`["a\u2028", {"k": "x"}]`), []byte(`{"a":"b\u2028","c":"d"}`),
	}
}

// yamlDocs is the corpus of YAML files for the reader.
func yamlDocs() [][]byte {
	docs := []string{
		"a: null", "a: Null", "a: NULL", "a: ~", "a:", "a: true", "a: True", "a: FALSE", "a: 4", "a: -12", "a: +12",
		"a: 007", "a: 0o17", "a: 0x1F", "a: 0X1F", "a: 0O17", "a: 123456789012345678901234567890",
		"a: -123456789012345678901234567890", "a: 9223372036854775807", "a: 9223372036854775808",
		"a: -9223372036854775808", "a: -9223372036854775809", "a: 0xFFFFFFFFFFFFFFFFFFFF", "a: 0o777777777777777777777777",
		"a: 1.5", "a: .5", "a: 1.", "a: -1e3", "a: 1e15", "a: 1e16", "a: 1e17", "a: 1e18", "a: 0.0001", "a: 0.00001",
		"a: 1e-400", "a: -1e-400", "a: -0.0", "a: -0", "a: +0.0", "a: 0.1", "a: 1.7976931348623157e308", "a: 2e308",
		"a: 5e-324", "a: 123456789012345678", "a: 12345678901234567.0", "a: 1.2345678901234567e-7", "a: 100.000",
		"a: 3.14159265358979323846", "a: 1E5", "a: 1e+5", "a: 1e05", "a: .5e1", "a: 9007199254740993.0",
		"a: yes", "a: on", "a: nULL", "a: tRUE", "a: 1.0.0", "a: 0x", "a: 0b101", "a: 1_000", "a: nan", "a: ^1.0.0",
		"a: https://example.com/a.zip", "a: '1'", "a: \"true\"", "a: ''", "a: \"null\"", "a: !!str 5", "a: ! 5",
		"a: |\n  x\n  y\n", "a: >-\n  4\n", "a: !!int 5", "a: !!int 0x10", "a: !!float 5", "a: !!bool true",
		"a: !!null ''", "a: !!seq [1]", "a: !!map {b: 1}", "a: !!str .inf", "a: !!str 1e999", "a: !!float abc",
		"a: !!float .nan", "a: !!int 1.5", "a: !!null x", "a: !!null", "a: !!str", "a: !", "a: !<!> 5",
		"a: !<tag:yaml.org,2002:int> 5", "a: !<tag:yaml.org,2002:str> 5", "a: !<> 5", "a: !!binary eA==",
		"a: !!timestamp 2001-12-14", "a: !foo x", "a: !foo!bar x", "%TAG !e! tag:example.com,2000:\n---\na: !e!x 1",
		"%TAG !! tag:example.com,2000:\n---\na: !!int 1", "%YAML 1.2\n---\na: 1", "%YAML 1.1\n---\na: 1",
		"%YAML 2.0\n---\na: 1", "%YAML 1.4\n---\na: 1", "%YAML 1.2\n%YAML 1.2\n---\na: 1", "%FOO bar\n---\na: 1",
		"a: 1\n%YAML 1.2\n---\nb: 2", "a: 1\n...\n%YAML 1.2\n---\nb: 2", "---\na: 1\n...\n", "--- a", "--- |\n  x",
		"", "# only a comment\n", "\n\n", "...", "---", "--- \n...", "a: 1\n---\nb: 2\n", "a: 1\n--- [\n",
		"a: 1\n...\n# x\n", "a: 1\n...\n...\n", "a: [1\n", "a: 1\na: 2\n", "1: a\n", "null: a\n", "true: a\n",
		"? [a]\n: b\n", "? {a: 1}\n: b\n", "1.5: a", "~: a", "a: *x\n", "a: .inf\n", "a: -.Inf\n", "a: .NaN\n",
		"a: 1e999\n", "a: !!int abc\n", "a: !!bool 1\n", "a: !custom x\n", "a: !!map [1]\n", "a: !!seq {b: 1}",
		"a: !foo [1]", "a: !foo {b: 1}", "a: &x [1, 2]\nb: *x\n", "a: &x 1\nb: *x\nc: &x 2\nd: *x",
		"a: &x [1, *x]", "&a a: b\n*a : c", "a: &x\nb: *x", "a: &x !!str\nb: *x", "- &x a\n- *x\n- *x",
		"{a: 1, b: [2, 3], c: {d: e}}", "[a, b, {c: d}, [e]]", "[a: b, c]", "{a, b: c}", "{? a: b}", "[? a: b]",
		"{a: 1,}", "[1,]", "[1,,2]", "{a: b c}", "{a: 1 b: 2}", "{\"a\":1}", "{\"a\":1, \"b\":\"x\"}",
		"{ \"a\": [1, -2.5, true, false, null, \"x\\tyé\\\\/\"], \"\": {}, \"b\": \"😀\" }", "[\"\\uD83D\\uDE00\"]",
		"[\"\\uD800\"]", "[\"\\uD800x\"]", "[\"\\uDC00\\uDC00\"]", "[\"\\uD83D\\U0000DE00\"]", "[\"\\U0001F600\"]",
		"[\"\\U00110000\"]", "[\"\\UFFFFFFFF\"]", "[\"\\x41\\u0042\\N\\_\\L\\P\\0\\a\\e\\ \\/\\t\\\t\"]", "[\"\\q\"]",
		"[\"\\x4\"]", "[\"a\\\n  b\"]", "[\"a\n  b\"]", "[\"a\n\n  b\"]", "['a''b']", "['a\n\n  b  ']",
		"a: \"x\n y\"", "a: \"x\ny\"", "a:\n  \"x\n  y\"", "a: 'x\n y'", "a: x\n y", "a: x\n\n y", "a: x\n  # c\n y",
		"a: |\n  x\n\n  y\n\n", "a: |+\n  x\n\n", "a: |-\n  x\n\n", "a: |2\n   x\n", "a: |0\n x", "a: >\n  x\n  y\n\n  z\n",
		"a: >\n  x\n   y\n  z\n", "a: |#c\n x", "a: | x", "a: |\n x\n y\n", "- |\n x\n-  y", "a: |\n  \n  x", "a: |\n   \n  x",
		"a: >\n   \n  x", "a: |1\n  x", "- a\n- b\n-\n- - c\n  - d\n- e: f\n  g: h", "a:\n- 1\n- 2\nb: 3",
		"a:\n  - 1\n  - 2", "a:\n  b:\n    c: d", "a:\n b: 1\n  c: 2", "a: 1\n b: 2", "a:\n\tb: 1", "a:\t1",
		"a: \t1", "\ta: 1", "- \ta", "-\ta", "a: b\t# c", "a: b #c\n#d\ne: f", "a: b#c", "a: 'b'#c", "a: \"b\"#c",
		"[a]#c", "[a] #c", "[a, #c\n b]", "[a,#c\n b]", "? a\n? b\n: c", "? a\n: b\n? c\n: d", ": a", "? : a",
		"a: b: c", "a: - b", "a:\n- b\n c", "- a: b\n - c", "- - - a", "-  - a\n   - b", "a: [b, c]: d",
		"\"a\": 1", "'a': 1", "\"a\":1", "a:1", "a: :1", "a: -1", "a: -", "a: - ", "a: ?", "a: ? b", "a: ?b",
		"a: :", "a: b:", "a: b:c", "a: *", "*: a", "&: a", "a: &", "a: !", "a: %", "a: @x", "a: `x", "a: |x",
		"a: >x", "a: [", "a: ]", "a: {", "a: }", "a: ,", "a: ,b", "a: b,", "a: [b,c],d", "{a: [b, c] }",
		"{a: {b: {c: [d]}}}", "[[[[[[[[a]]]]]]]]", "{a: b, c: d, a: e}", "{\"a\": 1, \"a\": 2}",
		"a: 1\r\nb: 2\r\n", "a: 1\rb: 2", "a: \"x\r\n y\"", "a: |\r\n  x\r\n  y\r\n", "a: 1\u0085b: 2",
		"a: 1\u2028b: 2", "a: x\u2028y", "a: x\u2029 y", "a: \"\u2028\"", "a: b\x00c", "a: \x00", "\x00",
		"a: \u00e9", "é: ü", "a: 中文", "a: 😀", "😀: 1", "a: \ufeffb", "\ufeffa: 1", "a: b\ufeff",
		"a: 'it''s'", "a: \"\\\"q\\\"\"", "<<: {a: 1}", "a: <<", "a: =", "?", ":", "-", "- ", "? \n:", ": \n",
		"--- !!map\na: 1", "--- !!seq\n- 1", "--- !!str\na", "--- &x\na: 1", "--- *x", "!!map {a: 1}", "!!map\na: 1",
		"&a a", "&a\na: 1", "&a [1]", "&a\n- 1", "&a &b x", "!!str !!int x", "! &a x", "&a ! x", "*a", "[*a]",
		"a: &a\n  b: *a", "a: &a b\nc: &a d\ne: *a", "a: &a1 x\nb: *a1", "a: &a: x", "a: &a:b x\nb: *a:b",
		"a: &[x] y", "a: *a,b", "[&a x, *a]", "{&a x: y, *a : z}", "{*a: b}", "- *a", "&a x: 1\n*a : 2",
		"a: 1\n\n\n# end", "#c\na: 1 #d\n#e", "  a: 1", "  - a", " a: 1\nb: 2", "a: 1\n  b: 2",
		"- a\nb: c", "a: b\n- c", "a:\n  - b\n  c: d", "a:\n  b\n  c: d", "a: b\n  c", "a: 'b\nc'", "a: \"b\nc\"",
		"key: value\nlist:\n  - one\n  - two\nmap:\n  inner: [1, 2]\n", "a: 1 # comment\n", "a: '#'", "a: \"#\"",
		"a: b # c # d", "? |\n  a\n: b", "? >\n  a\n: b", "? - a\n: b", "a: !!str\n  b", "a: !!null\nb: 1",
		"!!null", "!!str", "!", "~", "null", "''", "\"\"", "5", "-5", "1.5", "true", "x", "[]", "{}", "[ ]", "{ }",
		"%TAG ! tag:x.com,2000:\n--- !a b", "%TAG !x! y\n%TAG !x! z\n--- a", "%TAG !x! \n--- a", "%TAG x y\n--- a",
		"%TAG !x y z\n--- a", "%YAML\n--- a", "%YAML 1\n--- a", "%YAML 1.\n--- a", "%YAML 1.2 x\n--- a",
		"%YAML 1234567890.1\n--- a", "%\n--- a", "%Y@ x\n--- a", "--- a\n...\n--- b", "--- a\n--- b", "a\n---",
		"a\n...", "a\n... x", "a\n...x", "...\na", "--- >\n a\n...", "--- [a,\n...\n]", "\"a\n---\nb\"", "'a\n...'",
		"a: b\n--- \nc", "---\n---\n", "--- # c\n", "!a\n", "!a!b c", "!!", "!!!a b", "!<!a> b", "!<tag:x,y> z",
		"!<a b> c", "! ! a", "!%41 b", "!a%41 b", "!a%4 b", "!a%C3%A9 b", "!a%FF b", "!a%C3 b", "!a, b", "[!a, b]",
		"[!a,b]", "!a,b c", "[a, !]", "{!!str a: b}", "{!!int 1: b}", "{a: !!int}", "a: !!float 1e999",
		"a: !!float .inf", "a: !!int 0o", "a: !!bool yes", "a: !!str yes", "a: !!null ~", "a: !!null NULL",
		"a: !!map\n  b: 1", "a: !!seq\n  - 1", "a: !!omap\n  - b: 1", "a: !!set {b}", "a: !!pairs [b: 1]",
		"{a: 1, b: 2}x", "[a]b", "[a] b", "{a: 1} # c", "{a: 1}\n", "[a\n,b]", "[a\n, b\n]", "{a\n: b}",
		"{a: \n b}", "{\na: b\n}", "[\n- a]", "[- a]", "[-a]", "[a-]", "[a, -]", "{a: -}", "{-: a}", "[?]",
		"[? ]", "[:]", "[: a]", "[a:]", "[a: ]", "[\"a\":b]", "[\"a\": b]", "{\"a\":b}", "{'a':b}", "{a:b}",
		"[a, {b: c}: d]", "[[a]: b]", "[{a: b}: c]", "{[a]: b}", "{{a: b}: c}", "{a: [b]: c}",
		"- ? a\n  : b", "- a: b\n  ? c\n  : d", "? a\n? a", "{? a, ? a}", "{a, a}", "{a: 1, a}",
		"\"\\uD800\": 1", "a: \"\\uDBFF\\uDFFF\"", "a: \"\\U0010FFFF\"", "a: \"\\U0000D800\"", "a: \"\\uD800\\u0041\"",
		"a: \"\\uD800\\uDBFF\"", "a: \"\\uD800\\ux\"", "a: \"\\uD800\\uD\"", "a: \"\\uD800\\UGG\"", "a: \"\\xGG\"",
		"a: \"\\u00e9\"", "a: \"\\xe9\"", "a: \"\\x00\"", "a: \"\\0\"", "a: \"\\\\\"", "a: \"\\\"\"",
	}
	var out [][]byte
	for _, d := range docs {
		out = append(out, []byte(d))
	}
	// Floats.
	frng := rand.New(rand.NewSource(64))
	for range 600 {
		e := frng.Intn(80) - 40
		m := frng.Float64() * 10
		switch frng.Intn(4) {
		case 0:
			out = append(out, []byte(fmt.Sprintf("a: %.17ge%d", m, e)))
		case 1:
			out = append(out, []byte(fmt.Sprintf("a: %.3f", m*math.Pow(10, float64(e/4)))))
		case 2:
			out = append(out, []byte(fmt.Sprintf("a: %d.%d", frng.Int63(), frng.Intn(1000))))
		default:
			out = append(out, []byte(fmt.Sprintf("a: %g", math.Float64frombits(frng.Uint64()&^(1<<63)))))
		}
	}
	// Nesting depth.
	for _, n := range []int{63, 64, 65, 66, 100} {
		out = append(out, []byte(strings.Repeat("[", n)+strings.Repeat("]", n)))
		out = append(out, []byte(strings.Repeat("[", n)+"1"+strings.Repeat("]", n)))
		var b strings.Builder
		for i := range n {
			b.WriteString(strings.Repeat(" ", i) + "a:\n")
		}
		b.WriteString(strings.Repeat(" ", n) + "b")
		out = append(out, []byte(b.String()))
	}
	// Alias bomb.
	var bomb strings.Builder
	bomb.WriteString("l0: &l0 [a, a, a, a, a, a, a, a, a, a]\n")
	for i := 1; i < 12; i++ {
		fmt.Fprintf(&bomb, "l%d: &l%d [*l%d, *l%d, *l%d, *l%d, *l%d]\n", i, i, i-1, i-1, i-1, i-1, i-1)
	}
	out = append(out, []byte(bomb.String()))
	// Long keys and long input.
	out = append(out, []byte(strings.Repeat("k", 1023)+": v"), []byte(strings.Repeat("k", 1024)+": v"),
		[]byte(strings.Repeat("k", 1025)+": v"), []byte("{"+strings.Repeat("k", 1100)+": v}"),
		[]byte("? "+strings.Repeat("k", 1100)+"\n: v"), []byte("a: "+strings.Repeat("x ", 3000)))
	var long strings.Builder
	for i := range 400 {
		fmt.Fprintf(&long, "key%d: [value %d, {x: %d}]\n", i, i, i)
	}
	out = append(out, []byte(long.String()))
	// Encodings.
	enc := func(s string, bom []byte, f func(string) []byte) []byte {
		return append(append([]byte{}, bom...), f(s)...)
	}
	u16le := func(s string) []byte {
		var b []byte
		for _, u := range utf16Encode(s) {
			b = append(b, byte(u), byte(u>>8))
		}
		return b
	}
	u16be := func(s string) []byte {
		var b []byte
		for _, u := range utf16Encode(s) {
			b = append(b, byte(u>>8), byte(u))
		}
		return b
	}
	u32le := func(s string) []byte {
		var b []byte
		for _, r := range s {
			b = append(b, byte(r), byte(r>>8), byte(r>>16), byte(r>>24))
		}
		return b
	}
	u32be := func(s string) []byte {
		var b []byte
		for _, r := range s {
			b = append(b, byte(r>>24), byte(r>>16), byte(r>>8), byte(r))
		}
		return b
	}
	id := func(s string) []byte { return []byte(s) }
	out = append(out,
		enc("a: é😀", []byte{0xEF, 0xBB, 0xBF}, id), enc("a: é😀", []byte{0xFF, 0xFE}, u16le),
		enc("a: é😀", []byte{0xFE, 0xFF}, u16be), enc("a: é😀", []byte{0xFF, 0xFE, 0, 0}, u32le),
		enc("a: é😀", []byte{0, 0, 0xFE, 0xFF}, u32be), enc("a: b", []byte{0xFF, 0xFE}, func(s string) []byte { return append(u16le(s), 0x41) }),
		append(enc("a: b", []byte{0xFF, 0xFE}, u16le), 0x00, 0xD8), append(enc("a: ", []byte{0xFF, 0xFE}, u16le), 0x00, 0xDC, 0x41, 0x00),
		append(enc("a: ", []byte{0xFF, 0xFE, 0, 0}, u32le), 0x00, 0xD8, 0, 0), append(enc("a: ", []byte{0xFF, 0xFE, 0, 0}, u32le), 0, 0, 0x11, 0),
		append(enc("a: ", []byte{0xFF, 0xFE, 0, 0}, u32le), 0x41, 0),
		[]byte("a: \xC3\x28"), []byte("a: \xE2\x82"), []byte("a: \xED\xA0\x80"), []byte("a: \xFF"), []byte("\xEF\xBB"),
		[]byte("\xEF\xBB\xBFa: \xC3\x28"), []byte("\xEF\xBB\xBFa: \xE2\x82x\xF0\x9F\x98"), []byte("\xEF\xBB\xBFa: \xED\xA0\x80\xF4\x90\x80\x80"),
		[]byte("\xEF\xBB\xBF\xEF\xBB\xBFa: 1"), []byte("\xFF\xFE"), []byte("\xFE\xFF"), []byte("\xFF\xFE\x00"),
		[]byte("\xEF\xBB\xBF"), []byte("a"), []byte("\xFF"),
	)
	// Random fuzz over YAML-significant characters.
	rng := rand.New(rand.NewSource(38))
	alphabet := []string{"-", "?", ":", ",", "[", "]", "{", "}", "#", "&", "*", "!", "|", ">", "'", "\"", "%", "@", "`",
		" ", " ", " ", "\n", "\n", "\t", "a", "b", "1", ".", "~", "\\", "x: ", "- ", "? ", "\n  ", "!!", "&a ", "*a",
		"---", "...", "\"\\u", "0x", "é", "\r\n", "<<"}
	for range 3000 {
		var b strings.Builder
		n := 1 + rng.Intn(14)
		for range n {
			b.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		out = append(out, []byte(b.String()))
	}
	// Mutations of structured documents.
	seeds := []string{
		"a: 1\nb: [x, y]\nc:\n  - d: e\n    f: |\n      g\n", "- a\n- {b: c, d: [e, f]}\n- ? g\n  : h\n",
		"x: &a {y: z}\nw: *a\n", "k: \"q\\tr\"\nl: 'm''n'\n", "%YAML 1.2\n---\na: !!str 1\n...\n",
	}
	for range 2000 {
		s := []byte(seeds[rng.Intn(len(seeds))])
		for range 1 + rng.Intn(3) {
			i := rng.Intn(len(s) + 1)
			switch rng.Intn(3) {
			case 0: // insert
				ins := alphabet[rng.Intn(len(alphabet))]
				s = append(s[:i], append([]byte(ins), s[i:]...)...)
			case 1: // delete
				if i < len(s) {
					s = append(s[:i], s[i+1:]...)
				}
			case 2: // replace
				if i < len(s) {
					s[i] = alphabet[rng.Intn(len(alphabet))][0]
				}
			}
		}
		out = append(out, append([]byte(nil), s...))
	}
	return out
}

func utf16Encode(s string) []uint16 {
	var u []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			u = append(u, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			u = append(u, uint16(r))
		}
	}
	return u
}

// jsonDocs is the corpus for the strict JSON reader and the JSON writers.
func jsonDocs() [][]byte {
	docs := []string{
		`{}`, `[]`, `null`, `true`, `false`, `0`, `-0`, `1`, `-1`, `1.5`, `1e5`, `1E+5`, `1e-5`, `-0.0e0`,
		`123456789012345678901234567890`, `"x"`, `""`, ` {} `, "\t{}\r\n", `{"a":1}`, `{"a":1,"b":[1,2,{"c":null}]}`,
		`{"n":4}`, `{"n":4.0}`, `{"n":4e0}`, `{"n":4.5}`, `{"n":"4"}`, `{"n":-2147483648}`, `{"n":2147483647}`,
		`{"n":2147483648}`, `{"n":-2147483649}`, `{"n":1e400}`, `{"n":1e-400}`, `{"n":-0}`, `{"n":40e-1}`,
		`{"n":4.0000000000000001}`, `{"n":null}`, `{"m":4}`, `{"n":21474836470e-1}`,
		`{"a":1,"a":2}`, `{"a":1,"\u0061":2}`, `{"a":{"b":1,"b":2}}`, `[{"a":1},{"a":1}]`,
		`{"a":1,}`, `[1,]`, `[1,,2]`, `{"a":1}//c`, `/*c*/{}`, `{"a":1} x`, `{"a" 1}`, `{a:1}`, `{'a':1}`,
		`[01]`, `[1.]`, `[.5]`, `[+1]`, `[-]`, `[1e]`, `[1e+]`, `[0x1]`, `[NaN]`, `[Infinity]`, `[tru]`, `[truex]`,
		`[nul]`, `["\x"]`, `["\u12"]`, `["\u12G4"]`, "[\"\t\"]", "[\"a\nb\"]", `["\ud800"]`, `["\udc00"]`,
		`["\ud83d\ude00"]`, `["\ud83d"]`, `["\ud83dx"]`, `["\ud83d\u0041"]`, `["a\"b\\c\/d\b\f\n\r\t"]`,
		`["<>&'+` + "`" + `\u007f\u0080é😀\u2028\ufeff"]`, `{"é&<":"中文"}`, `["` + "\x7f" + `"]`,
		`["a` + "\u00e9" + `"]`, "\xEF\xBB\xBF{}", "\xEF\xBB\xBF\xEF\xBB\xBF{}", "", " ", "\xC3\x28", `["\xC3\x28"]`,
		"\xFF\xFE{\x00}\x00", `[1]]`, `[[1]`, `{"a":[}`, `{"a"}`, `{"a":}`, `{,}`, `[,]`,
		`[1.0, 1.50, 1e2, 1E-2, -0.0, 100]`, `{"\ud800":1,"\ud801":2}`, `{"\udc00":1,"\ufffd":2}`, `{"\ud800":1,"\ud800":2}`, `{"\ud800":1}`, `{"a":1,"\ud800":2}`, `{"\ud800":1,"a":2}`, `{"a":{"\udc00":1}}`, `[{"\ud800":1},{"\ud800":1}]`, `{"a":{"b":{"c":[[],{}]}},"d":[]}`,
	}
	var out [][]byte
	for _, d := range docs {
		out = append(out, []byte(d))
	}
	for _, n := range []int{63, 64, 65} {
		out = append(out, []byte(strings.Repeat("[", n)+strings.Repeat("]", n)))
		out = append(out, []byte(strings.Repeat(`{"a":`, n)+"1"+strings.Repeat("}", n)))
	}
	for _, s := range writerStrings() {
		out = append(out, MarshalJSONCompact([]any{s}))
		o := NewObject()
		o.Set(s, []any{s, NewObject()})
		out = append(out, MarshalJSONCompact(o))
	}
	return out
}
