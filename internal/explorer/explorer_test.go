package explorer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPathCreatesDirectoryAndPassesLiteralPath(t *testing.T) {
	// These characters must reach the OS handler without shell interpolation.
	dir := filepath.Join(t.TempDir(), "中文 space & $literal", "mods")
	var opened string
	if !openPath("  "+dir+"  ", func(path string) error {
		opened = path
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("directory must exist before opening: %v", err)
		}
		return nil
	}) {
		t.Fatal("OpenPath failed")
	}
	if opened != dir {
		t.Fatalf("opened %q, want literal path %q", opened, dir)
	}
}

func TestOpenPathRejectsInvalidDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"", "  ", file, filepath.Join(file, "child")} {
		t.Run(dir, func(t *testing.T) {
			if openPath(dir, func(string) error {
				t.Fatal("invalid directory must not reach the OS handler")
				return nil
			}) {
				t.Fatal("invalid directory accepted")
			}
		})
	}
}

func TestOpenPathReportsHandlerFailure(t *testing.T) {
	if openPath(t.TempDir(), func(string) error { return errors.New("handler failed") }) {
		t.Fatal("OpenPath must report the OS handler failure")
	}
}
