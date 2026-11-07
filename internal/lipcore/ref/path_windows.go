package ref

import (
	"strings"
	"syscall"
)

// unixPaths is whether local paths are Unix paths ("/x" is an implicit file
// path; "C:\x" is not).
const unixPaths = false

func isDirSep(c byte) bool { return c == '\\' || c == '/' }

// isPathRooted is Path.IsPathRooted on Windows.
func isPathRooted(p string) bool {
	return len(p) >= 1 && isDirSep(p[0]) || len(p) >= 2 && isASCIILetter(p[0]) && p[1] == ':'
}

// combinePath is Path.Combine(a, b), which adds a separator unless a ends
// with one (not after a bare drive "C:" either).
func combinePath(a, b string) string {
	switch {
	case b == "":
		return a
	case a == "" || isPathRooted(b):
		return b
	case isDirSep(a[len(a)-1]):
		return a + b
	}
	return a + `\` + b
}

// fullPath is Path.GetFullPath on Windows, which calls GetFullPathNameW (and
// leaves "\\?\" paths alone).
func fullPath(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", errNullChar
	}
	if strings.HasPrefix(p, `\\?\`) {
		return p, nil
	}
	return syscall.FullPath(p)
}

// rootLength is PathInternal.GetRootLength on Windows for a full path.
func rootLength(p string) int {
	switch {
	case len(p) >= 2 && isASCIILetter(p[0]) && p[1] == ':':
		if len(p) >= 3 && isDirSep(p[2]) {
			return 3
		}
		return 2
	case len(p) >= 2 && isDirSep(p[0]) && isDirSep(p[1]):
		// \\server\share\ or a device path \\?\ or \\.\
		i := 2
		if len(p) >= 4 && (p[2] == '?' || p[2] == '.') && isDirSep(p[3]) {
			i = 4
			if len(p) >= 8 && strings.EqualFold(p[4:8], `UNC\`) {
				i = 8
			} else {
				// \\?\C:\ and the like: up to the next separator, inclusive.
				for i < len(p) && !isDirSep(p[i]) {
					i++
				}
				if i < len(p) {
					i++
				}
				return i
			}
		}
		// server
		for i < len(p) && !isDirSep(p[i]) {
			i++
		}
		if i < len(p) {
			i++
		}
		// share
		for i < len(p) && !isDirSep(p[i]) {
			i++
		}
		return i
	case len(p) >= 1 && isDirSep(p[0]):
		return 1
	}
	return 0
}
