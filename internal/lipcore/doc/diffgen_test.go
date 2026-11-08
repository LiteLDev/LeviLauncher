package doc

// Differential golden generator: runs the corpus through lip's own C# code
// and writes testdata/*.jsonl.gz. It only runs when LIPCORE_DOC_HARNESS names
// a harness executable: a .NET console program referencing Lip.Core of lip
// main (Tomlyn 2.10.2) that reads base64 inputs from stdin, one per line, and
// per input writes "OK <result>", "ERR <base64 message>" (a
// TomlFormatException, JsonException or ArgumentException) or "CRASH <base64
// other exception>", with <result> for each mode (args[0]):
//
//	tomlserialize  base64 TomlJson.SerializeToUtf8Bytes(JsonDocument.Parse(input).RootElement)
//	tomlparse      base64 TomlJson.Parse(input).RootElement.GetRawText()
//	jsonfile       base64 JsonFile.SerializeToUtf8Bytes(JsonFile.Parse(input).RootElement)
//
// testdata/parsejson.jsonl.gz (PackageFileParser.ParseJson, which lip main
// did not change) was made by the harness of lip v0.38.0-beta.1.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	// --- serialize TOML ---
	var serIn [][]byte
	for _, s := range writerStrings() {
		o := NewObject()
		o.Set(s, s)
		o.Set("list", []any{s})
		inner := NewObject()
		inner.Set("k", s)
		o.Set("nested", []any{inner, []any{s}})
		table := NewObject()
		table.Set(s, inner)
		o.Set("t", table)
		serIn = append(serIn, MarshalJSONCompact(o))
	}
	serIn = append(serIn, writerDocs()...)
	// Random trees, to mix inline values, tables and arrays of tables.
	rng := rand.New(rand.NewSource(380))
	strs := writerStrings()
	var gen func(depth int) any
	gen = func(depth int) any {
		switch k := rng.Intn(10); {
		case depth > 4 || k < 4:
			switch rng.Intn(7) {
			case 0:
				return json.Number(fmt.Sprint(rng.Intn(2000) - 1000))
			case 1:
				return rng.Intn(2) == 0
			case 2:
				if rng.Intn(10) == 0 {
					return nil
				}
				return json.Number(fmt.Sprintf("%g", rng.NormFloat64()*1e6))
			default:
				return strs[rng.Intn(len(strs))]
			}
		case k < 7:
			n := rng.Intn(4)
			a := []any{}
			objects := rng.Intn(2) == 0
			for range n {
				if objects {
					o := NewObject()
					for range rng.Intn(3) {
						o.Set(strs[rng.Intn(len(strs))], gen(depth+1))
					}
					a = append(a, o)
				} else {
					a = append(a, gen(depth+1))
				}
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
	for range 1500 {
		o := NewObject()
		for range 1 + rng.Intn(4) {
			o.Set(strs[rng.Intn(len(strs))], gen(0))
		}
		serIn = append(serIn, MarshalJSONCompact(o))
	}
	res := runHarness(t, exe, "tomlserialize", serIn)
	var recs []map[string]any
	var written [][]byte
	for i, r := range res {
		kind, payload, _ := strings.Cut(r, " ")
		rec := map[string]any{"in": string(serIn[i]), "kind": kind}
		if kind == "OK" {
			out, _ := base64.StdEncoding.DecodeString(payload)
			rec["out"] = string(out)
			written = append(written, out)
		}
		recs = append(recs, rec)
	}
	writeJSONL(t, "tomlserialize.jsonl", recs)

	// --- parse TOML ---
	parseIn := append(written, tomlDocs()...)
	recs = nil
	res = runHarness(t, exe, "tomlparse", parseIn)
	for i, r := range res {
		kind, payload, _ := strings.Cut(r, " ")
		rec := map[string]any{"in": base64.StdEncoding.EncodeToString(parseIn[i]), "kind": kind}
		if payload != "" {
			rec["out"] = b64(payload)
		}
		recs = append(recs, rec)
	}
	writeJSONL(t, "tomlparse.jsonl", recs)

	// --- JSON files ---
	jin := jsonDocs()
	recs = nil
	res = runHarness(t, exe, "jsonfile", jin)
	for i, r := range res {
		kind, payload, _ := strings.Cut(r, " ")
		rec := map[string]any{"in": base64.StdEncoding.EncodeToString(jin[i]), "kind": kind}
		if kind == "OK" {
			rec["out"] = b64(payload)
		}
		recs = append(recs, rec)
	}
	writeJSONL(t, "jsonfile.jsonl", recs)
}

// tomlDocs is the corpus of TOML files for the reader.
func tomlDocs() [][]byte {
	docs := []string{
		"", "\n", "# c", "a = 1", "a=1", "a = 1 # c", "a = 1\n", "a = 1\r\n", "a = 1\r", "a = 1\nb = 2",
		"\"\" = 1", "'' = 1", "\"a b\" = 1", "'a.b' = 1", "a.b.c = 1", "a . b = 1", "a.\"b\".'c' = 1", "1 = 1",
		"1.2 = 3", "-_ = 1", "a-b_c = 1", "é = 1", "a = 1\na = 2", "a.b = 1\na = 2", "a = 1\na.b = 2",
		"a = true", "a = false", "a = True", "a = tru", "a = truex", "a = -", "a = +", "a = ", "a =", "= 1",
		"a = 0", "a = -0", "a = +0", "a = 00", "a = 01", "a = 1_0", "a = 1__0", "a = _1", "a = 1_", "a = 0_1",
		"a = 9223372036854775807", "a = 9223372036854775808", "a = -9223372036854775808",
		"a = -9223372036854775809", "a = 0x0", "a = 0xff", "a = 0xFF", "a = 0Xff", "a = 0x_f", "a = 0xf_f",
		"a = 0x7FFFFFFFFFFFFFFF", "a = 0x8000000000000000", "a = 0xFFFFFFFFFFFFFFFF", "a = -0x1", "a = +0x1",
		"a = 0o0", "a = 0o777", "a = 0o8", "a = 0b0", "a = 0b102", "a = 0b1_0", "a = 0x", "a = 0o", "a = 0b",
		"a = 1.0", "a = -1.0", "a = +1.0", "a = 1.", "a = .1", "a = 1.e1", "a = 1e1", "a = 1E1", "a = 1e+1",
		"a = 1e-1", "a = 1e01", "a = 1e_1", "a = 1_0.0_1e1_0", "a = 0.0", "a = -0.0", "a = 00.1", "a = 01.1",
		"a = 1e400", "a = -1e400", "a = 1e-400", "a = 3.14159265358979323846", "a = 0.1", "a = 1e15", "a = 1e16",
		"a = 1e17", "a = 1e-4", "a = 1e-5", "a = 123456789012345678.0", "a = 5e-324", "a = 1.7976931348623157e308",
		"a = inf", "a = +inf", "a = -inf", "a = nan", "a = +nan", "a = -nan", "a = Inf", "a = NaN", "a = infinity",
		"a = 1979-05-27", "a = 1979-05-27T07:32:00Z", "a = 1979-05-27t07:32:00z", "a = 1979-05-27 07:32:00",
		"a = 1979-05-27T07:32:00.999999-07:00", "a = 1979-05-27T07:32:00+07:00", "a = 1979-05-27T07:32",
		"a = 07:32:00", "a = 07:32", "a = 07:32:00.5", "a = 1979-13-27", "a = 1979-02-30", "a = 2000-02-29",
		"a = 1900-02-29", "a = 1979-05-27T25:00:00", "a = 1979-05-27T07:60:00", "a = 1979-05-27T07:32:60",
		"a = 24:00", "a = 7:32", "a = 1979-5-27", "a = 1979-05-27T", "a = 1979-05-27 ", "a = [1979-05-27]",
		"a = \"\"", "a = \"x\"", "a = \"\\\"\"", "a = \"\\\\\"", "a = \"\\b\\t\\n\\f\\r\\e\"", "a = \"\\x41\"",
		"a = \"\\x4\"", "a = \"\\xZZ\"", "a = \"\\u00e9\"", "a = \"\\u00E9\"", "a = \"\\u12\"", "a = \"\\U0001F600\"",
		"a = \"\\U00110000\"", "a = \"\\uD800\"", "a = \"\\uDFFF\"", "a = \"\\q\"", "a = \"\\ \"", "a = \"\\\"",
		"a = \"x", "a = \"x\ny\"", "a = \"x\ty\"", "a = \"x\x01y\"", "a = \"x\x7fy\"", "a = \"x\u0085y\"",
		"a = \"é😀\"", "a = ''", "a = 'x'", "a = 'x\\y'", "a = 'x\ty'", "a = 'x\x01y'", "a = 'x\ny'", "a = 'x",
		"a = \"\"\"\"\"\"", "a = \"\"\"x\"\"\"", "a = \"\"\"\nx\"\"\"", "a = \"\"\"\r\nx\"\"\"", "a = \"\"\"\n\nx\"\"\"",
		"a = \"\"\"x\ny\"\"\"", "a = \"\"\"x\r\ny\"\"\"", "a = \"\"\"x\ry\"\"\"", "a = \"\"\"x\\\ny\"\"\"",
		"a = \"\"\"x\\\r\ny\"\"\"", "a = \"\"\"x\\ \ny\"\"\"", "a = \"\"\"x\\\t\n  \n  y\"\"\"",
		"a = \"\"\"x\\  y\"\"\"", "a = \"\"\"x\\\"\"\"", "a = \"\"\"\\\n\"\"\"", "a = \"\"\"x\"\"\"\"",
		"a = \"\"\"x\"\"\"\"\"", "a = \"\"\"x\"\"\"\"\"\"", "a = \"\"\"\"x\"\"\"", "a = \"\"\"\"\"x\"\"\"",
		"a = \"\"\"\"\"\"x\"\"\"", "a = \"\"\"x\ty\x01\"\"\"", "a = \"\"\"x", "a = '''x'''", "a = '''\nx'''",
		"a = '''x\ny'''", "a = '''x''''", "a = '''x'''''", "a = '''x''''''", "a = '''x\\'''", "a = '''x\x01'''",
		"a = []", "a = [ ]", "a = [1]", "a = [1,]", "a = [1,,]", "a = [,]", "a = [1 2]", "a = [\n]",
		"a = [\n1,\n2\n]", "a = [1 # c\n,2]", "a = [1, \"x\", true, [], {}]", "a = [[1], [2, [3]]]",
		"a = [{}]", "a = [{b = 1}, {c = 2}]", "a = [", "a = [1", "a = ]",
		"a = {}", "a = { }", "a = {b = 1}", "a = { b = 1, c = 2 }", "a = { b = 1, }", "a = { , }",
		"a = { b = 1,, }", "a = { b = 1 c = 2 }", "a = {\nb = 1\n}", "a = {\n  b = 1, # c\n  c = 2,\n}",
		"a = { b.c = 1, b.d = 2 }", "a = { b = 1, b = 2 }", "a = { b = {c = 1}, b.d = 2 }",
		"a = { b.c = 1, b = 2 }", "a = { b = [1, {c = 2}] }", "a = {", "a = { b = 1", "a = }",
		"[a]", "[a]\nb = 1", "[ a ]", "[a.b]", "[ a . b ]", "[a.\"b\"]", "[\"\"]", "[]", "[a", "a]", "[a]]",
		"[a] b = 1", "[a] # c", "[a]\n[a]", "[a]\n[b]\n[a]", "[a.b]\n[a]", "[a]\n[a.b]", "[a.b]\n[a.b]",
		"[a]\nb = 1\n[a.b]", "[a]\nb.c = 1\n[a.b]", "[a]\nb.c = 1\n[a.b.d]", "a.b = 1\n[a]", "a.b = 1\n[a.c]",
		"a.b.c = 1\n[a.b.d]", "a = 1\n[a]", "a = {}\n[a]", "a = {b = 1}\n[a.c]", "a = []\n[a]", "a = []\n[[a]]",
		"[a.b.c]\n[a]\nb.d = 1", "[a.b]\nx = 1\n[a]\nb.y = 2", "[a]\nb = 1\nb.c = 2",
		"[[a]]", "[[a]]\n[[a]]", "[[ a ]]", "[ [a] ]", "[[a]", "[[a]]]", "[[a]]\n[a]", "[a]\n[[a]]",
		"[[a]]\n[a.b]\nc = 1\n[[a]]\n[a.b]\nc = 2", "[[a.b]]\n[a]", "[[a.b]]\n[[a]]", "[[a]]\n[[a.b]]\n[[a.b]]",
		"[[a]]\nb.c = 1\n[a.b]", "[[a]]\n[a.b]\n[a.b]", "[a]\n[[a.b]]\n[a]", "a = [{b = 1}]\n[[a]]",
		"a = [{b = 1}]\n[a.c]", "[[a]]\nb = 1\n[[a]]\nb = 2\n[a.c]\nd = 3",
		"format = 4\nid = \"github.com/a/b\"\nversion = \"1.0.0\"\n\n[[variants.\"\"]]\nplatform = \"server\"\nruntime = \"win-x64\"\n",
		"format = 4\nid = \"x\"\n[info]\nname = \"n\"\ntags = [\"a\"]\n[variants]\n\"\" = [{ platform = \"client\", runtime = \"win-x64\", deps = { \"github.com/a/b\" = \"^1.0.0\" } }]\n",
		"a = 1 # \x7f", "# \x01", "# \t", "a = 1 # é😀", "a = \"\u2028\"", "a = \"\ufeff\"",
	}
	var out [][]byte
	for _, d := range docs {
		out = append(out, []byte(d))
	}
	for _, n := range []int{62, 63, 64, 65, 66} {
		out = append(out, []byte("a = "+strings.Repeat("[", n)+strings.Repeat("]", n)))
		out = append(out, []byte("a = "+strings.Repeat("[", n)+"1"+strings.Repeat("]", n)))
		out = append(out, []byte("a = "+strings.Repeat("{b = ", n)+"1"+strings.Repeat("}", n)))
		keys := make([]string, n)
		for i := range keys {
			keys[i] = "k"
		}
		out = append(out, []byte(strings.Join(keys, ".")+" = 1"))
		out = append(out, []byte("["+strings.Join(keys, ".")+"]"))
		out = append(out, []byte("[["+strings.Join(keys, ".")+"]]"))
	}
	// Byte order marks and encodings.
	enc := func(s string, bom []byte, f func(string) []byte) []byte {
		return append(append([]byte(nil), bom...), f(s)...)
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
	id := func(s string) []byte { return []byte(s) }
	out = append(out,
		enc("a = \"é😀\"", []byte{0xEF, 0xBB, 0xBF}, id), enc("a = \"é😀\"", []byte{0xFF, 0xFE}, u16le),
		enc("a = \"é😀\"", []byte{0xFE, 0xFF}, u16be), enc("a = \"é😀\"", []byte{0xFF, 0xFE, 0, 0}, u32le),
		[]byte("a = \"\xC3\x28\""), []byte("a = \"\xFF\""), []byte("\xEF\xBB\xBF\xEF\xBB\xBFa = 1"),
		[]byte("\xEF\xBB\xBF"), []byte("\xFF\xFE"),
	)
	// Random fuzz over TOML-significant characters.
	rng := rand.New(rand.NewSource(40))
	alphabet := []string{"=", "=", ".", ",", "[", "]", "{", "}", "[[", "]]", "#", "\"", "'", "\"\"\"", "'''", "\\",
		" ", " ", " ", "\n", "\n", "\t", "\r\n", "a", "b", "a", "b", "1", "0", "-", "+", "_", "x", "e", ":",
		"a = ", "b = ", "a.b = ", "[a]\n", "[[a]]\n", "[a.b]\n", "1979-05-27", "07:32", "inf", "nan", "true",
		"0x", "\\u", "\\n", "é", "1.5", "T"}
	for range 4000 {
		var b strings.Builder
		n := 1 + rng.Intn(14)
		for range n {
			b.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		out = append(out, []byte(b.String()))
	}
	// Mutations of structured documents.
	seeds := []string{
		"a = 1\nb = [\"x\", 'y']\n[c]\nd = { e = 1.5, f = [true] }\n",
		"[[a]]\nb = 1\n[a.c]\nd = \"\"\"\nx\\\n  y\"\"\"\n[[a]]\ne.f = -0x1F\n",
		"x.y = 1\nx.z = '''q'''\n[w]\nv = 1979-05-27\n",
		"format = 4\nid = \"github.com/a/b\"\nversion = \"1.0.0\"\n[[variants.\"\"]]\nplatform = \"server\"\nruntime = \"win-x64\"\ndeps = { \"github.com/c/d\" = \">=1.0.0\" }\n",
		"a = {\n  b = 1, # c\n  c = [\n    2,\n  ],\n}\n",
	}
	for range 3000 {
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

// writerStrings is the corpus of strings for the writers.
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
