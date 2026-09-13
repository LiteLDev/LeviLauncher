package utils

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// NormalizeWindowsPath normalizes case, separators and Windows namespace
// prefixes for path comparisons without accessing the filesystem.
func NormalizeWindowsPath(path string) string {
	s := strings.TrimSpace(path)
	if s == "" {
		return ""
	}
	s = strings.ToLower(filepath.Clean(s))
	if strings.HasPrefix(s, `\\?\`) || strings.HasPrefix(s, `\??\`) {
		s = s[4:]
		if strings.HasPrefix(s, `unc\`) {
			s = `\\` + strings.TrimPrefix(s, `unc\`)
		}
	}
	return filepath.Clean(s)
}

// CanonicalWindowsPath resolves junctions and symlinks to the location Windows
// reports for process images and registered packages. If the path cannot be
// opened, retain its normalized spelling so comparisons still work for paths
// that are temporarily unavailable.
func CanonicalWindowsPath(path string) string {
	s := strings.TrimSpace(path)
	if s == "" {
		return ""
	}
	if resolved, err := resolveFinalPath(s); err == nil {
		s = resolved
	}
	return NormalizeWindowsPath(s)
}

func resolveFinalPath(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n+1)
	}
}
