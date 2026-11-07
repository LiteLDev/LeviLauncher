//go:build windows

package storage

import (
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// migrationMutexName serialises Prepare between launcher processes: the
// window and command-line invocations. Tests replace it with a private name.
var migrationMutexName = `Global\LeviLauncher_StorageMigration`

// migrationMutexSDDL lets every signed-in user wait on and release the mutex,
// so an elevated and a non-elevated launcher can share it. The default DACL
// of an elevated creator would deny the other one.
const migrationMutexSDDL = "D:(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x100001;;;AU)"

// lockMigration takes the migration mutex, waiting up to wait for it
// (forever when wait is negative). It returns ok=false when the mutex is
// still held elsewhere or cannot be opened. unlock must run on the calling
// goroutine.
func lockMigration(wait time.Duration) (unlock func(), ok bool) {
	name, err := windows.UTF16PtrFromString(migrationMutexName)
	if err != nil {
		return nil, false
	}
	var sa *windows.SecurityAttributes
	if sd, err := windows.SecurityDescriptorFromString(migrationMutexSDDL); err == nil {
		sa = &windows.SecurityAttributes{
			Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
			SecurityDescriptor: sd,
		}
	}
	// A mutex is owned by a thread: keep this goroutine on it until unlock.
	runtime.LockOSThread()
	h, err := windows.CreateMutexEx(sa, name, 0, windows.SYNCHRONIZE|windows.MUTEX_MODIFY_STATE)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		runtime.UnlockOSThread()
		return nil, false
	}
	timeout := uint32(windows.INFINITE)
	if wait >= 0 {
		timeout = uint32(wait / time.Millisecond)
	}
	// WAIT_ABANDONED: a process died while migrating; its journal is
	// recovered under the lock.
	switch ev, _ := windows.WaitForSingleObject(h, timeout); ev {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		return func() {
			_ = windows.ReleaseMutex(h)
			_ = windows.CloseHandle(h)
			runtime.UnlockOSThread()
		}, true
	}
	_ = windows.CloseHandle(h)
	runtime.UnlockOSThread()
	return nil, false
}
