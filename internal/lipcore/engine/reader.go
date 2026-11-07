package engine

import (
	"errors"

	"github.com/liteldev/LeviLauncher/internal/lipcore/archive"
	"github.com/liteldev/LeviLauncher/internal/lipcore/compat"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgfile"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Ports PackageDocument and ArchiveReader of src/Lip.Core/Mcmod/ArchiveReader.cs
// (v0.38.0-beta.1).

// packageDocument is what makes an archive a package (protocol/package.md
// §1): its package file (file), or, for an archive without one, its
// tooth.json read as format 3 (legacy, protocol/compatibility.md §2).
type packageDocument struct {
	file    *pkgmodel.Document
	legacy  *compat.Tooth
	version semver.Version // of file; legacy.Version for a tooth.json
}

// id is the package id.
func (d *packageDocument) id() string {
	if d.file != nil {
		return d.file.ID
	}
	return d.legacy.ID
}

// isLegacy reports whether the package is read from a tooth.json.
func (d *packageDocument) isLegacy() bool { return d.file == nil }

// fileName is the file the package is read from.
func (d *packageDocument) fileName() string {
	if d.isLegacy() {
		return compat.ToothFileName
	}
	return pkgmodel.PackageFileName
}

// readFor is ReadFor: the package file the package is read as for an
// environment, with version, when given, as the version of the package (its
// tag version). A *compat.FormatError is a tooth.json that cannot be read so.
func (d *packageDocument) readFor(platform, runtime string, version *semver.Version) (*pkgmodel.Document, error) {
	if d.file != nil {
		return d.file, nil
	}
	return d.legacy.Derive(platform, runtime, version)
}

// readArchive is ArchiveReader.Read. Every failure to read the archive as a
// package is an ArchiveUnavailableException (*archive.UnavailableError or
// *archive.NotPackageError).
func readArchive(a *archiveHandle) (*packageDocument, error) {
	name := describeArchive(a.Key)
	fileName, data, err := archive.ReadPackageFileOf(a.Source, name)
	if err != nil {
		// Cancellation is not a failure of the archive (ArchiveReader lets
		// OperationCanceledException through).
		if c := contextErrorOf(err); c != nil {
			return nil, c
		}
		return nil, err
	}

	var d *packageDocument
	if fileName == pkgmodel.PackageFileName {
		var doc *pkgmodel.Document
		doc, err = pkgfile.ParseYAML(data)
		if err == nil {
			var v semver.Version
			if v, err = semver.ParseStrict(doc.Version); err == nil {
				d = &packageDocument{file: doc, version: v}
			}
		}
	} else {
		var t *compat.Tooth
		if t, err = compat.ParseToothJSON(data); err == nil {
			d = &packageDocument{legacy: t, version: t.Version}
		}
	}
	if err != nil {
		if isContextError(err) {
			return nil, err
		}
		local := hasFilePrefix(a.Key)
		advice := " Ask the package author to fix it."
		var fe *pkgfile.FormatError
		switch {
		case errors.As(err, &fe) && fe.Newer:
			advice = ""
		case local:
			advice = " Fix the " + fileName + " in " + name + "."
		}
		return nil, &archive.UnavailableError{
			Message: "The " + fileName + " of " + name + " is not valid." + advice, Err: err}
	}
	return d, nil
}

// describeArchive is ArchiveReader.Describe: the archive as the user knows
// it, the path of a local file or directory, else its key.
func describeArchive(key string) string {
	if hasFilePrefix(key) {
		if p, err := (ref.Ref{Kind: ref.KindFile, Target: key}).LocalPath(); err == nil {
			return p
		}
	}
	return key
}

func hasFilePrefix(s string) bool {
	return len(s) >= 5 && ref.Fold(s[:5]) == "file:"
}
