package compat

import (
	"fmt"
	"slices"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/migrate"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

const (
	// ToothFileName is the file at the root of an archive of lip v0.x that
	// makes it a package (ToothCompat.ToothFileName).
	ToothFileName = "tooth.json"
	// FormatVersionKey is the member of tooth.json, tooth_lock.json and
	// liprc.json that holds their format (ToothCompat.FormatVersionKey).
	FormatVersionKey = "format_version"
)

// FormatError is lip's PackageFormatException for a tooth.json: it cannot be
// read as format 3, or not for the environment.
type FormatError struct {
	Msg string
	Err error
}

func (e *FormatError) Error() string {
	if e.Err != nil {
		return e.Msg + " " + e.Err.Error()
	}
	return e.Msg
}

func (e *FormatError) Unwrap() error { return e.Err }

func formatErrorf(err error, format string, args ...any) *FormatError {
	return &FormatError{Msg: fmt.Sprintf(format, args...), Err: err}
}

// unexpectedError is a tooth.json lip v0.38 reads but then fails on with an
// exception other than PackageFormatException.
type unexpectedError struct{ what string }

func (e *unexpectedError) Error() string { return e.what }

// FormatVersion is ToothCompat.FormatVersion: the format_version of a
// tooth.json, tooth_lock.json or liprc.json (a doc value) when it is an
// integer (PackageFileParser.IntegerMember).
func FormatVersion(root any) (int, bool) {
	o, ok := root.(*doc.Object)
	if !ok {
		return 0, false
	}
	v, ok := o.Get(FormatVersionKey)
	if !ok {
		return 0, false
	}
	n, ok := doc.IntegerMember(v)
	return int(n), ok
}

// Tooth is a tooth.json read as format 3 (lip's PackageManifest).
type Tooth struct {
	// ID is the "tooth" of the tooth.json, the package id.
	ID string
	// Version is its "version" (SemVersionStyles.Any); the zero Version
	// when it is null.
	Version semver.Version

	m *manifest.Manifest
	// info is the "info" object of format 3, or nil when it has none (absent
	// or null).
	info *doc.Object
	// raw are the "variants" of the format 3 document, one per variant of m,
	// to see the JSON nulls the manifest model reads as empty.
	raw []*doc.Object
	// nullVariants: "variants" is null, which lip's model keeps as a null
	// list that Derive dereferences.
	nullVariants bool
	// nullVersion: "version" is null, which lip's model keeps; Derive then
	// needs a tag version.
	nullVersion bool
}

// ParseToothJSON is PackageDocument.ParseToothJson: it reads tooth.json bytes
// as strict JSON (doc.ParseJSON), then as format 3 (ReadToothJSON). Its
// errors are those of ReadToothJSON.
func ParseToothJSON(data []byte) (*Tooth, error) {
	root, err := doc.ParseJSON(data, ToothFileName)
	if err != nil {
		return nil, &FormatError{Msg: err.Error()}
	}
	return readTooth(root, data)
}

// ReadToothJSON is ToothCompat.ReadToothJson: it reads a tooth.json, a doc
// value, as format 3. It fails with a *FormatError when it is not a JSON
// object with format_version 1, 2 or 3 that reads as format 3. Another error
// is a tooth.json lip v0.38 reads into its model but then fails on
// unexpectedly when it derives the package file (a null variant): callers fail as lip does, where they would skip a *FormatError.
func ReadToothJSON(root any) (*Tooth, error) {
	return readTooth(root, nil)
}

// readTooth reads root, whose JSON text is raw when known.
func readTooth(root any, raw []byte) (*Tooth, error) {
	fv, ok := FormatVersion(root)
	if !ok || fv < 1 || fv > 3 {
		has := "none."
		if ok {
			has = fmt.Sprintf("%d.", fv)
		}
		return nil, formatErrorf(nil, "The %s must have \"%s\": 1, 2 or 3, the formats of %s"+
			" that lip reads, but it has %s", ToothFileName, FormatVersionKey, ToothFileName, has)
	}
	if raw == nil {
		raw = doc.MarshalJSONCompact(root)
	}
	t, err := toothFromFormat3(raw)
	if ue, ok := err.(*unexpectedError); ok {
		return nil, fmt.Errorf("lip cannot read this %s of format %d: %s", ToothFileName, fv, ue.what)
	}
	if err != nil {
		return nil, formatErrorf(err, "The %s of format %d does not follow that format.",
			ToothFileName, fv)
	}
	return t, nil
}

// toothFromFormat3 runs PackageManifestMigration.Migrate and reads the
// result into the format 3 model.
func toothFromFormat3(raw []byte) (*Tooth, error) {
	v3, err := migrate.ToFormat3(raw)
	if err != nil {
		return nil, err
	}
	v, err := doc.ParseJSON(v3, ToothFileName)
	if err != nil {
		return nil, err
	}
	o := v.(*doc.Object)
	// lip v0.38 reads a null "version" into its model, which only a tag
	// version replaces, and null variants, which it fails on with a
	// NullReferenceException when it derives the package file. The manifest
	// model rejects both, so read the document without them, then report the
	// null variants unless the rest is not valid (a PackageFormatException).
	nullVersion, nullVariant := false, -1
	if version, ok := o.Get("version"); ok && version == nil {
		nullVersion = true
		o = o.Clone()
		o.Set("version", "0.0.0")
	}
	if list, ok := o.Get("variants"); ok {
		if items, ok := list.([]any); ok && slices.Contains(items, nil) {
			nullVariant = slices.Index(items, nil)
			o = o.Clone()
			o.Set("variants", slices.DeleteFunc(slices.Clone(items), func(v any) bool { return v == nil }))
		}
	}
	if nullVersion || nullVariant >= 0 {
		v3 = doc.MarshalJSONCompact(o)
	}
	m, err := manifest.Parse(v3)
	if err != nil {
		return nil, err
	}
	if nullVariant >= 0 {
		return nil, &unexpectedError{fmt.Sprintf("variants[%d] is null", nullVariant)}
	}
	t := &Tooth{ID: m.Tooth, m: m, nullVersion: nullVersion}
	if !nullVersion {
		if t.Version, err = semver.ParseAny(m.Version); err != nil {
			return nil, err
		}
	}

	// The manifest model reads JSON nulls as defaults; lip v0.38's model
	// keeps them, so look at the format 3 document itself.
	if info, ok := o.Get("info"); ok {
		t.info, _ = info.(*doc.Object)
	}
	variants, ok := o.Get("variants")
	switch list := variants.(type) {
	case nil:
		// lip's model has a null list of variants, which Derive
		// dereferences.
		t.nullVariants = ok
	case []any:
		for i, item := range list {
			vo, ok := item.(*doc.Object)
			if !ok {
				return nil, fmt.Errorf("variants[%d] is not an object", i)
			}
			// A label of null fails PackageId.IsValidVariant while reading.
			if label, ok := vo.Get("label"); ok && label == nil {
				return nil, fmt.Errorf("variants[%d].label is null", i)
			}
			t.raw = append(t.raw, vo)
		}
	}
	if len(t.raw) != len(m.Variants) {
		return nil, fmt.Errorf("the format 3 document has %d variants, the model %d",
			len(t.raw), len(m.Variants))
	}
	return t, nil
}
