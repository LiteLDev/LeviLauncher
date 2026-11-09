package compat

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// testdata/derive.jsonl.gz holds, for real-world tooth.json files and edge
// cases, what lip main derives from them: a throwaway C# harness
// referencing src/Lip.Core called PackageDocument.ParseToothJson(bytes)
// .ReadFor(environment, tag) and wrote TomlJson.Serialize(packageFile.Json),
// or the kind of exception: "format" (PackageFormatException) or "other".
//
// The inputs are the newest tooth.json at a tag of every package of
// tests/Lip.Core.RegressionTests/Compatibility/corpus.json, recent versions of
// LeviLamina, LegacyScriptEngine and LeviAntiCheat, synthetic edge cases, and
// the inputs of migrate/testdata/golden/cases.jsonl; each for (server,
// win-x64), (client, win-x64), (client, win-arm64), (server, linux-x64), and
// (client, win-x64) with a tag version.
//
// For lip v0.40, which renamed the keys "platform" and "runtime" of an entry
// to "side" and "platform", the file was rewritten, not derived again: the
// keys of results[] and the keys of the entries in results[].toml were
// renamed by a one-off script (rewrite_golden.py: "runtime =" to a
// placeholder, "platform =" to "side =", the placeholder to "platform =",
// line-anchored, and written with gzip mtime 0); the tooth inputs are
// unchanged.

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
				Side     string  `json:"side"`
				Platform string  `json:"platform"`
				Tag      *string `json:"tag"`
				TOML     *string `json:"toml"`
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
			got, err := Derive([]byte(row.Tooth), r.Side, r.Platform, tag)
			where := row.Name + " " + r.Side + " " + r.Platform
			if r.TOML != nil {
				if err != nil {
					t.Errorf("%s: %v; lip derives\n%s", where, err, *r.TOML)
					continue
				}
				if y := tomlOf(got.Raw); y != *r.TOML {
					t.Errorf("%s:\n got:\n%s\nwant:\n%s", where, y, *r.TOML)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s: derived\n%s\nlip fails (%s)", where, tomlOf(got.Raw), r.Err)
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
