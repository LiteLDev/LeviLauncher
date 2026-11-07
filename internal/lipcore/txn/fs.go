package txn

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// FS is the file system a Transaction, Recover and the Mutex helpers work on:
// the operating system by default (OS), or a wrapper that injects failures in
// tests (the seam of FaultyFileSystem.cs). Paths are absolute.
type FS interface {
	// Stat follows links, like Directory.Exists and File.Exists.
	Stat(name string) (fs.FileInfo, error)
	// Lstat does not follow links.
	Lstat(name string) (fs.FileInfo, error)
	// MkdirAll is Directory.CreateDirectory.
	MkdirAll(name string) error
	// Create creates or truncates a file for writing (File.Create).
	Create(name string) (io.WriteCloser, error)
	// CreateNew creates a file that must not exist, for writing, shared for
	// reading only (FileMode.CreateNew, FileShare.Read).
	CreateNew(name string) (io.WriteCloser, error)
	// Rename moves a file, replacing the destination (File.Move with
	// overwrite; lip only renames onto a missing destination, except for
	// files it replaces on purpose).
	Rename(oldname, newname string) error
	// Remove deletes a file or an empty directory.
	Remove(name string) error
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte) error
	// ReadDirNames lists at most n entries of a directory (n <= 0: all).
	ReadDirNames(name string, n int) ([]string, error)
	// Hidden reports File.GetAttributes(name).HasFlag(FileAttributes.Hidden):
	// the hidden attribute on Windows, a name starting with '.' elsewhere.
	Hidden(name string) (bool, error)
	// SetHidden sets or clears the hidden attribute on Windows; elsewhere it
	// does nothing (FileSystemExtensions.SetHidden).
	SetHidden(name string, hidden bool) error
	// ClearReadOnly clears the read-only attribute of a file, if set.
	ClearReadOnly(name string) error
}

// OS is the file system of the operating system.
var OS FS = osFS{}

type osFS struct{}

func (osFS) Stat(name string) (fs.FileInfo, error)  { return os.Stat(name) }
func (osFS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (osFS) MkdirAll(name string) error             { return os.MkdirAll(name, 0o777) }
func (osFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}
func (osFS) CreateNew(name string) (io.WriteCloser, error) { return createNewShareRead(name) }
func (osFS) Rename(oldname, newname string) error          { return os.Rename(oldname, newname) }
func (osFS) Remove(name string) error                      { return os.Remove(name) }
func (osFS) ReadFile(name string) ([]byte, error)          { return os.ReadFile(name) }
func (osFS) WriteFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0o666)
}
func (osFS) ReadDirNames(name string, n int) ([]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(n)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return names, err
}
func (osFS) Hidden(name string) (bool, error)         { return isHidden(name) }
func (osFS) SetHidden(name string, hidden bool) error { return setHidden(name, hidden) }
func (osFS) ClearReadOnly(name string) error          { return clearReadOnly(name) }

// fileExists is File.Exists: an existing path that is not a directory.
func fileExists(fsys FS, path string) bool {
	info, err := fsys.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists is Directory.Exists.
func dirExists(fsys FS, path string) bool {
	info, err := fsys.Stat(path)
	return err == nil && info.IsDir()
}

// dirEmpty is !Directory.EnumerateFileSystemEntries(path).Any().
func dirEmpty(fsys FS, path string) (bool, error) {
	names, err := fsys.ReadDirNames(path, 1)
	if err != nil {
		return false, err
	}
	return len(names) == 0, nil
}

// isLink is DirectoryInfo.LinkTarget != null: a symbolic link, or on
// Windows a junction (reported by Go as an irregular file).
func isLink(fsys FS, path string) (bool, error) {
	info, err := fsys.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0, nil
}
