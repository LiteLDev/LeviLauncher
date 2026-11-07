package ref

import (
	"errors"
	"strings"
	"testing"
)

// Ports of tests/Golang.Org.X.Mod.Tests/ModuleTests.cs (v0.38.0-beta.1).

var checkPathTests = []struct {
	path string
	ok   bool
}{
	{"x.y/z", true},
	{"x.y", true},

	{"", false},
	{"x.y/\xFFz", false},
	{"/x.y/z", false},
	{"x./z", false},
	{".x/z", false},
	{"-x/z", false},
	{"x..y/z", true},
	{"x.y/z/../../w", false},
	{"x.y//z", false},
	{"x.y/z//w", false},
	{"x.y/z/", false},

	{"x.y/z/v0", false},
	{"x.y/z/v1", false},
	{"x.y/z/v2", true},
	{"x.y/z/v2.0", false},
	{"X.y/z", false},

	{"!x.y/z", false},
	{"_x.y/z", false},
	{"x.y!/z", false},
	{"x.y\"/z", false},
	{"x.y#/z", false},
	{"x.y$/z", false},
	{"x.y%/z", false},
	{"x.y&/z", false},
	{"x.y'/z", false},
	{"x.y(/z", false},
	{"x.y)/z", false},
	{"x.y*/z", false},
	{"x.y+/z", false},
	{"x.y,/z", false},
	{"x.y-/z", true},
	{"x.y./zt", false},
	{"x.y:/z", false},
	{"x.y;/z", false},
	{"x.y</z", false},
	{"x.y=/z", false},
	{"x.y>/z", false},
	{"x.y?/z", false},
	{"x.y@/z", false},
	{"x.y[/z", false},
	{"x.y\\/z", false},
	{"x.y]/z", false},
	{"x.y^/z", false},
	{"x.y_/z", false},
	{"x.y`/z", false},
	{"x.y{/z", false},
	{"x.y}/z", false},
	{"x.y~/z", false},
	{"x.y/z!", false},
	{"x.y/z\"", false},
	{"x.y/z#", false},
	{"x.y/z$", false},
	{"x.y/z%", false},
	{"x.y/z&", false},
	{"x.y/z'", false},
	{"x.y/z(", false},
	{"x.y/z)", false},
	{"x.y/z*", false},
	{"x.y/z++", false},
	{"x.y/z,", false},
	{"x.y/z-", true},
	{"x.y/z.t", true},
	{"x.y/z/t", true},
	{"x.y/z:", false},
	{"x.y/z;", false},
	{"x.y/z<", false},
	{"x.y/z=", false},
	{"x.y/z>", false},
	{"x.y/z?", false},
	{"x.y/z@", false},
	{"x.y/z[", false},
	{"x.y/z\\", false},
	{"x.y/z]", false},
	{"x.y/z^", false},
	{"x.y/z_", true},
	{"x.y/z`", false},
	{"x.y/z{", false},
	{"x.y/z}", false},
	{"x.y/z~", true},
	{"x.y/x.foo", true},
	{"x.y/aux.foo", false},
	{"x.y/prn", false},
	{"x.y/prn2", true},
	{"x.y/com", true},
	{"x.y/com1", false},
	{"x.y/com1.txt", false},
	{"x.y/calm1", true},
	{"x.y/z~", true},
	{"x.y/z~0", false},
	{"x.y/z~09", false},
	{"x.y/z09", true},
	{"x.y/z09~", true},
	{"x.y/z09~09z", true},
	{"x.y/z09~09z~09", false},
	{"github.com/!123/logrus", false},
	{"github.com/user/unicode/испытание", false},
	{"../x", false},
	{"./y", false},
	{"x:y", false},
	{`\temp\foo`, false},
	{".gitignore", false},
	{".github/ISSUE_TEMPLATE", false},
	{"x☺y", false},
}

func TestCheckModulePath(t *testing.T) {
	for _, tc := range checkPathTests {
		err := CheckModulePath(tc.path)
		if (err == nil) != tc.ok {
			t.Errorf("CheckModulePath(%q) = %v, want ok %v", tc.path, err, tc.ok)
		}
	}
}

func TestCheckModulePathInvalidUTF8(t *testing.T) {
	// A string that is not valid UTF-8 is not a module path (the C# test
	// uses lone surrogates).
	for _, p := range []string{"example.com/a\xed\xa0\x80b", "example.com/\xed\xb0\x80"} {
		if CheckModulePath(p) == nil {
			t.Errorf("CheckModulePath(%q) succeeded", p)
		}
	}
}

func TestCheckModulePathMissingDotNamesPathAndReason(t *testing.T) {
	err := CheckModulePath("LeviLamina")
	var pe *ModulePathError
	if !errors.As(err, &pe) {
		t.Fatalf("error %v", err)
	}
	if pe.Path != "LeviLamina" || !strings.Contains(err.Error(), `"LeviLamina"`) ||
		!strings.Contains(err.Error(), "github.com") || !strings.Contains(err.Error(), pe.Reason) {
		t.Errorf("error %q", err)
	}
	want := `The module path "LeviLamina" is not valid: its first path element "LeviLamina" has no dot,` +
		` but the first element must be a host name such as github.com.`
	if err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
	if errors.Unwrap(err).Error() != pe.Reason {
		t.Error("Unwrap is not the reason")
	}
}

func TestCheckModulePathMessages(t *testing.T) {
	for p, reason := range map[string]string{
		"":                  "it is empty",
		"-x/z":              "it starts with '-'",
		"x.y//z":            "it contains '//'",
		"x.y/z/":            "it ends with '/'",
		"x.y/../z":          `the path element ".." consists only of dots`,
		".x/z":              `the path element ".x" starts with a dot`,
		"x./z":              `the path element "x." ends with a dot`,
		"x.y/a b":           `the path element "a b" contains ' ', but only ASCII letters, digits, '-', '.', '_' and '~' are allowed`,
		"x.y/é":             `the path element "é" contains the character U+00E9, but only ASCII letters, digits, '-', '.', '_' and '~' are allowed`,
		"x.y/😀":             `the path element "😀" contains the character U+D83D, but only ASCII letters, digits, '-', '.', '_' and '~' are allowed`,
		"x.y/\x01":          `the path element "` + "\x01" + `" contains the character U+0001, but only ASCII letters, digits, '-', '.', '_' and '~' are allowed`,
		"x.y/con.txt":       `the path element "con.txt" uses "con", a name that Windows reserves`,
		"x.y/a~1":           `the path element "a~1" ends with '~' followed by digits`,
		"X.y/z":             `its first path element "X.y" contains 'X', but a host name has only lowercase ASCII letters, digits, '-' and '.'`,
		"x.y/z/v1":          "the major version suffix at its end is invalid (it must be /v2 or higher without a leading zero or a dot, or .v0 or higher for gopkg.in)",
		"gopkg.in/yaml.v01": "the major version suffix at its end is invalid (it must be /v2 or higher without a leading zero or a dot, or .v0 or higher for gopkg.in)",
	} {
		err := CheckModulePath(p)
		var pe *ModulePathError
		if !errors.As(err, &pe) || pe.Reason != reason {
			t.Errorf("CheckModulePath(%q) = %v, want reason %q", p, err, reason)
		}
	}
}

func TestSplitPathVersion(t *testing.T) {
	for _, tc := range []struct{ prefix, version string }{
		{"x.y/z", ""},
		{"x.y/z", "/v2"},
		{"x.y/z", "/v3"},
		{"x.y/v", ""},
		{"gopkg.in/yaml", ".v0"},
		{"gopkg.in/yaml", ".v1"},
		{"gopkg.in/yaml", ".v2"},
		{"gopkg.in/yaml", ".v3"},
	} {
		prefix, version, ok := SplitPathVersion(tc.prefix + tc.version)
		if prefix != tc.prefix || version != tc.version || !ok {
			t.Errorf("SplitPathVersion(%q) = %q, %q, %v", tc.prefix+tc.version, prefix, version, ok)
		}
	}
	for _, tc := range checkPathTests {
		prefix, version, _ := SplitPathVersion(tc.path)
		if prefix+version != tc.path {
			t.Errorf("SplitPathVersion(%q) = %q, %q", tc.path, prefix, version)
		}
	}
}
