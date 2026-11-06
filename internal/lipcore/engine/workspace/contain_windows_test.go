//go:build windows

package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		{name: "device NUL", dest: "plugins/NUL", wantInErr: "reserved device name"},
		{name: "device with extension", dest: "plugins/con.txt", wantInErr: "reserved device name"},
		{name: "device with spaces", dest: "COM1 .log", wantInErr: "reserved device name"},
		{name: "device superscript", dest: "lpt¹", wantInErr: "reserved device name"},
		{name: "device console", dest: "a/CONOUT$", wantInErr: "reserved device name"},
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
		j("a.txt") + ":stream", j("NUL"), `\\?\` + j("a.txt"),
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
