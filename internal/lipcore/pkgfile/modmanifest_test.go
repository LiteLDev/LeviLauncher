package pkgfile

import (
	"errors"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
)

// The mod manifest of Candidate.LoadModManifest (src/Lip.Core/Mcmod/Contracts.cs).

func TestReadModName(t *testing.T) {
	const label = "github.com/a/b 1.0.0"
	for _, c := range []struct{ json, name, found string }{
		{`{"name":"LeviLamina","version":"1.0.0"}`, "LeviLamina", ""},
		{`{"name":"0a._-b"}`, "0a._-b", ""},
		{"\xef\xbb\xbf{\"name\":\"a\"}", "a", ""},
		{`[]`, "", "a JSON array"},
		{`"x"`, "", "a JSON string"},
		{`5`, "", "a JSON number"},
		{`true`, "", "a JSON boolean"},
		{`null`, "", "a JSON null"},
		{`{}`, "", `an object without "name"`},
		{`{"name":5}`, "", `an object whose "name" is a number`},
		{`{"name":{}}`, "", `an object whose "name" is a object`},
		{`{"name":""}`, "", `an object whose "name" is ""`},
		{`{"name":"-a"}`, "", `an object whose "name" is "-a"`},
		{`{"name":"a b"}`, "", `an object whose "name" is "a b"`},
	} {
		name, err := ReadModName([]byte(c.json), label)
		if c.found == "" {
			if err != nil || name != c.name {
				t.Errorf("%s: %q, %v", c.json, name, err)
			}
			continue
		}
		want := "Cannot install " + label + `: the manifest.json of its asset of type "mcmod" must be a JSON object` +
			` whose "name" is the mod name (letters, digits, '.', '_' and '-', starting with a letter or digit),` +
			" but it is " + c.found + ". This is a problem of the package; report it to its author."
		if err == nil || err.Error() != want {
			t.Errorf("%s: %v\nwant %s", c.json, err, want)
		}
	}
	for _, bad := range []string{`{"name":"a",}`, `{"name":"a","name":"b"}`, "{\"name\":\"\xc3\x28\"}", ``} {
		_, err := ReadModName([]byte(bad), label)
		var ie *InstallError
		var je *doc.JSONError
		if !errors.As(err, &ie) || ie.Message != "Cannot install "+label+`: lip could not read the manifest.json`+
			` of its asset of type "mcmod" to tell the mod name.` || !errors.As(err, &je) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if got := ModManifestMissing(label).Error(); got != "Cannot install "+label+`: its asset of type "mcmod" has`+
		" no manifest.json at the root of its archive, so lip cannot tell the mod name. This is a problem of the"+
		" package; report it to its author." {
		t.Errorf("%q", got)
	}
}
