package install

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
)

// Error is McmodInstallException: installing or uninstalling a package
// failed (protocol/workspace.md §4). Error() is lip's message; the cause, if
// any, is reachable through errors.Unwrap. A failure detected while planning
// leaves the workspace untouched.
type Error struct {
	Msg string
	Err error
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Err }

// isCancel reports whether err is a cancellation (OperationCanceledException),
// which lip never wraps.
func isCancel(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

var onWindows = runtime.GOOS == "windows"

// pathKey is a path as WorkspaceLayout.PathComparer compares it: ignoring
// case on Windows, ordinal elsewhere (as txn compares paths).
func pathKey(p string) string {
	if onWindows {
		return strings.ToUpper(p)
	}
	return p
}

func pathsEqual(a, b string) bool { return pathKey(a) == pathKey(b) }

func hasPathPrefix(p, prefix string) bool {
	return len(p) >= len(prefix) && pathsEqual(p[:len(prefix)], prefix)
}

// IsModsDirectory is WorkspaceLayout.IsModsDirectory: whether a
// workspace-relative path is mods itself, which lip never deletes.
func IsModsDirectory(path string) bool {
	return pathsEqual(strings.TrimRight(path, "/"), pkgmodel.ModsDir)
}

// IsLipPath is WorkspaceLayout.IsLipPath: whether a workspace-relative path
// is .lip or below it, where lip keeps its own files: packages never write
// there, and purge never matches it.
func IsLipPath(path string) bool {
	return pathsEqual(strings.TrimRight(path, "/"), pkgmodel.LipDir) ||
		hasPathPrefix(path, pkgmodel.LipDir+"/")
}

// fullPath is McmodInstaller.ToFullPath: the absolute path of a
// workspace-relative path ("" or "/"-only is the root). Path.GetFullPath
// (GetFullPathName on Windows, which filepath.Abs also calls) resolves it.
func fullPath(root, relative string) string {
	relative = strings.TrimRight(relative, "/")
	if relative == "" {
		return root
	}
	p := filepath.Join(root, filepath.FromSlash(relative))
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// trimEndingSeparator is Path.TrimEndingDirectorySeparator.
func trimEndingSeparator(p string) string {
	if len(p) > 1 && isSeparator(p[len(p)-1]) && len(p) > len(filepath.VolumeName(p))+1 {
		return p[:len(p)-1]
	}
	return p
}

func isSeparator(c byte) bool { return c == '/' || c == filepath.Separator }

// isInside is McmodInstaller.IsInside: whether path is strictly below
// directory.
func isInside(directory, path string) bool {
	return hasPathPrefix(path, trimEndingSeparator(directory)+string(filepath.Separator))
}

// parentDir is Path.GetDirectoryName of a full path; false at a root.
func parentDir(p string) (string, bool) {
	d := filepath.Dir(p)
	if d == p || d == "." || d == "" {
		return "", false
	}
	return d, true
}

// relativePath is Path.GetRelativePath(root, path), '/'-separated.
func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return strings.ReplaceAll(rel, string(filepath.Separator), "/")
}

// show is WorkspaceErrors.Show: a path relative to the workspace root and
// '/'-separated when it is in the workspace, else as it is.
func show(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return path
	}
	return strings.ReplaceAll(rel, `\`, "/")
}

// normalizeRelative is McmodInstaller.NormalizeRelative: a recorded path,
// "a/./b" being "a/b" as when it was recorded, '\' read as '/'.
func normalizeRelative(path string) string {
	segments := strings.Split(strings.ReplaceAll(path, `\`, "/"), "/")
	kept := segments[:0]
	for _, s := range segments {
		if s != "." {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "/")
}

// isSafeRelativePath is McmodInstaller.IsSafeRelativePath: a non-empty
// '/'-separated relative path as lip.schema.toml #/definitions/path allows
// it: without empty or ".." segments, backslashes, or a leading ASCII letter
// and ':'.
func isSafeRelativePath(path string) bool {
	if path == "" || strings.Contains(path, `\`) || startsWithDrive(path) {
		return false
	}
	for _, s := range strings.Split(path, "/") {
		if s == "" || s == ".." {
			return false
		}
	}
	return true
}

// startsWithDrive reports whether a path starts with an ASCII letter and
// ':', a drive on Windows.
func startsWithDrive(path string) bool {
	return len(path) >= 2 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) &&
		path[1] == ':'
}

// aliasesOnWindows reports whether, on Windows, a '/'-separated path has a
// name that ends with '.' or a space, or contains ':': Windows drops
// trailing dots and spaces, which would alias another path, and takes ':'
// for an alternate data stream. Elsewhere it is false.
func aliasesOnWindows(path string) bool {
	if !onWindows {
		return false
	}
	for _, s := range strings.Split(strings.TrimRight(path, "/"), "/") {
		if strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") || strings.Contains(s, ":") {
			return true
		}
	}
	return false
}
