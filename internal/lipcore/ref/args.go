package ref

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// AcceptedForms lists the accepted package argument forms, for messages.
const AcceptedForms = "A package is written as a remote repository such as" +
	" github.com/owner/repo (optionally followed by #<variant>, then by @<version>," +
	" @branch:<name>, @tag:<name> or @rev:<full commit id>), an https:// URL of a ZIP" +
	" archive, or the path of a local ZIP file or directory (e.g. ./my-package.zip)."

// errNullChar is the ArgumentException of Path.GetFullPath for a path with a
// null character.
var errNullChar = errors.New("Null character in path. (Parameter 'path')")

// otherSchemeRegex matches a URI scheme followed by "://".
var otherSchemeRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// ParsePackageArgument turns a package argument into the source of a request
// (protocol/cli.md §3; PackageArguments.Parse in PackageArguments.cs). cwd is
// the current directory (the workspace root).
//
// Accepted forms:
//   - "https://..." [ "#" variant ]: a URL.
//   - a local path [ "#" variant ]: a path, absolute or relative to cwd,
//     recognized when it starts with "./", "../", ".\", "..\", is "." or "..",
//     is rooted, or has no '@' and ends with ".zip" or names an existing file
//     or directory. It becomes the file: URI of its absolute path.
//   - "repo" [ "#" variant ] [ "@" ref ]: a remote repository.
//
// It fails when the argument is none of these, or names a local path that
// does not exist.
func ParsePackageArgument(arg, cwd string) (Ref, error) {
	if strings.HasPrefix(arg, "https://") {
		r, err := parseArgRef(arg)
		if err != nil {
			return Ref{}, err
		}
		if r.Kind != KindURL {
			return Ref{}, invalidArgument(arg, "it is not an https:// URL")
		}
		return r, nil
	}

	if otherSchemeRegex.MatchString(arg) {
		return Ref{}, invalidArgument(arg, schemeReason(arg[:strings.IndexByte(arg, ':')]))
	}

	file, isLocal, err := localFileRef(arg, cwd)
	if err != nil {
		return Ref{}, err
	}
	if isLocal {
		local, err := file.LocalPath()
		if err != nil {
			return Ref{}, err
		}
		if !pathExists(local) {
			return Ref{}, fmt.Errorf("Cannot find the local package '%s': %s does not exist. Check"+
				" the path (a relative path is relative to the workspace root %s). %s",
				arg, local, cwd, AcceptedForms)
		}
		return file, nil
	}

	if hasFileScheme(arg) {
		return Ref{}, invalidArgument(arg, schemeReason("file"))
	}

	r, err := parseArgRef(arg)
	if err != nil {
		return Ref{}, err
	}
	if r.Kind != KindSpec {
		return Ref{}, invalidArgument(arg, "it is not a remote repository")
	}
	return r, nil
}

// schemeReason says why an argument with a URI scheme other than https is no
// package.
func schemeReason(scheme string) string {
	switch {
	case strings.EqualFold(scheme, "file"):
		return "a local package is written as its path (e.g. ./my-package.zip), not as a file: URI"
	case strings.EqualFold(scheme, "http"):
		return "lip downloads only over https://, so use the https:// URL of the archive"
	}
	return fmt.Sprintf("lip cannot download from %s:// URLs, only from https:// URLs", scheme)
}

func invalidArgument(arg, reason string) error {
	return fmt.Errorf("'%s' is not a valid package: %s. %s", arg, reason, AcceptedForms)
}

func parseArgRef(arg string) (Ref, error) {
	r, reason, ok := TryParse(arg, false)
	if !ok {
		return Ref{}, invalidArgument(arg, reason)
	}
	return r, nil
}

// localFileRef is the source of an argument that names a local path, whether
// or not it exists; isLocal is false when the argument is no local path. A
// variant follows the last '#' unless the whole argument names an existing
// path, so that file names may contain '#'.
func localFileRef(arg, cwd string) (r Ref, isLocal bool, err error) {
	path, variant := arg, ""
	found := exists(path, cwd)
	if !found {
		if hash := strings.LastIndexByte(arg, '#'); hash >= 0 && IsValidVariant(arg[hash+1:]) {
			path, variant = arg[:hash], arg[hash+1:]
			found = exists(path, cwd)
		}
	}

	isLocal = strings.HasPrefix(path, "./") || strings.HasPrefix(path, `.\`) ||
		strings.HasPrefix(path, "../") || strings.HasPrefix(path, `..\`) ||
		path == "." || path == ".." ||
		isPathRooted(path) ||
		!strings.Contains(path, "@") && (hasZipSuffix(path) || found)
	if !isLocal {
		return Ref{}, false, nil
	}

	full, err := fullPath(combinePath(cwd, path))
	if err != nil {
		return Ref{}, false, err
	}
	full = trimEndingDirectorySeparator(full)
	r, err = FromFile(full, variant)
	if err != nil {
		return Ref{}, false, err
	}
	return r, true, nil
}

// hasZipSuffix is EndsWith(".zip", StringComparison.OrdinalIgnoreCase).
func hasZipSuffix(p string) bool {
	return len(p) >= 4 && strings.EqualFold(p[len(p)-4:], ".zip")
}

// trimEndingDirectorySeparator is Path.TrimEndingDirectorySeparator.
func trimEndingDirectorySeparator(p string) string {
	if len(p) > 0 && isDirSep(p[len(p)-1]) && len(p) != rootLength(p) {
		return p[:len(p)-1]
	}
	return p
}

// exists is PackageArguments.Exists: whether the path, relative to cwd,
// names an existing file or directory; false when it cannot be resolved.
func exists(path, cwd string) bool {
	if path == "" {
		return false
	}
	full, err := fullPath(combinePath(cwd, path))
	if err != nil {
		return false
	}
	return pathExists(full)
}

// pathExists is File.Exists(p) || Directory.Exists(p), which read the
// attributes of p itself, so a dangling symbolic link or junction exists.
func pathExists(p string) bool {
	fi, err := os.Lstat(p)
	if err != nil {
		return false
	}
	// File.Exists is false for a path ending in a separator.
	if len(p) > 0 && isDirSep(p[len(p)-1]) && !fi.IsDir() {
		return false
	}
	return true
}
