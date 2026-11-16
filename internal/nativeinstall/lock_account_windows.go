package nativeinstall

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Cache operations already hold device.lock, but sign-in must be able to replace
// the account during a long installation. This separate, short-lived lock keeps
// another launcher process from overwriting a concurrent account switch.
func lockAccountSelection(dir string) windows.Handle {
	path, err := windows.UTF16PtrFromString(filepath.Join(dir, "account.lock"))
	must(err)
	lock, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		failCode("ERR_AUTH_DEVICE_BUSY", "another process is updating the selected account")
	}
	return lock
}
