//go:build windows

package install

import (
	"strings"
	"syscall"
	"unicode/utf16"
)

// getLongPath is McmodInstaller.GetLongPath: the absolute path with its 8.3
// short names replaced by long names (GetLongPathNameW); false when it cannot
// be found. A path of MAX_PATH characters or more is passed with the "\\?\"
// prefix, without which GetLongPathNameW fails on it.
func getLongPath(path string) (string, bool) {
	const prefix = `\\?\`
	const uncPrefix = `\\?\UNC\`
	query := path
	if len(utf16.Encode([]rune(path))) >= 260 && !strings.HasPrefix(path, prefix) {
		if strings.HasPrefix(path, `\\`) {
			query = uncPrefix + path[2:]
		} else {
			query = prefix + path
		}
	}
	p, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		return "", false
	}
	buffer := make([]uint16, len(utf16.Encode([]rune(query)))+260)
	for {
		n, _ := syscall.GetLongPathName(p, &buffer[0], uint32(len(buffer)))
		if n == 0 {
			return "", false
		}
		if int(n) < len(buffer) {
			result := syscall.UTF16ToString(buffer[:n])
			if query == path {
				return result, true
			}
			switch {
			case len(result) >= len(uncPrefix) && strings.EqualFold(result[:len(uncPrefix)], uncPrefix):
				return `\\` + result[len(uncPrefix):], true
			case strings.HasPrefix(result, prefix):
				return result[len(prefix):], true
			}
			return result, true
		}
		buffer = make([]uint16, n)
	}
}
