package semver

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// Ported from tests/Lip.Core.Tests/Mcmod/VersionRangeTests.cs
// (IsSatisfiedBy_Table_FollowsSemVersionRangeParse and
// IsSatisfiedBy_Prerelease_OnlyWhenTheRangeIncludesPrereleases).
func TestVersionRangeContains(t *testing.T) {
	cases := []struct {
		rng, version string
		want         bool
	}{
		{">=1.2.0 <2.0.0", "1.2.0", true},
		{">=1.2.0 <2.0.0", "2.0.0", false},
		{">=1.2.0 <2.0.0 || 3.0.0", "3.0.0", true},
		{">1.2.3", "1.2.4", true},
		{"<=1.2.3", "1.2.4", false},
		{"^1.2.3", "1.9.0", true},
		{"^1.2.3", "2.0.0", false},
		{"^0.2.3", "0.2.9", true},
		{"^0.2.3", "0.3.0", false},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"*", "42.0.0", true},
		{"1.*", "1.5.0", true},
		{"1.*", "2.0.0", false},
		// Prereleases only when the range includes them.
		{"*", "1.0.0-alpha", false},
		{"^1.0.0", "1.1.0-rc.1", false},
		{"*-*", "1.0.0-alpha", true},
		{">=2.0.0-0", "2.0.0-beta", true},
	}
	for _, c := range cases {
		r, err := ParseVersionRange(c.rng)
		if err != nil {
			t.Fatalf("ParseVersionRange(%q): %v", c.rng, err)
		}
		v, err := ParseStrict(c.version)
		if err != nil {
			t.Fatalf("ParseStrict(%q): %v", c.version, err)
		}
		if got := r.Contains(v); got != c.want {
			t.Errorf("%q.Contains(%q) = %v, want %v", c.rng, c.version, got, c.want)
		}
	}
}

// Parse_InvalidRange_ThrowsFormatException, plus npm-only syntax that the
// default (standard) options reject.
func TestParseVersionRangeInvalid(t *testing.T) {
	for _, s := range []string{
		"not a range", ">=1.0.0 <", "^^1.0.0",
		"", "v1.0.0", "1.x", "1.0.0 - 2.0.0", "1.2", ">=01.0.0",
	} {
		_, err := ParseVersionRange(s)
		if err == nil {
			t.Errorf("ParseVersionRange(%q) succeeded, want error", s)
			continue
		}
		var re *RangeError
		if !errors.As(err, &re) {
			t.Fatalf("ParseVersionRange(%q) error %T, want *RangeError", s, err)
		}
		want := "The version range '" + s + "' is not valid. " + VersionRangeSyntax
		if err.Error() != want {
			t.Errorf("ParseVersionRange(%q) error = %q, want %q", s, err.Error(), want)
		}
		if !strings.Contains(err.Error(), "'"+s+"'") {
			t.Errorf("message does not name the range: %q", err.Error())
		}
		if errors.Unwrap(err) == nil {
			t.Errorf("ParseVersionRange(%q) error has no inner reason", s)
		}
	}
}

// Text_AnyRange_KeepsOriginalString.
func TestVersionRangeKeepsText(t *testing.T) {
	for _, s := range []string{">= 1.0.0", "^1.2.3 || ~2.0.0", ">=1.0.0  <2.0.0"} {
		r, err := ParseVersionRange(s)
		if err != nil {
			t.Fatalf("ParseVersionRange(%q): %v", s, err)
		}
		if r.Text != s || r.String() != s {
			t.Errorf("Text/String = %q/%q, want %q", r.Text, r.String(), s)
		}
	}
	var c Constraint = MustParseVersionRange("1.*")
	if c.String() != "1.*" {
		t.Errorf("Constraint.String() = %q", c.String())
	}
}

// ExactVersionConstraint_IgnoresBuildMetadata.
func TestExactVersionConstraint(t *testing.T) {
	exact := ExactVersion(MustParse("1.2.3+a"))
	if !exact.Contains(MustParse("1.2.3+b")) {
		t.Error("exact 1.2.3+a should accept 1.2.3+b")
	}
	if !exact.Contains(MustParse("1.2.3")) {
		t.Error("exact 1.2.3+a should accept 1.2.3")
	}
	if exact.Contains(MustParse("1.2.4")) {
		t.Error("exact 1.2.3+a should reject 1.2.4")
	}
	if exact.Contains(MustParse("1.2.3-rc.1")) {
		t.Error("exact 1.2.3+a should reject 1.2.3-rc.1")
	}
	if got := exact.String(); got != "=1.2.3+a" {
		t.Errorf("String() = %q, want =1.2.3+a", got)
	}
	pre := ExactVersion(MustParse("1.0.0-rc.1"))
	if !pre.Contains(MustParse("1.0.0-rc.1+x")) || pre.Contains(MustParse("1.0.0")) {
		t.Error("exact prerelease constraint mismatched")
	}
}

func TestParseStrictRejectsLenientForms(t *testing.T) {
	for _, s := range []string{"v1.2.3", "V1.2.3", "1.2", "01.2.3", "1.2.3.4", " 1.2.3", "1.2.3 ", "1.2.3-01", ""} {
		if _, err := ParseStrict(s); err == nil {
			t.Errorf("ParseStrict(%q) succeeded, want error", s)
		}
	}
	for _, s := range []string{"1.2.3", "0.0.0", "1.2.3-rc.1+build.5", "1.0.0-0A.is.legal", "1.2.3+001"} {
		v, err := ParseStrict(s)
		if err != nil {
			t.Errorf("ParseStrict(%q): %v", s, err)
		} else if v.String() != s {
			t.Errorf("ParseStrict(%q).String() = %q", s, v.String())
		}
	}
}

func TestEqualAndCompare(t *testing.T) {
	a, b, plain := MustParse("2.0.0+a"), MustParse("2.0.0+b"), MustParse("2.0.0")
	if a.Equal(b) || a.Equal(plain) || !a.Equal(MustParse("2.0.0+a")) {
		t.Error("Equal must include metadata")
	}
	if ComparePrecedence(a, b) != 0 || ComparePrecedence(a, plain) != 0 {
		t.Error("precedence must ignore metadata")
	}
	if ComparePrecedence(MustParse("2.0.0-rc.1"), plain) >= 0 {
		t.Error("prerelease must precede release")
	}

	// SortOrderComparer: precedence, then metadata ordinally, shorter first.
	order := []string{
		"1.0.0-alpha", "1.0.0-alpha+a", "1.0.0", "1.0.0+1", "1.0.0+A", "1.0.0+a",
		"1.0.0+a.0", "1.0.0+a.b", "1.0.0+b", "1.0.1",
	}
	for i := range order {
		for j := range order {
			got := CompareSortOrder(MustParse(order[i]), MustParse(order[j]))
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got != want {
				t.Errorf("CompareSortOrder(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
	// Metadata identifiers compare ordinally, not numerically.
	if CompareSortOrder(MustParse("1.0.0+10"), MustParse("1.0.0+9")) >= 0 {
		t.Error("metadata must compare ordinally")
	}
}

func TestParseVersionTag(t *testing.T) {
	cases := []struct {
		ref  string
		want string // "" = not a version tag
	}{
		{"refs/tags/v1.0.0", "1.0.0"},
		{"refs/tags/v2.0.0-rc.1", "2.0.0-rc.1"},
		{"refs/tags/v2.0.0+b", "2.0.0+b"},
		{"refs/tags/v1.2", ""},
		{"refs/tags/V2.0.0", ""},
		{"refs/tags/2.0.0", ""},
		{"refs/tags/v01.0.0", ""},
		{"refs/tags/v1.7.0.1", ""},
		{"refs/tags/vlatest", ""},
		{"refs/tags/vv1.0.0", ""},
		{"refs/tags/v", ""},
		{"refs/heads/v1.0.0", ""},
		{"v1.0.0", ""},
		{"refs/tags/v1.0.0^{}", ""},
	}
	for _, c := range cases {
		v, ok := ParseVersionTag(c.ref)
		if ok != (c.want != "") {
			t.Errorf("ParseVersionTag(%q) ok = %v", c.ref, ok)
			continue
		}
		if ok && v.String() != c.want {
			t.Errorf("ParseVersionTag(%q) = %q, want %q", c.ref, v, c.want)
		}
	}
}

// Ported from ArchiveProviderTests.ListVersions_MixedTags_ReturnsStrictSemVerTagsByPrecedence.
func TestVersionTagOrder(t *testing.T) {
	refs := []string{
		"refs/tags/v1.0.0", "refs/tags/v1.2", "refs/tags/V2.0.0", "refs/tags/2.0.0",
		"refs/tags/v2.0.0-rc.1", "refs/tags/v2.0.0+b", "refs/tags/v2.0.0+a",
		"refs/tags/v01.0.0", "refs/tags/v1.7.0.1", "refs/tags/vlatest",
	}
	var tags []TagVersion
	for _, ref := range refs {
		if v, ok := ParseVersionTag(ref); ok {
			tags = append(tags, TagVersion{Version: v, Tag: strings.TrimPrefix(ref, TagsRefPrefix)})
		}
	}
	slices.SortStableFunc(tags, CompareTagVersions)
	var got []string
	for _, tv := range tags {
		got = append(got, tv.Tag)
	}
	want := []string{"v2.0.0+b", "v2.0.0+a", "v2.0.0-rc.1", "v1.0.0"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if tags[0].Version.String() != "2.0.0+b" {
		t.Errorf("first version = %q, want 2.0.0+b", tags[0].Version)
	}
}
