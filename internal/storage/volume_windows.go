//go:build windows

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// sameVolume reports whether a and b, or their nearest existing ancestors,
// are on one volume, so that a rename between them is possible.
func sameVolume(a, b string) bool {
	va, vb := volumeOf(a), volumeOf(b)
	return va != "" && strings.EqualFold(va, vb)
}

func volumeOf(p string) string {
	cur := filepath.Clean(p)
	for {
		if _, err := os.Stat(cur); err == nil {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
	ptr, err := windows.UTF16PtrFromString(cur)
	if err != nil {
		return ""
	}
	buf := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(ptr, &buf[0], uint32(len(buf))); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// isTransient reports an error that another process holding a file causes.
func isTransient(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// isCrossDevice reports a rename that would cross volumes, for example
// through a junction to another drive.
func isCrossDevice(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}

// secureDir gives an account directory copied across volumes the protected
// DACL the account code sets (internal/nativeinstall secureCache): only the
// user and SYSTEM. A copy inherits its new parent's DACL instead.
func secureDir(dir string) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return
	}
	_ = filepath.Walk(dir, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		_ = windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
			nil, nil, dacl, nil)
		return nil
	})
}

// openLockFile opens the lock file so that it can still be deleted, by an
// uninstaller or a test removing its directory, while the lock is held.
func openLockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

const lockWait = 2 * time.Minute

func lockFileNow(f *os.File, exclusive bool) bool {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	ol := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, ol) == nil
}

// lockFileShared waits for a shared lock, that is for a process that holds
// it exclusively to finish its migration, for at most lockWait.
func lockFileShared(f *os.File) bool {
	deadline := time.Now().Add(lockWait)
	for {
		if lockFileNow(f, false) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func unlockFile(f *os.File) {
	ol := new(windows.Overlapped)
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}
