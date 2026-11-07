//go:build windows

package archive

import (
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// Junctions need no privilege, unlike symbolic links: they are reparse
// points, which DirectorySource and Pack neither list nor descend into.
func TestDirectorySource_JunctionIsSkipped(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), []byte("s"))
	writeFile(t, filepath.Join(dir, "a.txt"), []byte("a"))
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(dir, "junction"), outside).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction: %v %s", err, out)
	}
	s, err := NewDirectorySource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("keys %q", got)
	}

	output := filepath.Join(t.TempDir(), "out.zip")
	if err := Pack(dir, output); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("packed %q", keys)
	}
}
