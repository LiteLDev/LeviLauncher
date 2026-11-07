package pkgfile

import (
	"encoding/json"
	"fmt"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
)

// InstallError is McmodInstallException: installing a package failed
// (protocol/workspace.md §4). Message is lip's message exactly; Err is the
// inner exception (nil when none).
type InstallError struct {
	Message string
	Err     error
}

func (e *InstallError) Error() string { return e.Message }
func (e *InstallError) Unwrap() error { return e.Err }

// ReadModName reads the mod name of a mod (protocol/package.md §4) from the
// bytes of the manifest.json at the root of the archive of its asset of type
// "mcmod", as Candidate.LoadModManifest (src/Lip.Core/Mcmod/Contracts.cs)
// does: strict JSON, an object whose "name" is a string that is a mod name
// (ref.IsValidName). label names the package in messages: its identity and
// version, "<identity> <version>". The errors are *InstallError.
func ReadModName(data []byte, label string) (string, error) {
	file := pkgmodel.ModManifestFileName
	root, err := doc.ParseJSON(data, file)
	if err != nil {
		return "", ModManifestUnreadable(label, err)
	}
	if o, ok := root.(*doc.Object); ok {
		if v, ok := o.Get("name"); ok {
			if name, ok := v.(string); ok && ref.IsValidName(name) {
				return name, nil
			}
		}
	}
	var found string
	o, isObject := root.(*doc.Object)
	value, hasName := o.Get("name")
	switch {
	case !isObject:
		found = "a JSON " + kind(root)
	case !hasName:
		found = "an object without \"name\""
	default:
		if s, ok := value.(string); ok {
			found = fmt.Sprintf("an object whose \"name\" is \"%s\"", s)
		} else {
			found = "an object whose \"name\" is a " + kind(value)
		}
	}
	return "", &InstallError{Message: fmt.Sprintf("Cannot install %s: the %s of its asset of type \"mcmod\""+
		" must be a JSON object whose \"name\" is the mod name (letters, digits, '.', '_' and '-',"+
		" starting with a letter or digit), but it is %s. This is a problem of the package; report it"+
		" to its author.", label, file, found)}
}

// ModManifestMissing is the error of a mod whose asset of type "mcmod" has
// no manifest.json at the root of its archive.
func ModManifestMissing(label string) error {
	return &InstallError{Message: fmt.Sprintf("Cannot install %s: its asset of type \"mcmod\" has no %s at"+
		" the root of its archive, so lip cannot tell the mod name. This is a problem of the package;"+
		" report it to its author.", label, pkgmodel.ModManifestFileName)}
}

// ModManifestUnreadable is the error of a mod whose manifest.json could not
// be read: its asset could not be obtained (ArchiveUnavailableException), the
// archive or the file could not be read, or the file is not strict JSON;
// cause is why.
func ModManifestUnreadable(label string, cause error) error {
	return &InstallError{Message: fmt.Sprintf("Cannot install %s: lip could not read the %s of its asset"+
		" of type \"mcmod\" to tell the mod name.", label, pkgmodel.ModManifestFileName), Err: cause}
}

// kind is Candidate.Kind.
func kind(element any) string {
	switch element.(type) {
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
