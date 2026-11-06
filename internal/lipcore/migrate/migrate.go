// Package migrate converts tooth.json documents of any supported format
// (1, 2, 3) into a rendered format 3 document, porting lip 0.37.0
// src/Lip.Core/Migration/PackageManifests/*.
package migrate

import (
	"errors"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("migrate: not implemented")

// ToV3 takes raw tooth.json bytes and returns format 3 JSON bytes
// (any formatting; package manifest parses it).
//
//   - format 3: every JSON string value is rendered as a Scriban template
//     whose model is the top-level object (so "{{version}}" and "{{tooth}}"
//     work).
//   - format 1/2: converted per PackageManifestMigration ($(var) -> {{var}}
//     with model {version, path}; GOOS/GOARCH -> RID; asset type inferred
//     from URL extension; place src ending in "*" -> dir placement; script
//     mapping; invalid dependencies dropped; when any variant platform
//     contains "*", an empty variant with platform = rid is inserted at
//     index 0), then rendered.
//
// rid is the current .NET runtime identifier (e.g. "win-x64").
func ToV3(raw []byte, rid string) ([]byte, error) {
	return nil, ErrNotImplemented
}

// FormatVersion returns the "format_version" field of raw, or an error.
func FormatVersion(raw []byte) (int, error) {
	return 0, ErrNotImplemented
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
