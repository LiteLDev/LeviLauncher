package peeditor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWritePatchedExecutablePreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")
	original := []byte("original image")
	patched := []byte("complete patched image")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writePatchedExecutable(path, patched); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string][]byte{path: patched, path + ".orig": original} {
		got, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(got, expected) {
			t.Fatalf("%s: got %q, want %q (err=%v)", name, got, expected, err)
		}
	}
	assertNoStagedImages(t, filepath.Dir(path))
}

func TestWritePatchedExecutableLeavesLockedOriginalIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")
	original := []byte("original image")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Allow reads and writes but deny replacement, as an in-use image can do.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	err = writePatchedExecutable(path, []byte("patched image"))
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("expected sharing violation, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed replacement changed the original: %q (err=%v)", got, err)
	}
	assertNoStagedImages(t, filepath.Dir(path))
}

func assertNoStagedImages(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".leviloader-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("staged files were not cleaned up: %v (err=%v)", files, err)
	}
}
