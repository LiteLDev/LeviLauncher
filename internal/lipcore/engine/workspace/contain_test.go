package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// containFixture is a workspace "inst" next to a directory "out" that
// packages must not reach.
type containFixture struct {
	dir, out string
	scripts  *scriptLog
	rep      *recReporter
	w        *Workspace
}

func newContainFixture(t *testing.T) *containFixture {
	t.Helper()
	base := t.TempDir()
	f := &containFixture{dir: filepath.Join(base, "inst"), out: filepath.Join(base, "out"), scripts: &scriptLog{}, rep: &recReporter{}}
	for _, d := range []string{f.dir, f.out} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.w = &Workspace{Dir: f.dir, Reporter: f.rep, Glob: fakeCompile, RunCommand: f.scripts.run}
	return f
}

func (f *containFixture) install(src *memSource, placements ...manifest.Placement) ([]string, error) {
	variant := &manifest.Variant{
		Assets: []manifest.Asset{
			// A good asset first: nothing of it may be written either.
			{Type: manifest.AssetZip, URLs: []string{"good"}, Placements: []manifest.Placement{{Type: manifest.PlacementFile, Src: "good.txt", Dest: "plugins/good.txt"}}},
			{Type: manifest.AssetZip, URLs: []string{"bad"}, Placements: placements},
		},
		Scripts: manifest.Scripts{PreInstall: []string{"pre"}, PostInstall: []string{"post"}},
	}
	good := newMemSource("good.txt", "G")
	return f.w.InstallPackage(context.Background(), variant, func(_ context.Context, a manifest.Asset) (Source, error) {
		if a.URLs[0] == "good" {
			return good, nil
		}
		return src, nil
	})
}

// assertRejected checks that the package was refused up front: no script
// ran and nothing was written in or next to the workspace.
func (f *containFixture) assertRejected(t *testing.T, files []string, err error, wantInErr ...string) {
	t.Helper()
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("err = %v, want ErrUnsafePath", err)
	}
	for _, s := range wantInErr {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("err %q does not mention %q", err, s)
		}
	}
	if files != nil || f.scripts.calls != nil {
		t.Errorf("files=%q scripts=%q", files, f.scripts.calls)
	}
	names := func(d string) []string {
		entries, _ := os.ReadDir(d)
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if got := names(filepath.Dir(f.dir)); !reflect.DeepEqual(got, []string{"inst", "out"}) {
		t.Errorf("wrote next to the workspace: %q", got)
	}
	if got := names(f.out); got != nil {
		t.Errorf("wrote outside the workspace: %q", got)
	}
	// Only a link the test made may be in the workspace.
	if got := names(f.dir); got != nil && !reflect.DeepEqual(got, []string{"link"}) {
		t.Errorf("wrote in the workspace: %q", got)
	}
}

func TestInstallPackageRejectsEscapingTargets(t *testing.T) {
	file := func(src, dest string) manifest.Placement {
		return manifest.Placement{Type: manifest.PlacementFile, Src: src, Dest: dest}
	}
	cases := []struct {
		name      string
		keys      []string
		placement manifest.Placement
		wantInErr string
	}{
		{name: "dest dotdot", keys: []string{"a"}, placement: file("a", "../out/evil.txt"), wantInErr: "outside the workspace directory"},
		{name: "dest nested dotdot", keys: []string{"a"}, placement: file("a", "plugins/../../out/evil.txt"), wantInErr: "outside the workspace directory"},
		{name: "dest absolute", keys: []string{"a"}, placement: file("a", "OUT/evil.txt"), wantInErr: "outside the workspace directory"},
		{name: "dest sibling with same prefix", keys: []string{"a"}, placement: file("a", "../instX/evil.txt"), wantInErr: "outside the workspace directory"},
		{name: "dest is the workspace", keys: []string{"a"}, placement: file("a", "."), wantInErr: "itself"},
		{name: "dest replaces the workspace", keys: []string{"a"}, placement: file("a", "plugins/.."), wantInErr: "itself"},
		// GetFileName("x/..") is "..", so a glob placement with an empty dest
		// places the key at the workspace's parent.
		{name: "key basename dotdot", keys: []string{"x/.."}, placement: file("x/*", ""), wantInErr: "outside the workspace directory"},
		{name: "key basename dotdot nested", keys: []string{"x/.."}, placement: file("x/*", "plugins"), wantInErr: "itself"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newContainFixture(t)
			p := c.placement
			p.Dest = strings.ReplaceAll(p.Dest, "OUT", f.out)
			kv := []string{}
			for _, k := range c.keys {
				kv = append(kv, k, "EVIL")
			}
			files, err := f.install(newMemSource(kv...), p)
			f.assertRejected(t, files, err, c.wantInErr, c.keys[0])
		})
	}
}

func TestInstallPackageRejectsLinkOutside(t *testing.T) {
	f := newContainFixture(t)
	makeDirLink(t, filepath.Join(f.dir, "link"), f.out)
	files, err := f.install(newMemSource("a", "EVIL"), manifest.Placement{Type: manifest.PlacementFile, Src: "a", Dest: "link/sub/evil.txt"})
	f.assertRejected(t, files, err, "goes through the link '"+filepath.Join(f.dir, "link")+"'", "points outside")
}

func TestInstallPackageRejectsFileLinkOutside(t *testing.T) {
	f := newContainFixture(t)
	outside := filepath.Join(f.out, "victim.txt")
	mustWrite(t, outside, "victim")
	if err := os.Symlink(outside, filepath.Join(f.dir, "link")); err != nil {
		t.Skipf("cannot create a file symlink: %v", err)
	}
	files, err := f.install(newMemSource("a", "EVIL"), manifest.Placement{Type: manifest.PlacementFile, Src: "a", Dest: "link"})
	if !errors.Is(err, ErrUnsafePath) || !strings.Contains(err.Error(), "points outside") || files != nil || f.scripts.calls != nil {
		t.Fatalf("files=%q err=%v scripts=%q", files, err, f.scripts.calls)
	}
	if got := readFile(t, outside); got != "victim" {
		t.Errorf("outside file = %q", got)
	}
}

func TestInstallPackageNestedPlacementStillWorks(t *testing.T) {
	f := newContainFixture(t)
	// A LeviLamina-style manifest: one dir placement into a nested
	// directory, plus files through a link that stays inside the workspace
	// and an absolute dest inside it (in different case where paths ignore
	// it).
	if err := os.MkdirAll(filepath.Join(f.dir, "shared", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeDirLink(t, filepath.Join(f.dir, "link"), filepath.Join(f.dir, "shared"))
	abs := filepath.Join(f.dir, "abs.txt")
	if pathIgnoreCase {
		abs = strings.ToUpper(f.dir) + string(filepath.Separator) + "abs.txt"
	}
	zip := newMemSource(
		"LeviLamina/LeviLamina.dll", "DLL",
		"LeviLamina/lang/en_US.json", "EN",
		"LeviLamina/../x", "X", // skipped by the dir placement
		"other.txt", "O",
	)
	variant := &manifest.Variant{
		Assets: []manifest.Asset{{Type: manifest.AssetZip, URLs: []string{"z"}, Placements: []manifest.Placement{
			{Type: manifest.PlacementDir, Src: "LeviLamina/", Dest: "mods/LeviLamina/"},
			{Type: manifest.PlacementFile, Src: "other.txt", Dest: "link/data/other.txt"},
			{Type: manifest.PlacementFile, Src: "other.txt", Dest: abs},
		}}},
		Scripts: manifest.Scripts{PreInstall: []string{"pre"}},
	}
	files, err := f.w.InstallPackage(context.Background(), variant, func(context.Context, manifest.Asset) (Source, error) { return zip, nil })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(f.dir, "mods", "LeviLamina", "LeviLamina.dll"),
		filepath.Join(f.dir, "mods", "LeviLamina", "lang", "en_US.json"),
		filepath.Join(f.dir, "link", "data", "other.txt"),
		abs,
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files\n got  %q\n want %q", files, want)
	}
	for p, c := range map[string]string{want[0]: "DLL", want[1]: "EN", filepath.Join(f.dir, "shared", "data", "other.txt"): "O", filepath.Join(f.dir, "abs.txt"): "O"} {
		if got := readFile(t, p); got != c {
			t.Errorf("%s = %q want %q", p, got, c)
		}
	}
	if !reflect.DeepEqual(f.scripts.calls, []string{"pre"}) {
		t.Errorf("scripts %q", f.scripts.calls)
	}
}

func TestUninstallPackageSkipsFilesOutside(t *testing.T) {
	f := newContainFixture(t)
	j := func(p string) string { return filepath.Join(f.dir, filepath.FromSlash(p)) }
	outside := filepath.Join(f.out, "victim.txt")
	mustWrite(t, outside, "victim")
	mustWrite(t, j("plugins/a.dll"), "A")
	makeDirLink(t, j("link"), f.out)

	files := []string{
		outside,              // absolute, outside
		"../out/victim.txt",  // relative escape, as a hand-edited lock might have
		j("link/victim.txt"), // through a link pointing outside
		f.dir,                // the workspace itself
		j("plugins/a.dll"),   // a normal file
	}
	variant := &manifest.Variant{Scripts: manifest.Scripts{PostUninstall: []string{"post"}}}
	if err := f.w.UninstallPackage(context.Background(), variant, files); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, outside); got != "victim" {
		t.Errorf("outside file = %q", got)
	}
	if exists(j("plugins/a.dll")) {
		t.Error("plugins/a.dll still exists")
	}
	if !exists(f.dir) || !exists(j("link")) {
		t.Error("workspace or link removed")
	}
	if !reflect.DeepEqual(f.scripts.calls, []string{"post"}) {
		t.Errorf("scripts %q", f.scripts.calls)
	}
	var warnings []string
	for _, l := range f.rep.lines {
		if strings.HasPrefix(l, "warning: ") {
			warnings = append(warnings, l)
		}
	}
	wantWarn := []string{
		"File '" + outside + "' is not in the workspace (it is outside the workspace directory '" + f.dir + "'), skipping removal.",
		"File '" + outside + "' is not in the workspace (it is outside the workspace directory '" + f.dir + "'), skipping removal.",
		"File '" + j("link/victim.txt") + "' is not in the workspace (it goes through the link '" + j("link") + "', which points outside the workspace directory (to '",
		"File '" + f.dir + "' is not in the workspace (it is the workspace directory '" + f.dir + "' itself), skipping removal.",
	}
	if len(warnings) != len(wantWarn) {
		t.Fatalf("warnings %q", warnings)
	}
	for i, w := range wantWarn {
		if !strings.HasPrefix(warnings[i], "warning: "+w) {
			t.Errorf("warning %d\n got  %q\n want %q...", i, warnings[i], w)
		}
	}
}

func TestUninstallRemoveFilesStaysInside(t *testing.T) {
	f := newContainFixture(t)
	j := func(p string) string { return filepath.Join(f.dir, filepath.FromSlash(p)) }
	mustWrite(t, filepath.Join(f.out, "sub", "victim.txt"), "victim")
	mustWrite(t, filepath.Join(f.out, "victim.log"), "victim")
	mustWrite(t, j("mine.log"), "mine")
	makeDirLink(t, j("link"), f.out)
	// A file symlink needs a privilege on Windows; test it where possible.
	_ = os.Symlink(filepath.Join(f.out, "victim.log"), j("flink.log"))

	variant := &manifest.Variant{RemoveFiles: []string{"**/*.log", "**/*.txt", "link/*", "link/sub/", "../out/*", "../*", "*"}}
	if err := f.w.UninstallPackage(context.Background(), variant, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(f.out, "sub", "victim.txt"), filepath.Join(f.out, "victim.log")} {
		if got := readFile(t, p); got != "victim" {
			t.Errorf("%s = %q", p, got)
		}
	}
	// "*" matches the links themselves, which are removed without touching
	// their targets.
	for _, p := range []string{"mine.log", "link", "flink.log"} {
		if exists(j(p)) {
			t.Errorf("%s still exists", p)
		}
	}
}
