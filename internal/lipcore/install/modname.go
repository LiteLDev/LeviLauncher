package install

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// ReadModName is Candidate.LoadModManifest (src/Lip.Core/Mcmod/Contracts.cs):
// the mod name of a mod (protocol/package.md §4), the "name" of the
// manifest.json at the root of the archive of its asset of type "mcmod".
// label names the package in messages ("<identity> <version>"). It fails
// with *Error when the manifest is missing or invalid, or cannot be read.
func ReadModName(asset archive.Source, label string) (string, error) {
	const manifestFile = pkgmodel.ModManifestFileName
	cannotRead := func(err error) error {
		return &Error{Msg: fmt.Sprintf("Cannot install %s: lip could not read the %s of its asset"+
			" of type \"mcmod\" to tell the mod name.", label, manifestFile), Err: err}
	}

	keys, err := archive.ListKeys(asset)
	if err != nil {
		if isCancel(err) {
			return "", err
		}
		return "", cannotRead(err)
	}
	if !slices.Contains(keys, manifestFile) {
		return "", &Error{Msg: fmt.Sprintf("Cannot install %s: its asset of type \"mcmod\" has no"+
			" %s at the root of its archive, so lip cannot tell the mod name. This is a problem"+
			" of the package; report it to its author.", label, manifestFile)}
	}

	r, err := asset.Open(manifestFile)
	if err != nil {
		return "", cannotRead(err)
	}
	data, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		return "", cannotRead(err)
	}
	root, err := doc.ParseJSON(data, manifestFile)
	if err != nil {
		return "", cannotRead(err)
	}

	obj, isObject := root.(*doc.Object)
	var found string
	switch {
	case !isObject:
		found = "a JSON " + jsonKind(root)
	default:
		value, ok := obj.Get("name")
		name, isString := value.(string)
		switch {
		case !ok:
			found = `an object without "name"`
		case !isString:
			found = `an object whose "name" is a ` + jsonKind(value)
		case !ref.IsValidName(name):
			found = `an object whose "name" is "` + name + `"`
		default:
			return name, nil
		}
	}
	return "", &Error{Msg: fmt.Sprintf("Cannot install %s: the %s of its asset of type \"mcmod\""+
		" must be a JSON object whose \"name\" is the mod name (letters, digits, '.', '_' and '-',"+
		" starting with a letter or digit), but it is %s. This is a problem of the package;"+
		" report it to its author.", label, manifestFile, found)}
}

// jsonKind is Candidate.Kind: the kind of a JSON value for messages.
func jsonKind(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "object"
	}
}
