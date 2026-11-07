//go:build windows

package txn

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// createNewShareRead opens a new file for writing that other processes may
// only read (FileMode.CreateNew, FileAccess.Write, FileShare.Read).
func createNewShareRead(name string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(longPath(name))
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ, nil,
		syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(h), name), nil
}

func attributes(name string) (uint32, *uint16, error) {
	p, err := syscall.UTF16PtrFromString(longPath(name))
	if err != nil {
		return 0, nil, &os.PathError{Op: "getattributes", Path: name, Err: err}
	}
	a, err := syscall.GetFileAttributes(p)
	if err != nil {
		return 0, nil, &os.PathError{Op: "getattributes", Path: name, Err: err}
	}
	return a, p, nil
}

func isHidden(name string) (bool, error) {
	a, _, err := attributes(name)
	if err != nil {
		return false, err
	}
	return a&syscall.FILE_ATTRIBUTE_HIDDEN != 0, nil
}

// setHidden is FileSystemExtensions.SetHidden: attributes that end up empty
// are written as FILE_ATTRIBUTE_NORMAL.
func setHidden(name string, hidden bool) error {
	a, p, err := attributes(name)
	if err != nil {
		return err
	}
	updated := a &^ syscall.FILE_ATTRIBUTE_HIDDEN
	if hidden {
		updated = a | syscall.FILE_ATTRIBUTE_HIDDEN
	}
	if updated == a {
		return nil
	}
	if updated == 0 {
		updated = syscall.FILE_ATTRIBUTE_NORMAL
	}
	if err := syscall.SetFileAttributes(p, updated); err != nil {
		return &os.PathError{Op: "setattributes", Path: name, Err: err}
	}
	return nil
}

// clearReadOnly is WorkspaceTransaction.ClearReadOnly: File.SetAttributes
// with the read-only flag removed (0 becomes FILE_ATTRIBUTE_NORMAL, as .NET
// does).
func clearReadOnly(name string) error {
	a, p, err := attributes(name)
	if err != nil {
		return err
	}
	if a&syscall.FILE_ATTRIBUTE_READONLY == 0 {
		return nil
	}
	updated := a &^ syscall.FILE_ATTRIBUTE_READONLY
	if updated == 0 {
		updated = syscall.FILE_ATTRIBUTE_NORMAL
	}
	if err := syscall.SetFileAttributes(p, updated); err != nil {
		return &os.PathError{Op: "setattributes", Path: name, Err: err}
	}
	return nil
}

// longPath prefixes a long absolute path with \\?\ so that the Win32 calls
// made here accept it, as the os package and .NET do.
func longPath(name string) string {
	if len(name) < 248 || strings.HasPrefix(name, `\\?\`) || !filepath.IsAbs(name) {
		return name
	}
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, `\\`) {
		return `\\?\UNC\` + clean[2:]
	}
	return `\\?\` + clean
}
