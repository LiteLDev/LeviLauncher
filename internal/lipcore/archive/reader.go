package archive

import (
	"io"
	"slices"
	"strings"
)

const (
	// PackageFileName is the package file at the root of an archive
	// (protocol/package.md §1).
	PackageFileName = "lip.toml"
	// ToothFileName is the legacy package file (protocol/compatibility.md §2).
	ToothFileName = "tooth.json"
)

// NotPackageError is an archive without lip.toml or tooth.json at its root
// (ArchiveReader.cs). It is an *UnavailableError through errors.As.
type NotPackageError struct {
	// Archive names the archive as the user knows it: the path of a local
	// file or directory, else its key.
	Archive string
	// Nested is a package file exactly one folder deep, as in GitHub's
	// "Download ZIP" archives, or "".
	Nested string
}

func (e *NotPackageError) Error() string {
	archive := e.Archive
	if archive == "" {
		archive = "The archive"
	}
	msg := archive + " is not a lip package: it has no " + PackageFileName + " at its root."
	if e.Nested != "" {
		msg += " There is one at '" + e.Nested + "': the files of a package must be at the root of" +
			" the archive, not in a folder."
	}
	return msg
}

// As lets errors.As find an *UnavailableError.
func (e *NotPackageError) As(target any) bool {
	if t, ok := target.(**UnavailableError); ok {
		*t = &UnavailableError{Message: e.Error()}
		return true
	}
	return false
}

// ReadPackageFile reads the file that makes an archive a package
// (protocol/package.md §1): lip.toml at its root, or, when there is none,
// tooth.json (protocol/compatibility.md §2). The archive is called "The
// archive" in errors; ReadPackageFileOf names it ("" is "the archive").
func ReadPackageFile(src Source) (name string, data []byte, err error) {
	return ReadPackageFileOf(src, "")
}

// ReadPackageFileOf is ReadPackageFile naming the archive archiveName in
// errors (ArchiveReader.Describe: the path of a local file or directory, else
// the key of the archive). It fails with *NotPackageError when the archive
// has neither file, or *UnavailableError when the file cannot be read.
// Parsing the file is left to the caller.
func ReadPackageFileOf(src Source, archiveName string) (name string, data []byte, err error) {
	keys, err := ListKeys(src)
	if err != nil {
		return "", nil, err
	}
	switch {
	case slices.Contains(keys, PackageFileName):
		name = PackageFileName
	case slices.Contains(keys, ToothFileName):
		name = ToothFileName
	default:
		// Only a package file one folder deep suggests the whole package is in
		// a folder; deeper ones are usually examples or vendored packages.
		nested := ""
		for _, k := range keys {
			if (strings.HasSuffix(k, "/"+PackageFileName) || strings.HasSuffix(k, "/"+ToothFileName)) &&
				strings.Count(k, "/") == 1 {
				nested = k
				break
			}
		}
		return "", nil, &NotPackageError{Archive: archiveName, Nested: nested}
	}

	rc, err := src.Open(name)
	if err == nil {
		data, err = io.ReadAll(rc)
		rc.Close()
	}
	if err != nil {
		switch err.(type) {
		case *UnavailableError, *NotPackageError:
			return "", nil, err // as it is, like lip's ArchiveUnavailableException
		}
		of := archiveName
		if of == "" {
			of = "the archive"
		}
		return "", nil, &UnavailableError{Message: "Could not read the " + name + " of " + of + ".", Err: err}
	}
	return name, data, nil
}
