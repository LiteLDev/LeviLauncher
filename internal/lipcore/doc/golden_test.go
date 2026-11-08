package doc

import (
	"bufio"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The testdata/*.jsonl files hold lip's own results for a corpus of inputs,
// produced by running lip's C# code (TomlJson.Serialize, TomlJson.Parse,
// JsonFile.Parse / SerializeToUtf8Bytes of lip main; PackageFileParser.
// ParseJson / IntegerMember and JsonNode.ToJsonString of lip 0.38.0-beta.1)
// over it (diffgen_test.go).

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

func TestGoldenSerializeTOML(t *testing.T) {
	fails := 0
	for _, r := range readGolden(t, "tomlserialize.jsonl") {
		v, err := ParseJSON([]byte(r["in"]), "in.json")
		if err != nil {
			t.Fatalf("ParseJSON(%q): %v", r["in"], err)
		}
		out, err := MarshalTOML(v)
		var got, want string
		if err != nil {
			got = "error"
		} else {
			got = string(out)
		}
		if r["kind"] == "OK" {
			want = r["out"]
		} else {
			want = "error"
		}
		if got != want {
			fails++
			if fails <= 40 {
				t.Errorf("MarshalTOML(%s)\n got: %q\nwant: %q", r["in"], got, want)
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d mismatches", fails)
	}
}

func TestGoldenParseTOML(t *testing.T) {
	fails := 0
	for _, r := range readGolden(t, "tomlparse.jsonl") {
		in := unb64(t, r["in"])
		v, err := ParseTOML(in)
		var got, want string
		switch {
		case err == nil:
			got = string(MarshalJSONCompact(v))
		case strings.HasPrefix(err.Error(), "The file is not valid TOML: "):
			// Tomlyn's own messages are not reproduced; only that it fails.
			got = "syntax error"
		default:
			got = "error: " + err.Error()
		}
		switch {
		case r["kind"] == "OK":
			want = r["out"]
		case r["kind"] == "ERR" && strings.HasPrefix(r["out"], "TomlFormatException: The file is not valid TOML: "):
			want = "syntax error"
		case r["kind"] == "ERR" && strings.HasPrefix(r["out"], "TomlFormatException: "):
			want = "error: " + strings.TrimSpace(strings.TrimPrefix(r["out"], "TomlFormatException: "))
		default:
			// lip fails with an exception it does not catch; any error will do.
			if err != nil {
				continue
			}
			want = "an error: " + r["out"]
		}
		if got != want {
			fails++
			if fails <= 40 {
				t.Errorf("ParseTOML(%q)\n got: %s\nwant: %s", in, got, want)
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d mismatches", fails)
	}
}

func TestGoldenJSONFile(t *testing.T) {
	for _, r := range readGolden(t, "jsonfile.jsonl") {
		in := unb64(t, r["in"])
		v, err := ParseJSONFile(in)
		if r["kind"] != "OK" {
			// JsonFile.Parse accepts an unpaired surrogate escape in a string;
			// only the harness's writing of it failed.
			if strings.Contains(strings.ToLower(string(in)), `\ud`) {
				continue
			}
			if err == nil {
				t.Errorf("ParseJSONFile(%q) succeeded; lip: %s", in, r["out"])
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseJSONFile(%q): %v", in, err)
			continue
		}
		if got := string(MarshalJSONFile(v)); got != r["out"] {
			t.Errorf("MarshalJSONFile(%q)\n got: %q\nwant: %q", in, got, r["out"])
		}
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
