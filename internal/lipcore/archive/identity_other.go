//go:build !windows

package archive

import (
	"os"
	"syscall"
)

// fileIdentityOf is FileIdentity.OfUnix: the device and inode number of the
// file, following links.
func fileIdentityOf(path string) (fileIdentity, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fileIdentity{}, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}, false
	}
	return fileIdentity{device: uint64(st.Dev), fileLow: uint64(st.Ino)}, true
}

// isSpecialFile is UnixFileType.IsSpecialFile: whether the file at path, not
// following links, exists and is neither a regular file nor a directory nor
// a link (e.g. a FIFO, socket or device node). False when the type cannot be
// read, so that reading the file reports why.
func isSpecialFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	m := info.Mode()
	return !m.IsRegular() && !m.IsDir() && m&os.ModeSymlink == 0
}
