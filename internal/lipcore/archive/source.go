// Package archive holds the file trees lip installs packages and assets from
// (protocol/package.md §1, §2.3): ZIP archives validated by content
// (ZipValidator.cs, ZipSource.cs), directory archives (Sources/DirectorySource.cs),
// mounted and deferred sources (MountSource.cs, DeferredSource.cs), the
// package file of an archive (ArchiveReader.cs) and lip pack (Packer.cs), as
// in lip v0.38.0-beta.1.
package archive

import (
	"errors"
	"io"
)

// Source is a file tree (lip's ISource). Keys are '/'-separated relative file
// paths, in the order of the source: ordinal for a directory, entry order for
// a ZIP archive, mount order for a mount.
type Source interface {
	Keys() []string
	Open(key string) (io.ReadCloser, error)
	// Size is the size of a file in bytes, when the source knows it.
	Size(key string) (int64, bool)
}

// UnavailableError is lip's ArchiveUnavailableException: an archive that
// cannot be obtained or is invalid (protocol/package.md §2). Error() is only
// the message; the cause stays reachable through errors.Unwrap.
type UnavailableError struct {
	Message string
	Err     error
}

func (e *UnavailableError) Error() string { return e.Message }
func (e *UnavailableError) Unwrap() error { return e.Err }

// DamagedError is lip's InvalidDataException: an archive that is not a valid
// ZIP archive or an entry that cannot be decompressed.
type DamagedError struct {
	Message string
	Err     error
}

func (e *DamagedError) Error() string { return e.Message }
func (e *DamagedError) Unwrap() error { return e.Err }

// FileError is a file system failure with a message saying what lip was
// doing (FileSystemExtensions.WrapFileError); the cause is kept.
type FileError struct {
	Message string
	Err     error
}

func (e *FileError) Error() string { return e.Message }
func (e *FileError) Unwrap() error { return e.Err }

// ErrNoFile is matched (errors.Is) by the error of opening a key a source
// does not have (lip's ArgumentException).
var ErrNoFile = errors.New("archive: no such file")

type noFileError struct{ message string }

func (e *noFileError) Error() string        { return e.message }
func (e *noFileError) Is(target error) bool { return target == ErrNoFile }

func capitalize(text string) string {
	if text == "" {
		return text
	}
	// char.ToUpperInvariant of the first UTF-16 unit; descriptions start
	// with ASCII.
	if c := text[0]; 'a' <= c && c <= 'z' {
		return string(c-'a'+'A') + text[1:]
	}
	return text
}
