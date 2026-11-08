package install

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// absRoot is Path.GetFullPath of a workspace root.
func absRoot(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

// longName is McmodInstaller.LongName: a workspace-relative path with each
// 8.3 short name in it replaced by the long name it stands for, on Windows:
// "LIP~1/state.json" is ".lip/state.json" when ".lip" has the short name
// "LIP~1". Only names that exist can be short names, and every short name
// contains '~', so other paths are returned as they are without asking the
// file system, and so are paths that are not safe workspace paths. False
// when a name containing '~' exists but its long name cannot be read: such a
// path cannot be checked, so it fails the checks.
//
// Elsewhere, on a file system that ignores case, each name in the path that
// exists is spelled as the file system has it (spelled): ".LIP/lock" is
// ".lip/lock" when ".lip" exists.
func (in *Installer) longName(root, relative string) (string, bool) {
	if !isSafeRelativePath(strings.TrimRight(relative, "/")) {
		return relative, true
	}
	if !onWindows {
		if in.ignoresCase(root) {
			return in.spelled(root, relative)
		}
		return relative, true
	}
	if !strings.Contains(relative, "~") {
		return relative, true
	}

	// The longest leading part of the path that exists.
	segments := strings.Split(strings.TrimRight(relative, "/"), "/")
	existing := len(segments)
	full := fullPath(root, relative)
	for existing > 0 && !fileExists(full) && !dirExists(full) {
		existing--
		full = fullPath(root, strings.Join(segments[:existing], "/"))
	}
	short := false
	for _, s := range segments[:existing] {
		if strings.Contains(s, "~") {
			short = true
			break
		}
	}
	if !short {
		return relative, true
	}
	longRoot, ok := getLongPath(root)
	if !ok {
		return "", false
	}
	longPath, ok := getLongPath(full)
	if !ok {
		return "", false
	}

	rel, err := filepath.Rel(longRoot, longPath)
	if err != nil {
		return "", false
	}
	head := strings.ReplaceAll(rel, string(filepath.Separator), "/")
	if head == "." || head == ".." || strings.HasPrefix(head, "../") || filepath.IsAbs(head) ||
		filepath.VolumeName(head) != "" {
		return "", false
	}
	result := strings.Join(append([]string{head}, segments[existing:]...), "/")
	if strings.HasSuffix(relative, "/") {
		result += "/"
	}
	return result, true
}

// spelled is McmodInstaller.Spelled: a workspace-relative path with each name
// in it that exists spelled as the file system has it, on a file system that
// ignores case: "MODS/a.dll" is "mods/a.dll" when "mods" exists. False when a
// directory on the way cannot be listed: such a path cannot be checked.
func (in *Installer) spelled(root, relative string) (string, bool) {
	segments := strings.Split(strings.TrimRight(relative, "/"), "/")
	directory := root
	for i, segment := range segments {
		if !dirExists(directory) {
			break
		}
		names, err := listNames(directory)
		if err != nil {
			return "", false
		}
		name, found := "", false
		for _, n := range names {
			if n == segment {
				name, found = n, true
				break
			}
		}
		if !found {
			for _, n := range names {
				if equalOrdinalIgnoreCase(norm.NFC.String(n), norm.NFC.String(segment)) {
					name, found = n, true
					break
				}
			}
		}
		if !found {
			break
		}
		segments[i] = name
		directory = filepath.Join(directory, name)
	}
	result := strings.Join(segments, "/")
	if strings.HasSuffix(relative, "/") {
		result += "/"
	}
	return result, true
}

// ignoresCase is McmodInstaller.IgnoresCase: whether the file system of the
// workspace root ignores case in names, as macOS does by default: a name in
// the root spelled in another case finds the same entry, and no other entry
// has that spelling. Taken as false while the root holds no name with a
// letter.
func (in *Installer) ignoresCase(root string) bool {
	if known, ok := in.caseInsensitive.Load(root); ok {
		return known.(bool)
	}
	names, err := listNames(root)
	if err != nil {
		return false
	}
	exists := func(name string) bool {
		p := filepath.Join(root, name)
		return fileExists(p) || dirExists(p)
	}
	for _, name := range names {
		swapped := strings.Map(func(c rune) rune {
			if unicode.IsUpper(c) {
				return unicode.ToLower(c)
			}
			return unicode.ToUpper(c)
		}, name)
		// A link that leads nowhere does not tell.
		if swapped == name || !exists(name) {
			continue
		}
		ignores := exists(swapped)
		for _, n := range names {
			if n == swapped {
				ignores = false
				break
			}
		}
		in.caseInsensitive.Store(root, ignores)
		return ignores
	}
	return false
}

// listNames is the names of the entries of a directory
// (Directory.EnumerateFileSystemEntries).
func listNames(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// equalOrdinalIgnoreCase is string.Equals(a, b, OrdinalIgnoreCase): the
// characters compared by their simple uppercase mappings.
func equalOrdinalIgnoreCase(a, b string) bool {
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
