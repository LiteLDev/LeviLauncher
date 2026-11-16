package nativeinstall

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func lockDeviceCache(dir string) windows.Handle {
	secureCache(dir)
	path, err := windows.UTF16PtrFromString(filepath.Join(dir, "device.lock"))
	must(err)
	lock, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		failCode("ERR_AUTH_DEVICE_BUSY", "the account device cache is in use")
	}
	return lock
}
