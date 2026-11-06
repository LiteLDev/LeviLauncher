//go:build windows

package lockfile

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// readAll reads path with FILE_SHARE_DELETE, so a concurrent Save can rename
// over tooth_lock.json while it is being read (os.ReadFile omits that share
// mode, which makes the rename fail with "Access is denied").
func readAll(path string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
