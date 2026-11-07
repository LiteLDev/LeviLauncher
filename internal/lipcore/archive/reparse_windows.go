//go:build windows

package archive

import (
	"io/fs"
	"syscall"
)

// isReparsePoint reports whether an entry has FILE_ATTRIBUTE_REPARSE_POINT
// (FileAttributes.ReparsePoint): a symbolic link, a junction or another
// reparse point, which lip skips rather than follows.
func isReparsePoint(info fs.FileInfo) bool {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return info.Mode()&fs.ModeSymlink != 0
}
