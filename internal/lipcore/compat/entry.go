package compat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pathglob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// This file ports the part of PackageFileParser
// (src/Lip.Core/Mcmod/Parsing/PackageFileParser.cs, v0.40.0) that
// ToothCompat.Derive runs on the entries it derives (ParseEntry) and on the
// placements it converts (IsValidPlacement). A derived entry only holds
// members of the types the schema wants, so what is left to check are the
// values: the side, the platform, the dependency keys and ranges, the URLs of a zip
// asset, the placements and the globs.

var (
	// runtimeRegex is PackageFileParser.RuntimeRegex: a RID, dot- or
	// dash-separated parts of letters, digits and '_'.
	runtimeRegex = regexp.MustCompile(`^[A-Za-z0-9_]+(?:[.-][A-Za-z0-9_]+)*$`)
	// driveLetterRegex is PackageFileParser.DriveLetterRegex.
	driveLetterRegex = regexp.MustCompile(`^[A-Za-z]:`)
)

// IsRuntime is PackageFileParser.IsRuntime: whether a string is a runtime, a
// .NET runtime identifier without wildcards.
func IsRuntime(value string) bool { return runtimeRegex.MatchString(value) }

// checkPath is PackageFileParser.ReadPath: a path of lip.schema.toml.
func checkPath(value string) error {
	var reason string
	switch {
	case strings.HasPrefix(value, "/"):
		reason = "must not start with '/'"
	case driveLetterRegex.MatchString(value):
		reason = "must not start with a drive letter"
	case strings.Contains(value, "//"):
		reason = "must not contain '//'"
	case strings.Contains(value, `\`):
		reason = `must not contain '\'`
	case hasDotDotSegment(value):
		reason = "must not contain a '..' segment"
	default:
		return nil
	}
	return fmt.Errorf("'%s' %s", value, reason)
}

func hasDotDotSegment(path string) bool {
	for _, s := range strings.Split(path, "/") {
		if s == ".." {
			return true
		}
	}
	return false
}

// checkPlacement is PackageFileParser.ReadPlacement.
func checkPlacement(src, dest string) error {
	if err := checkPath(src); err != nil {
		return fmt.Errorf("src: %w", err)
	}
	if err := checkPath(dest); err != nil {
		return fmt.Errorf("dest: %w", err)
	}
	destIsDirectory := dest == "" || strings.HasSuffix(dest, "/")
	switch (pkgmodel.Placement{Src: src, Dest: dest}).Form() {
	case pkgmodel.PlacementGlob:
		if _, err := pathglob.Compile(src); err != nil {
			return fmt.Errorf("src: '%s' is not a glob: %w", src, err)
		}
		if !destIsDirectory {
			return fmt.Errorf("dest: '%s' must be \"\" or end with '/' for a glob src", dest)
		}
	case pkgmodel.PlacementDirectory:
		if !destIsDirectory {
			return fmt.Errorf("dest: '%s' must be \"\" or end with '/' for a directory src", dest)
		}
	case pkgmodel.PlacementFile:
		if destIsDirectory {
			return fmt.Errorf("dest: '%s' must name a file for a file src", dest)
		}
	}
	return nil
}

// IsValidPlacement is PackageFileParser.IsValidPlacement: whether a member of
// place is valid (lip.schema.toml "place").
func IsValidPlacement(src, dest string) bool { return checkPlacement(src, dest) == nil }

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.(string))
	}
	return out
}

// parseEntry is PackageFileParser.ParseEntry for an entry convertEntry
// derived, returning it as a pkgmodel.Entry whose Raw is e.
func parseEntry(e *doc.Object) (*pkgmodel.Entry, error) {
	entry := &pkgmodel.Entry{Raw: e}
	nMcmod := 0
	if v, ok := e.Get("assets"); ok {
		for i, item := range v.([]any) {
			a := item.(*doc.Object)
			typ, _ := stringMember(a, "type")
			asset := pkgmodel.Asset{Type: pkgmodel.AssetType(typ)}
			if asset.Type == pkgmodel.AssetMcmod {
				nMcmod++
			}
			if asset.Type != pkgmodel.AssetSelf {
				urls, _ := a.Get("urls")
				asset.URLs = stringList(urls)
				if len(asset.URLs) == 0 {
					return nil, fmt.Errorf("Invalid package file at $.assets[%d].urls: must not be empty.", i)
				}
			}
			if p, ok := a.Get("place"); ok {
				for j, item := range p.([]any) {
					po := item.(*doc.Object)
					src, _ := stringMember(po, "src")
					dest, _ := stringMember(po, "dest")
					if err := checkPlacement(src, dest); err != nil {
						return nil, fmt.Errorf("Invalid package file at $.assets[%d].place[%d].%w.", i, j, err)
					}
					asset.Place = append(asset.Place, pkgmodel.Placement{Src: src, Dest: dest})
				}
			}
			entry.Assets = append(entry.Assets, asset)
		}
	}
	if nMcmod > 1 {
		return nil, fmt.Errorf("Invalid package file at $.assets: has more than one asset of type \"mcmod\".")
	}

	entry.Side, _ = stringMember(e, "side")
	if entry.Side != pkgmodel.SideServer && entry.Side != pkgmodel.SideClient {
		return nil, fmt.Errorf("Invalid package file at $.side: must be \"server\" or \"client\".")
	}
	entry.Platform, _ = stringMember(e, "platform")
	if !IsRuntime(entry.Platform) {
		return nil, fmt.Errorf("Invalid package file at $.platform: '%s' is not a runtime identifier.", entry.Platform)
	}

	if v, ok := e.Get("deps"); ok {
		deps := v.(*doc.Object)
		for _, key := range deps.Keys() {
			r, reason, ok := ref.TryParse(key, false)
			if !ok || r.Kind != ref.KindSpec || r.GitRef != nil {
				detail := ""
				if !ok {
					detail = " (" + reason + ")"
				}
				return nil, fmt.Errorf("Invalid package file at $.deps[\"%s\"]: '%s' is not <package id> or"+
					" <package id>#<variant>%s.", key, key, detail)
			}
			text, _ := stringMember(deps, key)
			if _, err := semver.ParseVersionRange(text); err != nil {
				return nil, fmt.Errorf("Invalid package file at $.deps[\"%s\"]: '%s' is not a version range.", key, text)
			}
			entry.Deps = append(entry.Deps, pkgmodel.Dep{Key: key, Range: text})
		}
	}

	for _, f := range []struct {
		key string
		dst *[]string
	}{{"preserve", &entry.Preserve}, {"purge", &entry.Purge}} {
		if v, ok := e.Get(f.key); ok {
			for i, p := range stringList(v) {
				if _, err := pathglob.Compile(p); err != nil {
					return nil, fmt.Errorf("Invalid package file at $.%s[%d]: '%s' is not a glob.", f.key, i, p)
				}
				*f.dst = append(*f.dst, p)
			}
		}
	}

	if v, ok := e.Get("hooks"); ok {
		h := v.(*doc.Object)
		get := func(key string) []string {
			l, _ := h.Get(key)
			return stringList(l)
		}
		entry.Hooks = pkgmodel.Hooks{
			PreInstall:    get("pre_install"),
			PostInstall:   get("post_install"),
			PreUninstall:  get("pre_uninstall"),
			PostUninstall: get("post_uninstall"),
		}
	}
	return entry, nil
}
