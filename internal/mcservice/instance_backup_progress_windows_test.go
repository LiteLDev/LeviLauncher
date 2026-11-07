//go:build windows

package mcservice

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/liteldev/LeviLauncher/internal/types"
)

// A source file locked against reading fails the backup mid-write: the
// progress stops there, with no "done" event.
func TestBackupInstanceCreateProgressStopsWithoutDoneOnWriteError(t *testing.T) {
	_, appDataDir, versionsDir := setupInstanceBackupEnv(t)
	sourceDir := createProgressTestSource(t, versionsDir, appDataDir)
	locked := filepath.Join(sourceDir, "mods", "raw_b", "manifest.json")
	name, err := windows.UTF16PtrFromString(locked)
	if err != nil {
		t.Fatalf("encode path: %v", err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("open %s: %v", locked, err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	// A byte-range lock fails every read of the file. A share mode of 0 is not enough: some machines,
	// such as the GitHub Actions runners, still let the file be opened and read.
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		t.Fatalf("lock %s: %v", locked, err)
	}
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
