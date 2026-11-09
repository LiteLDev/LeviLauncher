package lipindex

import (
	"regexp"
	"strings"
)

// License families (the shared contract's A5, ported from levimc.org
// mods/src/data/licenses.ts and mirrored by frontend/src/utils/lipCatalog.ts).
// The catalogue carries the SPDX id GitHub reports; only listings sort it
// into a family, after the overrides below.
const (
	LicensePermissive   = "permissive"
	LicenseWeakCopyleft = "weak-copyleft"
	LicenseCopyleft     = "copyleft"
	LicenseOther        = "other"
	LicenseNone         = "none"
)

// LicenseFamilies are the families in the order filters list them.
var LicenseFamilies = []string{LicensePermissive, LicenseWeakCopyleft, LicenseCopyleft, LicenseOther, LicenseNone}

// IsLicenseFamily reports whether v is a family id.
func IsLicenseFamily(v string) bool { return contains(LicenseFamilies, v) }

// LicenseOverrides are licenses GitHub cannot tell (it reports
// NOASSERTION), read from the repositories' license files, by lower-case
// owner/repo.
var LicenseOverrides = map[string]string{
	"copper-lamp/bschat":   "AGPL-3.0",
	"ieckseyr/hologramlib": "MIT",
}

var licenseFamilyIDs = []struct {
	family string
	re     *regexp.Regexp
}{
	{LicensePermissive, regexp.MustCompile(`^(?:MIT|APACHE-2\.0|BSD-.+|ISC|ZLIB|CC0-1\.0|UNLICENSE|0BSD)$`)},
	{LicenseWeakCopyleft, regexp.MustCompile(`^(?:LGPL-.+|MPL-2\.0|EPL-2\.0)$`)},
	{LicenseCopyleft, regexp.MustCompile(`^(?:A?GPL-.+)$`)},
}

// LicenseFamily is the family of an SPDX id (any case); LicenseNone
// without one.
func LicenseFamily(spdx string) string {
	if spdx == "" {
		return LicenseNone
	}
	id := strings.ToUpper(spdx)
	for _, f := range licenseFamilyIDs {
		if f.re.MatchString(id) {
			return f.family
		}
	}
	return LicenseOther
}

// LicenseOf is a mod's license as the listing shows it: the override for
// its repository, else the SPDX id GitHub reports ("" for none).
func LicenseOf(id, license string) string {
	parts := strings.Split(strings.ToLower(id), "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	if o, ok := LicenseOverrides[strings.Join(parts, "/")]; ok {
		return o
	}
	return license
}
