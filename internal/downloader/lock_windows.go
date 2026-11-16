package downloader

import (
	"errors"

	"golang.org/x/sys/windows"
)

// lockDownload takes an exclusive lock file next to dest so two processes do
// not write the same partial file. The lock disappears with the process.
func lockDownload(dest string) (func(), error) {
	p, err := windows.UTF16PtrFromString(dest + ".lock")
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return func() { _ = windows.CloseHandle(h) }, nil
}
