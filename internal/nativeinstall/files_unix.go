//go:build darwin

package nativeinstall

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

type cacheLock = *os.File

func closeCacheLock(lock cacheLock) { unix.Flock(int(lock.Fd()), unix.LOCK_UN); lock.Close() }
func lockCache(dir, name, code string) cacheLock {
	secureCache(dir)
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		failCode(code, "cannot open account cache lock")
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		failCode(code, "another operation is using this account cache")
	}
	return f
}
func lockDeviceCache(dir string) cacheLock {
	return lockCache(dir, "device.lock", "ERR_AUTH_DEVICE_BUSY")
}
func lockAccountSelection(dir string) cacheLock {
	return lockCache(dir, "account.lock", "ERR_AUTH_ACCOUNT_SELECTION")
}
func openSourceFile(path string) (*os.File, error) { return os.Open(path) }
func secureCache(dir string) {
	must(os.MkdirAll(dir, 0700))
	info, err := os.Lstat(dir)
	must(err)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		fail("credential cache must be a real directory")
	}
	must(os.Chmod(dir, 0700))
}
func deviceInfo() (string, error) {
	// Xodus also uses an empty SMBIOS block when hardware probing is unavailable.
	// Credentials and the service-assigned device identity are persisted separately.
	return makeDeviceInfo(nil), nil
}
