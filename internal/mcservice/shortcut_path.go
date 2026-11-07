package mcservice

import (
	"path/filepath"
	"strings"
)

// ShortcutFileName is the file name CreateDesktopShortcut gives the desktop
// shortcut of an instance: "Minecraft - <sanitized name>.lnk".
func ShortcutFileName(name string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if r < 32 {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	safe = strings.TrimRight(strings.TrimSpace(safe), ". ")
	if safe == "" {
		safe = "Minecraft"
	}
	return "Minecraft - " + safe + ".lnk"
}

// DesktopShortcutPath is where CreateDesktopShortcut writes the shortcut of
// an instance, "" when the desktop folder cannot be found. The file may not
// exist.
func DesktopShortcutPath(name string) string {
	desktop := userDesktopDir()
	if desktop == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	return filepath.Join(desktop, ShortcutFileName(name))
}
