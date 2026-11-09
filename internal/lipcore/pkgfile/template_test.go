package pkgfile

import (
	"errors"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// parseTOMLOfJSON parses the lip.toml of the value a JSON text writes.
func parseTOMLOfJSON(t *testing.T, text string) error {
	t.Helper()
	_, err := parseTOMLOf(t, text)
	return err
}

func parseTOMLOf(t *testing.T, text string) (*pkgmodel.Document, error) {
	t.Helper()
	v, err := doc.ParseJSON([]byte(text), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	toml, err := doc.MarshalTOML(v)
	if err != nil {
		t.Fatal(err)
	}
	return ParseTOML(toml)
}

func TestParseTOMLRendersTemplates(t *testing.T) {
	text := `{"format":4,"id":"example.com/owner/tool","version":"1.2.3","tag":"v{{version}}",` +
		`"info":{"name":"Tool"},"variants":{"":[{"side":"server","platform":"win-x64",` +
		`"assets":[{"type":"zip","urls":["https://example.com/{{id}}/{{tag}}/{{info.name}}-{{version}}.zip"]}]}]}}`
	d, err := parseTOMLOf(t, text)
	if err != nil {
		t.Fatal(err)
	}
	a := d.Variants[0].Entries[0].Assets[0]
	// Variables are the fields as written: {{tag}} is "v{{version}}", not "v1.2.3".
	if got, want := a.URLs[0], "https://example.com/example.com/owner/tool/v{{version}}/Tool-1.2.3.zip"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
}

func TestParseTOMLUndefinedVariableIsEmpty(t *testing.T) {
	text := strings.Replace(validDoc, `"id":"example.com/owner/tool"`, `"id":"example.com/owner/tool{{nope}}"`, 1)
	d, err := parseTOMLOf(t, text)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "example.com/owner/tool" {
		t.Errorf("id = %q", d.ID)
	}
}

func TestParseTOMLBadTemplateIsFormatError(t *testing.T) {
	for _, tpl := range []string{"{{ if }}", "{{ version | string.upcase }}"} {
		text := strings.Replace(validDoc, `"version":"1.2.3"`, `"version":"1.2.3","x":"`+tpl+`"`, 1)
		err := parseTOMLOfJSON(t, text)
		var fe *FormatError
		if !errors.As(err, &fe) {
			t.Errorf("%q: err = %v, want *FormatError", tpl, err)
		}
	}
}
