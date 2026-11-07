package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// utf8Corpus is the corpus testdata/dotnet_utf8_errors.json was made from:
// a C# harness over lip v0.38.0-beta.1's Lip.Core ran YamlJson.Parse(bytes)
// and PackageFileParser.ParseJson(bytes, "lip.json") on each input and
// recorded the message of the DecoderFallbackException under lip's "not
// valid UTF-8" message ("" when the input failed otherwise or not at all).
func utf8Corpus() [][]byte {
	pad := func(n int) []byte {
		if n == 0 {
			return nil
		}
		if n < 3 {
			return bytes.Repeat([]byte("x"), n)
		}
		return append([]byte("a: "), bytes.Repeat([]byte("x"), n-3)...)
	}
	var cases [][]byte
	add := func(parts ...[]byte) { cases = append(cases, bytes.Join(parts, nil)) }
	bad := [][]byte{
		{0xFF}, {0xFE}, {0x80}, {0xBF}, {0xC0, 0x80}, {0xC1, 0xBF}, {0xC2}, {0xC2, 0x41}, {0xE0, 0x80, 0x80},
		{0xE0, 0x9F, 0x80}, {0xE2, 0x82}, {0xE2, 0x82, 0x41}, {0xE2, 0x41}, {0xED, 0xA0, 0x80}, {0xED, 0xBF, 0xBF},
		{0xF0, 0x80, 0x80, 0x80}, {0xF0, 0x8F, 0xBF, 0xBF}, {0xF0, 0x9F, 0x98}, {0xF0, 0x9F, 0x98, 0x41}, {0xF0, 0x9F, 0x41},
		{0xF4, 0x90, 0x80, 0x80}, {0xF5, 0x80}, {0xF8, 0x88, 0x80, 0x80, 0x80}, {0xFF, 0xFE}, {0xC2, 0xC2, 0x80},
		{0x80, 0x80, 0x80}, {0xE2, 0x82, 0xFF},
	}
	for _, b := range bad {
		add(pad(0), b)
		add(pad(5), b)
		add(pad(5), b, []byte("yz\n"))
	}
	// Around the boundaries of StreamReader's 1024-byte buffers.
	for _, n := range []int{126, 127, 128, 129, 1020, 1021, 1022, 1023, 1024, 1025, 2045, 2046, 2047, 2048, 4095, 4096, 4097, 5000} {
		add(pad(n), []byte{0xFF})
		add(pad(n), []byte{0xE2, 0x82, 0x41})
		add(pad(n), []byte{0xF0, 0x9F, 0x98, 0x41})
		add(pad(n), []byte{0xE2, 0x82})
		add(pad(n), []byte{0xF0, 0x9F, 0x98})
		add(pad(n), []byte{0xE2, 0x82, 0xAC}, []byte("x"), []byte{0xFF})
		add(pad(n), []byte{0xC0, 0x80})
		add(pad(n), []byte{0xED, 0xA0, 0x80})
	}
	bom := []byte{0xEF, 0xBB, 0xBF}
	add(bom, pad(5), []byte{0xFF})
	add(bom, pad(1022), []byte{0xFF})
	add(bom, pad(1021), []byte{0xE2, 0x82, 0x41})
	add([]byte{0xEF, 0xBB}, pad(5), []byte{0xFF})
	add([]byte{0xEF}, []byte{0xFF})
	add([]byte{0xEF, 0xBB})
	add([]byte{0xFF, 0xFE}, []byte("a\x00:\x00 \x00b\x00"))
	add([]byte{0xFE, 0xFF}, []byte("\x00a\x00:\x00 \x00b"))
	add([]byte{0xFE}, pad(5))
	add(pad(10))
	add([]byte("a: \xE2\x82\xAC\n"))
	return cases
}

func TestDotnetUTF8Errors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "dotnet_utf8_errors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct{ YAML, JSON []string }
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	cases := utf8Corpus()
	if len(golden.YAML) != len(cases) || len(golden.JSON) != len(cases) {
		t.Fatalf("golden has %d/%d cases, corpus %d", len(golden.YAML), len(golden.JSON), len(cases))
	}
	cause := func(err error, want string) string {
		var de *DecoderError
		if !errors.As(err, &de) {
			return ""
		}
		if err.Error() != want {
			t.Errorf("message %q, want %q", err.Error(), want)
		}
		return de.Error()
	}
	for i, in := range cases {
		_, err := ParseYAML(in, "")
		if got := cause(err, "The file is not valid UTF-8 text. Save it with the UTF-8 encoding."); got != golden.YAML[i] {
			t.Errorf("ParseYAML(case %d, % x...) cause %q, want %q", i, in[:min(len(in), 8)], got, golden.YAML[i])
		}
		_, err = ParseJSON(in, "lip.json")
		if got := cause(err, "Invalid lip.json: not valid UTF-8."); got != golden.JSON[i] {
			t.Errorf("ParseJSON(case %d, % x...) cause %q, want %q", i, in[:min(len(in), 8)], got, golden.JSON[i])
		}
	}
}
