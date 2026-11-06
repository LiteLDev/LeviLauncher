//go:build windows

package lockfile

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileRenameInfo is FILE_RENAME_INFO with the Flags member of the union.
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// replaceRename renames tmp over dst with POSIX semantics, so the rename
// succeeds while readers still have the old dst open and its name is free
// at once (MoveFileEx leaves a delete-pending file behind, on which the next
// rename fails with "Access is denied"). File systems without POSIX rename
// support fall back to os.Rename.
func replaceRename(tmp, dst string) error {
	if err := posixRename(tmp, dst); err == nil {
		return nil
	}
	return os.Rename(tmp, dst)
}

func posixRename(tmp, dst string) error {
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	name, err := windows.UTF16FromString(absDst)
	if err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(from, windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	nameBytes := (len(name) - 1) * 2 // without the terminating NUL
	size := int(unsafe.Offsetof(fileRenameInfo{}.FileName)) + nameBytes + 2
	buf := make([]byte, size)
	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	return windows.SetFileInformationByHandle(h, windows.FileRenameInfoEx, &buf[0], uint32(size))
}
