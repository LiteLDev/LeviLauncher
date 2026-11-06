//go:build windows

package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// getFullPath ports Path.GetFullPath with base as the working directory. It
// uses GetFullPathNameW like .NET, so "/" becomes "\", "." and ".." are
// collapsed and trailing dots and spaces of segments are removed; a
// trailing separator is kept. Like .NET, 8.3 short names are expanded when
// the result contains '~'.
func getFullPath(base, p string) string {
	var in string
	switch {
	case len(p) >= 2 && isDirSep(rune(p[0])) && isDirSep(rune(p[1])):
		in = p // UNC or device path
	case len(p) >= 1 && isDirSep(rune(p[0])):
		in = filepath.VolumeName(base) + p
	case len(p) >= 2 && isDriveLetter(p[0]) && p[1] == ':':
		in = p
	default:
		in = strings.TrimRight(base, `\/`) + `\` + p
	}
	full, err := syscall.FullPath(in)
	if err != nil {
		return in
	}
	if strings.ContainsRune(full, '~') {
		full = expandShortName(full)
	}
	return full
}

// expandShortName ports PathHelper.TryExpandShortFileName: expand the
// longest existing prefix with GetLongPathNameW and re-append the rest.
func expandShortName(p string) string {
	existing := p
	rest := ""
	for {
		if l, ok := longPathName(existing); ok {
			return l + rest
		}
		dir := filepath.Dir(existing)
		if dir == existing || len(dir) <= len(filepath.VolumeName(existing))+1 {
			return p
		}
		rest = existing[len(dir):] + rest
		if !strings.HasSuffix(dir, `\`) && !strings.HasPrefix(rest, `\`) {
			rest = `\` + rest
		}
		existing = dir
	}
}

func longPathName(p string) (string, bool) {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 260)
	for {
		n, err := syscall.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return "", false
		}
		if int(n) <= len(buf) {
			return string(utf16.Decode(buf[:n])), true
		}
		buf = make([]uint16, n)
	}
}

func fileAttributes(p string) (uint32, bool) {
	fi, err := os.Lstat(p)
	if err != nil {
		return 0, false
	}
	if d, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes, true
	}
	if fi.IsDir() {
		return syscall.FILE_ATTRIBUTE_DIRECTORY, true
	}
	return 0, true
}

// fileExists ports File.Exists: the path exists and is not a directory
// (attributes of the entry itself; reparse points are not followed).
func fileExists(p string) bool {
	a, ok := fileAttributes(p)
	return ok && a&syscall.FILE_ATTRIBUTE_DIRECTORY == 0
}

// dirExists ports Directory.Exists.
func dirExists(p string) bool {
	a, ok := fileAttributes(p)
	return ok && a&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
}

// deleteFile ports File.Delete / FileInfo.Delete: a missing file is not an
// error, but a missing parent directory or a directory at p is. Like .NET
// (EnsureExtendedPrefixIfNeeded) a long path gets the \\?\ prefix, so files
// os.Create placed beyond MAX_PATH can be deleted when long paths are not
// enabled system-wide. Like .NET the path is made full first, which trims
// trailing periods and spaces of its segments.
func deleteFile(p string) error {
	if full, err := syscall.FullPath(p); err == nil {
		p = full
	}
	ptr, err := syscall.UTF16PtrFromString(extendedPath(p))
	if err != nil {
		return err
	}
	err = syscall.DeleteFile(ptr)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.ERROR_FILE_NOT_FOUND):
		return nil
	case errors.Is(err, syscall.ERROR_PATH_NOT_FOUND):
		return &os.PathError{Op: "remove", Path: p, Err: fmt.Errorf("Could not find a part of the path '%s'.: %w", p, err)}
	case errors.Is(err, syscall.ERROR_ACCESS_DENIED):
		return &os.PathError{Op: "remove", Path: p, Err: fmt.Errorf("Access to the path '%s' is denied.: %w", p, err)}
	}
	return &os.PathError{Op: "remove", Path: p, Err: err}
}

// extendedPath ports PathInternal.EnsureExtendedPrefixIfNeeded for a full
// path: a path of MAX_PATH (260) UTF-16 units or more, or one ending in a
// period or space, gets the \\?\ prefix (\\?\UNC\ for a UNC path); device
// paths are kept.
func extendedPath(p string) string {
	if len(utf16.Encode([]rune(p))) < syscall.MAX_PATH && !strings.HasSuffix(p, ".") && !strings.HasSuffix(p, " ") {
		return p
	}
	if strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\\.\`) || strings.HasPrefix(p, `\??\`) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}

// isLink reports whether p exists and whether it is a symbolic link or a
// junction (mount point), without following it. Other reparse points
// (OneDrive placeholders, deduplicated files, app execution aliases) are
// not redirections Win32 follows to another path, so they are not links.
func isLink(p string) (link, exists bool) {
	a, ok := fileAttributes(p)
	if !ok || a&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false, ok
	}
	name, err := windows.UTF16PtrFromString(extendedPath(p))
	if err != nil {
		return true, true
	}
	var data windows.Win32finddata
	h, err := windows.FindFirstFile(name, &data)
	if err != nil {
		// Cannot tell: treat it as a link, to be resolved.
		return true, true
	}
	windows.FindClose(h)
	// For a reparse point, dwReserved0 holds the reparse tag.
	return data.Reserved0 == windows.IO_REPARSE_TAG_SYMLINK || data.Reserved0 == windows.IO_REPARSE_TAG_MOUNT_POINT, true
}

// realPath returns the final path of the existing p with every link and
// junction resolved (GetFinalPathNameByHandleW), in DOS form without the
// \\?\ prefix.
func realPath(p string) (string, error) {
	name, err := windows.UTF16PtrFromString(extendedPath(p))
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: p, Err: err}
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, syscall.MAX_PATH)
	for {
		// Flags 0: FILE_NAME_NORMALIZED | VOLUME_NAME_DOS.
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", &os.PathError{Op: "GetFinalPathNameByHandle", Path: p, Err: err}
		}
		if int(n) < len(buf) {
			s := windows.UTF16ToString(buf[:n])
			if rest, ok := strings.CutPrefix(s, `\\?\UNC\`); ok {
				return `\\` + rest, nil
			}
			return strings.TrimPrefix(s, `\\?\`), nil
		}
		buf = make([]uint16, n)
	}
}
