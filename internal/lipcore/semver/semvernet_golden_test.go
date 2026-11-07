package semver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The files in testdata/semvernet were produced by a C# harness over lip
// v0.38.0-beta.1's Lip.Core (Semver 3.0.0): for each input it records
// whether parsing succeeded, the result's ToString (or the comparison
// result), and the exception message with its inner exception's message.
//
//	range:    VersionRange.Parse(in), str = Range.ToString()
//	npm:      SemVersionRange.ParseNpm(in)
//	strict:   SemVersion.Parse(in, SemVersionStyles.Strict)
//	any:      SemVersion.Parse(in, SemVersionStyles.Any)
//	contains: in = "range\tversion", str = VersionRange.IsSatisfiedBy
//	cmp:      in = "a\tb", str = sign(ComparePrecedence) sign(CompareSortOrder)
type semverNetCase struct {
	In    string `json:"in"`
	OK    bool   `json:"ok"`
	Str   string `json:"str"`
	Err   string `json:"err"`
	Inner string `json:"inner"`
}

func loadSemverNet(t *testing.T, mode string) []semverNetCase {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "semvernet", mode+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []semverNetCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	return cases
}

func short(s string) string {
	if len(s) > 60 {
		return fmt.Sprintf("%q...(%d bytes)", s[:60], len(s))
	}
	return fmt.Sprintf("%q", s)
}

func checkSemverNet(t *testing.T, c semverNetCase, str string, err error, wantInner bool) {
	t.Helper()
	if c.OK {
		if err != nil {
			t.Errorf("%s: error %q, want %q", short(c.In), err, c.Str)
		} else if str != c.Str {
			t.Errorf("%s: got %q, want %q", short(c.In), str, c.Str)
		}
		return
	}
	if err == nil {
		t.Errorf("%s: got %q, want error %q", short(c.In), str, c.Err)
		return
	}
	if err.Error() != c.Err {
		t.Errorf("%s: error\n got  %q\n want %q", short(c.In), err.Error(), c.Err)
	}
	if wantInner {
		inner := errors.Unwrap(err)
		if inner == nil {
			t.Errorf("%s: no inner error, want %q", short(c.In), c.Inner)
		} else if inner.Error() != c.Inner {
			t.Errorf("%s: inner error\n got  %q\n want %q", short(c.In), inner.Error(), c.Inner)
		}
	}
}

func TestSemverNetVersionRange(t *testing.T) {
	for _, c := range loadSemverNet(t, "range") {
		r, err := ParseVersionRange(c.In)
		checkSemverNet(t, c, r.Range.String(), err, true)
		if err == nil && r.String() != c.In {
			t.Errorf("%s: Text = %q", short(c.In), r.String())
		}
	}
}

func TestSemverNetNpmRange(t *testing.T) {
	for _, c := range loadSemverNet(t, "npm") {
		r, err := ParseNpmRange(c.In)
		checkSemverNet(t, c, r.String(), err, false)
	}
}

func TestSemverNetParseStrict(t *testing.T) {
	for _, c := range loadSemverNet(t, "strict") {
		v, err := ParseStrict(c.In)
		checkSemverNet(t, c, v.String(), err, false)
	}
}

func TestSemverNetParseAny(t *testing.T) {
	for _, c := range loadSemverNet(t, "any") {
		v, err := ParseAny(c.In)
		checkSemverNet(t, c, v.String(), err, false)
	}
}

func TestSemverNetContains(t *testing.T) {
	for _, c := range loadSemverNet(t, "contains") {
		rs, vs, _ := strings.Cut(c.In, "\t")
		r := MustParseVersionRange(rs)
		v := MustParse(vs)
		checkSemverNet(t, c, fmt.Sprint(r.Contains(v)), nil, false)
	}
}

func TestSemverNetCompare(t *testing.T) {
	for _, c := range loadSemverNet(t, "cmp") {
		as, bs, _ := strings.Cut(c.In, "\t")
		a, b := MustParse(as), MustParse(bs)
		got := fmt.Sprintf("%d %d", ComparePrecedence(a, b), CompareSortOrder(a, b))
		checkSemverNet(t, c, got, nil, false)
	}
}
