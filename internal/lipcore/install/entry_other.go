//go:build !windows

package install

import "os"

// entryKind is how DirectoryInfo.EnumerateFileSystemInfos reports an entry
// on Unix: a symbolic link is a link (FileAttributes.ReparsePoint), and a
// DirectoryInfo when its target is a directory.
func entryKind(path string) (isDir, isLink bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Stat(path)
		return err == nil && target.IsDir(), true, nil
	}
	return info.IsDir(), false, nil
}
