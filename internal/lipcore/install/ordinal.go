package install

import (
	"unicode/utf16"
	"unicode/utf8"
)

// compareOrdinal compares strings by UTF-16 code units (.NET's
// StringComparer.Ordinal), as archive does. It differs from Go's byte order
// only between a character above U+FFFF and one in U+E000..U+FFFF.
func compareOrdinal(a, b string) int {
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb {
			ua := utf16.AppendRune(nil, ra)
			ub := utf16.AppendRune(nil, rb)
			for i := 0; i < len(ua) && i < len(ub); i++ {
				if ua[i] != ub[i] {
					return int(ua[i]) - int(ub[i])
				}
			}
			return len(ua) - len(ub)
		}
		a, b = a[na:], b[nb:]
	}
	return len(a) - len(b)
}
