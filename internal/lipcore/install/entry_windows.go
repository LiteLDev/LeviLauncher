//go:build windows

package install

import (
	"os"
	"syscall"
)

// entryKind is how DirectoryInfo.EnumerateFileSystemInfos reports an entry
// on Windows: a DirectoryInfo when it has FILE_ATTRIBUTE_DIRECTORY (a
// directory link or junction included), a link when it has
// FILE_ATTRIBUTE_REPARSE_POINT. Links are not followed.
func entryKind(path string) (isDir, isLink bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, false, err
	}
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0,
			d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
	}
	return info.IsDir(), info.Mode()&os.ModeSymlink != 0, nil
}
