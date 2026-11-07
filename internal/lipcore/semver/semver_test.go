package semver

import (
	"cmp"
	"slices"
	"strings"
	"testing"
)

func TestParseStrict(t *testing.T) {
	valid := []struct {
		in   string
		want string // String() of the parsed version
		pre  []string
		meta []string
	}{
		{"0.0.4", "0.0.4", nil, nil},
		{"10.20.30", "10.20.30", nil, nil},
		{"1.1.2-prerelease+meta", "1.1.2-prerelease+meta", []string{"prerelease"}, []string{"meta"}},
		{"1.1.2+meta-valid", "1.1.2+meta-valid", nil, []string{"meta-valid"}},
		{"1.0.0-alpha.beta.1", "1.0.0-alpha.beta.1", []string{"alpha", "beta", "1"}, nil},
		{"1.0.0-alpha.0valid", "1.0.0-alpha.0valid", []string{"alpha", "0valid"}, nil},
		{"1.0.0-rc.1+build.1", "1.0.0-rc.1+build.1", []string{"rc", "1"}, []string{"build", "1"}},
		{"10.2.3-DEV-SNAPSHOT", "10.2.3-DEV-SNAPSHOT", []string{"DEV-SNAPSHOT"}, nil},
		{"1.2.3----RC-SNAPSHOT.12.9.1--.12+788", "1.2.3----RC-SNAPSHOT.12.9.1--.12+788", []string{"---RC-SNAPSHOT", "12", "9", "1--", "12"}, []string{"788"}},
		{"1.0.0+0.build.1-rc.10000aaa-kk-0.1", "1.0.0+0.build.1-rc.10000aaa-kk-0.1", nil, []string{"0", "build", "1-rc", "10000aaa-kk-0", "1"}},
		{"1.0.0-0A.is.legal", "1.0.0-0A.is.legal", []string{"0A", "is", "legal"}, nil},
		{"1.2.3--+b", "1.2.3--+b", []string{"-"}, []string{"b"}},
		{"1.2.3---", "1.2.3---", []string{"--"}, nil},
		{"1.2.3--.a+b", "1.2.3--.a+b", []string{"-", "a"}, []string{"b"}},
		{"1.2.3-00000a", "1.2.3-00000a", []string{"00000a"}, nil},
		{"1.2.3-pre.0.42", "1.2.3-pre.0.42", []string{"pre", "0", "42"}, nil},
		{"2147483648.2.3", "2147483648.2.3", nil, nil},
		{"1.2.3-18446744073709551616", "1.2.3-18446744073709551616", []string{"18446744073709551616"}, nil},
		{"26.51.1-client.11", "26.51.1-client.11", []string{"client", "11"}, nil},
	}
	for _, tc := range valid {
		v, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.in, err)
			continue
		}
		if got := v.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
		if !slices.Equal(v.Prerelease, tc.pre) || !slices.Equal(v.Metadata, tc.meta) {
			t.Errorf("Parse(%q) = pre %q meta %q, want %q %q", tc.in, v.Prerelease, v.Metadata, tc.pre, tc.meta)
		}
	}

	invalid := []string{
		"", " ", "1", "1.2", "1.2.3-0123", "1.2.3-0123.0123", "1.1.2+.123", "+invalid",
		"-invalid", "-invalid+invalid", "alpha", "alpha.beta.1", "alpha_beta", "alpha..",
		"1.0.0-alpha_beta", "1.0.0-alpha..", "1.0.0-alpha..1", "01.1.1", "1.01.1",
		"1.1.01", "1.2.3.DEV", "1.2-SNAPSHOT", "1.2.31.2.3----RC-SNAPSHOT.12.09.1--..12+788",
		"9.8.7+meta+meta", "9.8.7-whatever+meta+meta", "1@.2.3", "1.0.0@-alpha",
		"1.2.3-😞+b", "1.2.3-a+😞", "1.0.0-á", "v1.2.3", "V1.2.3", " 1.2.3", "1.2.3 ",
		"1.2.3-", "1.2.3+", "1.2.3-*", "*",
	}
	for _, s := range invalid {
		if v, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = %v, want error", s, v)
		}
	}
}

func TestParseAny(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1", "1.0.0"},
		{"1.2", "1.2.0"},
		{"v1.2.3", "1.2.3"},
		{"V1.2", "1.2.0"},
		{"  1.2.3\t", "1.2.3"},
		{"01.02.03", "1.2.3"},
		{"1.2.00", "1.2.0"},
		{"1.2.3-a.050.c", "1.2.3-a.50.c"},
		{"1.2.3-01", "1.2.3-1"},
		{"7-rc.1", "7.0.0-rc.1"},
		{"6+sha.a3456b", "6.0.0+sha.a3456b"},
		{"1.6-zeta.5+nightly.23.43-bla", "1.6.0-zeta.5+nightly.23.43-bla"},
		{"0.1.0", "0.1.0"},
	}
	for _, tc := range tests {
		v, err := ParseAny(tc.in)
		if err != nil {
			t.Errorf("ParseAny(%q) error: %v", tc.in, err)
			continue
		}
		if got := v.String(); got != tc.want {
			t.Errorf("ParseAny(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, s := range []string{"", "   ", "a.b.c", "1.2.3.4", "1.2.3-", "vv1", "1.*"} {
		if v, err := ParseAny(s); err == nil {
			t.Errorf("ParseAny(%q) = %v, want error", s, v)
		}
	}
	long := "1.0.0-" + strings.Repeat("a", 1020)
	if _, err := ParseAny(long); err == nil {
		t.Errorf("ParseAny(len %d) succeeded, want too-long error", len(long))
	}
}

func TestCompare(t *testing.T) {
	// Ascending SemVer 2.0 precedence order (spec section 11 example + more).
	ordered := []string{
		"0.0.0-0",
		"0.0.0",
		"1.0.0-0",
		"1.0.0-2",
		"1.0.0-10",
		"1.0.0-18446744073709551616",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
		"1.0.1",
		"1.1.0",
		"1.10.0",
		"2.0.0",
		"10.0.0",
	}
	for i := range ordered {
		for j := range ordered {
			a, b := MustParse(ordered[i]), MustParse(ordered[j])
			want := cmp.Compare(i, j)
			if got := a.Compare(b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
	if c := MustParse("1.2.3+a").Compare(MustParse("1.2.3+b")); c != 0 {
		t.Errorf("metadata must be ignored, got %d", c)
	}
	if c := MustParse("1.0.0-A").Compare(MustParse("1.0.0-a")); c != -1 {
		t.Errorf("alphanumeric identifiers compare ordinally, got %d", c)
	}
}

// describe renders the internal unbroken ranges of r for structural checks:
// "<left> <right>" with "-inf"/"+inf" for unbounded ends and a trailing
// " all" when the range includes all prereleases.
func describe(ranges []unbrokenRange) string {
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		l, rr := "-inf", "+inf"
		if r.left.v != nil {
			l = leftString(r.left)
		}
		if r.right.v != nil {
			rr = rightString(r.right)
		}
		parts[i] = l + " " + rr
		if r.includeAllPrerelease {
			parts[i] += " all"
		}
	}
	return strings.Join(parts, " | ")
}

const (
	allRelease = "-inf +inf"
	all        = "-inf +inf all"
	empty      = ""
)

func eqv(v string) string { return ">=" + v + " <=" + v }

func TestParseRangeStandard(t *testing.T) {
	tests := []struct {
		in         string
		includeAll bool
		want       string
	}{
		{"1.2.3", false, eqv("1.2.3")},
		{"  1.2.3   ", false, eqv("1.2.3")},
		{"1.2.3 4.5.6", false, empty},
		{"1.2.3 || 4.5.6", false, eqv("1.2.3") + " | " + eqv("4.5.6")},
		{"=1.2.3", false, eqv("1.2.3")},
		{" =   1.2.3 ", false, eqv("1.2.3")},
		{"=4.5.6||=1.2.3", false, eqv("1.2.3") + " | " + eqv("4.5.6")},
		{">1.2.3", false, ">1.2.3 +inf"},
		{" >   1.2.3 ", false, ">1.2.3 +inf"},
		{">1.2.3 >4.5.6", false, ">4.5.6 +inf"},
		{">1.2.3 || >4.5.6", false, ">1.2.3 +inf"},
		{">=1.2.3 >=4.5.6", false, ">=4.5.6 +inf"},
		{">=1.2.3 || >=4.5.6", false, ">=1.2.3 +inf"},
		{"<1.2.3 <4.5.6", false, "-inf <1.2.3"},
		{"<1.2.3 || <4.5.6", false, "-inf <4.5.6"},
		{"<=1.2.3 <=4.5.6", false, "-inf <=1.2.3"},
		{"<=1.2.3 || <=4.5.6", false, "-inf <=4.5.6"},
		{"*-* >=2.0.0-0", false, ">=2.0.0-0 +inf all"},
		{"~1.2.3", false, ">=1.2.3 <1.3.0-0"},
		{"~1.2.3-rc", false, ">=1.2.3-rc <1.3.0-0"},
		{"^1.2.3", false, ">=1.2.3 <2.0.0-0"},
		{"^1.2.3-rc", false, ">=1.2.3-rc <2.0.0-0"},
		{"^0.2.3", false, ">=0.2.3 <0.3.0-0"},
		{"^0.0.3", false, ">=0.0.3 <0.0.4-0"},
		{"^0.0.3-rc", false, ">=0.0.3-rc <0.0.4-0 all"},
		{">=4.0.0", true, ">=4.0.0 +inf all"},
		{"*", false, allRelease},
		{"*.*", false, allRelease},
		{"*.*.*", false, allRelease},
		{"*-*", false, all},
		{"*.*-*", false, all},
		{"*.*.*-*", false, all},
		{"3.*", false, ">=3.0.0 <4.0.0-0"},
		{"3.*-*", false, ">=3.0.0-0 <4.0.0-0 all"},
		{"3.*.*", false, ">=3.0.0 <4.0.0-0"},
		{"3.*.*-*", false, ">=3.0.0-0 <4.0.0-0 all"},
		{"3.1.*", false, ">=3.1.0 <3.2.0-0"},
		{"3.1.*-*", false, ">=3.1.0-0 <3.2.0-0 all"},
		{"3.1.4-*", false, ">=3.1.4-0 <3.1.5-0 all"},
		{"3.1.4-rc.*", false, ">=3.1.4-rc.0 <3.1.4-rc- all"},
		{"3.1.4-5.*", false, ">=3.1.4-5.0 <3.1.4-6 all"},
		{"3.1.4-2147483647.*", false, ">=3.1.4-2147483647.0 <3.1.4-2147483648 all"},
		{"3.1.4-99.*", false, ">=3.1.4-99.0 <3.1.4-100 all"},
		{"1.*||1.2.*", false, ">=1.0.0 <2.0.0-0"},
		{"1.*-*||1.2.*", false, ">=1.0.0-0 <2.0.0-0 all"},
		{"1.*||1.2.*-*", false, ">=1.0.0 <2.0.0-0 | >=1.2.0-0 <1.3.0-0 all"},
		{"1.*||2.*", false, ">=1.0.0 <3.0.0-0"},
		{"<2.0.0||>=2.0.0", false, allRelease},
		{">=1.2.3 <=2.5.0||>=2.0.0 <=3.1.4", false, ">=1.2.3 <=3.1.4"},
		{">=1.2.3 <=2.5.0|| *-* >=2.0.0 <=3.1.4", false, ">=1.2.3 <=2.5.0 | >=2.0.0 <=3.1.4 all"},
		{">=1.2.3 <=2.5.0||>=2.0.0-rc <=3.1.4", false, ">=1.2.3 <=2.5.0 | >=2.0.0-rc <=3.1.4"},
		{"*-* >=1.2.3 <=2.5.0||>=2.0.0-rc <=3.1.4", false, ">=1.2.3 <=2.5.0 all | >=2.0.0-rc <=3.1.4"},
		{">=1.0.0<2.0.0", false, ">=1.0.0 <2.0.0"},
		{"~1.2147483647.3", false, ">=1.2147483647.3 <1.2147483648.0-0"},
		{"^2147483647.2.3", false, ">=2147483647.2.3 <2147483648.0.0-0"},
		{"3.1.2147483647-*", false, ">=3.1.2147483647-0 <3.1.2147483648-0 all"},
		{"<0.0.0-0", false, empty},
		{"<0.0.0", false, empty},
		{">1.0.0 <1.0.1", false, empty},
	}
	for _, tc := range tests {
		got, err := parseStandardRange(tc.in, tc.includeAll, maxRangeLength)
		if err != nil {
			t.Errorf("parse %q: %v", tc.in, err)
			continue
		}
		if d := describe(normalizeRanges(got)); d != tc.want {
			t.Errorf("parse %q = %q, want %q", tc.in, d, tc.want)
		}
	}
}

func TestParseRangeStandardInvalid(t *testing.T) {
	invalid := []string{
		">*.2.3", ">1.*.3", ">*.*.3", ">*1.2.3", ">1.*2.3", ">1.2.*3",
		"1.2.3-*a", "1.2.3-a*b", "1.2.3-a*", "1.2.3-a.*a", "1.2.3-a.a*b", "1.2.3-a.a*",
		"1.2.3-*.a", "1.2.3-a.*.a",
		"*-rc", "*-rc.*", "1.*-rc", "1.*-rc.*", "1.2.*-rc", "1.2.*-rc.*",
		"", "   ", "1.2.3||", " ||1.2.3",
		"  \t", "\t=1.2.3", "=\t1.2.3", "=1.2.3\t",
		"~>1.2.3", "==1.2.3", "=1.2.3|4.5.6", "@&%1.2.3", "≥1.2.3",
		"<1.*", "<=1.2.*", ">*", ">=1.2.3-*", "=1.2.*", "^1.2.*", "~1.2.*",
		// Strict: no "v", no partial versions, no metadata, no hyphen ranges.
		"v1.2.3", "1.2", "1", "1.2.3+build", "1.0.0 - 2.0.0", "1.x", ">=",
		strings.Repeat("1", maxRangeLength+1),
	}
	for _, s := range invalid {
		if r, err := ParseRange(s); err == nil {
			t.Errorf("ParseRange(%q) = %q, want error", s, r)
		}
	}
}

func TestRangeString(t *testing.T) {
	tests := []struct {
		in         string
		includeAll bool
		want       string
	}{
		{"*", false, "*"},
		{"*.*.*", false, "*"},
		{"*", true, "*-*"},
		{"*-*", false, "*-*"},
		{"*.*.*-*", false, "*-*"},
		{"1.2.3", false, "1.2.3"},
		{"1.2.3-rc", false, "1.2.3-rc"},
		{"1.2.3-rc", true, "1.2.3-rc"},
		{"=1.2.3", false, "1.2.3"},
		{"=1.2.3-rc", true, "1.2.3-rc"},
		{"1.2.3-*", false, "1.2.3-*"},
		{"1.2.3-*", true, "1.2.3-*"},
		{"1.2.*", false, "1.2.*"},
		{"1.2.*", true, "*-* 1.2.*"},
		{"1.2.* *-*", false, "*-* 1.2.*"},
		{"1.2.*-*", false, "1.2.*-*"},
		{"1.*.*", false, "1.*"},
		{"1.*.*", true, "*-* 1.*"},
		{"1.*.* *-*", false, "*-* 1.*"},
		{"1.*-*", false, "1.*-*"},
		{"1.2.3-rc.*", false, "1.2.3-rc.*"},
		{"1.2.3-5.*", false, "1.2.3-5.*"},
		{"1.2.3-2147483647.*", false, "1.2.3-2147483647.*"},
		{"3.1.4-l.o.n.g.e.r.rc.*", false, "3.1.4-l.o.n.g.e.r.rc.*"},
		{"~2.5.3", false, "~2.5.3"},
		{"~2.5.3", true, "*-* ~2.5.3"},
		{"~2.5.3-rc", false, "~2.5.3-rc"},
		{"~0.5.3", false, "~0.5.3"},
		{"~0.5.3", true, "*-* ~0.5.3"},
		{"~0.5.3-rc", false, "~0.5.3-rc"},
		{"^6.2.3", false, "^6.2.3"},
		{"^6.2.3", true, "*-* ^6.2.3"},
		{"^6.2.3-alpha.1", false, "^6.2.3-alpha.1"},
		{"^0.0.7", false, "^0.0.7"},
		{"^0.0.7", true, "*-* ^0.0.7"},
		{"^0.0.7-now", false, "^0.0.7-now"},
		{"^0.2.3", false, "~0.2.3"},
		{">=1.2.3", false, ">=1.2.3"},
		{"<1.2.3", false, "<1.2.3"},
		{"<6.0.0 >=1.0.0", false, ">=1.0.0 <6.0.0"},
		{">1.0.0 <=3.2.6", false, ">1.0.0 <=3.2.6"},
		{">1.0.0 <=3.2.6 || >=1.0.0 <6.0.0", false, ">=1.0.0 <6.0.0"},
		{">= 1.2.3", false, ">=1.2.3"},
		{" >= 5.6.7 ||  < 1.2.3", false, "<1.2.3 || >=5.6.7"},
		{">=1.2.0 <1.3.0-0", false, "1.2.*"},
		{"1.2.3 4.5.6", false, ""},
		{"26.51.1-client.11", false, "26.51.1-client.11"},
		{"1.3.*", false, "1.3.*"},
		{">=1.0.0", false, ">=1.0.0"},
		{"^1.2.3", false, "^1.2.3"},
	}
	for _, tc := range tests {
		ranges, err := parseStandardRange(tc.in, tc.includeAll, maxRangeLength)
		if err != nil {
			t.Errorf("parse %q: %v", tc.in, err)
			continue
		}
		got := newRange(ranges).String()
		if got != tc.want {
			t.Errorf("String(%q) = %q, want %q", tc.in, got, tc.want)
		}
		// The normalized text must parse back to the same range.
		if tc.want == "" {
			continue
		}
		again, err := ParseRange(got)
		if err != nil {
			t.Errorf("reparse %q: %v", got, err)
			continue
		}
		if again.String() != got || describe(again.ranges) != describe(newRange(ranges).ranges) {
			t.Errorf("round trip %q -> %q (%s vs %s)", got, again.String(), describe(again.ranges), describe(newRange(ranges).ranges))
		}
	}
}

func TestParseNpmRange(t *testing.T) {
	tests := []struct {
		in         string
		includeAll bool
		want       string
	}{
		{"<1.2.3", false, "-inf <1.2.3"},
		{"<= 1.2.3", false, "-inf <=1.2.3"},
		{">1.2.3", false, ">1.2.3 +inf"},
		{">=1.2.3", false, ">=1.2.3 +inf"},
		{"=1.2.3", false, eqv("1.2.3")},
		{"1.2.3", false, eqv("1.2.3")},
		{">=1.2.7 <1.3.0", false, ">=1.2.7 <1.3.0"},
		{"1.2.7 || >=1.2.9 <2.0.0", false, eqv("1.2.7") + " | >=1.2.9 <2.0.0"},
		{">1.2.3-alpha.3", false, ">1.2.3-alpha.3 +inf"},
		{"*", false, allRelease},
		{"*", true, all},
		{"1.x", false, ">=1.0.0 <2.0.0-0"},
		{"1.2.x", false, ">=1.2.0 <1.3.0-0"},
		{"", false, allRelease},
		{"", true, all},
		{"   ", false, allRelease},
		{"1.2.3||", false, allRelease},
		{"1", false, ">=1.0.0 <2.0.0-0"},
		{"1.2", false, ">=1.2.0 <1.3.0-0"},
		{"1.X", false, ">=1.0.0 <2.0.0-0"},
		{"1.x.x+build", false, ">=1.0.0 <2.0.0-0"},
		{"1+build", false, ">=1.0.0 <2.0.0-0"},
		{"v1.2.3", false, eqv("1.2.3")},
		{"<1.2.*", false, "-inf <1.2.0-0"},
		{"<1.2.*", true, "-inf <1.2.0-0 all"},
		{"<*", false, empty},
		{"<=1.2.*", false, "-inf <1.3.0-0"},
		{"<=*", true, all},
		{">1.2.*", false, ">=1.3.0 +inf"},
		{">1.2.*", true, ">=1.3.0-0 +inf all"},
		{">1.*", false, ">=2.0.0 +inf"},
		{">*", false, empty},
		{">=1.2.*", true, ">=1.2.0-0 +inf all"},
		{">=*", false, allRelease},
		{"=1.2.*", true, ">=1.2.0-0 <1.3.0-0 all"},
		{"1.2.3 - 2.3.4", false, ">=1.2.3 <=2.3.4"},
		{"1.2.3-rc - 2.3.4-rc", false, ">=1.2.3-rc <=2.3.4-rc"},
		{"1.2 - 2.3.4", false, ">=1.2.0 <=2.3.4"},
		{"1.2.3 - 2.3", false, ">=1.2.3 <2.4.0-0"},
		{"1.2.3 - 2", false, ">=1.2.3 <3.0.0-0"},
		{"1.2.0 - 1.0.0", false, empty},
		{"~1.2.3", true, ">=1.2.3 <1.3.0-0 all"},
		{"~1.2.*", false, ">=1.2.0 <1.3.0-0"},
		{"~1.*", true, ">=1.0.0-0 <2.0.0-0 all"},
		{"~0.*", false, ">=0.0.0 <1.0.0-0"},
		{"~1.2.3-beta.2", false, ">=1.2.3-beta.2 <1.3.0-0"},
		{"~>1.2.3", false, ">=1.2.3 <1.3.0-0"},
		{"~*", false, allRelease},
		{"^1.2.3", false, ">=1.2.3 <2.0.0-0"},
		{"^0.2.3", false, ">=0.2.3 <0.3.0-0"},
		{"^0.0.3", false, ">=0.0.3 <0.0.4-0"},
		{"^0.0.3-beta", false, ">=0.0.3-beta <0.0.4-0 all"},
		{"^1.2.x", true, ">=1.2.0-0 <2.0.0-0 all"},
		{"^0.0.x", false, ">=0.0.0 <0.1.0-0"},
		{"^0.0", false, ">=0.0.0 <0.1.0-0"},
		{"^0.x", false, ">=0.0.0 <1.0.0-0"},
		{"^4", true, ">=4.0.0-0 <5.0.0-0 all"},
		{"^*", true, all},
		{"~2147483647.*.*", false, ">=2147483647.0.0 <2147483648.0.0-0"},
		{">=1.0.0\t<2.0.0", false, ">=1.0.0 <2.0.0"},
	}
	for _, tc := range tests {
		got, err := parseNpmRange(tc.in, tc.includeAll, maxRangeLength)
		if err != nil {
			t.Errorf("npm parse %q: %v", tc.in, err)
			continue
		}
		if d := describe(normalizeRanges(got)); d != tc.want {
			t.Errorf("npm parse %q = %q, want %q", tc.in, d, tc.want)
		}
	}

	invalid := []string{
		"1.x.x-rc", "1.x-rc", "1-rc", "1.2.3-*", "1.x.3", "x.x.3", "x.2.3",
		">*1.2.3", ">1.*2.3", ">1.2.*3",
		"1.0.0 - 2.a.*", ">1.0.0 - 2.0.0", ">1.0.0 1.5.0 - 2.0.0", "1.5.0 < - 2.0.0",
		"1.5.0 - <2.0.0", "1.5.0 - 2.0.0 <4.0.0", " - 2.0.0", "    - 2.0.0", "1.0.0 - ",
		"1.0.0 -    ", "~<1.2.3", "~=1.2.3", "==1.2.3", "=1.2.3|4.5.6", "@&%1.2.3", "≥1.2.3",
	}
	for _, s := range invalid {
		if r, err := ParseNpmRange(s); err == nil {
			t.Errorf("ParseNpmRange(%q) = %q, want error", s, r)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		rng     string
		version string
		want    bool
	}{
		// Exact
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.3+build", true},
		{"1.2.3", "1.2.4", false},
		{"1.2.3-5", "1.2.3-5", true},
		{"1.2.3-5", "1.2.3-6", false},
		// Comparators and the prerelease rule.
		{">1.2.3", "1.2.4", true},
		{">1.2.3", "1.2.4-0", false},
		{">1.2.3", "2.0.0-pre", false},
		{">1.2.3-5", "1.2.3-6", true},
		{">1.2.3-5", "1.2.3", true},
		{">1.2.3-5", "1.2.4-0", false},
		{">=1.2.3", "1.2.3", true},
		{">=1.2.3", "1.2.3-6", false},
		{">=1.2.3-5", "1.2.3-5", true},
		{">=1.2.3-5", "1.2.3-4", false},
		{"<1.2.3", "1.2.2", true},
		{"<1.2.3", "1.2.2-pre", false},
		{"<1.2.3", "1.2.3-4", false},
		{"<1.2.3", "0.0.0", true},
		{"<1.2.3", "0.0.0-0", false},
		{"<=1.2.3-5", "1.2.3-4", true},
		{"<=1.2.3-5", "1.2.3-6", false},
		// Wildcards
		{"*", "1.2.3", true},
		{"*", "1.2.3-rc", false},
		{"*-*", "1.2.3-rc", true},
		{"*-*", "0.0.0-0", true},
		{"1.3.*", "1.3.0", true},
		{"1.3.*", "1.3.99", true},
		{"1.3.*", "1.4.0", false},
		{"1.3.*", "1.3.1-rc", false},
		{"1.3.*-*", "1.3.1-rc", true},
		{"1.3.*-*", "1.3.0-0", true},
		{"1.3.*-*", "1.4.0-0", false},
		{"1.*", "1.9.9", true},
		{"1.*", "2.0.0-0", false},
		{"1.2.3-*", "1.2.3-anything", true},
		{"1.2.3-*", "1.2.3", true},
		{"1.2.3-*", "1.2.4-0", false},
		{"1.2.3-rc.*", "1.2.3-rc.1", true},
		{"1.2.3-rc.*", "1.2.3-rc", false},
		{"1.2.3-rc.*", "1.2.3-rcb", false},
		{"1.2.3-rc.*", "1.2.3", false},
		// Caret / tilde
		{"^1.2.3", "1.9.0", true},
		{"^1.2.3", "2.0.0", false},
		{"^1.2.3", "1.2.2", false},
		{"^1.2.3", "1.5.0-beta", false},
		{"^1.2.3-beta.2", "1.2.3-beta.4", true},
		{"^1.2.3-beta.2", "1.2.4-beta.2", false},
		{"^0.2.3", "0.2.9", true},
		{"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.4", false},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"*-* ^1.2.3", "1.5.0-beta", true},
		// Unions
		{"1.2.3 || 2.*", "2.5.0", true},
		{"1.2.3 || 2.*", "1.2.4", false},
		{"<1.0.0 || >=2.0.0", "1.5.0", false},
		{"<1.0.0 || >=2.0.0", "2.0.0", true},
		// Empty
		{"1.2.3 4.5.6", "1.2.3", false},
	}
	for _, tc := range tests {
		r, err := ParseRange(tc.rng)
		if err != nil {
			t.Errorf("ParseRange(%q): %v", tc.rng, err)
			continue
		}
		if got := r.Contains(MustParse(tc.version)); got != tc.want {
			t.Errorf("%q.Contains(%s) = %v, want %v", tc.rng, tc.version, got, tc.want)
		}
	}

	var zero Range
	if zero.Contains(MustParse("1.0.0")) || zero.String() != "" {
		t.Errorf("zero Range must be empty")
	}
}

func TestNextIdentifier(t *testing.T) {
	tests := []struct{ in, want string }{
		{"0", "1"}, {"9", "10"}, {"199", "200"}, {"2147483647", "2147483648"},
		{"18446744073709551615", "18446744073709551616"}, {"rc", "rc-"}, {"a1", "a1-"},
	}
	for _, tc := range tests {
		if got := nextIdentifier(tc.in); got != tc.want {
			t.Errorf("nextIdentifier(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
