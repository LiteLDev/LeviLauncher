//go:build !windows

package txn

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func createNewShareRead(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
}

// isHidden is FileAttributes.Hidden as .NET reports it outside Windows: the
// name starts with '.'.
func isHidden(name string) (bool, error) {
	if _, err := os.Lstat(name); err != nil {
		return false, err
	}
	return strings.HasPrefix(filepath.Base(name), "."), nil
}

// setHidden does nothing outside Windows, where the name hides a file.
func setHidden(string, bool) error { return nil }

// clearReadOnly gives the owner write permission when it lacks it, as .NET
// does for File.SetAttributes without FileAttributes.ReadOnly.
func clearReadOnly(name string) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&syscall.S_IWUSR != 0 {
		return nil
	}
	return os.Chmod(name, info.Mode().Perm()|syscall.S_IWUSR)
}
