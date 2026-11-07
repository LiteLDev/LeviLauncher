//go:build !windows

package archive

import "io/fs"

// isReparsePoint reports whether an entry is a symbolic link, which .NET
// reports as FileAttributes.ReparsePoint on Unix.
func isReparsePoint(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0
}
