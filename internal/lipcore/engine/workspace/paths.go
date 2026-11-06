package workspace

import (
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file ports the parts of .NET's System.IO.Path that lip's placement
// and removal logic depends on. Platform-specific pieces (GetFullPath and
// file system probes) live in paths_windows.go and paths_other.go.

const isWindows = runtime.GOOS == "windows"

// pathIgnoreCase mirrors PathInternal.StringComparison: paths compare
// case-insensitively on Windows and macOS.
const pathIgnoreCase = runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios"

func isDirSep(c rune) bool {
	if isWindows {
		return c == '\\' || c == '/'
	}
	return c == '/'
}

func dirSep() string {
	if isWindows {
		return `\`
	}
	return "/"
}

// isPathRooted ports Path.IsPathRooted.
func isPathRooted(p string) bool {
	if isWindows {
		return (len(p) >= 1 && isDirSep(rune(p[0]))) ||
			(len(p) >= 2 && isDriveLetter(p[0]) && p[1] == ':')
	}
	return strings.HasPrefix(p, "/")
}

func isDriveLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// pathCombine ports Path.Combine(first, second).
func pathCombine(first, second string) string {
	if first == "" {
		return second
	}
	if second == "" {
		return first
	}
	if isPathRooted(second) {
		return second
	}
	if isDirSep(rune(first[len(first)-1])) || isDirSep(rune(second[0])) {
		return first + second
	}
	return first + dirSep() + second
}

// pathGetFileName ports Path.GetFileName.
func pathGetFileName(p string) string {
	root := pathRootLength(p)
	i := -1
	if isWindows {
		i = strings.LastIndexAny(p, `\/`)
	} else {
		i = strings.LastIndexByte(p, '/')
	}
	if i < root {
		return p[root:]
	}
	return p[i+1:]
}

// pathRootLength ports PathInternal.GetRootLength for the forms lip can
// encounter (drive roots, drive-relative, rooted, UNC and device paths).
func pathRootLength(p string) int {
	if !isWindows {
		if strings.HasPrefix(p, "/") {
			return 1
		}
		return 0
	}
	n := len(p)
	sep := func(i int) bool { return i < n && (p[i] == '\\' || p[i] == '/') }
	switch {
	case n >= 2 && sep(0) && sep(1):
		// UNC (\\server\share) or device (\\?\, \\.\) path.
		i := 2
		if n >= 4 && (p[2] == '?' || p[2] == '.') && sep(3) {
			i = 4
			if n >= 8 && strings.EqualFold(p[4:8], `UNC\`) {
				i = 8
			} else {
				// Device path: the root is the first segment.
				for i < n && !sep(i) {
					i++
				}
				if i < n {
					i++
				}
				return i
			}
		}
		// Skip server and share.
		for k := 0; k < 2; k++ {
			for i < n && !sep(i) {
				i++
			}
			if k == 0 && i < n {
				i++
			}
		}
		return i
	case n >= 1 && sep(0):
		return 1
	case n >= 2 && isDriveLetter(p[0]) && p[1] == ':':
		if sep(2) {
			return 3
		}
		return 2
	}
	return 0
}

// getRelativePath ports Path.GetRelativePath(relativeTo, path), resolving
// relative inputs against base (the process working directory in lip).
func getRelativePath(base, relativeTo, path string) string {
	return relativePathBetween(getFullPath(base, relativeTo), getFullPath(base, path))
}

// relativePathBetween is Path.GetRelativePath for already full paths.
func relativePathBetween(relativeTo, path string) string {
	if !areRootsEqual(relativeTo, path) {
		return path
	}

	rt := []rune(relativeTo)
	pt := []rune(path)

	commonLength := commonPathLength(rt, pt, pathIgnoreCase)
	if commonLength == 0 {
		return path
	}

	relativeToLength := len(rt)
	if endsInDirSep(rt) {
		relativeToLength--
	}
	pathEndsInSeparator := endsInDirSep(pt)
	pathLength := len(pt)
	if pathEndsInSeparator {
		pathLength--
	}

	if relativeToLength == pathLength && commonLength >= relativeToLength {
		return "."
	}

	var sb strings.Builder
	if commonLength < relativeToLength {
		sb.WriteString("..")
		for i := commonLength + 1; i < relativeToLength; i++ {
			if isDirSep(rt[i]) {
				sb.WriteString(dirSep())
				sb.WriteString("..")
			}
		}
	} else if isDirSep(pt[commonLength]) {
		// No parent segments and we need to eat the initial separator.
		commonLength++
	}

	differenceLength := pathLength - commonLength
	if pathEndsInSeparator {
		differenceLength++
	}
	if differenceLength > 0 {
		if sb.Len() > 0 {
			sb.WriteString(dirSep())
		}
		sb.WriteString(string(pt[commonLength : commonLength+differenceLength]))
	}
	return sb.String()
}

func endsInDirSep(p []rune) bool {
	return len(p) > 0 && isDirSep(p[len(p)-1])
}

func areRootsEqual(first, second string) bool {
	a := first[:pathRootLength(first)]
	b := second[:pathRootLength(second)]
	if pathIgnoreCase {
		return equalFoldOrdinal(a, b)
	}
	return a == b
}

// commonPathLength ports PathInternal.GetCommonPathLength.
func commonPathLength(first, second []rune, ignoreCase bool) int {
	commonChars := 0
	for commonChars < len(first) && commonChars < len(second) {
		a, b := first[commonChars], second[commonChars]
		if a != b && !(ignoreCase && unicode.ToUpper(a) == unicode.ToUpper(b)) {
			break
		}
		commonChars++
	}

	if commonChars == 0 {
		return 0
	}
	if commonChars == len(first) && (commonChars == len(second) || isDirSep(second[commonChars])) {
		return commonChars
	}
	if commonChars == len(second) && isDirSep(first[commonChars]) {
		return commonChars
	}
	for commonChars > 0 && !isDirSep(first[commonChars-1]) {
		commonChars--
	}
	return commonChars
}

// cutPrefixFoldOrdinal is strings.CutPrefix comparing like
// StringComparison.OrdinalIgnoreCase.
func cutPrefixFoldOrdinal(s, prefix string) (string, bool) {
	i := 0
	for _, pr := range prefix {
		if i >= len(s) {
			return s, false
		}
		sr, size := utf8.DecodeRuneInString(s[i:])
		if sr != pr && unicode.ToUpper(sr) != unicode.ToUpper(pr) {
			return s, false
		}
		i += size
	}
	return s[i:], true
}

// equalFoldOrdinal approximates StringComparison.OrdinalIgnoreCase
// (per-character upper-case comparison).
func equalFoldOrdinal(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) {
		return false
	}
	for i := range ra {
		if ra[i] != rb[i] && unicode.ToUpper(ra[i]) != unicode.ToUpper(rb[i]) {
			return false
		}
	}
	return true
}
