package ref

import "regexp"

// Fold lowercases ASCII letters only: package ids, identities and mod names
// are compared ASCII case-insensitively (protocol/package.md §1;
// PackageNames.Fold in Packages.cs).
func Fold(name string) string {
	b := []byte(name)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c | 0x20
		}
	}
	return string(b)
}

// NamesEqual is PackageNames.AreEqual.
func NamesEqual(a, b string) bool { return asciiEqualFold(a, b) }

var validNameRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// IsValidName is PackageNames.IsValid: whether a name is a mod name
// (protocol/package.md §4).
func IsValidName(name string) bool { return validNameRegex.MatchString(name) }

// Identity is Package.GetIdentity: the id, followed by "#" and the variant
// when the variant is not "".
func Identity(id, variant string) string {
	if variant == "" {
		return id
	}
	return id + "#" + variant
}
