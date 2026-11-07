package semver

import (
	"cmp"
	"strconv"
)

// Number is a major, minor or patch version number: a non-negative integer
// of arbitrary size, like the BigInteger Semver 3.0.0 uses, so
// "99999999999999999999.0.0" is a valid version. The zero value is 0.
// Numbers are comparable with ==.
type Number struct {
	// digits are the decimal digits without leading zeros; "" is zero.
	digits string
}

// NewNumber returns the Number n.
func NewNumber(n uint64) Number {
	if n == 0 {
		return Number{}
	}
	return Number{digits: strconv.FormatUint(n, 10)}
}

// numberFromDigits returns the Number for a non-empty string of ASCII
// digits, leading zeros allowed.
func numberFromDigits(s string) Number {
	s = trimLeadingZeros(s)
	if s == "0" {
		return Number{}
	}
	return Number{digits: s}
}

// String formats n in decimal.
func (n Number) String() string {
	if n.digits == "" {
		return "0"
	}
	return n.digits
}

// IsZero reports whether n is 0.
func (n Number) IsZero() bool { return n.digits == "" }

// Uint64 returns n as a uint64; ok is false when it does not fit.
func (n Number) Uint64() (v uint64, ok bool) {
	if n.digits == "" {
		return 0, true
	}
	v, err := strconv.ParseUint(n.digits, 10, 64)
	return v, err == nil
}

// Compare returns -1, 0 or 1 as n is less than, equal to or greater than o.
func (n Number) Compare(o Number) int {
	if c := cmp.Compare(len(n.digits), len(o.digits)); c != 0 {
		return c
	}
	return cmp.Compare(n.digits, o.digits)
}

// inc returns n + 1.
func (n Number) inc() Number {
	if n.digits == "" {
		return Number{digits: "1"}
	}
	return Number{digits: nextIdentifier(n.digits)}
}

// isSucc reports whether o == n + 1.
func (n Number) isSucc(o Number) bool { return n.inc() == o }
