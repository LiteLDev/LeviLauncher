//go:build darwin

package utils

import (
	"path/filepath"
	"strings"
)

func NormalizeWindowsPath(path string) string {
	s := strings.TrimSpace(path)
	if s == "" {
		return ""
	}
	if filepath.IsAbs(s) {
		return filepath.Clean(s)
	}
	return strings.ToLower(strings.ReplaceAll(s, `\`, "/"))
}
func CanonicalWindowsPath(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(p)
	}
	return NormalizeWindowsPath(path)
}
