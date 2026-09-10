package explorer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenPathCreatesDirectoryAndPassesLiteralPath(t *testing.T) {
	// These characters must reach the OS handler without shell interpolation.
	dir := filepath.Join(t.TempDir(), "中文 space & $literal", "mods")
	var opened string
	if err := openPath("  "+dir+"  ", func(path string) error {
		opened = path
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("directory must exist before opening: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("OpenPath failed: %v", err)
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
			err := openPath(dir, func(string) error {
				t.Fatal("invalid directory must not reach the OS handler")
				return nil
			})
			if err == nil {
				t.Fatal("invalid directory accepted")
			}
			if strings.TrimSpace(dir) != "" {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) {
					t.Fatalf("filesystem error was lost: %v", err)
				}
			}
		})
	}
}

func TestOpenPathReportsHandlerFailure(t *testing.T) {
	dir := t.TempDir()
	want := errors.New("handler failed")
	err := openPath(dir, func(string) error { return want })
	if !errors.Is(err, want) || !strings.Contains(err.Error(), dir) {
		t.Fatalf("handler failure must retain its cause and directory, got: %v", err)
	}
}

func TestOpenPathWithoutApplication(t *testing.T) {
	if err := OpenPath(t.TempDir()); err == nil {
		t.Fatal("opening without the Wails application must return an error")
	}
}

func TestOpenModsRejectsEmptyVersion(t *testing.T) {
	if err := OpenMods("  "); err == nil {
		t.Fatal("empty version must return an error")
	}
}
