package compat

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// testdata/derive.jsonl.gz holds, for real-world tooth.json files and edge
// cases, what lip v0.38.0-beta.1 derives from them: a throwaway C# harness
// referencing src/Lip.Core called PackageDocument.ParseToothJson(bytes)
// .ReadFor(environment, tag) and wrote YamlJson.Serialize(packageFile.Json),
// or the kind of exception: "format" (PackageFormatException) or "other".
//
// The inputs are the newest tooth.json at a tag of every package of
// tests/Lip.Core.RegressionTests/Compatibility/corpus.json, recent versions of
// LeviLamina, LegacyScriptEngine and LeviAntiCheat, synthetic edge cases, and
// the inputs of migrate/testdata/golden/cases.jsonl; each for (server,
// win-x64), (client, win-x64), (client, win-arm64), (server, linux-x64), and
// (client, win-x64) with a tag version.

// knownKindDivergence are inputs where lip fails with an exception other than
// PackageFormatException and this port with a *FormatError: a placement whose
// "src" is null, which lip's model keeps (and dereferences only when it
// converts the placement) and the manifest model rejects.
var knownKindDivergence = map[string]bool{"edge55": true}

func TestDeriveGolden(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "derive.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(nil, 16<<20)
	rows, checked := 0, 0
	for sc.Scan() {
		var row struct {
			Name    string `json:"name"`
			Tooth   string `json:"tooth"`
			Results []struct {
				Platform string  `json:"platform"`
				Runtime  string  `json:"runtime"`
				Tag      *string `json:"tag"`
				YAML     *string `json:"yaml"`
				Err      string  `json:"err"`
			} `json:"results"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows++
		for _, r := range row.Results {
			checked++
			var tag *semver.Version
			if r.Tag != nil {
				v := semver.MustParse(*r.Tag)
				tag = &v
			}
			got, err := Derive([]byte(row.Tooth), r.Platform, r.Runtime, tag)
			where := row.Name + " " + r.Platform + " " + r.Runtime
			if r.YAML != nil {
				if err != nil {
					t.Errorf("%s: %v; lip derives\n%s", where, err, *r.YAML)
					continue
				}
				if y := string(doc.MarshalYAML(got.Raw)); y != *r.YAML {
					t.Errorf("%s:\n got:\n%s\nwant:\n%s", where, y, *r.YAML)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s: derived\n%s\nlip fails (%s)", where, doc.MarshalYAML(got.Raw), r.Err)
				continue
			}
			var fe *FormatError
			kind := "other"
			if errors.As(err, &fe) {
				kind = "format"
			}
			if kind != r.Err && !knownKindDivergence[row.Name] {
				t.Errorf("%s: %s error %v, lip fails with a %s error", where, kind, err, r.Err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if rows < 400 || checked < 2000 {
		t.Fatalf("expected the golden corpus, found %d rows, %d results", rows, checked)
	}
}
