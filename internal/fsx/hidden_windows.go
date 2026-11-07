//go:build windows

package fsx

import "golang.org/x/sys/windows"

// SetHidden gives path the hidden attribute, best effort. Only state
// directories such as <instance>\.levilauncher are hidden.
func SetHidden(path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return
	}
	_ = windows.SetFileAttributes(p, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
}
