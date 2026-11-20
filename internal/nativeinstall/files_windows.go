//go:build windows

package nativeinstall

import (
	"golang.org/x/sys/windows"
	"os"
)

type cacheLock = windows.Handle

func closeCacheLock(lock cacheLock) { windows.CloseHandle(lock) }
func openSourceFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
