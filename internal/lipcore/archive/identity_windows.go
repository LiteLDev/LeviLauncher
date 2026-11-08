//go:build windows

package archive

import (
	"encoding/binary"

	"golang.org/x/sys/windows"
)

// fileIdentityOf is FileIdentity.OfWindows: the volume serial number and the
// file id of FILE_ID_INFO (the 128-bit numbers of ReFS), or the 64-bit file
// index where the file system lacks it (e.g. FAT).
func fileIdentityOf(path string) (fileIdentity, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fileIdentity{}, false
	}
	// No access is needed to read the identity, and sharing everything keeps
	// it readable while other programs have the file open.
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return fileIdentity{}, false
	}
	defer windows.CloseHandle(h)

	// FILE_ID_INFO: a 64-bit volume serial number and a 128-bit file id.
	var info [24]byte
	if windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, &info[0], uint32(len(info))) == nil {
		return fileIdentity{device: binary.LittleEndian.Uint64(info[0:8]),
			fileLow: binary.LittleEndian.Uint64(info[8:16]), fileHigh: binary.LittleEndian.Uint64(info[16:24])}, true
	}
	var byHandle windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &byHandle) != nil {
		return fileIdentity{}, false
	}
	return fileIdentity{device: uint64(byHandle.VolumeSerialNumber),
		fileLow: uint64(byHandle.FileIndexHigh)<<32 | uint64(byHandle.FileIndexLow)}, true
}

// isSpecialFile is UnixFileType.IsSpecialFile: always false on Windows.
func isSpecialFile(string) bool { return false }
