package migrate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The files under testdata/golden were produced by lip 0.37.0 itself: a
// small .NET harness referencing Lip.Core (built from the v0.37.0 sources)
// called PackageManifestMigration.Migrate for format 1/2 inputs and
// serialized the result with System.Text.Json (as `lip migrate` does); for
// format 3 inputs it ran ParseV3's Scriban rendering pass; templates.jsonl
// was rendered with Scriban 7.5.0 against a Dictionary<string, object>
// model; ranges.json holds SemVersionRange.Parse/ParseNpm(...).ToString()
// for every dependency range in the inputs (null when lip rejects it).
// RuntimeInformation.RuntimeIdentifier was "win-x64".

const testRID = "win-x64"

// useGoldenRanges replaces NormalizeRange with lip's recorded results so the
// tests do not depend on package semver.
func useGoldenRanges(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "golden", "ranges.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table map[string]*string
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	orig := NormalizeRange
	t.Cleanup(func() { NormalizeRange = orig })
	NormalizeRange = func(s string, npm bool) (string, error) {
		key := "parse:" + s
		if npm {
			key = "npm:" + s
		}
		r, ok := table[key]
		if !ok {
			t.Errorf("NormalizeRange(%q, npm=%v) not in golden table", s, npm)
			return "", errors.New("unknown range")
		}
		if r == nil {
			return "", errors.New("invalid range")
		}
		return *r, nil
	}
}

// canonical re-encodes a JSON document compactly, keeping member order.
func canonical(t *testing.T, data []byte) string {
	t.Helper()
	v, err := parseJSON(data)
	if err != nil {
		t.Fatalf("invalid JSON %s: %v", data, err)
	}
	var buf bytes.Buffer
	encodeJSON(&buf, v)
	return buf.String()
}

// lipVersions maps versions in the inputs to SemVersion.Parse(v,
// SemVersionStyles.Any).ToString(); ToV3 leaves that to manifest.Parse.
var lipVersions = map[string]string{"v1.0": "1.0.0"}

// lipGlobs maps patterns to DotNet.Glob's Glob.Parse(p).ToString(), which
// lip writes for preserve/remove files; ToV3 emits the rendered pattern and
// leaves glob handling to the manifest/glob packages.
var lipGlobs = map[string]string{"[": "[[]"}

func normalizeVersionForCompare(t *testing.T, out []byte) []byte {
	t.Helper()
	v, err := parseJSON(out)
	if err != nil || v.kind != jObject {
		return out
	}
	for i, m := range v.obj {
		if m.key == "version" && m.val.kind == jString {
			if n, ok := lipVersions[m.val.str]; ok {
				v.obj[i].val = jstr(n)
			}
		}
	}
	if variants, ok := v.get("variants"); ok && variants.kind == jArray {
		for _, vr := range variants.arr {
			for _, key := range []string{"preserve_files", "remove_files"} {
				if l, ok := vr.get(key); ok && l.kind == jArray {
					for _, e := range l.arr {
						if n, ok := lipGlobs[e.str]; ok {
							e.str = n
						}
					}
				}
			}
		}
	}
	var buf bytes.Buffer
	encodeJSON(&buf, v)
	return buf.Bytes()
}

func TestGoldenManifests(t *testing.T) {
	useGoldenRanges(t)
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		n++
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "golden", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ToV3(raw, testRID)
			if err != nil {
				t.Fatalf("ToV3: %v", err)
			}
			if g, w := canonical(t, got), canonical(t, want); g != w {
				t.Errorf("ToV3 mismatch\n got: %s\nwant: %s", g, w)
			}
		})
	}
	if n < 8 {
		t.Fatalf("expected golden manifests, found %d", n)
	}
}

// deferredErrors are lip migration failures that ToV3 leaves to
// manifest.Parse (which validates the same fields when parsing the output).
var deferredErrors = []string{
	"Invalid package tag",
	"Invalid destination path",
}

func TestGoldenCases(t *testing.T) {
	useGoldenRanges(t)
	f, err := os.Open(filepath.Join("testdata", "golden", "cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	line := 0
	for sc.Scan() {
		line++
		var c struct {
			Input string          `json:"input"`
			Want  json.RawMessage `json:"want"`
			Err   *string         `json:"err"`
		}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		got, err := ToV3([]byte(c.Input), testRID)
		if c.Err != nil {
			if err != nil {
				continue
			}
			deferred := false
			for _, d := range deferredErrors {
				deferred = deferred || strings.Contains(*c.Err, d)
			}
			if !deferred {
				t.Errorf("line %d: ToV3(%s) succeeded with %s, lip failed: %s", line, c.Input, got, *c.Err)
			}
			continue
		}
		if err != nil {
			t.Errorf("line %d: ToV3(%s): %v", line, c.Input, err)
			continue
		}
		if g, w := canonical(t, normalizeVersionForCompare(t, got)), canonical(t, c.Want); g != w {
			t.Errorf("line %d: ToV3(%s)\n got: %s\nwant: %s", line, c.Input, g, w)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if line < 50 {
		t.Fatalf("expected golden cases, found %d", line)
	}
}

func TestGoldenTemplates(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "golden", "templates.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		var c struct {
			Template string  `json:"template"`
			Model    string  `json:"model"`
			Want     *string `json:"want"`
			Err      *string `json:"err"`
		}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		model, err := parseJSON([]byte(c.Model))
		if err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		got, err := renderTemplate(c.Template, model.get)
		switch {
		case c.Err != nil && err == nil:
			t.Errorf("line %d: render(%q) = %q, Scriban failed with %s", line, c.Template, got, *c.Err)
		case c.Err == nil && err != nil:
			t.Errorf("line %d: render(%q): %v, Scriban rendered %q", line, c.Template, err, *c.Want)
		case c.Want != nil && got != *c.Want:
			t.Errorf("line %d: render(%q) = %q, want %q", line, c.Template, got, *c.Want)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if line < 100 {
		t.Fatalf("expected golden templates, found %d", line)
	}
}

func TestRenderTemplateEdgeCases(t *testing.T) {
	model, err := parseJSON([]byte(`{"a":[1,2,3],"s":"abc","e":"😀"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ tmpl, want string }{
		{"{{ a[-0] }}", "1"},
		{"{{ s[-0] }}", "a"},
		{"{{ a[-3] }}", "1"},
		{"{{ s[-1] }}", "c"},
		{`{{ "😀" }}`, "\U0001F600"},
		{`{{ "A\ud83d" }}`, "A�"},
		{"{{ e }}", "\U0001F600"},
		{"{{ e.size }}", "2"},
	} {
		got, err := renderTemplate(tt.tmpl, model.get)
		if err != nil || got != tt.want {
			t.Errorf("render(%q) = %q, %v; want %q", tt.tmpl, got, err, tt.want)
		}
	}
}

func TestRenderTemplateUnsupported(t *testing.T) {
	model, _ := parseJSON([]byte(`{"version":"1.0.0","tags":["a"]}`))
	for _, tmpl := range []string{
		"{{ version | string.upcase }}",
		"{{ 1 + 2 }}",
		"{{ ver-sion }}",
		"{{ if true }}x{{ end }}",
		"{{ string }}",
		"{{ math.abs -1 }}",
		"{{ tags[1-1] }}",
		"{{ 1.5 }}",
		"{{ version[\"x\"] }}",
		"{{ this }}",
	} {
		_, err := renderTemplate(tmpl, model.get)
		if !errors.Is(err, ErrUnsupportedTemplate) {
			t.Errorf("render(%q) error = %v, want ErrUnsupportedTemplate", tmpl, err)
		}
	}
}

// TestRenderV3NestedAndUnknown documents the decisions for expressions
// beyond plain top-level variables: nested members resolve like Scriban's
// JsonElement accessor, unknown variables render as "", and member access
// on an unknown variable is an error (Scriban: "Cannot get the member ...
// for a null object").
func TestRenderV3NestedAndUnknown(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"nested", `{"format_version":3,"info":{"name":"N"},"s":"{{info.name}}"}`, `{"format_version":3,"info":{"name":"N"},"s":"N"}`, false},
		{"unknown", `{"format_version":3,"s":"[{{nope}}]"}`, `{"format_version":3,"s":"[]"}`, false},
		{"missing member", `{"format_version":3,"info":{},"s":"[{{info.nope}}]"}`, `{"format_version":3,"info":{},"s":"[]"}`, false},
		{"member of unknown", `{"format_version":3,"s":"{{nope.x}}"}`, "", true},
		{"model is unrendered original", `{"format_version":3,"a":"{{b}}","b":"x","c":"{{a}}"}`, `{"format_version":3,"a":"x","b":"x","c":"{{b}}"}`, false},
		{"keys not rendered", `{"format_version":3,"d":{"{{x}}":"{{x}}"},"x":"1"}`, `{"format_version":3,"d":{"{{x}}":"1"},"x":"1"}`, false},
		{"whitespace tolerant", `{"format_version":3,"version":"1.2.3","tooth":"t","s":"{{ tooth }}@{{  version\t}}"}`, `{"format_version":3,"version":"1.2.3","tooth":"t","s":"t@1.2.3"}`, false},
		{"numbers kept", `{"format_version":3,"n":1.50,"m":[1e3,-0]}`, `{"format_version":3,"n":1.50,"m":[1e3,-0]}`, false},
		{"duplicate keys", `{"format_version":3,"a":{"b":1,"b":2}}`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToV3([]byte(tt.in), testRID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ToV3 = %s, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("ToV3 =\n %s\nwant\n %s", got, tt.want)
			}
		})
	}
}

// TestToV3KeyOrder checks with encoding/json that dependency order survives
// (including lip's first-position/last-value handling of keys that collapse
// to the same package id).
func TestToV3KeyOrder(t *testing.T) {
	orig := NormalizeRange
	t.Cleanup(func() { NormalizeRange = orig })
	NormalizeRange = func(s string, npm bool) (string, error) { return "=" + s, nil }

	in := `{"format_version":2,"tooth":"github.com/a/b","version":"1.0.0",
	  "info":{"name":"n","description":"d","author":"a","tags":[]},
	  "dependencies":{"github.com/z/z":"1","github.com/a/a":"2","github.com/m/m":"3","github.com/a/a#":"4","Bad Path":"5"}}`
	out, err := ToV3([]byte(in), testRID)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Variants []struct {
			Dependencies json.RawMessage `json:"dependencies"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Variants) != 1 {
		t.Fatalf("variants = %d", len(doc.Variants))
	}
	dec := json.NewDecoder(bytes.NewReader(doc.Variants[0].Dependencies))
	var keys, vals []string
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		k, _ := dec.Token()
		v, _ := dec.Token()
		keys = append(keys, k.(string))
		vals = append(vals, v.(string))
	}
	wantKeys := []string{"github.com/z/z", "github.com/a/a", "github.com/m/m"}
	wantVals := []string{"=1", "=4", "=3"}
	if strings.Join(keys, ",") != strings.Join(wantKeys, ",") || strings.Join(vals, ",") != strings.Join(wantVals, ",") {
		t.Errorf("dependencies = %v %v, want %v %v", keys, vals, wantKeys, wantVals)
	}
}

// TestToV3WildcardPlatformInsertsEmptyVariant: lip v0.38 inserts a variant
// without a platform whatever runtime reads it (rid is unused).
func TestToV3WildcardPlatformInsertsEmptyVariant(t *testing.T) {
	useGoldenRanges(t)
	in := `{"format_version":2,"tooth":"github.com/a/b","version":"1.0.0",
	  "info":{"name":"n","description":"d","author":"a","tags":[]},
	  "asset_url":"https://x/$(version).zip",
	  "platforms":[{"goos":"windows"},{"goos":"linux","goarch":"arm64"}]}`
	out, err := ToV3([]byte(in), "linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Variants []struct {
			Platform string `json:"platform"`
			Assets   []struct {
				Type string   `json:"type"`
				URLs []string `json:"urls"`
			} `json:"assets"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var platforms []string
	for _, v := range doc.Variants {
		platforms = append(platforms, v.Platform)
	}
	if got := strings.Join(platforms, ","); got != ",win-*,linux-arm64" {
		t.Errorf("platforms = %s", got)
	}
	if a := doc.Variants[1].Assets; len(a) != 1 || a[0].Type != "zip" || a[0].URLs[0] != "https://x/1.0.0.zip" {
		t.Errorf("assets = %+v", a)
	}
	if len(doc.Variants[0].Assets) != 0 {
		t.Errorf("inserted variant has assets: %+v", doc.Variants[0].Assets)
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{`{"format_version":1}`, 1, false},
		{`{"format_version":2,"x":[]}`, 2, false},
		{"\xef\xbb\xbf" + `{"format_version":3}`, 3, false},
		{`{"format_version":1,"format_version":3}`, 3, false},
		{`{"format_version":-0}`, 0, false},
		{`{"format_version":4}`, 4, false},
		{`{"format_version":3.0}`, 0, true},
		{`{"format_version":3e0}`, 0, true},
		{`{"format_version":"3"}`, 0, true},
		{`{"format_version":null}`, 0, true},
		{`{"format_version":2147483648}`, 0, true},
		{`{}`, 0, true},
		{`[]`, 0, true},
		{`{"format_version":3,}`, 0, true},
		{`{"format_version":3} x`, 0, true},
		{``, 0, true},
	}
	for _, tt := range tests {
		got, err := FormatVersion([]byte(tt.in))
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("FormatVersion(%q) = %d, %v; want %d, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestToV3Errors(t *testing.T) {
	for _, in := range []string{
		`{"format_version":4}`,
		`{"format_version":0}`,
		`not json`,
		`{"format_version":3,"s":"{{ nope.x }}"}`,
		strings.Repeat("[", 65) + strings.Repeat("]", 65),
		"{\"format_version\":3,\"s\":\"\xff\"}",
		`{"format_version":3,"s":"\ud800"}`,
		`{"format_version":3,"s":"\udc00x"}`,
		`{"format_version":3,"s":"\ud800A"}`,
	} {
		if out, err := ToV3([]byte(in), testRID); err == nil {
			t.Errorf("ToV3(%q) = %s, want error", in, out)
		}
	}
	if _, err := ToV3([]byte(`{"format_version":4}`), testRID); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("format 4 error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct{ in, want string }{
		{"3", "3"}, {"-0", "0"}, {"-5", "-5"}, {"3.50", "3.50"}, {"1e3", "1000"},
		{"1.5e1", "15"}, {"100e-2", "1.00"}, {"0.050", "0.050"}, {"-0.0", "0.0"},
		{"-1.50", "-1.50"}, {"9223372036854775808", "9223372036854775808"},
		{"1E+2", "100"},
	}
	for _, tt := range tests {
		got, err := formatNumber(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("formatNumber(%s) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, tt := range []struct{ in, want string }{
		{"1e28", "10000000000000000000000000000"},
		{"79228162514264337593543950335", "79228162514264337593543950335"},
		{"1234567890123456789012345678.9", "1234567890123456789012345678.9"},
	} {
		got, err := formatNumber(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("formatNumber(%s) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, in := range []string{"1e400", "123456789012345678901234567890123", "1e-30", "79228162514264337593543950336"} {
		if _, err := formatNumber(in); !errors.Is(err, ErrUnsupportedTemplate) {
			t.Errorf("formatNumber(%s) error = %v, want ErrUnsupportedTemplate", in, err)
		}
	}
}

func TestParsePackageID(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"github.com/a/b", "github.com/a/b", true},
		{"github.com/a/b#", "github.com/a/b", true},
		{"github.com/a/b#client", "github.com/a/b#client", true},
		{"github.com/a/b#ok/v_1", "github.com/a/b#ok/v_1", true},
		{"github.com/a/b#a\n", "github.com/a/b#a\n", true},
		{"github.com/a/b#Bad", "", false},
		{"github.com/a/b#a#b", "", false},
		{"Github.com/a/b", "", false},
		{"bad path", "", false},
		{"#x", "", false},
	}
	for _, tt := range tests {
		got, ok := parsePackageID(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parsePackageID(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

// migratedVariant decodes a variant of the output of ToFormat3.
type migratedVariant struct {
	Platform string `json:"platform"`
	Assets   []struct {
		Type       string `json:"type"`
		Placements []struct {
			Type string `json:"type"`
			Src  string `json:"src"`
			Dest string `json:"dest"`
		} `json:"placements"`
	} `json:"assets"`
	Scripts struct {
		PostInstall []string `json:"post_install"`
	} `json:"scripts"`
}

func migrated(t *testing.T, in string) []migratedVariant {
	t.Helper()
	out, err := ToFormat3([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Variants []migratedVariant `json:"variants"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Variants
}

// Ports Migrate_V2PlatformWithoutGoarch_AddsEmptyVariantWithoutPlatform of
// PackageManifestMigrationTests.cs (v0.38.0-beta.1).
func TestToFormat3V2PlatformWithoutGoarchAddsEmptyVariantWithoutPlatform(t *testing.T) {
	v := migrated(t, `{
	  "format_version": 2, "tooth": "github.com/test/tool", "version": "1.0.0",
	  "info": { "name": "", "description": "", "author": "", "tags": [] },
	  "platforms": [ { "goos": "windows", "commands": { "post_install": ["echo"] } } ]
	}`)
	if len(v) != 2 || v[0].Platform != "" || v[1].Platform != "win-*" {
		t.Fatalf("variants = %+v", v)
	}
	if len(v[0].Scripts.PostInstall) != 0 || len(v[0].Assets) != 0 {
		t.Errorf("inserted variant = %+v", v[0])
	}
	if strings.Join(v[1].Scripts.PostInstall, ",") != "echo" {
		t.Errorf("post_install = %v", v[1].Scripts.PostInstall)
	}
}

// Ports Migrate_V2WithoutPlatforms_PlacesWithoutAssetUrlAreOfASelfAsset.
func TestToFormat3V2WithoutPlatformsPlacesWithoutAssetURLAreOfASelfAsset(t *testing.T) {
	v := migrated(t, `{
	  "format_version": 2, "tooth": "github.com/test/tool", "version": "1.0.0",
	  "info": { "name": "", "description": "", "author": "", "tags": [] },
	  "files": { "place": [ { "src": "a/*", "dest": "b" } ] }
	}`)
	if len(v) != 1 || v[0].Platform != "" || len(v[0].Assets) != 1 || v[0].Assets[0].Type != "self" {
		t.Fatalf("variants = %+v", v)
	}
	p := v[0].Assets[0].Placements
	if len(p) != 1 || p[0].Src != "a/" || p[0].Dest != "b" || p[0].Type != "dir" {
		t.Errorf("placements = %+v", p)
	}
}
