package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// fakeGlob matches with path.Match, plus a "**/" prefix meaning "any depth".
type fakeGlob struct{ pattern string }

func (g fakeGlob) Match(s string) bool {
	if rest, ok := strings.CutPrefix(g.pattern, "**/"); ok {
		parts := strings.Split(s, "/")
		for i := range parts {
			if ok, _ := path.Match(rest, strings.Join(parts[i:], "/")); ok {
				return true
			}
		}
		return false
	}
	ok, _ := path.Match(g.pattern, s)
	return ok
}

func fakeCompile(pattern string) (Matcher, error) {
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, fmt.Errorf("bad glob %q: %w", pattern, err)
	}
	return fakeGlob{pattern}, nil
}

type memSource struct {
	keys       []string
	data       map[string]string
	prefetched [][]string
	opened     []string
}

func newMemSource(kv ...string) *memSource {
	s := &memSource{data: map[string]string{}}
	for i := 0; i < len(kv); i += 2 {
		s.keys = append(s.keys, kv[i])
		s.data[kv[i]] = kv[i+1]
	}
	return s
}

func (s *memSource) Keys() []string { return s.keys }
func (s *memSource) Open(key string) (io.ReadCloser, error) {
	s.opened = append(s.opened, key)
	v, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("no such key %q", key)
	}
	return io.NopCloser(strings.NewReader(v)), nil
}
func (s *memSource) Prefetch(_ context.Context, keys []string) error {
	s.prefetched = append(s.prefetched, keys)
	return nil
}

type recReporter struct {
	mu    sync.Mutex
	lines []string
}

func (r *recReporter) add(kind, m string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, kind+": "+m)
}
func (r *recReporter) Info(m string)                    { r.add("info", m) }
func (r *recReporter) Success(m string)                 { r.add("success", m) }
func (r *recReporter) Warning(m string)                 { r.add("warning", m) }
func (r *recReporter) Error(m string)                   { r.add("error", m) }
func (r *recReporter) Progress(string, string, float64) {}

func sep(p string) string { return filepath.FromSlash(p) }

func TestGetRelativePath(t *testing.T) {
	base := t.TempDir()
	// Expected values were produced by .NET 10 Path.GetRelativePath on
	// Windows; separators are converted for other platforms.
	cases := []struct{ rel, path, want string }{
		{".", "a/b", "a/b"},
		{".", "a", "a"},
		{"a", "a", "."},
		{"a", "ab/c", "../ab/c"},
		{"a/", "a/b/", "b/"},
		{".", "..foo", "..foo"},
		{"a", "a/../b", "../b"},
		{"a/b", "a", ".."},
		{"a", "a//b", "b"},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct{ rel, path, want string }{"a", "A/b", "b"},
			struct{ rel, path, want string }{"x.", "x/y", "y"},
			struct{ rel, path, want string }{"a ", "a/b", "b"},
			struct{ rel, path, want string }{"a", "a/b.", "b"},
		)
	} else if runtime.GOOS == "linux" {
		cases = append(cases, struct{ rel, path, want string }{"a", "A/b", "../A/b"})
	}
	for _, c := range cases {
		if got := getRelativePath(base, c.rel, c.path); got != sep(c.want) {
			t.Errorf("getRelativePath(%q, %q) = %q, want %q", c.rel, c.path, got, sep(c.want))
		}
	}
}

func TestGetFullPathWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path normalization")
	}
	base := `C:\ws\inst`
	// Expected values from .NET 10 FileInfo.FullName.
	cases := map[string]string{
		"a/b":        `C:\ws\inst\a\b`,
		"a/./b/../c": `C:\ws\inst\a\c`,
		"a/b/":       `C:\ws\inst\a\b\`,
		"x./y.":      `C:\ws\inst\x\y`,
		"a /b ":      `C:\ws\inst\a \b`,
		"a/b/.":      `C:\ws\inst\a\b`,
		`a\b`:        `C:\ws\inst\a\b`,
		"a//b":       `C:\ws\inst\a\b`,
		`..\..\x`:    `C:\x`,
		`p\.`:        `C:\ws\inst\p`,
		`\abs`:       `C:\abs`,
		`D:\o\.\p`:   `D:\o\p`,
	}
	for in, want := range cases {
		if got := getFullPath(base, in); got != want {
			t.Errorf("getFullPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	if got := pathCombine("plugins", "x.dll"); got != sep("plugins/x.dll") {
		t.Errorf("combine = %q", got)
	}
	if got := pathCombine("plugins/", "x"); got != "plugins/x" {
		t.Errorf("combine = %q", got)
	}
	if got := pathCombine("", "x"); got != "x" {
		t.Errorf("combine = %q", got)
	}
	if got := pathGetFileName("a/b/c.txt"); got != "c.txt" {
		t.Errorf("filename = %q", got)
	}
	want := "a\\b"
	if runtime.GOOS == "windows" {
		want = "b"
	}
	if got := pathGetFileName(`a\b`); got != want {
		t.Errorf("filename = %q, want %q", got, want)
	}
}

func TestMatchPlacements(t *testing.T) {
	keys := []string{"tooth.json", "a.dll", "lib/b.dll", "lib/sub/c.txt", "libx/d.txt", "..e", "README.md"}
	placements := []manifest.Placement{
		{Type: manifest.PlacementFile, Src: "a.dll", Dest: "plugins/A.dll"},
		{Type: manifest.PlacementFile, Src: "*.dll", Dest: "plugins"},
		{Type: manifest.PlacementDir, Src: "lib", Dest: "mods/lib"},
		{Type: manifest.PlacementDir, Src: "", Dest: "all"},
	}
	w := &Workspace{Dir: t.TempDir(), Glob: fakeCompile}
	got, err := w.MatchPlacements(keys, placements)
	if err != nil {
		t.Fatal(err)
	}
	want := []Target{
		{"tooth.json", sep("all/tooth.json")},
		{"a.dll", "plugins/A.dll"},
		{"a.dll", sep("plugins/a.dll")},
		{"a.dll", sep("all/a.dll")},
		{"lib/b.dll", "mods/lib" + sep("/b.dll")},
		{"lib/b.dll", sep("all/lib/b.dll")},
		{"lib/sub/c.txt", "mods/lib" + sep("/sub/c.txt")},
		{"lib/sub/c.txt", sep("all/lib/sub/c.txt")},
		{"libx/d.txt", sep("all/libx/d.txt")},
		// "..e" starts with ".." and is skipped by every dir placement (lip quirk).
		{"README.md", sep("all/README.md")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Dir placement whose src equals a key yields "dest/.".
	got, err = w.MatchPlacements([]string{"x"}, []manifest.Placement{{Type: manifest.PlacementDir, Src: "x", Dest: "d"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Target{{"x", sep("d/.")}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMatchPlacementsLazyGlob(t *testing.T) {
	compiled := 0
	compile := func(p string) (Matcher, error) {
		compiled++
		return nil, errors.New("must not compile")
	}
	got, err := MatchPlacements([]string{"[bad"}, []manifest.Placement{{Type: manifest.PlacementFile, Src: "[bad", Dest: "x"}}, compile)
	if err != nil || compiled != 0 || len(got) != 1 {
		t.Fatalf("got %v %v compiled=%d", got, err, compiled)
	}
	_, err = MatchPlacements([]string{"other"}, []manifest.Placement{{Type: manifest.PlacementFile, Src: "[bad", Dest: "x"}}, compile)
	if err == nil {
		t.Fatal("expected glob compile error")
	}
}

type scriptLog struct {
	calls []string
	fail  string
}

func (s *scriptLog) run(_ context.Context, dir, command string, stdout, stderr io.Writer) error {
	s.calls = append(s.calls, command)
	fmt.Fprintf(stdout, "out %s\r\npartial", command)
	fmt.Fprintf(stderr, "err %s\n", command)
	if command == s.fail {
		return errors.New("boom")
	}
	return nil
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallPackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("old content that is long"), 0o644); err != nil {
		t.Fatal(err)
	}
	self := newMemSource("tooth.json", "{}", "a.dll", "A", "res/x/y.txt", "Y", "res/z.txt", "Z")
	zip := newMemSource("bin/tool.exe", "T", "doc.txt", "D")
	empty := newMemSource("unused", "U")

	variant := &manifest.Variant{
		Assets: []manifest.Asset{
			{Type: manifest.AssetSelf, Placements: []manifest.Placement{
				{Type: manifest.PlacementFile, Src: "a.dll", Dest: "plugins/a.dll"},
				{Type: manifest.PlacementFile, Src: "*.dll", Dest: "plugins"}, // duplicate dest
				{Type: manifest.PlacementDir, Src: "res", Dest: "resources/./r"},
			}},
			{Type: manifest.AssetZip, URLs: []string{"z"}, Placements: []manifest.Placement{
				{Type: manifest.PlacementFile, Src: "bin/tool.exe", Dest: "tools/../tool.exe"},
				{Type: manifest.PlacementFile, Src: "doc.txt", Dest: "existing.txt"},
			}},
			{Type: manifest.AssetTar, URLs: []string{"e"}, Placements: []manifest.Placement{
				{Type: manifest.PlacementFile, Src: "nothing", Dest: "x"},
			}},
		},
		Scripts: manifest.Scripts{PreInstall: []string{"pre"}, Install: []string{"inst"}, PostInstall: []string{"post1", "post2"}, PreUninstall: []string{"never"}},
	}
	var openedAssets []manifest.AssetType
	open := func(_ context.Context, a manifest.Asset) (Source, error) {
		openedAssets = append(openedAssets, a.Type)
		switch a.Type {
		case manifest.AssetSelf:
			return self, nil
		case manifest.AssetZip:
			return zip, nil
		}
		return empty, nil
	}
	scripts := &scriptLog{}
	rep := &recReporter{}
	w := &Workspace{Dir: dir, Reporter: rep, Glob: fakeCompile, RunCommand: scripts.run}

	files, err := w.InstallPackage(context.Background(), variant, open)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "plugins", "a.dll"),
		filepath.Join(dir, "plugins", "a.dll"),
		filepath.Join(dir, "resources", "r", "x", "y.txt"),
		filepath.Join(dir, "resources", "r", "z.txt"),
		filepath.Join(dir, "tool.exe"),
		filepath.Join(dir, "existing.txt"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files\n got  %q\n want %q", files, want)
	}
	for p, c := range map[string]string{want[0]: "A", want[2]: "Y", want[3]: "Z", want[4]: "T", want[5]: "D"} {
		if got := readFile(t, p); got != c {
			t.Errorf("%s = %q want %q", p, got, c)
		}
	}
	if !reflect.DeepEqual(openedAssets, []manifest.AssetType{"self", "zip", "tar"}) {
		t.Errorf("opened assets %v", openedAssets)
	}
	if !reflect.DeepEqual(self.prefetched, [][]string{{"a.dll", "res/x/y.txt", "res/z.txt"}}) {
		t.Errorf("prefetch %q", self.prefetched)
	}
	if empty.prefetched != nil || empty.opened != nil {
		t.Errorf("source without matches was read")
	}
	if !reflect.DeepEqual(scripts.calls, []string{"pre", "inst", "post1", "post2"}) {
		t.Errorf("scripts %q", scripts.calls)
	}
	// Lines are emitted as they complete; a trailing partial line is flushed
	// after the command returns.
	wantLog := []string{"info: out pre", "error: err pre", "info: partial"}
	if got := rep.lines[:3]; !reflect.DeepEqual(got, wantLog) {
		t.Errorf("reporter %q", rep.lines)
	}

	// IgnoreScripts skips every script.
	scripts.calls = nil
	w.IgnoreScripts = true
	if _, err := w.InstallPackage(context.Background(), variant, open); err != nil {
		t.Fatal(err)
	}
	if scripts.calls != nil {
		t.Errorf("scripts ran: %q", scripts.calls)
	}
}

func TestInstallPackageScriptFailure(t *testing.T) {
	dir := t.TempDir()
	scripts := &scriptLog{fail: "pre"}
	w := &Workspace{Dir: dir, Glob: fakeCompile, RunCommand: scripts.run}
	variant := &manifest.Variant{
		Assets:  []manifest.Asset{{Type: manifest.AssetSelf, Placements: []manifest.Placement{{Type: manifest.PlacementFile, Src: "a", Dest: "a"}}}},
		Scripts: manifest.Scripts{PreInstall: []string{"pre"}, Install: []string{"inst"}},
	}
	opened := false
	_, err := w.InstallPackage(context.Background(), variant, func(context.Context, manifest.Asset) (Source, error) {
		opened = true
		return newMemSource("a", "A"), nil
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
	if opened || len(scripts.calls) != 1 {
		t.Errorf("continued after failing script: opened=%v calls=%q", opened, scripts.calls)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestUninstallPackage(t *testing.T) {
	dir := t.TempDir()
	j := func(p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
	for _, p := range []string{"plugins/a.dll", "plugins/keep.txt", "config/c.json", "config/sub/d.json", "logs/extra.log", "extra.log", "temp/x/y.bin", "notmine.txt", "deep/temp/z"} {
		mustWrite(t, j(p), p)
	}
	files := []string{
		j("plugins/a.dll"),
		j("plugins/keep.txt"),    // preserved by bare name "*.txt"
		j("config/c.json"),       // preserved by relative path "config/*.json"
		j("config/sub/d.json"),   // not preserved: "config/*.json" does not cross "/"
		j("plugins/missing.dll"), // missing: warning
		j("plugins/a.dll"),       // duplicate: missing by now, warning again
	}
	scripts := &scriptLog{}
	rep := &recReporter{}
	w := &Workspace{Dir: dir, Reporter: rep, Glob: fakeCompile, RunCommand: scripts.run}
	variant := &manifest.Variant{
		PreserveFiles: []string{"*.txt", "config/*.json"},
		RemoveFiles:   []string{"extra.log", "temp/", "plugins"},
		Scripts:       manifest.Scripts{PreUninstall: []string{"pre"}, Uninstall: []string{"un"}, PostUninstall: []string{"post"}, Install: []string{"never"}},
	}
	if err := w.UninstallPackage(context.Background(), variant, files); err != nil {
		t.Fatal(err)
	}

	gone := []string{"plugins/a.dll", "config/sub/d.json", "extra.log", "temp", "plugins"}
	kept := []string{"config/c.json", "config/sub", "logs/extra.log", "notmine.txt", "deep/temp/z"}
	for _, p := range gone {
		if exists(j(p)) {
			t.Errorf("%s still exists", p)
		}
	}
	for _, p := range kept {
		if !exists(j(p)) {
			t.Errorf("%s was removed", p)
		}
	}
	if !reflect.DeepEqual(scripts.calls, []string{"pre", "un", "post"}) {
		t.Errorf("scripts %q", scripts.calls)
	}
	var warnings []string
	for _, l := range rep.lines {
		if strings.HasPrefix(l, "warning: ") {
			warnings = append(warnings, l)
		}
	}
	wantWarn := []string{
		"warning: File '" + j("plugins/missing.dll") + "' does not exist, skipping removal.",
		"warning: File '" + j("plugins/a.dll") + "' does not exist, skipping removal.",
	}
	if !reflect.DeepEqual(warnings, wantWarn) {
		t.Errorf("warnings\n got  %q\n want %q", warnings, wantWarn)
	}
}

func TestUninstallPackageRemoveDeepestFirst(t *testing.T) {
	dir := t.TempDir()
	j := func(p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
	for _, p := range []string{"a/b/c.log", "a/b.log", "a/x.txt", "b.log"} {
		mustWrite(t, j(p), p)
	}
	w := &Workspace{Dir: dir, Glob: fakeCompile}
	// "**/*" matches every entry, including directories that contain other
	// matches; files must be deleted before their directories are removed.
	variant := &manifest.Variant{RemoveFiles: []string{"**/*.log", "a/b/"}}
	if err := w.UninstallPackage(context.Background(), variant, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a/b/c.log", "a/b.log", "b.log", "a/b"} {
		if exists(j(p)) {
			t.Errorf("%s still exists", p)
		}
	}
	if !exists(j("a/x.txt")) {
		t.Error("a/x.txt removed")
	}

	variant = &manifest.Variant{RemoveFiles: []string{"*", "a/*"}}
	if err := w.UninstallPackage(context.Background(), variant, nil); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left %v", entries)
	}
}

func TestUninstallPackageInvalidGlobRunsNoScript(t *testing.T) {
	scripts := &scriptLog{}
	w := &Workspace{Dir: t.TempDir(), Glob: fakeCompile, RunCommand: scripts.run}
	err := w.UninstallPackage(context.Background(), &manifest.Variant{
		RemoveFiles: []string{"[bad"},
		Scripts:     manifest.Scripts{PreUninstall: []string{"pre"}},
	}, nil)
	if err == nil || scripts.calls != nil {
		t.Fatalf("err=%v calls=%q", err, scripts.calls)
	}
}

func TestUninstallMissingParentDirectoryFails(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("File.Delete throws DirectoryNotFoundException on Windows only")
	}
	dir := t.TempDir()
	rep := &recReporter{}
	w := &Workspace{Dir: dir, Reporter: rep, Glob: fakeCompile}
	p := filepath.Join(dir, "gone", "f.txt")
	err := w.UninstallPackage(context.Background(), &manifest.Variant{}, []string{p})
	if err == nil || !strings.Contains(err.Error(), "Could not find a part of the path '"+p+"'.") {
		t.Fatalf("err = %v", err)
	}
	if len(rep.lines) != 1 || !strings.HasPrefix(rep.lines[0], "warning: ") {
		t.Errorf("reporter %q", rep.lines)
	}
}

func TestInstallPackageDirPlacementOnEmptyKeyFails(t *testing.T) {
	dir := t.TempDir()
	// An uncompressed asset's only key is "" (SingleFileSource).
	src := newMemSource("", "DLL")
	variant := &manifest.Variant{Assets: []manifest.Asset{
		{Type: manifest.AssetUncompressed, URLs: []string{"u"}, Placements: []manifest.Placement{
			{Type: manifest.PlacementDir, Src: "", Dest: "plugins/x.dll"},
		}},
	}}
	w := &Workspace{Dir: dir, Glob: fakeCompile}
	open := func(context.Context, manifest.Asset) (Source, error) { return src, nil }
	files, err := w.InstallPackage(context.Background(), variant, open)
	if err == nil || err.Error() != "The value cannot be an empty string. (Parameter 'path')" {
		t.Fatalf("err = %v", err)
	}
	if len(files) != 0 || exists(filepath.Join(dir, "plugins", "x.dll")) {
		t.Errorf("placed %q", files)
	}

	// Path.GetFullPath rejects a path of spaces on Windows, as src or key.
	_, srcErr := w.MatchPlacements([]string{"a"}, []manifest.Placement{{Type: manifest.PlacementDir, Src: "  ", Dest: "d"}})
	_, keyErr := w.MatchPlacements([]string{"  "}, []manifest.Placement{{Type: manifest.PlacementDir, Src: "", Dest: "d"}})
	for _, err := range []error{srcErr, keyErr} {
		if isWindows && (err == nil || err.Error() != "The path is empty. (Parameter 'path')") || !isWindows && err != nil {
			t.Errorf("spaces: err = %v", err)
		}
	}

	// A file placement on the empty key still matches exactly.
	got, err := w.MatchPlacements([]string{""}, []manifest.Placement{{Type: manifest.PlacementFile, Src: "", Dest: "a.dll"}})
	if err != nil || !reflect.DeepEqual(got, []Target{{"", "a.dll"}}) {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestDefaultCommandRunnerCancelKillsTree(t *testing.T) {
	dir := t.TempDir()
	// The nested shell is a grandchild holding the output pipes; it writes
	// late.txt after about 5s unless it is killed with the tree.
	var script string
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(dir, "outer.cmd"), []byte("@cmd /c .\\inner.cmd\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "inner.cmd"), []byte("@ping -n 6 127.0.0.1 >nul\r\n@echo late> late.txt\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		script = `.\outer.cmd`
	} else {
		if _, err := os.Stat("/bin/sh"); err != nil {
			t.Skip("no sh")
		}
		script = `sh -c "sleep 5; echo late > late.txt"; true`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := DefaultCommandRunner(ctx, dir, script, io.Discard, io.Discard)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("returned after %v", elapsed)
	}
	time.Sleep(6*time.Second - elapsed)
	if exists(filepath.Join(dir, "late.txt")) {
		t.Error("grandchild survived cancellation")
	}
}

func TestEscapeArgument(t *testing.T) {
	// Expected values from CliWrap 3.10.5 Command.Arguments.
	cases := map[string]string{
		"exit 3":           `"exit 3"`,
		`echo "q" a\"b`:    `"echo \"q\" a\\\"b"`,
		"":                 `""`,
		"echo&echo":        "echo&echo",
		`a"b`:              `"a\"b"`,
		`dir C:\x y\`:      `"dir C:\x y\\"`,
		`C:\path\no\space`: `C:\path\no\space`,
	}
	for in, want := range cases {
		if got := escapeArgument(in); got != want {
			t.Errorf("escapeArgument(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultCommandRunner(t *testing.T) {
	dir := t.TempDir()
	rep := &recReporter{}
	w := &Workspace{Dir: dir, Reporter: rep, Glob: fakeCompile}

	var create, echo, fail, failCmd, shell string
	wantEcho := `"quoted"`
	if runtime.GOOS == "windows" {
		create, echo, fail = "echo hi> x.txt", `echo "quoted" & echo err 1>&2`, "exit 3"
		failCmd, shell = `cmd.exe /c "exit 3"`, "cmd.exe"
		// cmd.exe does not understand CliWrap's \" escapes; lip prints the
		// same (verified with CliWrap 3.10.5).
		wantEcho = `\"quoted\"`
	} else {
		if _, err := os.Stat("/bin/sh"); err != nil {
			t.Skip("no sh")
		}
		create, echo, fail = "echo hi > x.txt", `echo '"quoted"'; echo err 1>&2`, "exit 3"
		failCmd, shell = `sh -c "exit 3"`, "sh"
	}

	variant := &manifest.Variant{Scripts: manifest.Scripts{Install: []string{create, echo}}}
	if _, err := w.InstallPackage(context.Background(), variant, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(dir, "x.txt"))); got != "hi" {
		t.Errorf("x.txt = %q", got)
	}
	var info, errs []string
	for _, l := range rep.lines {
		if s, ok := strings.CutPrefix(l, "info: "); ok {
			info = append(info, strings.TrimSpace(s))
		} else if s, ok := strings.CutPrefix(l, "error: "); ok {
			errs = append(errs, strings.TrimSpace(s))
		}
	}
	if !reflect.DeepEqual(info, []string{wantEcho}) || !reflect.DeepEqual(errs, []string{"err"}) {
		t.Errorf("info=%q errors=%q", info, errs)
	}

	variant = &manifest.Variant{Scripts: manifest.Scripts{PostInstall: []string{fail, "echo never> never.txt"}}}
	_, err := w.InstallPackage(context.Background(), variant, nil)
	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	if ce.ExitCode != 3 || ce.Command != failCmd || ce.ProcessName != shell {
		t.Errorf("CommandError %+v", ce)
	}
	wantPrefix := fmt.Sprintf("Command execution failed because the underlying process (%s#%d) returned a non-zero exit code (3).\n\nCommand:\n%s\n\nYou can suppress", shell, ce.PID, failCmd)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("message %q", err.Error())
	}
	if exists(filepath.Join(dir, "never.txt")) {
		t.Error("script after failure ran")
	}
}
