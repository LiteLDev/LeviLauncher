package semver

import (
	"math"
	"testing"
)

func TestNumber(t *testing.T) {
	if n := NewNumber(0); !n.IsZero() || n.String() != "0" || n != (Number{}) {
		t.Errorf("NewNumber(0) = %q", n)
	}
	if v, ok := NewNumber(math.MaxUint64).Uint64(); !ok || v != math.MaxUint64 {
		t.Errorf("Uint64 = %d, %v", v, ok)
	}
	big := MustParse("99999999999999999999.0.0")
	if big.Major.String() != "99999999999999999999" {
		t.Errorf("Major = %s", big.Major)
	}
	if _, ok := big.Major.Uint64(); ok {
		t.Error("Uint64 of 99999999999999999999 succeeded")
	}
	if big.Major.inc().String() != "100000000000000000000" || !big.Major.isSucc(MustParse("100000000000000000000.0.0").Major) {
		t.Error("inc")
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"0", "0", 0}, {"9", "10", -1}, {"18446744073709551616", "18446744073709551615", 1},
		{"99999999999999999999", "99999999999999999999", 0}, {"100000000000000000000", "99999999999999999999", 1},
	} {
		a, b := MustParse(tc.a+".0.0").Major, MustParse(tc.b+".0.0").Major
		if got := a.Compare(b); got != tc.want {
			t.Errorf("Compare(%s, %s) = %d", tc.a, tc.b, got)
		}
		if (a == b) != (tc.want == 0) {
			t.Errorf("%s == %s is %v", tc.a, tc.b, a == b)
		}
	}
	// Leading zeros (ParseAny) are not part of the number.
	if v := MustParseAny("v007.0.0"); v.Major != NewNumber(7) {
		t.Errorf("Major = %s", v.Major)
	}
}

func MustParseAny(s string) Version {
	v, err := ParseAny(s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestBigVersionRanges(t *testing.T) {
	r := MustParseVersionRange("^99999999999999999999.0.0")
	if r.Range.String() != "99999999999999999999.*" { // SemVersionRange.ToString
		t.Errorf("String = %q", r.Range.String())
	}
	for v, want := range map[string]bool{
		"99999999999999999999.0.0":     true,
		"99999999999999999999.7.1":     true,
		"100000000000000000000.0.0":    false,
		"100000000000000000000.0.0-0":  false,
		"99999999999999999998.99999.0": false,
	} {
		if got := r.Contains(MustParse(v)); got != want {
			t.Errorf("Contains(%s) = %v", v, got)
		}
	}
}
