//go:build windows

package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
)

// makeDirLink makes link a junction to the directory target (no privilege
// needed, unlike a directory symlink).
func makeDirLink(t *testing.T, link, target string) {
	t.Helper()
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v: %s", err, out)
	}
}

// otherDrive returns a drive letter that is not the one of dir, preferring
// one that does not exist so that a regression cannot write to it.
func otherDrive(dir string) string {
	for c := 'Z'; c >= 'D'; c-- {
		d := string(c) + ":"
		if _, err := os.Stat(d + `\`); err != nil && !strings.EqualFold(filepath.VolumeName(dir), d) {
			return d
		}
	}
	if strings.EqualFold(filepath.VolumeName(dir), "Z:") {
		return "Y:"
	}
	return "Z:"
}

func TestInstallPackageRejectsWindowsForms(t *testing.T) {
	cases := []struct {
		name      string
		key, src  string
		dest      string
		dir       bool
		wantInErr string
	}{
		{name: "alternate data stream", dest: "plugins/evil.txt:stream", wantInErr: "alternate data stream"},
		{name: "default data stream", dest: "evil.txt::$DATA", wantInErr: "alternate data stream"},
		// GetFullPathName keeps names of only dots and spaces, which Win32
		// cannot create.
		{name: "dotdot with trailing space", dest: ".. /out/evil.txt", wantInErr: "ends in a dot or space"},
		{name: "dots and space", dest: "plugins/... /evil.txt", wantInErr: "ends in a dot or space"},
		{name: "trailing dots then dotdot", dest: "plugins./../../out/evil.txt", wantInErr: "outside the workspace directory"},
		{name: "backslash dotdot", dest: `plugins\..\..\out\evil.txt`, wantInErr: "outside the workspace directory"},
		{name: "rooted", dest: `\evil.txt`, wantInErr: "outside the workspace directory"},
		{name: "other drive", dest: "DRIVE\\evil.txt", wantInErr: "outside the workspace directory"},
		{name: "drive relative", dest: "DRIVEevil.txt", wantInErr: "outside the workspace directory"},
		{name: "UNC", dest: `\\localhost\lipcore-no-such-share\evil.txt`, wantInErr: "outside the workspace directory"},
		// Device paths are not normalized, so they never count as inside,
		// even when they name a file in the workspace.
		{name: "device path inside", dest: `\\?\DIR\evil.txt`, wantInErr: "outside the workspace directory"},
		{name: "dot device path inside", dest: `\\.\DIR\evil.txt`, wantInErr: "outside the workspace directory"},
		{name: "absolute key", key: `DRIVE\evil.txt`, dir: true, dest: "d", wantInErr: "outside the workspace directory"},
		{name: "backslash key basename dotdot", key: `x\..`, src: "x*", wantInErr: "outside the workspace directory"},
		// Beyond 32767 characters GetFullPathNameW fails and the ".."
		// segments stay; os.MkdirAll would then create "out\newdir".
		{name: "overlong key slash dotdot", key: "d/" + strings.Repeat("aaaaaaaaaa/", 3000) + strings.Repeat("../", 3001) + "../out/newdir/evil.txt", dir: true, wantInErr: "cannot be normalized"},
		{name: "overlong key backslash dotdot", key: `d\` + strings.Repeat(`aaaaaaaaaa\`, 3000) + strings.Repeat(`..\`, 3001) + `..\out\newdir\evil.txt`, dir: true, wantInErr: "outside the workspace directory"},
		{name: "overlong dest slash dotdot", dest: "d/" + strings.Repeat("aaaaaaaaaa/", 3000) + strings.Repeat("../", 3001) + "../out/newdir/evil.txt", wantInErr: "cannot be normalized"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newContainFixture(t)
			r := strings.NewReplacer("DRIVE", otherDrive(f.dir), "DIR", f.dir)
			key, src, dest := r.Replace(c.key), c.src, r.Replace(c.dest)
			if key == "" {
				key = "a"
			}
			if src == "" {
				src = key
			}
			p := manifest.Placement{Type: manifest.PlacementFile, Src: src, Dest: dest}
			if c.dir {
				p = manifest.Placement{Type: manifest.PlacementDir, Src: "", Dest: dest}
			}
			files, err := f.install(newMemSource(key, "EVIL"), p)
			f.assertRejected(t, files, err, c.wantInErr, "'"+key+"'")
		})
	}
}

func TestUninstallPackageSkipsWindowsForms(t *testing.T) {
	f := newContainFixture(t)
	j := func(p string) string { return filepath.Join(f.dir, filepath.FromSlash(p)) }
	mustWrite(t, j("a.txt"), "A")
	victim := filepath.Join(f.out, "victim.txt")
	mustWrite(t, victim, "V")
	files := []string{
		j("a.txt") + ":stream", `\\?\` + j("a.txt"), `\\.\` + j("a.txt"),
		// GetFullPathNameW cannot normalize these.
		"d/../../out/victim.txt\x00",
		"d/" + strings.Repeat("aaaaaaaaaa/", 3000) + strings.Repeat("../", 3002) + "out/victim.txt",
	}
	if err := f.w.UninstallPackage(context.Background(), &manifest.Variant{}, files); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, j("a.txt")); got != "A" {
		t.Errorf("a.txt = %q", got)
	}
	if got := readFile(t, victim); got != "V" {
		t.Errorf("victim.txt = %q", got)
	}
	if len(f.rep.lines) != len(files) {
		t.Fatalf("reporter %q", f.rep.lines)
	}
	for i, l := range f.rep.lines {
		if !strings.HasPrefix(l, "warning: File '") || !strings.HasSuffix(l, "skipping removal.") || !strings.Contains(l, "is not in the workspace") {
			t.Errorf("line %d = %q", i, l)
		}
	}
}

// caseSensitiveTempDir returns a new directory marked case-sensitive, as
// WSL interop marks directories, or skips the test when that needs a
// privilege or WSL support this machine does not have.
func caseSensitiveTempDir(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if out, err := exec.Command("fsutil.exe", "file", "setCaseSensitiveInfo", base, "enable").CombinedOutput(); err != nil {
		t.Skipf("cannot mark a directory case-sensitive: %v: %s", err, out)
	}
	mustWrite(t, filepath.Join(base, "probe"), "")
	defer os.Remove(filepath.Join(base, "probe"))
	if _, err := os.Stat(filepath.Join(base, "PROBE")); err == nil {
		t.Skip("the directory still ignores case")
	}
	return base
}

// In a case-sensitive directory "INST" is another directory than "inst".
func TestInstallPackageRejectsCaseVariantInCaseSensitiveDirectory(t *testing.T) {
	f := newContainFixtureIn(t, caseSensitiveTempDir(t), "inst")
	files, err := f.install(newMemSource("a", "EVIL"), manifest.Placement{Type: manifest.PlacementFile, Src: "a", Dest: "../INST/evil.txt"})
	f.assertRejected(t, files, err, "outside the workspace directory")
}

func TestUninstallPackageSkipsCaseVariantInCaseSensitiveDirectory(t *testing.T) {
	f := newContainFixtureIn(t, caseSensitiveTempDir(t), "inst")
	mine := filepath.Join(f.dir, "victim.txt")
	victim := filepath.Join(filepath.Dir(f.dir), "INST", "victim.txt")
	mustWrite(t, mine, "mine")
	mustWrite(t, victim, "victim")
	if err := f.w.UninstallPackage(context.Background(), &manifest.Variant{}, []string{victim}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, victim); got != "victim" {
		t.Errorf("victim = %q", got)
	}
	if got := readFile(t, mine); got != "mine" {
		t.Errorf("workspace file = %q", got)
	}
	want := []string{"warning: File '" + victim + "' is not in the workspace (it is outside the workspace directory '" + f.dir + "'), skipping removal."}
	if !reflect.DeepEqual(f.rep.lines, want) {
		t.Errorf("reporter\n got  %q\n want %q", f.rep.lines, want)
	}
}

// Windows 11 opens an ordinary file named like a device (aux.json, CON)
// in a directory, and lip installs it; older Windows turns the name into
// a device path (\\.\AUX), which is refused.
func TestInstallPackageDeviceNames(t *testing.T) {
	for _, dest := range []string{"aux.json", "config/aux.json", "CON", "NUL.log", "com1.cfg", "COM1 .log", "lpt¹", "CONIN$", "a/CONOUT$", "plugins/con.txt"} {
		t.Run(dest, func(t *testing.T) {
			f := newContainFixture(t)
			full := f.w.FullPath(dest)
			files, err := f.install(newMemSource("a", "DATA"), manifest.Placement{Type: manifest.PlacementFile, Src: "a", Dest: dest})
			if strings.HasPrefix(full, `\\.\`) || strings.HasPrefix(full, `\\?\`) {
				f.assertRejected(t, files, err, "device path")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{filepath.Join(f.dir, "plugins", "good.txt"), full}; !reflect.DeepEqual(files, want) {
				t.Errorf("files\n got  %q\n want %q", files, want)
			}
			if fi, err := os.Lstat(full); err != nil || !fi.Mode().IsRegular() {
				t.Fatalf("%s: %v %v", full, fi, err)
			}
			if got := readFile(t, full); got != "DATA" {
				t.Errorf("%s = %q", full, got)
			}
		})
	}
}

// FullPath expands a path containing '~' with GetLongPathNameW, which also
// gives existing components their on-disk case: a file placed in a Dir
// spelled in another case keeps being accepted.
func TestInstallPackageLongNameWithDirInOtherCase(t *testing.T) {
	f := newContainFixture(t)
	f.w.Dir = filepath.Join(filepath.Dir(f.dir), "INST")
	if _, err := os.Stat(f.w.Dir); err != nil {
		t.Skipf("the file system distinguishes case here: %v", err)
	}
	files, err := f.install(newMemSource("a", "DATA"), manifest.Placement{Type: manifest.PlacementFile, Src: "a", Dest: "plugins/a~1.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || !strings.HasSuffix(files[1], `\plugins\a~1.txt`) {
		t.Errorf("files %q", files)
	}
	if got := readFile(t, filepath.Join(f.dir, "plugins", "a~1.txt")); got != "DATA" {
		t.Errorf("a~1.txt = %q", got)
	}
}
