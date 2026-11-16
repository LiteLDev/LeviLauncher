package peeditor

import (
	"bytes"

	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

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
