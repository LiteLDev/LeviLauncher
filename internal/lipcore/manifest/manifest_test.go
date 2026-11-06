package manifest

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
)

// useFakes replaces the semver/glob hooks with simple fakes: values
// starting with "bad" fail, "v"-prefixed versions lose the prefix (to
// observe normalization), everything else is returned unchanged.
func useFakes(t *testing.T) {
	t.Helper()
	nv, nr, vg := NormalizeVersion, NormalizeRange, ValidateGlob
	t.Cleanup(func() { NormalizeVersion, NormalizeRange, ValidateGlob = nv, nr, vg })
	NormalizeVersion = func(s string) (string, error) {
		if s == "" || strings.HasPrefix(s, "bad") {
			return "", errors.New("bad version")
		}
		return strings.TrimPrefix(s, "v"), nil
	}
	NormalizeRange = func(s string) (string, error) {
		if s == "" || strings.HasPrefix(s, "bad") {
			return "", errors.New("bad range")
		}
		return strings.TrimSpace(s), nil
	}
	ValidateGlob = func(p string) error {
		if strings.HasPrefix(p, "bad") {
			return errors.New("bad glob")
		}
		return nil
	}
}

func setNewLine(t *testing.T, nl string) {
	t.Helper()
	old := stjson.NewLine
	stjson.NewLine = nl
	t.Cleanup(func() { stjson.NewLine = old })
}

func setWindowsPaths(t *testing.T, v bool) {
	t.Helper()
	old := windowsPaths
	windowsPaths = v
	t.Cleanup(func() { windowsPaths = old })
}

const header = `"format_version": 3, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d"`

func TestParseDefaultsAndNormalization(t *testing.T) {
	useFakes(t)
	m, err := Parse([]byte("\xef\xbb\xbf{" + header + `,
		"tooth": "github.com/LiteLDev/LeviLamina",
		"version": "v1.2.3",
		"unknown": {"ignored": true},
		"info": {"name": "LL", "tags": ["a", "b:c", "tag\n"], "avatar_url": null},
		"variants": [
			{},
			{
				"label": "client",
				"platform": "win-*",
				"dependencies": {"github.com/a/b#": " 1.x ", "github.com/a/c#client": "2.0.0"},
				"assets": [{"type": "zip", "urls": ["", " HTTPS://GitHub.com/a b "], "placements": [{"type": "dir", "src": "x/", "dest": "mods/x/"}]}],
				"preserve_files": ["", "config/**"],
				"remove_files": null,
				"scripts": {"post_install": ["a && b"], "install": null}
			}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := &Manifest{
		FormatVersion: 3,
		FormatUUID:    FormatUUID,
		Tooth:         "github.com/LiteLDev/LeviLamina",
		Version:       "1.2.3",
		Info:          Info{Name: "LL", Tags: []string{"a", "b:c", "tag\n"}},
		Variants: []Variant{
			{
				Dependencies:  Dependencies{},
				Assets:        []Asset{},
				PreserveFiles: []string{},
				RemoveFiles:   []string{},
				Scripts:       emptyScripts(),
			},
			{
				Label:    "client",
				Platform: "win-*",
				Dependencies: Dependencies{
					{ID: "github.com/a/b", Range: "1.x"},
					{ID: "github.com/a/c#client", Range: "2.0.0"},
				},
				Assets: []Asset{{
					Type:       AssetZip,
					URLs:       []string{"https://github.com/a%20b"},
					Placements: []Placement{{Type: PlacementDir, Src: "x/", Dest: "mods/x/"}},
				}},
				PreserveFiles: []string{"config/**"},
				RemoveFiles:   []string{},
				Scripts: Scripts{
					PreInstall: []string{}, Install: []string{}, PostInstall: []string{"a && b"},
					PreUninstall: []string{}, Uninstall: []string{}, PostUninstall: []string{},
				},
			},
		},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", m, want)
	}
}

func TestParseMinimal(t *testing.T) {
	useFakes(t)
	m, err := Parse([]byte(`{` + header + `, "tooth": "example.com/x", "version": "1.0.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Info.Tags == nil || m.Variants == nil || m.Info.Name != "" {
		t.Errorf("defaults not filled: %#v", m)
	}
	setNewLine(t, "\n")
	got, err := m.MarshalIndented()
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "tooth": "example.com/x",
  "version": "1.0.0",
  "info": {
    "name": "",
    "description": "",
    "tags": [],
    "avatar_url": ""
  },
  "variants": []
}`
	if string(got) != want {
		t.Errorf("MarshalIndented =\n%s\nwant\n%s", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	useFakes(t)
	base := func(body string) string {
		return `{` + header + `, "tooth": "example.com/x", "version": "1.0.0"` + body + `}`
	}
	variant := func(v string) string { return base(`, "variants": [` + v + `]`) }
	tests := []struct {
		name, doc string
	}{
		{"not json", `{`},
		{"null", `null`},
		{"array", `[]`},
		{"trailing data", base("") + "x"},
		{"format version 2", `{"format_version": 2, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "tooth": "example.com/x", "version": "1.0.0"}`},
		{"format version float", `{"format_version": 3.0, "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "tooth": "example.com/x", "version": "1.0.0"}`},
		{"missing format version", `{"format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d", "tooth": "example.com/x", "version": "1.0.0"}`},
		{"missing uuid", `{"format_version": 3, "tooth": "example.com/x", "version": "1.0.0"}`},
		{"wrong uuid", `{"format_version": 3, "format_uuid": "x", "tooth": "example.com/x", "version": "1.0.0"}`},
		{"case-sensitive keys", `{` + header + `, "Tooth": "example.com/x", "version": "1.0.0"}`},
		{"missing version", `{` + header + `, "tooth": "example.com/x"}`},
		{"null tooth", `{` + header + `, "tooth": null, "version": "1.0.0"}`},
		{"bad tooth", `{` + header + `, "tooth": "Example..com/x#", "version": "1.0.0"}`},
		{"bad version", `{` + header + `, "tooth": "example.com/x", "version": "bad"}`},
		{"numeric version", `{` + header + `, "tooth": "example.com/x", "version": 1}`},
		{"bad tag", base(`, "info": {"tags": ["Upper"]}`)},
		{"null tag", base(`, "info": {"tags": [null]}`)},
		{"info not object", base(`, "info": []`)},
		{"bad label", variant(`{"label": "Client"}`)},
		{"label with 3 parts", variant(`{"label": "a/b/c"}`)},
		{"dup dependency", variant(`{"dependencies": {"example.com/a": "1", "example.com/a#": "2"}}`)},
		{"dependency value not string", variant(`{"dependencies": {"example.com/a": 1}}`)},
		{"dependency bad id", variant(`{"dependencies": {"example.com/a#B": "1"}}`)},
		{"dependency bad range", variant(`{"dependencies": {"example.com/a": "bad"}}`)},
		{"asset missing type", variant(`{"assets": [{}]}`)},
		{"asset wrong case", variant(`{"assets": [{"type": "Zip"}]}`)},
		{"asset unknown index", variant(`{"assets": [{"type": 7}]}`)},
		{"asset url null", variant(`{"assets": [{"type": "zip", "urls": [null]}]}`)},
		{"placement missing src", variant(`{"assets": [{"type": "self", "placements": [{"type": "file", "dest": "a"}]}]}`)},
		{"placement dotdot", variant(`{"assets": [{"type": "self", "placements": [{"type": "file", "src": "a", "dest": "a/../b"}]}]}`)},
		{"placement rooted", variant(`{"assets": [{"type": "self", "placements": [{"type": "file", "src": "a", "dest": "/b"}]}]}`)},
		{"placement type", variant(`{"assets": [{"type": "self", "placements": [{"type": "directory", "src": "a", "dest": "b"}]}]}`)},
		{"bad glob", variant(`{"preserve_files": ["bad["]}`)},
		{"scripts not list", variant(`{"scripts": {"install": "x"}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if m, err := Parse([]byte(tt.doc)); err == nil {
				t.Errorf("Parse succeeded: %#v", m)
			}
		})
	}
}

func TestParseEnumForms(t *testing.T) {
	useFakes(t)
	for in, want := range map[string]AssetType{
		`"zip"`: AssetZip, `" zip "`: AssetZip, `0`: AssetSelf, `4`: AssetZip, `"3"`: AssetUncompressed, `"tgz"`: AssetTgz,
	} {
		m, err := Parse([]byte(`{` + header + `, "tooth": "example.com/x", "version": "1.0.0", "variants": [{"assets": [{"type": ` + in + `}]}]}`))
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if got := m.Variants[0].Assets[0].Type; got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
}

func TestMarshalIndentedRoundTrip(t *testing.T) {
	useFakes(t)
	setNewLine(t, "\r\n")
	want := strings.ReplaceAll(`{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "tooth": "github.com/LiteLDev/LeviLamina",
  "version": "1.0.0",
  "info": {
    "name": "LeviLamina",
    "description": "d",
    "tags": [
      "levilamina"
    ],
    "avatar_url": "https://example.com/a.svg"
  },
  "variants": [
    {
      "label": "client",
      "platform": "win-x64",
      "dependencies": {
        "github.com/LiteLDev/CrashLogger#client": "1.3.*",
        "github.com/LiteLDev/bds": "1.26.51"
      },
      "assets": [
        {
          "type": "zip",
          "urls": [
            "https://github.com/x.zip"
          ],
          "placements": [
            {
              "type": "dir",
              "src": "LeviLamina/",
              "dest": "mods/LeviLamina/"
            }
          ]
        }
      ],
      "preserve_files": [],
      "remove_files": [
        "bedrock_server_mod.exe"
      ],
      "scripts": {
        "pre_install": [],
        "install": [],
        "post_install": [
          ".\\PeEditor.exe -mb"
        ],
        "pre_uninstall": [],
        "uninstall": [],
        "post_uninstall": [
          "IF EXIST a (DEL a BSu0026BSu0026 REN b a)"
        ]
      }
    }
  ]
}`, "\n", "\r\n")
	want = strings.ReplaceAll(want, "BS", `\`)
	m, err := Parse([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Variants[0].Scripts.PostUninstall[0]; got != "IF EXIST a (DEL a && REN b a)" {
		t.Errorf("script = %q", got)
	}
	got, err := m.MarshalIndented()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("round trip mismatch:\n%s\nwant\n%s", got, want)
	}
}

func TestMarshalWritesConstantsAndNormalizedURLs(t *testing.T) {
	setNewLine(t, "\n")
	m := &Manifest{Tooth: "example.com/x", Version: "1.0.0", Variants: []Variant{{
		Assets: []Asset{{Type: AssetZip, URLs: []string{"HTTPS://X.com/a/./b c"}}},
	}}}
	got, err := m.MarshalIndented()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"format_version": 3,`, `"format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",`, `"https://x.com/a/b%20c"`, `"dependencies": {},`, `"placements": []`, `"post_uninstall": []`} {
		if !strings.Contains(string(got), s) {
			t.Errorf("output lacks %s:\n%s", s, got)
		}
	}
}

func TestValidate(t *testing.T) {
	useFakes(t)
	good := func() *Manifest {
		return &Manifest{FormatVersion: 3, FormatUUID: FormatUUID, Tooth: "example.com/x", Version: "1.0.0",
			Variants: []Variant{{Label: "a/b", Dependencies: Dependencies{{ID: "example.com/y", Range: "1"}},
				Assets: []Asset{{Type: AssetSelf, Placements: []Placement{{Type: PlacementFile, Src: "a", Dest: "b"}}}}}}}
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
	for name, mutate := range map[string]func(*Manifest){
		"format":  func(m *Manifest) { m.FormatVersion = 2 },
		"uuid":    func(m *Manifest) { m.FormatUUID = "" },
		"path":    func(m *Manifest) { m.Tooth = "" },
		"version": func(m *Manifest) { m.Version = "bad" },
		"tag":     func(m *Manifest) { m.Info.Tags = []string{"a b"} },
		"label":   func(m *Manifest) { m.Variants[0].Label = "A" },
		"dep id":  func(m *Manifest) { m.Variants[0].Dependencies[0].ID = "#x" },
		"dep dup": func(m *Manifest) {
			m.Variants[0].Dependencies = append(m.Variants[0].Dependencies, Dependency{"example.com/y#", "2"})
		},
		"dep range":   func(m *Manifest) { m.Variants[0].Dependencies[0].Range = "bad" },
		"asset type":  func(m *Manifest) { m.Variants[0].Assets[0].Type = "rar" },
		"place type":  func(m *Manifest) { m.Variants[0].Assets[0].Placements[0].Type = "" },
		"dest":        func(m *Manifest) { m.Variants[0].Assets[0].Placements[0].Dest = ".." },
		"remove glob": func(m *Manifest) { m.Variants[0].RemoveFiles = []string{"bad"} },
	} {
		m := good()
		mutate(m)
		if err := m.Validate(); err == nil {
			t.Errorf("%s: Validate succeeded", name)
		}
	}
}

func TestIsValidDest(t *testing.T) {
	tests := []struct {
		path        string
		win, nonWin bool
	}{
		{"", true, true},
		{"mods/x", true, true},
		{"a\\b", true, true},
		{"/a", false, false},
		{"\\a", false, true},
		{"C:a", false, true},
		{"C:\\a", false, true},
		{"\\\\server\\share", false, true},
		{"a/../b", false, false},
		{"a..b", false, false},
		{"1:a", true, true},
	}
	for _, tt := range tests {
		setWindowsPaths(t, true)
		if got := IsValidDest(tt.path); got != tt.win {
			t.Errorf("windows IsValidDest(%q) = %v", tt.path, got)
		}
		setWindowsPaths(t, false)
		if got := IsValidDest(tt.path); got != tt.nonWin {
			t.Errorf("unix IsValidDest(%q) = %v", tt.path, got)
		}
	}
}

func TestResolveVariant(t *testing.T) {
	glob := func(pattern, rid string) bool {
		return strings.HasSuffix(pattern, "*") && strings.HasPrefix(rid, strings.TrimSuffix(pattern, "*"))
	}
	m := &Manifest{Variants: []Variant{
		{Label: "", Platform: "", Dependencies: Dependencies{{"example.com/a", "1"}}, PreserveFiles: []string{"p1"},
			Scripts: Scripts{PreInstall: []string{"pre"}}},
		{Label: "", Platform: "win-*", Dependencies: Dependencies{{"example.com/b", "2"}},
			Assets: []Asset{{Type: AssetZip}}, RemoveFiles: []string{"r"}, Scripts: Scripts{Install: []string{"inst"}}},
		{Label: "", Platform: "linux-x64", Dependencies: Dependencies{{"example.com/c", "3"}}},
		{Label: "client", Platform: "win-x64", Scripts: Scripts{PreInstall: []string{"c1"}}},
		{Label: "client", Platform: "", Scripts: Scripts{PreInstall: []string{"c2"}}},
		{Label: "dup", Dependencies: Dependencies{{"example.com/a", "1"}}},
		{Label: "dup", Platform: "win-x64", Dependencies: Dependencies{{"example.com/a#", "2"}}},
	}}

	v, err := m.ResolveVariant("", "win-x64", glob)
	if err != nil {
		t.Fatal(err)
	}
	want := &Variant{
		Label: "", Platform: "win-x64",
		Dependencies:  Dependencies{{"example.com/a", "1"}, {"example.com/b", "2"}},
		Assets:        []Asset{{Type: AssetZip}},
		PreserveFiles: []string{"p1"},
		RemoveFiles:   []string{"r"},
		Scripts: Scripts{PreInstall: []string{"pre"}, Install: []string{"inst"}, PostInstall: []string{},
			PreUninstall: []string{}, Uninstall: []string{}, PostUninstall: []string{}},
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("ResolveVariant =\n%#v\nwant\n%#v", v, want)
	}

	if _, err := m.ResolveVariant("client", "win-x64", glob); err == nil {
		t.Error("two variants with pre_install scripts: want error")
	}
	if v, err := m.ResolveVariant("client", "linux-x64", glob); err != nil || v.Scripts.PreInstall[0] != "c2" {
		t.Errorf("client/linux: %v, %v", v, err)
	}
	if _, err := m.ResolveVariant("dup", "win-x64", glob); err == nil {
		t.Error("duplicate dependency keys: want error")
	}
	if _, err := m.ResolveVariant("server", "win-x64", glob); err == nil {
		t.Error("unknown label: want error")
	}
	if _, err := m.ResolveVariant("", "osx-arm64", nil); err != nil {
		t.Errorf("empty platform should match any rid: %v", err)
	}
	if v, err := m.ResolveVariant("", "win-arm64", nil); err != nil || len(v.Dependencies) != 1 {
		t.Errorf("nil matcher must not match globs: %v, %v", v, err)
	}
}

func TestParsePackageID(t *testing.T) {
	tests := []struct {
		in      string
		want    PackageID
		wantErr bool
	}{
		{in: "github.com/LiteLDev/LeviLamina", want: PackageID{Path: "github.com/LiteLDev/LeviLamina"}},
		{in: "github.com/LiteLDev/LeviLamina#client", want: PackageID{"github.com/LiteLDev/LeviLamina", "client"}},
		{in: "github.com/a/b#", want: PackageID{Path: "github.com/a/b"}},
		{in: "github.com/a/b#x_1/y2", want: PackageID{"github.com/a/b", "x_1/y2"}},
		{in: "github.com/a/b#x\n", want: PackageID{"github.com/a/b", "x\n"}},
		{in: "", wantErr: true},
		{in: "#x", wantErr: true},
		{in: "github.com/a/b#x#y", wantErr: true},
		{in: "github.com/a/b#X", wantErr: true},
		{in: "github.com/a/b#a/b/c", wantErr: true},
		{in: "github.com/a/b#a-b", wantErr: true},
		{in: "not a path", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParsePackageID(tt.in)
		if (err != nil) != tt.wantErr || (!tt.wantErr && got != tt.want) {
			t.Errorf("ParsePackageID(%q) = %#v, %v", tt.in, got, err)
		}
		if err == nil && tt.in != "github.com/a/b#" && got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestParsePackageSpec(t *testing.T) {
	tests := []struct {
		in      string
		want    PackageSpec
		wantErr bool
	}{
		{in: "github.com/a/b@1.0.0", want: PackageSpec{PackageID{Path: "github.com/a/b"}, "1.0.0"}},
		{in: "github.com/a/b#client@1.0.0-rc.1", want: PackageSpec{PackageID{"github.com/a/b", "client"}, "1.0.0-rc.1"}},
		{in: "github.com/a/b", wantErr: true},
		{in: "github.com/a/b@", wantErr: true},
		{in: "@1.0.0", wantErr: true},
		{in: "github.com/a/b@1@2", wantErr: true},
		{in: "github.com/a/b#B@1", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParsePackageSpec(tt.in)
		if (err != nil) != tt.wantErr || (!tt.wantErr && got != tt.want) {
			t.Errorf("ParsePackageSpec(%q) = %#v, %v", tt.in, got, err)
		}
		if err == nil && got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestDependenciesJSON(t *testing.T) {
	useFakes(t)
	var d Dependencies
	if err := json.Unmarshal([]byte(`{"example.com/z": "1", "example.com/a#x": " 2 "}`), &d); err != nil {
		t.Fatal(err)
	}
	want := Dependencies{{"example.com/z", "1"}, {"example.com/a#x", "2"}}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("got %#v", d)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"example.com/z":"1","example.com/a#x":"2"}` {
		t.Errorf("MarshalJSON = %s", b)
	}
	for _, bad := range []string{`[]`, `{"example.com/a": null}`, `{"example.com/a": "1", "example.com/a": "2"}`} {
		var d Dependencies
		if err := json.Unmarshal([]byte(bad), &d); err == nil {
			t.Errorf("Unmarshal(%s) succeeded", bad)
		}
	}
}
