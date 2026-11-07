package doc

import (
	"bufio"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The testdata/*.jsonl files hold lip's own results for a corpus of inputs,
// produced by running lip 0.38.0-beta.1's C# code (YamlJson.Serialize,
// YamlJson.Parse, PackageFileParser.ParseJson / IntegerMember and
// JsonNode.ToJsonString) over it.

func readGolden(t *testing.T, name string) []map[string]string {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var recs []map[string]string
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r map[string]string
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		recs = append(recs, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return recs
}

func unb64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGoldenSerialize(t *testing.T) {
	for _, r := range readGolden(t, "serialize.jsonl") {
		v, err := ParseJSON([]byte(r["in"]), "in.json")
		if err != nil {
			t.Fatalf("ParseJSON(%q): %v", r["in"], err)
		}
		if got := string(MarshalYAML(v)); got != r["out"] {
			t.Errorf("MarshalYAML(%s)\n got: %q\nwant: %q", r["in"], got, r["out"])
		}
	}
}

func TestGoldenParseYAML(t *testing.T) {
	fails := 0
	for _, r := range readGolden(t, "parse.jsonl") {
		in := unb64(t, r["in"])
		v, err := ParseYAML(in, "x.yaml")
		var got string
		switch r["kind"] {
		case "OK":
			if err != nil {
				got = "error: " + err.Error()
			} else {
				got = string(MarshalJSONCompact(v))
			}
		case "ERR":
			var ye *YAMLError
			if errors.As(err, &ye) {
				got = ye.Message
			} else {
				got = "no error: " + string(MarshalJSONCompact(v))
			}
		case "CRASH":
			// lip throws an exception it does not catch; any error will do,
			// but not one from a bug in the port.
			if err != nil && !strings.HasPrefix(err.Error(), "x.yaml: internal error") {
				continue
			}
			got = "no error: " + string(MarshalJSONCompact(v))
		}
		if got != r["out"] {
			fails++
			if fails <= 40 {
				t.Errorf("ParseYAML(%q) [%s]\n got: %s\nwant: %s", in, r["kind"], got, r["out"])
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d mismatches", fails)
	}
}

func TestGoldenParseJSON(t *testing.T) {
	for _, r := range readGolden(t, "parsejson.jsonl") {
		in := unb64(t, r["in"])
		v, err := ParseJSON(in, "x.json")
		if r["kind"] == "CRASH" {
			// An unpaired surrogate escape: in a property name, ParseJson
			// throws an exception lip does not catch, and ParseJSON fails
			// too; in a string value, JsonDocument accepts it (the harness
			// failed later, turning it into a .NET string) and ParseJSON
			// reads it as U+FFFD.
			inParse := strings.Contains(r["out"], "IN-PARSEJSON")
			if (err != nil) != inParse {
				t.Errorf("ParseJSON(%q) = %v; lip: %s", in, err, r["out"])
			}
			continue
		}
		if r["kind"] != "OK" {
			if err == nil || err.Error() != r["out"] {
				t.Errorf("ParseJSON(%q) = %v, want error %q", in, err, r["out"])
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseJSON(%q): %v", in, err)
			continue
		}
		if got := string(MarshalJSONCompact(v)); got != r["compact"] {
			t.Errorf("compact(%q)\n got: %s\nwant: %s", in, got, r["compact"])
		}
		if got := strings.ReplaceAll(string(MarshalJSONIndented(v)), "\r\n", "\n"); got != r["indented"] {
			t.Errorf("indented(%q)\n got: %s\nwant: %s", in, got, r["indented"])
		}
		want := r["int"]
		got := "-"
		if o, ok := v.(*Object); ok {
			got = "none"
			if n, ok := o.Get("n"); ok {
				if i, ok := IntegerMember(n); ok {
					got = jsonInt(i)
				}
			}
		}
		if got != want {
			t.Errorf("IntegerMember(%q) = %s, want %s", in, got, want)
		}
	}
}

func jsonInt(i int64) string { return strconv.FormatInt(i, 10) }
