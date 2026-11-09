package pkgfile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// This file ports the part of LockStore.Parse that deserializes the
// state file: root.Deserialize<LockFile>(new JsonSerializerOptions {
// RespectNullableAnnotations = true, RespectRequiredConstructorParameters =
// true }) of System.Text.Json, with the members of LockFile and LockedPackage
// (src/Lip.Core/Mcmod/Contracts.cs). The JsonExceptions it throws are
// reproduced with their messages: their positions are byte offsets in the
// compact JSON text of the document (doc.MarshalJSONCompact), which has a
// single line.

const (
	lockFileType     = "Lip.Core.Mcmod.LockFile"
	lockedPackageTyp = "Lip.Core.Mcmod.LockedPackage"
)

// jsonError is a System.Text.Json JsonException (or the
// InvalidOperationException behind one): Message is its message, Err its
// inner exception.
type jsonError struct {
	Message string
	Err     error
}

func (e *jsonError) Error() string { return e.Message }
func (e *jsonError) Unwrap() error { return e.Err }

func jsonLen(v any) int { return len(doc.MarshalJSONCompact(v)) }

// tokenEnd is the position of the reader after the first token of the value
// starting at start: past "{" or "[", or past a whole scalar.
func tokenEnd(v any, start offset) int {
	switch v.(type) {
	case *doc.Object, []any:
		return start() + 1
	}
	return start() + jsonLen(v)
}

// tokenType is the JsonTokenType of the first token of a value.
func tokenType(v any) string {
	switch t := v.(type) {
	case *doc.Object:
		return "StartObject"
	case []any:
		return "StartArray"
	case string:
		return "String"
	case json.Number:
		return "Number"
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		return "Null"
	}
}

func position(path string, pos int) string {
	return fmt.Sprintf(" Path: %s | LineNumber: 0 | BytePositionInLine: %d.", path, pos)
}

func nullError(member, typ, path string, pos int) error {
	return &jsonError{Message: fmt.Sprintf("The property or field '%s' on type '%s' doesn't allow setting"+
		" null values. Consider updating its nullability annotation.", member, typ) + position(path, pos)}
}

func convertError(typ, path string, pos int, inner error) error {
	return &jsonError{Message: "The JSON value could not be converted to " + typ + "." + position(path, pos),
		Err: inner}
}

func missingError(typ string, names []string) error {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "'" + n + "'"
	}
	return &jsonError{Message: fmt.Sprintf("JSON deserialization for type '%s' was missing required properties"+
		" including: %s.", typ, strings.Join(quoted, ", "))}
}

// readSTJString reads a non-nullable string member.
func readSTJString(v any, member, owner, path string, start offset) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case nil:
		return "", nullError(member, owner, path, tokenEnd(v, start))
	}
	return "", convertError("System.String", path, tokenEnd(v, start), &jsonError{
		Message: fmt.Sprintf("Cannot get the value of a token type '%s' as a string.", tokenType(v))})
}

// offset is the byte offset of a value in the compact JSON text. Offsets are
// needed only for the messages of errors, so they are computed on demand.
type offset func() int

// members walks the members of an object written compactly from start (the
// offset of its "{"), calling f with each member and the offset of its value.
func members(o *doc.Object, start offset, f func(key string, value any, pos offset) error) error {
	keys := o.Keys()
	for i, k := range keys {
		v, _ := o.Get(k)
		pos := func() int {
			p := start() + 1
			for _, kj := range keys[:i] {
				vj, _ := o.Get(kj)
				p += jsonLen(kj) + 1 + jsonLen(vj) + 1 // "k":v,
			}
			return p + jsonLen(k) + 1
		}
		if err := f(k, v, pos); err != nil {
			return err
		}
	}
	return nil
}

// items is members for the items of an array.
func items(a []any, start offset, f func(i int, value any, pos offset) error) error {
	for i, v := range a {
		pos := func() int {
			p := start() + 1
			for _, vj := range a[:i] {
				p += jsonLen(vj) + 1
			}
			return p
		}
		if err := f(i, v, pos); err != nil {
			return err
		}
	}
	return nil
}

// lockedPackage is a LockedPackage as deserialized: Files may hold nils.
type lockedPackage struct {
	manifest any
	variant  string
	explicit bool
	ref      string
	commit   *string
	files    []*string
}

type lockFile struct {
	side, platform string
	// packages may hold nils, for JSON null items.
	packages []*lockedPackage
}

// deserializeLock is root.Deserialize<LockFile>(_readOptions) of
// LockStore.Parse, root being an object. "format" was checked to be the
// integer 4, which deserializes.
func deserializeLock(root *doc.Object) (*lockFile, error) {
	lf := &lockFile{}
	seen := map[string]bool{}
	err := members(root, func() int { return 0 }, func(k string, v any, pos offset) error {
		var err error
		switch k {
		case "side":
			lf.side, err = readSTJString(v, k, lockFileType, "$."+k, pos)
		case "platform":
			lf.platform, err = readSTJString(v, k, lockFileType, "$."+k, pos)
		case "packages":
			lf.packages, err = deserializePackages(v, pos)
		case pkgmodel.FormatKey:
		default:
			return nil
		}
		seen[k] = true
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := checkRequired(lockFileType, seen, "side", "platform", "packages"); err != nil {
		return nil, err
	}
	return lf, nil
}

func checkRequired(typ string, seen map[string]bool, names ...string) error {
	var missing []string
	for _, n := range names {
		if !seen[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return missingError(typ, missing)
	}
	return nil
}

func deserializePackages(v any, start offset) ([]*lockedPackage, error) {
	const path = "$.packages"
	arr, ok := v.([]any)
	if !ok {
		if v == nil {
			return nil, nullError("packages", lockFileType, path, tokenEnd(v, start))
		}
		return nil, convertError("System.Collections.Generic.List`1["+lockedPackageTyp+"]", path,
			tokenEnd(v, start), nil)
	}
	packages := make([]*lockedPackage, 0, len(arr))
	err := items(arr, start, func(i int, item any, pos offset) error {
		at := fmt.Sprintf("%s[%d]", path, i)
		switch o := item.(type) {
		case nil:
			packages = append(packages, nil)
			return nil
		case *doc.Object:
			p, err := deserializePackage(o, at, pos)
			packages = append(packages, p)
			return err
		default:
			return convertError(lockedPackageTyp, at, tokenEnd(item, pos), nil)
		}
	})
	if err != nil {
		return nil, err
	}
	return packages, nil
}

func deserializePackage(o *doc.Object, at string, start offset) (*lockedPackage, error) {
	p := &lockedPackage{}
	seen := map[string]bool{}
	err := members(o, start, func(k string, v any, pos offset) error {
		path := at + "." + k
		var err error
		switch k {
		case "manifest":
			p.manifest = v
		case "variant":
			p.variant, err = readSTJString(v, k, lockedPackageTyp, path, pos)
		case "explicit":
			b, ok := v.(bool)
			if !ok {
				return convertError("System.Boolean", path, tokenEnd(v, pos), &jsonError{
					Message: fmt.Sprintf("Cannot get the value of a token type '%s' as a boolean.", tokenType(v))})
			}
			p.explicit = b
		case "ref":
			p.ref, err = readSTJString(v, k, lockedPackageTyp, path, pos)
		case "commit":
			// Checked to be a string before deserializing.
			if s, ok := v.(string); ok {
				p.commit = &s
			}
		case "files":
			p.files, err = deserializeFiles(v, path, pos)
		default:
			return nil
		}
		seen[k] = true
		return err
	})
	if err != nil {
		return nil, err
	}
	return p, checkRequired(lockedPackageTyp, seen, "manifest", "variant", "explicit", "ref", "files")
}

func deserializeFiles(v any, path string, start offset) ([]*string, error) {
	arr, ok := v.([]any)
	if !ok {
		if v == nil {
			return nil, nullError("files", lockedPackageTyp, path, tokenEnd(v, start))
		}
		return nil, convertError("System.Collections.Generic.List`1[System.String]", path, tokenEnd(v, start), nil)
	}
	files := make([]*string, 0, len(arr))
	err := items(arr, start, func(i int, item any, pos offset) error {
		switch s := item.(type) {
		case nil:
			files = append(files, nil)
			return nil
		case string:
			files = append(files, &s)
			return nil
		}
		return convertError("System.String", fmt.Sprintf("%s[%d]", path, i), tokenEnd(item, pos), &jsonError{
			Message: fmt.Sprintf("Cannot get the value of a token type '%s' as a string.", tokenType(item))})
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
