// Package fsx writes files the way internal/apppath/README.md requires:
// through a staging entry named "<final>.<random>.tmp" next to the target,
// renamed into place, so that no reader sees a partial file.
package fsx

import (
	"os"
	"path/filepath"
	"strings"
)

// TempPattern is the os.CreateTemp / os.MkdirTemp pattern of a staging file
// or directory for final: "<final>.<random>.tmp" (rule N5). The staging
// entry belongs next to its target, on the same volume, so that it can be
// renamed into place.
func TempPattern(final string) string {
	return filepath.Base(final) + ".*.tmp"
}

// IsTempName reports whether name is a staging entry by TempPattern.
func IsTempName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".tmp")
}

// CreateTemp creates a staging file for final in dir. It is not hidden: the
// attribute would stay with the file once it is renamed into place.
func CreateTemp(dir, final string) (*os.File, error) {
	return os.CreateTemp(dir, TempPattern(final))
}

// MkdirTemp creates a staging directory for final in dir.
func MkdirTemp(dir, final string) (string, error) {
	return os.MkdirTemp(dir, TempPattern(final))
}

// WriteFileAtomic writes data to path through a staging file in the same
// directory, creating the directory, so a reader sees the old file or the new
// one.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := CreateTemp(dir, path)
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp, perm)
	}
	if werr == nil {
		werr = os.Rename(tmp, path)
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	return nil
}

// PartialPath is the deterministic staging file of a download to final that
// a later attempt resumes: "<final>.partial.tmp". Only one writer at a time
// may use it.
func PartialPath(final string) string {
	return final + ".partial.tmp"
}
