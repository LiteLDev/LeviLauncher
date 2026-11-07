// Package migrate converts tooth.json documents of any supported format
// (1, 2, 3) into a rendered format 3 document, porting
// src/Lip.Core/Migration/PackageManifests/* of lip 0.37.0 with the changes
// of lip v0.38.0-beta.1 (an empty variant without a platform for platform
// patterns; see ToFormat3).
//
// Template rendering implements the subset of Scriban 7.5 that manifests use
// (see renderTemplate); constructs outside it fail with
// ErrUnsupportedTemplate rather than rendering differently from lip.
package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// ErrNotImplemented was returned by the skeleton stubs.
//
// Deprecated: nothing in this package returns it any more; kept so the
// skeleton's exported API only grows.
var ErrNotImplemented = errors.New("migrate: not implemented")

// ErrUnsupportedFormat is returned for a format_version other than 1, 2, 3.
var ErrUnsupportedFormat = errors.New("migrate: unsupported format_version")

// formatUUID is PackageManifest's format 3 UUID.
const formatUUID = "289f771f-2c9a-4d73-9f3f-8492495a924d"

// ToFormat3 takes raw tooth.json bytes and returns format 3 JSON bytes
// (compact; object member order is preserved, so "dependencies" keep their
// order).
//
//   - format 3: every JSON string value (not object keys) is rendered as a
//     Scriban template whose model is the top-level object of the original
//     document (so "{{version}}" and "{{tooth}}" work, and "{{info.name}}"
//     reaches nested members); everything else is copied as-is. Duplicate
//     object keys anywhere are an error, as in lip.
//   - format 1/2: converted per PackageManifestMigration ($(var) -> {{var}}
//     with model {version, path}; GOOS/GOARCH -> RID; asset type inferred
//     from URL extension; place src ending in "*" -> dir placement; script
//     mapping; invalid dependencies dropped; when any variant platform
//     contains "*", an empty variant without a platform is inserted at
//     index 0, as lip v0.38 does), then rendered. The output has every field lip serializes;
//     "version" is copied unnormalized and tooth path, tags and placement
//     destinations are left for manifest.Parse to validate.
//
// Any error means lip cannot read the tooth.json as format 3 (lip v0.38's
// ToothCompat.ReadToothJson turns every one into a PackageFormatException).
func ToFormat3(raw []byte) ([]byte, error) {
	root, err := parseJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	fv, err := formatVersionOf(root)
	if err != nil {
		return nil, err
	}
	var out *jvalue
	switch fv {
	case 1:
		v1, err := bindV1(root)
		if err != nil {
			return nil, err
		}
		v2, err := migrateV1ToV2(v1)
		if err != nil {
			return nil, err
		}
		if out, err = migrateV2ToV3(v2); err != nil {
			return nil, err
		}
	case 2:
		v2, err := bindV2(root)
		if err != nil {
			return nil, err
		}
		if out, err = migrateV2ToV3(v2); err != nil {
			return nil, err
		}
	case 3:
		if out, err = renderV3(root); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedFormat, fv)
	}
	var buf bytes.Buffer
	encodeJSON(&buf, out)
	return buf.Bytes(), nil
}

// ToV3 is ToFormat3. lip v0.37.0 named its own runtime identifier as the
// platform of the variant it inserted for platform patterns; since lip
// v0.38 that variant has no platform, so rid is unused.
//
// Deprecated: use ToFormat3.
func ToV3(raw []byte, rid string) ([]byte, error) { return ToFormat3(raw) }

// FormatVersion returns the "format_version" field of raw, or an error.
// Like JsonElement.GetProperty(...).GetInt32() it requires an object root
// and an integer literal that fits in an int32 (so 3.0 and 3e0 are errors).
func FormatVersion(raw []byte) (int, error) {
	root, err := parseJSON(raw)
	if err != nil {
		return 0, fmt.Errorf("migrate: %w", err)
	}
	return formatVersionOf(root)
}

func formatVersionOf(root *jvalue) (int, error) {
	if root.kind != jObject {
		return 0, fmt.Errorf("migrate: tooth.json root is %s, not an object", root.kind)
	}
	v, ok := root.get("format_version")
	if !ok {
		return 0, errors.New("migrate: format_version is missing")
	}
	if v.kind != jNumber {
		return 0, fmt.Errorf("migrate: format_version is %s, not a number", v.kind)
	}
	n, err := strconv.ParseInt(v.str, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("migrate: format_version %s is not an int32", v.str)
	}
	return int(n), nil
}

// renderV3 ports ParseV3's rendering pass: every string value is replaced
// by Scriban.Template.Parse(s).Render(model) where model is the original
// top-level object.
// Duplicate member names at any depth are an error (lip's Dictionary and
// JsonObject materialization throws on them).
func renderV3(root *jvalue) (*jvalue, error) {
	scope := func(name string) (*jvalue, bool) { return root.get(name) }
	var walk func(v *jvalue) (*jvalue, error)
	walk = func(v *jvalue) (*jvalue, error) {
		switch v.kind {
		case jString:
			s, err := renderTemplate(v.str, scope)
			if err != nil {
				return nil, err
			}
			return jstr(s), nil
		case jArray:
			out := &jvalue{kind: jArray, arr: make([]*jvalue, len(v.arr))}
			for i, e := range v.arr {
				r, err := walk(e)
				if err != nil {
					return nil, err
				}
				out.arr[i] = r
			}
			return out, nil
		case jObject:
			if k, dup := v.hasDuplicateKeys(); dup {
				return nil, fmt.Errorf("migrate: duplicate key %q in tooth.json", k)
			}
			out := &jvalue{kind: jObject, obj: make([]jmember, len(v.obj))}
			for i, m := range v.obj {
				r, err := walk(m.val)
				if err != nil {
					return nil, err
				}
				out.obj[i] = jmember{key: m.key, val: r}
			}
			return out, nil
		}
		return v, nil
	}
	return walk(root)
}

// NormalizeRange validates a dependency range and returns the text lip would
// write back: npm=true uses SemVersionRange.ParseNpm (v1/v2 platform-level
// dependencies), false uses SemVersionRange.Parse. Tests may replace it to
// stay independent of package semver.
var NormalizeRange = func(s string, npm bool) (string, error) {
	var (
		r   semver.Range
		err error
	)
	if npm {
		r, err = semver.ParseNpmRange(s)
	} else {
		r, err = semver.ParseRange(s)
	}
	if err != nil {
		return "", err
	}
	return r.String(), nil
}
