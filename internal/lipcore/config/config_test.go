package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from tests/Lip.Core.Tests/Services/ConfigServiceTests.cs (lip main).

const liprcJSON = `{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "github_proxy": "https://proxy.com",
  "go_module_proxy": "https://goproxy.cn"
}`

func setup(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func files(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func errorsIn(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		if m.Level == Error {
			n++
		}
	}
	return n
}

func TestListFileNotExistsReturnsDefaultWithoutSaving(t *testing.T) {
	dir := setup(t, map[string]string{"placeholder.txt": ""})
	got, msgs, err := ListIn(dir)
	if err != nil || got[GithubProxyKey] != "" || len(msgs) != 0 {
		t.Fatalf("%v %v %v", got, msgs, err)
	}
	if len(files(t, dir)) != 1 {
		t.Fatal("wrote a file")
	}
	c, _, _ := LoadIn(t.TempDir())
	if c.HasGithubProxy {
		t.Fatal("default is none")
	}
	// A missing directory is fine too.
	if c, _, err := LoadIn(filepath.Join(t.TempDir(), "missing")); err != nil || c.HasGithubProxy {
		t.Fatal(c, err)
	}
}

func TestGetValidFileReturnsConfig(t *testing.T) {
	dir := setup(t, map[string]string{FileName: "format = 4\ngithub_proxy = \"https://proxy.com\"\n"})
	if v, msgs, err := GetIn(dir, GithubProxyKey); v != "https://proxy.com" || err != nil || len(msgs) != 0 {
		t.Fatal(v, msgs, err)
	}
}

func TestGetInvalidFilePrintsErrorAndReturnsDefault(t *testing.T) {
	for toml, reason := range map[string]string{
		"github_proxy = \"https://p.com\"\n": "Its field format is missing or not 4; it must be 'format = 4'.",
		"":                                   "Its field format is missing or not 4; it must be 'format = 4'.",
		"format = 3\ngithub_proxy = \"https://p.com\"\n":     "Its field format is missing or not 4; it must be 'format = 4'.",
		"format = \"4\"\ngithub_proxy = \"https://p.com\"\n": "Its field format is missing or not 4; it must be 'format = 4'.",
		"format = 4\ngithub_proxy = 5\n": "Its field github_proxy is not a single text value; it must be a URL" +
			" prefix like https://example.com/.",
		"format = 4\ngithub_proxy = [\"x\"]\n":       "Its field github_proxy is not a single text value",
		"format = 4\ngithub_proxy = [unclosed\n":     "The file is not valid TOML: ",
		"format = 4\ngithub_proxy = https://p.com\n": "The file is not valid TOML: ",
		"format = 4\nwhen = 1979-05-27\n":            "The value of 'when' is a date or a time, which lip does not read.",
		"format: 4\n":                                "The file is not valid TOML: ",
	} {
		dir := setup(t, map[string]string{FileName: toml})
		c, msgs, err := LoadIn(dir)
		if err != nil || c.HasGithubProxy || len(msgs) != 2 || errorsIn(msgs) != 1 {
			t.Fatalf("%q: %+v %+v %v", toml, c, msgs, err)
		}
		path := filepath.Join(dir, FileName)
		if !strings.HasPrefix(msgs[0].Text, "Cannot read the configuration file "+path+": "+reason) {
			t.Errorf("%q: %q", toml, msgs[0].Text)
		}
		if msgs[1].Level != Info || msgs[1].Text != "lip continues with the default configuration (no"+
			" github_proxy). Fix the file so that it is TOML with the lines 'format = 4' and"+
			" 'github_proxy = \"https://example.com/\"', or run 'lip config set github_proxy <URL>' or"+
			" 'lip config delete github_proxy' to rewrite it." {
			t.Errorf("%q", msgs[1].Text)
		}
	}
}

func TestGetFormatOnlyOrUnknownKeys(t *testing.T) {
	for toml, want := range map[string]string{
		"format = 4\n": "",
		"format = 4\ngo_module_proxy = \"https://goproxy.cn\"\ngithub_proxy = \"https://proxy.com\"\n": "https://proxy.com",
		"format = 4\ngithub_proxy = \"https://proxy.com\"\n[foo]\na = 1\n":                             "https://proxy.com",
		"format = 4\nfoo = [1, 2]\ngithub_proxy = \"https://proxy.com\"\n":                             "https://proxy.com",
		"format = 4.0\ngithub_proxy = \"x\"\n":                                                         "x",
		"\xef\xbb\xbfformat = 4\ngithub_proxy = 'x'\n":                                                 "x",
	} {
		dir := setup(t, map[string]string{FileName: toml})
		v, msgs, err := GetIn(dir, GithubProxyKey)
		if v != want || err != nil || len(msgs) != 0 {
			t.Errorf("%q: %q %v %v", toml, v, msgs, err)
		}
	}
}

func TestSetAndDelete(t *testing.T) {
	dir := setup(t, map[string]string{"placeholder.txt": ""})
	if _, err := SetIn(dir, GithubProxyKey, "https://new.com"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, FileName); got != "format = 4\ngithub_proxy = \"https://new.com\"\n" {
		t.Fatalf("%q", got)
	}
	if v, _, _ := GetIn(dir, GithubProxyKey); v != "https://new.com" {
		t.Fatal(v)
	}
	if _, err := SetIn(dir, GithubProxyKey, "https://newer.com"); err != nil {
		t.Fatal(err)
	}
	if got := files(t, dir); len(got) != 2 {
		t.Fatalf("left %v", got)
	}
	if _, err := DeleteIn(dir, GithubProxyKey); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, FileName); got != "format = 4\n" {
		t.Fatalf("%q", got)
	}
	for _, v := range []string{"", "true", "1.0", "null", "a\"b\\c\n\u0001é😀"} {
		if _, err := SetIn(dir, GithubProxyKey, v); err != nil {
			t.Fatal(err)
		}
		if c, _, _ := LoadIn(dir); !c.HasGithubProxy || c.GithubProxy != v {
			t.Errorf("%q read back as %+v", v, c)
		}
	}
	// An invalid config.toml is reported, then replaced.
	dir = setup(t, map[string]string{FileName: "bad: [\n"})
	msgs, err := DeleteIn(dir, GithubProxyKey)
	if err != nil || errorsIn(msgs) != 1 || read(t, dir, FileName) != "format = 4\n" {
		t.Fatal(msgs, err)
	}
}

func TestUnknownKey(t *testing.T) {
	dir := setup(t, map[string]string{FileName: "format = 4\ngo_module_proxy = \"https://goproxy.cn\"\n"})
	_, _, err := GetIn(dir, "go_module_proxy")
	var ke *KeyError
	if !errors.As(err, &ke) || err.Error() != "'go_module_proxy' is not a configuration key. The only"+
		" configuration key is github_proxy, a URL prefix through which lip reaches GitHub." {
		t.Fatal(err)
	}
	dir = setup(t, map[string]string{LiprcFileName: liprcJSON})
	if _, err := SetIn(dir, "go_module_proxy", "x"); !errors.As(err, &ke) {
		t.Fatal(err)
	}
	if _, err := DeleteIn(dir, "x"); !errors.As(err, &ke) {
		t.Fatal(err)
	}
	// Nothing was read, so nothing was migrated.
	if got := files(t, dir); len(got) != 1 || got[0] != LiprcFileName {
		t.Fatal(got)
	}
}

func TestLiprcIsMigrated(t *testing.T) {
	dir := setup(t, map[string]string{LiprcFileName: liprcJSON})
	got, msgs, err := ListIn(dir)
	if err != nil || got[GithubProxyKey] != "https://proxy.com" || len(got) != 1 {
		t.Fatal(got, err)
	}
	if len(msgs) != 1 || msgs[0].Level != Info || msgs[0].Text != "Converted the lip v0.x configuration file "+
		filepath.Join(dir, LiprcFileName)+" to "+filepath.Join(dir, FileName)+"." {
		t.Fatal(msgs)
	}
	if names := files(t, dir); len(names) != 1 || names[0] != FileName {
		t.Fatal(names)
	}
	if read(t, dir, FileName) != "format = 4\ngithub_proxy = \"https://proxy.com\"\n" {
		t.Fatal(read(t, dir, FileName))
	}

	for _, j := range []string{`{ "format_version": 3 }`, `{ "format_version": 3, "github_proxy": 5 }`,
		`{ "format_version": 3, "github_proxy": null }`, `{}`} {
		dir := setup(t, map[string]string{LiprcFileName: j})
		c, _, err := LoadIn(dir)
		if err != nil || c.HasGithubProxy || read(t, dir, FileName) != "format = 4\n" || len(files(t, dir)) != 1 {
			t.Errorf("%s: %+v %v", j, c, err)
		}
	}
	for j, want := range map[string]string{
		`{ "format_version": 3, "github_proxy": "a", "github_proxy": "b" }`:            "b",
		`{ "format_version": 3.0, "github_proxy": "x" }`:                               "x",
		`{ "github_proxy": "https://proxy.com" }`:                                      "https://proxy.com",
		`{ "format_version": 3, "github_proxy": "😀\/" }`:                               "\U0001F600/",
		"{ \"format_version\": 3, \"other\": \"\xc3\x28\", \"github_proxy\": \"ok\" }": "ok",
	} {
		dir := setup(t, map[string]string{LiprcFileName: j})
		if c, _, err := LoadIn(dir); err != nil || c.GithubProxy != want {
			t.Errorf("%s: %+v %v", j, c, err)
		}
	}
	dir = setup(t, map[string]string{LiprcFileName: liprcJSON})
	if _, err := SetIn(dir, GithubProxyKey, "https://new.com"); err != nil {
		t.Fatal(err)
	}
	if names := files(t, dir); len(names) != 1 || read(t, dir, FileName) != "format = 4\ngithub_proxy = \"https://new.com\"\n" {
		t.Fatal(names)
	}
}

func TestLiprcNotOfFormat3IsKept(t *testing.T) {
	for j, reason := range map[string]string{
		"not json": "'not json' is an invalid JSON literal. Expected the literal 'null'. LineNumber: 0 |" +
			" BytePositionInLine: 1.",
		"[]": "It is not a JSON object whose field format_version, when present, is 3, the only lip v0.x" +
			" configuration that lip can convert.",
		`{ "format_version": null, "github_proxy": "https://proxy.com" }`: "It is not a JSON object",
		`{ "format_version": 2, "github_proxy": "https://proxy.com" }`:    "It is not a JSON object",
		`{ "format_version": "3", "github_proxy": "https://proxy.com" }`:  "It is not a JSON object",
		"\xef\xbb\xbf{ \"format_version\": 3 }":                           "'0xEF' is an invalid start of a value. LineNumber: 0 | BytePositionInLine: 0.",
		`{ "format_version": 3, "github_proxy": "x", }`:                   "The JSON object contains a trailing comma",
		`{ "format_version": 3, "github_proxy": "\uD800" }`:               "Cannot read incomplete UTF-16 JSON text as string with missing low surrogate.",
		"{ \"format_version\": 3, \"github_proxy\": \"\xc3\x28\" }":       "Cannot transcode invalid UTF-8 JSON text to UTF-16 string.",
		"":                          "The input does not contain any JSON tokens.",
		`{ "format_version": 3 } x`: "'x' is invalid after a single JSON value. Expected end of data. LineNumber: 0 | BytePositionInLine: 24.",
		`{"a": 01}`:                 "Invalid leading zero before '1'. LineNumber: 0 | BytePositionInLine: 7.",
	} {
		dir := setup(t, map[string]string{LiprcFileName: j})
		c, msgs, err := LoadIn(dir)
		if err != nil || c.HasGithubProxy {
			t.Errorf("%q: %+v %v", j, c, err)
			continue
		}
		liprc := filepath.Join(dir, LiprcFileName)
		if len(msgs) != 2 || !strings.HasPrefix(msgs[0].Text, "Cannot read the lip v0.x configuration file "+
			liprc+": "+reason) {
			t.Errorf("%q: %+v", j, msgs)
			continue
		}
		if msgs[1].Text != "lip continues with the default configuration (no github_proxy) and keeps "+liprc+
			" as it is. Run 'lip config set github_proxy <URL>' to write "+filepath.Join(dir, FileName)+
			", or delete "+liprc+" if you no longer need it." {
			t.Errorf("%q", msgs[1].Text)
		}
		if names := files(t, dir); len(names) != 1 || read(t, dir, LiprcFileName) != j {
			t.Errorf("%q: liprc.json not kept: %v", j, names)
		}
	}
}

func TestConfigTOMLWinsOverLiprc(t *testing.T) {
	// liprc.json is read only when there is no config.toml, even an invalid one.
	for toml, want := range map[string]string{"format = 4\ngithub_proxy = \"https://toml.com\"\n": "https://toml.com",
		"github_proxy = \"https://toml.com\"\n": ""} {
		dir := setup(t, map[string]string{FileName: toml, LiprcFileName: liprcJSON})
		got, _, err := ListIn(dir)
		if err != nil || got[GithubProxyKey] != want || read(t, dir, LiprcFileName) != liprcJSON ||
			read(t, dir, FileName) != toml {
			t.Errorf("%q: %v %v", toml, got, err)
		}
	}
}

func TestSaveFailureLeavesTheFile(t *testing.T) {
	dir := setup(t, map[string]string{})
	// config.toml is a directory: the rename fails, and the .tmp file is deleted.
	if err := os.Mkdir(filepath.Join(dir, FileName), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName, "x"), nil, 0o666); err != nil {
		t.Fatal(err)
	}
	_, err := SetIn(dir, GithubProxyKey, "x")
	var we *WriteError
	if !errors.As(err, &we) || !strings.HasSuffix(we.Message, " the configuration file "+filepath.Join(dir, FileName)+
		"; it was not changed.") {
		t.Fatal(err)
	}
	if names := files(t, dir); len(names) != 1 {
		t.Fatalf("left %v", names)
	}
}
