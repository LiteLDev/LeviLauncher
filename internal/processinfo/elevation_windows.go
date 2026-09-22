//go:build windows

package processinfo

import "golang.org/x/sys/windows"

// IsElevated reports the current process token, not administrator group membership.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
