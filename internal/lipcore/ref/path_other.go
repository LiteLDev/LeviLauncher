//go:build !windows

package ref

import (
	"path/filepath"
	"strings"
)

// unixPaths is whether local paths are Unix paths. Off Windows, which
// LeviLauncher does not ship for, the System.Uri emulation also takes "/x"
// as an implicit file path and keeps the '/' of a local path, but is
// otherwise that of Windows and not checked against .NET on Unix.
const unixPaths = true

func isDirSep(c byte) bool { return c == '/' }

// isPathRooted is Path.IsPathRooted on Unix.
func isPathRooted(p string) bool { return strings.HasPrefix(p, "/") }

// combinePath is Path.Combine(a, b).
func combinePath(a, b string) string {
	switch {
	case b == "":
		return a
	case a == "" || isPathRooted(b):
		return b
	case isDirSep(a[len(a)-1]):
		return a + b
	}
	return a + "/" + b
}

// fullPath is Path.GetFullPath on Unix: the path made absolute against the
// working directory, with "." and ".." segments and repeated separators
// removed, keeping a trailing separator.
func fullPath(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", errNullChar
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(p, "/") && abs != "/" {
		abs += "/"
	}
	return abs, nil
}

// rootLength is PathInternal.GetRootLength on Unix.
func rootLength(p string) int {
	if strings.HasPrefix(p, "/") {
		return 1
	}
	return 0
}
