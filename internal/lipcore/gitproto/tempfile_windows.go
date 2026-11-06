package gitproto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// createDeleteOnCloseTemp creates lip-<guid>.pack in the temp directory,
// opened like lip's CreateTempFile (FileShare.None, DeleteOnClose), so the
// system removes it when the handle is closed, even if the process dies.
func createDeleteOnCloseTemp() (*os.File, error) {
	for range 100 {
		var b [16]byte
		_, _ = rand.Read(b[:])
		name := filepath.Join(os.TempDir(), "lip-"+hex.EncodeToString(b[:])+".pack")
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.CREATE_NEW, windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) {
			continue
		}
		if err != nil {
			return nil, &os.PathError{Op: "open", Path: name, Err: err}
		}
		return os.NewFile(uintptr(h), name), nil
	}
	return nil, errors.New("gitproto: cannot create a temporary pack file")
}
