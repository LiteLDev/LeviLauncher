package peeditor

import (
	"fmt"
	"os"
	"path/filepath"

	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

// writePatchedExecutable stages the complete image before replacing the game.
// ReplaceFileW preserves the original file's ACLs and saves it as .orig.
func writePatchedExecutable(exePath string, data []byte) error {
	tmp, err := fsx.CreateTemp(filepath.Dir(exePath), exePath)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	original, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	replacement, err := windows.UTF16PtrFromString(tmp.Name())
	if err != nil {
		return err
	}
	backup, err := windows.UTF16PtrFromString(exePath + ".orig")
	if err != nil {
		return err
	}
	ok, _, err := procReplaceFile.Call(
		uintptr(unsafe.Pointer(original)),
		uintptr(unsafe.Pointer(replacement)),
		uintptr(unsafe.Pointer(backup)),
		0, 0, 0,
	)
	if ok == 0 {
		return fmt.Errorf("replace %s (backup: %s.orig): %w", exePath, exePath, err)
	}
	return nil
}

var procReplaceFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")
