// Package pkgfile ports lip 0.38's package file parser
// (src/Lip.Core/Mcmod/Parsing/PackageFileParser.cs), the mod manifest reader
// (Candidate.GetModManifest of src/Lip.Core/Mcmod/Contracts.cs) and the
// workspace file store (src/Lip.Core/Mcmod/Lock/LockStore.cs), as of lip
// v0.38.0-beta.1 (protocol/package.md §3-4, protocol/workspace.md §7.1,
// protocol/compatibility.md §3, lip.schema.yaml, workspace.schema.yaml).
//
// Documents are the JSON values of package doc (*doc.Object and friends);
// the results are the plain types of package pkgmodel.
package pkgfile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pathglob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// FormatError is PackageFormatException: a package file, or an entry of one,
// is not valid (protocol/package.md §3). Message is lip's message exactly;
// Err is the inner exception (nil when none). Newer marks
// NewerPackageFormatException: the package file is of a format newer than
// this lip knows, and updating lip helps.
type FormatError struct {
	Message string
	Newer   bool
	Err     error
}

func (e *FormatError) Error() string { return e.Message }
func (e *FormatError) Unwrap() error { return e.Err }

// invalid is PackageFileParser.Invalid.
func invalid(path, reason string, inner error) *FormatError {
	return &FormatError{Message: fmt.Sprintf("Invalid package file at %s: %s.", path, reason), Err: inner}
}

var (
	// runtimeRegex is a RID: dot- or dash-separated parts of letters, digits
	// and '_' (e.g. "win-x64", "linux-musl-arm64", "osx.13-arm64").
	runtimeRegex     = regexp.MustCompile(`^[A-Za-z0-9_]+(?:[.-][A-Za-z0-9_]+)*$`)
	driveLetterRegex = regexp.MustCompile(`^[A-Za-z]:`)
)

// IsRuntime is PackageFileParser.IsRuntime: whether a string is a runtime, a
// .NET runtime identifier without wildcards.
func IsRuntime(value string) bool { return runtimeRegex.MatchString(value) }

// IsPlatform is McmodPlatformExtensions.TryParse: "server" or "client",
// case-sensitively.
func IsPlatform(s string) bool { return s == pkgmodel.PlatformServer || s == pkgmodel.PlatformClient }

// Format is PackageFileParser.Format: the "format" of a package file or of
// the workspace file when it is an integer (see doc.IntegerMember).
func Format(root any) (int64, bool) { return IntegerMember(root, pkgmodel.FormatKey) }

// IntegerMember is PackageFileParser.IntegerMember: member key of an object
// when it is an integer.
func IntegerMember(root any, key string) (int64, bool) {
	o, ok := root.(*doc.Object)
	if !ok {
		return 0, false
	}
	v, ok := o.Get(key)
	if !ok {
		return 0, false
	}
	return doc.IntegerMember(v)
}

// ParseYAML is PackageFileParser.Parse(byte[]): reads lip.yaml, a YAML file
// (protocol/README.md), and parses it as a package file. A file that is not a
// YAML file of the kind lip reads is a *FormatError whose Err is the
// *doc.YAMLError (whose Error() is lip's message, the file name not being
// part of it).
func ParseYAML(data []byte) (*pkgmodel.Document, error) {
	root, err := doc.ParseYAML(data, "")
	if err != nil {
		return nil, &FormatError{
			Message: "The " + pkgmodel.PackageFileName + " is not a valid YAML file of the kind lip reads.",
			Err:     err,
		}
	}
	return Parse(root)
}

// Parse is PackageFileParser.Parse(JsonElement): parses and validates a
// package file (lip.schema.yaml, protocol/package.md §3), checking in lip's
// order and failing with lip's message. Keys the schema does not describe are
// ignored. The errors are *FormatError.
func Parse(root any) (*pkgmodel.Document, error) {
	obj, err := requireObject(root, "$")
	if err != nil {
		return nil, err
	}
	format, ok := Format(root)
	if ok && format > pkgmodel.Format {
		return nil, &FormatError{Newer: true, Message: fmt.Sprintf("The package file has \"%s\": %d, a format"+
			" this lip does not know (it knows format %d). The package needs a newer lip: update lip.",
			pkgmodel.FormatKey, format, pkgmodel.Format)}
	}
	if !ok || format != pkgmodel.Format {
		return nil, invalid("$."+pkgmodel.FormatKey, fmt.Sprintf("must be %d", pkgmodel.Format), nil)
	}

	id, err := readRequired(obj, "id", "$", readString)
	if err != nil {
		return nil, err
	}
	if err := ref.CheckModulePath(id); err != nil {
		return nil, invalid("$.id", fmt.Sprintf("'%s' is not a valid Go module path", id), err)
	}

	version, err := readRequired(obj, "version", "$", readSemVersion)
	if err != nil {
		return nil, err
	}
	// Raw is a copy, as upstream's Json = root.Clone().
	d := &pkgmodel.Document{Format: pkgmodel.Format, ID: id, Version: version, Raw: obj.Clone()}
	if info, ok := obj.Get("info"); ok {
		if d.Info, err = readInfo(info, "$.info"); err != nil {
			return nil, err
		}
	}

	variants, err := readRequired(obj, "variants", "$", requireObject)
	if err != nil {
		return nil, err
	}
	d.Variants = []pkgmodel.VariantEntries{}
	for _, name := range variants.Keys() {
		value, _ := variants.Get(name)
		path := fmt.Sprintf("$.variants[\"%s\"]", name)
		if name != "" && !ref.IsValidVariant(name) {
			return nil, invalid(path, fmt.Sprintf("'%s' is not a variant", name), nil)
		}
		list, err := readArray(value, path, true, parseEntryAt)
		if err != nil {
			return nil, err
		}
		for i := range list {
			for j := 0; j < i; j++ {
				if list[i].Platform == list[j].Platform && list[i].Runtime == list[j].Runtime {
					return nil, invalid(fmt.Sprintf("%s[%d]", path, i),
						fmt.Sprintf("has the platform and the runtime of %s[%d]", path, j), nil)
				}
			}
		}
		d.Variants = append(d.Variants, pkgmodel.VariantEntries{Name: name, Entries: list})
	}
	return d, nil
}

// ParseEntry is PackageFileParser.ParseEntry: parses an entry of a package
// file, its paths in messages starting at "$".
func ParseEntry(element any) (pkgmodel.Entry, error) { return parseEntryAt(element, "$") }

// IsValidPlacement is PackageFileParser.IsValidPlacement: whether a member of
// place is valid (lip.schema.yaml "place").
func IsValidPlacement(src, dest string) bool {
	o := doc.NewObject()
	o.Set("src", src)
	o.Set("dest", dest)
	_, err := readPlacement(o, "$")
	return err == nil
}

func parseEntryAt(element any, path string) (pkgmodel.Entry, error) {
	var e pkgmodel.Entry
	obj, err := requireObject(element, path)
	if err != nil {
		return e, err
	}

	assets, err := readOptional(obj, "assets", path, arrayOf(readAsset))
	if err != nil {
		return e, err
	}
	mcmods := 0
	for _, a := range assets {
		if a.Type == pkgmodel.AssetMcmod {
			mcmods++
		}
	}
	if mcmods > 1 {
		return e, invalid(path+".assets", "has more than one asset of type \"mcmod\"", nil)
	}

	if e.Platform, err = readRequired(obj, "platform", path, readPlatform); err != nil {
		return e, err
	}
	if e.Runtime, err = readRequired(obj, "runtime", path, readRuntime); err != nil {
		return e, err
	}
	if e.Deps, err = readOptional(obj, "deps", path, readDependencies); err != nil {
		return e, err
	}
	if e.Preserve, err = readOptional(obj, "preserve", path, arrayOf(readGlob)); err != nil {
		return e, err
	}
	if e.Purge, err = readOptional(obj, "purge", path, arrayOf(readGlob)); err != nil {
		return e, err
	}
	if obj.Has("hooks") {
		if e.Hooks, err = readHooks(mustGet(obj, "hooks"), path+".hooks"); err != nil {
			return e, err
		}
	} else {
		e.Hooks = emptyHooks()
	}
	e.Assets = emptyIfNil(assets)
	e.Deps = emptyIfNil(e.Deps)
	e.Preserve = emptyIfNil(e.Preserve)
	e.Purge = emptyIfNil(e.Purge)
	e.Raw = obj.Clone()
	return e, nil
}

func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func readInfo(element any, path string) (*pkgmodel.Info, error) {
	obj, err := requireObject(element, path)
	if err != nil {
		return nil, err
	}
	info := &pkgmodel.Info{}
	if info.Name, err = readOptional(obj, "name", path, readString); err != nil {
		return nil, err
	}
	if info.Description, err = readOptional(obj, "description", path, readString); err != nil {
		return nil, err
	}
	if info.Tags, err = readOptional(obj, "tags", path, arrayOf(readString)); err != nil {
		return nil, err
	}
	if obj.Has("logo") {
		logo, err := readString(mustGet(obj, "logo"), path+".logo")
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(logo, "https://") {
			return nil, invalid(path+".logo", fmt.Sprintf("'%s' does not start with \"https://\"", logo), nil)
		}
		info.Logo = logo
	}
	return info, nil
}

func mustGet(o *doc.Object, k string) any {
	v, _ := o.Get(k)
	return v
}

func readPlatform(element any, path string) (string, error) {
	if s, ok := element.(string); ok && IsPlatform(s) {
		return s, nil
	}
	return "", invalid(path, "must be \"server\" or \"client\"", nil)
}

func readRuntime(element any, path string) (string, error) {
	runtime, err := readString(element, path)
	if err != nil {
		return "", err
	}
	if !IsRuntime(runtime) {
		return "", invalid(path, fmt.Sprintf("'%s' is not a runtime identifier", runtime), nil)
	}
	return runtime, nil
}

func readDependencies(element any, path string) ([]pkgmodel.Dep, error) {
	obj, err := requireObject(element, path)
	if err != nil {
		return nil, err
	}
	deps := []pkgmodel.Dep{}
	for _, name := range obj.Keys() {
		value, _ := obj.Get(name)
		memberPath := fmt.Sprintf("%s[\"%s\"]", path, name)
		source, reason, ok := ref.TryParse(name, false)
		if !ok || source.Kind != ref.KindSpec || source.GitRef != nil {
			suffix := ""
			if !ok {
				suffix = " (" + reason + ")"
			}
			return nil, invalid(memberPath,
				fmt.Sprintf("'%s' is not <package id> or <package id>#<variant>%s", name, suffix), nil)
		}
		text, err := readString(value, memberPath)
		if err != nil {
			return nil, err
		}
		if _, err := semver.ParseVersionRange(text); err != nil {
			return nil, invalid(memberPath, fmt.Sprintf("'%s' is not a version range", text), err)
		}
		deps = append(deps, pkgmodel.Dep{Key: name, Range: text})
	}
	return deps, nil
}

func readAsset(element any, path string) (pkgmodel.Asset, error) {
	obj, err := requireObject(element, path)
	if err != nil {
		return pkgmodel.Asset{}, err
	}
	typ, err := readRequired(obj, "type", path, readString)
	if err != nil {
		return pkgmodel.Asset{}, err
	}
	switch typ {
	case "self":
		place, err := readOptional(obj, "place", path, readPlacements)
		if err != nil {
			return pkgmodel.Asset{}, err
		}
		return pkgmodel.Asset{Type: pkgmodel.AssetSelf, URLs: []string{}, Place: emptyIfNil(place)}, nil
	case "zip":
		urls, err := readRequired(obj, "urls", path, readURLs)
		if err != nil {
			return pkgmodel.Asset{}, err
		}
		place, err := readOptional(obj, "place", path, readPlacements)
		if err != nil {
			return pkgmodel.Asset{}, err
		}
		return pkgmodel.Asset{Type: pkgmodel.AssetZip, URLs: urls, Place: emptyIfNil(place)}, nil
	case "mcmod":
		if obj.Has("place") {
			return pkgmodel.Asset{}, invalid(path+".place", "is not allowed for an asset of type \"mcmod\"", nil)
		}
		urls, err := readRequired(obj, "urls", path, readURLs)
		if err != nil {
			return pkgmodel.Asset{}, err
		}
		return pkgmodel.Asset{Type: pkgmodel.AssetMcmod, URLs: urls, Place: []pkgmodel.Placement{}}, nil
	default:
		return pkgmodel.Asset{}, invalid(path+".type", "must be \"self\", \"zip\" or \"mcmod\"", nil)
	}
}

func readURLs(element any, path string) ([]string, error) {
	return readArray(element, path, true, readString)
}

func readPlacements(element any, path string) ([]pkgmodel.Placement, error) {
	return readArray(element, path, false, readPlacement)
}

func readPlacement(element any, path string) (pkgmodel.Placement, error) {
	obj, err := requireObject(element, path)
	if err != nil {
		return pkgmodel.Placement{}, err
	}
	src, err := readRequired(obj, "src", path, readPath)
	if err != nil {
		return pkgmodel.Placement{}, err
	}
	dest, err := readRequired(obj, "dest", path, readPath)
	if err != nil {
		return pkgmodel.Placement{}, err
	}
	p := pkgmodel.Placement{Src: src, Dest: dest}

	destIsDirectory := dest == "" || strings.HasSuffix(dest, "/")
	switch p.Form() {
	case pkgmodel.PlacementGlob:
		if _, err := pathglob.Compile(src); err != nil {
			return p, invalid(path+".src", fmt.Sprintf("'%s' is not a glob", src), err)
		}
		if !destIsDirectory {
			return p, invalid(path+".dest", fmt.Sprintf("'%s' must be \"\" or end with '/' for a glob src", dest), nil)
		}
	case pkgmodel.PlacementDirectory:
		if !destIsDirectory {
			return p, invalid(path+".dest",
				fmt.Sprintf("'%s' must be \"\" or end with '/' for a directory src", dest), nil)
		}
	case pkgmodel.PlacementFile:
		if destIsDirectory {
			return p, invalid(path+".dest", fmt.Sprintf("'%s' must name a file for a file src", dest), nil)
		}
	}
	return p, nil
}

// readPath reads a path (lip.schema.yaml "path").
func readPath(element any, path string) (string, error) {
	value, err := readString(element, path)
	if err != nil {
		return "", err
	}
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
	case slices.Contains(strings.Split(value, "/"), ".."):
		reason = "must not contain a '..' segment"
	default:
		return value, nil
	}
	return "", invalid(path, fmt.Sprintf("'%s' %s", value, reason), nil)
}

func readGlob(element any, path string) (string, error) {
	pattern, err := readString(element, path)
	if err != nil {
		return "", err
	}
	if _, err := pathglob.Compile(pattern); err != nil {
		return "", invalid(path, fmt.Sprintf("'%s' is not a glob", pattern), err)
	}
	return pattern, nil
}

func emptyHooks() pkgmodel.Hooks {
	return pkgmodel.Hooks{PreInstall: []string{}, PostInstall: []string{}, PreUninstall: []string{},
		PostUninstall: []string{}}
}

func readHooks(element any, path string) (pkgmodel.Hooks, error) {
	h := emptyHooks()
	obj, err := requireObject(element, path)
	if err != nil {
		return h, err
	}
	for _, m := range []struct {
		key string
		dst *[]string
	}{{"pre_install", &h.PreInstall}, {"post_install", &h.PostInstall},
		{"pre_uninstall", &h.PreUninstall}, {"post_uninstall", &h.PostUninstall}} {
		if obj.Has(m.key) {
			if *m.dst, err = readArray(mustGet(obj, m.key), path+"."+m.key, false, readString); err != nil {
				return h, err
			}
		}
	}
	return h, nil
}

func readSemVersion(element any, path string) (string, error) {
	s, err := readString(element, path)
	if err != nil {
		return "", err
	}
	if _, err := semver.ParseStrict(s); err != nil {
		return "", invalid(path, fmt.Sprintf("'%s' is not a version", s), nil)
	}
	return s, nil
}

// readString reads a string. (lip also rejects a string holding an escaped
// lone surrogate, "is not a valid Unicode string"; package doc never yields
// one: YAML files with one are rejected while reading.)
func readString(element any, path string) (string, error) {
	s, ok := element.(string)
	if !ok {
		return "", invalid(path, "must be a string, not "+describe(element), nil)
	}
	return s, nil
}

func requireObject(element any, path string) (*doc.Object, error) {
	o, ok := element.(*doc.Object)
	if !ok {
		return nil, invalid(path, "must be an object, not "+describe(element), nil)
	}
	return o, nil
}

type reader[T any] func(element any, path string) (T, error)

func arrayOf[T any](readItem reader[T]) reader[[]T] {
	return func(e any, p string) ([]T, error) { return readArray(e, p, false, readItem) }
}

func readArray[T any](element any, path string, nonEmpty bool, readItem reader[T]) ([]T, error) {
	arr, ok := element.([]any)
	if !ok {
		return nil, invalid(path, "must be an array, not "+describe(element), nil)
	}
	if nonEmpty && len(arr) == 0 {
		return nil, invalid(path, "must not be empty", nil)
	}
	out := make([]T, 0, len(arr))
	for i, item := range arr {
		v, err := readItem(item, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func readRequired[T any](parent *doc.Object, key, parentPath string, read reader[T]) (T, error) {
	value, ok := parent.Get(key)
	if !ok {
		var zero T
		return zero, invalid(parentPath+"."+key, "is required", nil)
	}
	return read(value, parentPath+"."+key)
}

// readOptional reads member key when present; the zero value otherwise.
func readOptional[T any](parent *doc.Object, key, parentPath string, read reader[T]) (T, error) {
	value, ok := parent.Get(key)
	if !ok {
		var zero T
		return zero, nil
	}
	return read(value, parentPath+"."+key)
}

// describe is PackageFileParser.Describe.
func describe(element any) string {
	switch element.(type) {
	case *doc.Object:
		return "an object"
	case []any:
		return "an array"
	case string:
		return "a string"
	case json.Number:
		return "a number"
	case bool:
		return "a boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", element)
	}
}
