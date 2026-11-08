package archive

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Ports of the tests PackerTests.cs, DirectorySourceTests.cs and
// FileIdentityTests.cs gained with lip's pack fixes (2b77373, b8d5cc5,
// bb0f7dd, eb2b40d, f33b8c7).

func TestFileIdentitySameFileByAnotherPathIsEqual(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	writeFile(t, file, []byte("a"))

	a, ok := identityOf(file)
	b, ok2 := identityOf(filepath.Join(root, ".", "a.txt"))

	if !ok || !ok2 || a != b {
		t.Fatalf("%v %v %v %v", a, ok, b, ok2)
	}
}

func TestFileIdentityOtherFilesDiffer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	writeFile(t, filepath.Join(root, "b.txt"), []byte("a"))

	a, _ := identityOf(filepath.Join(root, "a.txt"))
	b, _ := identityOf(filepath.Join(root, "b.txt"))

	if a == b {
		t.Fatal("equal")
	}
}

func TestFileIdentitySymbolicLinkIsTheIdentityOfItsTarget(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	writeFile(t, file, []byte("a"))
	link := filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("cannot create a link: %v", err)
	}

	a, _ := identityOf(file)
	b, ok := identityOf(link)

	if !ok || a != b {
		t.Fatal("differ")
	}
}

func TestFileIdentityMissingIsNone(t *testing.T) {
	if _, ok := identityOf(filepath.Join(t.TempDir(), "missing")); ok {
		t.Fatal("identity of a missing file")
	}
}

func TestPackNameWithBackslashFailsWithoutWriting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`Windows names cannot contain '\'.`)
	}
	// A ZIP entry cannot contain '\' (protocol/package.md §1), so the file
	// cannot be packed at its own path (protocol/cli.md §5.11).
	for _, path := range []string{`a\b.txt`, `d\e/f.txt`} {
		root := t.TempDir()
		full := filepath.Join(root, path)
		writeFile(t, full, []byte("x"))
		output := filepath.Join(root, "out.zip")
		writeFile(t, output, []byte("previous"))

		err := Pack(root, output)

		// Nothing is written, not even the truncation of output.
		if err == nil || !strings.Contains(err.Error(), full) {
			t.Fatalf("%s: err = %v", path, err)
		}
		if b, _ := os.ReadFile(output); string(b) != "previous" {
			t.Fatalf("%s: output %q", path, b)
		}
	}
}

func TestPackOutputThroughALinkToTheDirectoryIsLeftOut(t *testing.T) {
	// Output is never part of the archive (protocol/cli.md §5.11), whatever
	// path reaches it; here a symbolic link (a junction on Windows) to the
	// directory.
	temp := t.TempDir()
	root := filepath.Join(temp, "pkg")
	link := filepath.Join(temp, "link")
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	writeFile(t, filepath.Join(root, "out.zip"), []byte("previous"))
	if err := linkDirectory(link, root); err != nil {
		t.Skipf("directory links cannot be created here: %v", err)
	}
	defer os.Remove(link)

	if err := Pack(root, filepath.Join(link, "out.zip")); err != nil {
		t.Fatal(err)
	}

	if keys := sortedKeys(zipContents(t, filepath.Join(root, "out.zip"))); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestPackGitAndLipInAnotherCaseAreLeftOutWhereTheyAreThem(t *testing.T) {
	// On a case-insensitive file system (e.g. macOS, Windows) .GIT and .Lip
	// are .git and .lip; on a case-sensitive one they are other directories,
	// which are packed.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	writeFile(t, filepath.Join(root, ".GIT", "x"), []byte("x"))
	writeFile(t, filepath.Join(root, ".Lip", "y"), []byte("y"))
	_, err := os.Stat(filepath.Join(root, ".git"))
	ignoresCase := err == nil
	output := filepath.Join(root, "out.zip")

	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}

	want := []string{".GIT/x", ".Lip/y", "a.txt"}
	if ignoresCase {
		want = []string{"a.txt"}
	}
	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys, want) {
		t.Fatalf("keys %q, want %q", keys, want)
	}
}

func TestPackOutputByAnotherCaseIsLeftOut(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	writeFile(t, filepath.Join(root, "pkg.zip"), []byte("previous"))
	if _, err := os.Stat(filepath.Join(root, "PKG.ZIP")); err != nil {
		t.Skip("The file system is case-sensitive.")
	}

	if err := Pack(root, filepath.Join(root, "Pkg.zip")); err != nil {
		t.Fatal(err)
	}

	if keys := sortedKeys(zipContents(t, filepath.Join(root, "pkg.zip"))); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestPackOutputByItsShortNameIsLeftOut(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Only Windows has 8.3 short names.")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	output := filepath.Join(root, "longfilename.zip")
	writeFile(t, output, []byte("previous"))
	short := shortName(output)
	if short == "" || short == "longfilename.zip" {
		t.Skip("8.3 short names are disabled on this volume.")
	}

	if err := Pack(root, filepath.Join(root, short)); err != nil {
		t.Fatal(err)
	}

	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestPackFifoAndSocketAreLeftOut(t *testing.T) {
	// Only regular files are packed; reading a FIFO would wait for a writer
	// forever.
	root := shortTempDir(t)
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	makeSpecialFiles(t, filepath.Join(root, "sub", "fifo"), filepath.Join(root, "socket"))
	output := filepath.Join(root, "out.zip")

	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}

	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestDirectorySourceFifoAndSocketAreLeftOut(t *testing.T) {
	// Only regular files are part of a directory archive.
	root := shortTempDir(t)
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	makeSpecialFiles(t, filepath.Join(root, "sub", "fifo"), filepath.Join(root, "socket"))

	s, err := NewDirectorySource(root)

	if err != nil {
		t.Fatal(err)
	}
	if keys := s.Keys(); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestDirectorySourceNameWithBackslashFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`Windows names cannot contain '\'.`)
	}
	for _, path := range []string{`a\b.txt`, `d\e/f.txt`} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, path), []byte("x"))

		_, err := NewDirectorySource(root)

		var ue *UnavailableError
		if !errors.As(err, &ue) || !strings.Contains(ue.Message, `'\'`) {
			t.Fatalf("%s: err = %v", path, err)
		}
	}
}

// linkDirectory links the directory link to target: a junction on Windows,
// which needs no privilege, and a symbolic link elsewhere.
func linkDirectory(link, target string) error {
	if runtime.GOOS != "windows" {
		return os.Symlink(target, link)
	}
	return exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).Run()
}

// shortTempDir is a new temporary directory short enough to hold a Unix
// domain socket: sun_path is limited to 104 bytes on macOS.
func shortTempDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no FIFOs or socket files.")
	}
	dir, err := os.MkdirTemp("/tmp", "lip-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
