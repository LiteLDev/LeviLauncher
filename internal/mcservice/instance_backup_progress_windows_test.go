//go:build windows

package mcservice

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
	"golang.org/x/sys/windows"
)

// A source file locked against reading fails the backup mid-write: the
// progress stops there, with no "done" event.
func TestBackupInstanceCreateProgressStopsWithoutDoneOnWriteError(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createProgressTestSource(t, versionsDir, appDataDir)
	locked := filepath.Join(sourceDir, "mods", "raw_b", "manifest.json")
	name, err := syscall.UTF16PtrFromString(locked)
	if err != nil {
		t.Fatalf("encode path: %v", err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("lock %s: %v", locked, err)
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(handle) })
	// The share mode keeps others from opening the file; the byte-range lock
	// makes reading it fail too where the share mode is not enforced, as on
	// GitHub's runners under bash.
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(handle), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, ^uint32(0), ^uint32(0), overlapped); err != nil {
		t.Fatalf("lock range of %s: %v", locked, err)
	}
	t.Cleanup(func() { _ = windows.UnlockFileEx(windows.Handle(handle), 0, ^uint32(0), ^uint32(0), overlapped) })
	if data, err := os.ReadFile(locked); err == nil {
		t.Fatalf("%s is readable through the lock: %q", locked, data)
	} else if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Logf("the share mode is not enforced here; reading fails with: %v", err)
	}
	events := recordInstanceBackupCreateProgress(t)

	result := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": instanceBackupModeSafe},
	})
	if result.ErrorCode == "" {
		names := []string{}
		if z, err := zip.OpenReader(result.ArchivePath); err == nil {
			for _, f := range z.File {
				names = append(names, f.Name)
			}
			_ = z.Close()
		}
		t.Fatalf("expected the locked file to fail the backup; the archive %s has %q", result.ArchivePath, names)
	}
	if len(*events) < 2 || (*events)[0].Phase != "scanning" || (*events)[1].Phase != "writing" {
		t.Fatalf("expected scanning then writing events, got %+v", *events)
	}
	for _, event := range *events {
		if event.Phase == "done" {
			t.Fatalf("a failed backup must not report done: %+v", *events)
		}
	}
}
