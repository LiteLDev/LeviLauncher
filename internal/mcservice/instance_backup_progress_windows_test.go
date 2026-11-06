//go:build windows

package mcservice

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
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
	events := recordInstanceBackupCreateProgress(t)

	result := BackupInstance("Source", types.InstanceBackupRequest{
		Scopes:     []string{"gameData", "mods"},
		ScopeModes: map[string]string{"gameData": instanceBackupModeSafe},
	})
	if result.ErrorCode == "" {
		t.Fatalf("expected the locked file to fail the backup")
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
