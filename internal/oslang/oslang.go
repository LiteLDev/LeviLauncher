// Package oslang reports the language Windows renders its own UI in. The
// startup dialogs need it because they can fire before the frontend, and with
// it the launcher's own language setting, is available.
package oslang

import (
	"syscall"

	win "golang.org/x/sys/windows"
)

var (
	kernel32                     = win.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
)

// IsChineseUI reports whether the Windows UI language is Chinese.
func IsChineseUI() bool {
	langID, _, err := procGetUserDefaultUILanguage.Call()
	if langID == 0 || err != nil && err != syscall.Errno(0) {
		return false
	}
	const langChinese = 0x04 // LANG_CHINESE
	return uint16(langID)&0x03ff == langChinese
}
