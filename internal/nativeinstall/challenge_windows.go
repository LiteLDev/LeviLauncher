// SPDX-License-Identifier: GPL-3.0-only
// Experimental adaptation of Xodus 0670e25aeb0e0e9f800f8f2f4968ae3b681842a7.
// Adapted under GPL-3.0-only; see THIRD_PARTY_NOTICES.
package nativeinstall

import (
	"fmt"
	"golang.org/x/sys/windows"

	"unsafe"
)

func deviceInfo() (string, error) {
	get := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemFirmwareTable")
	n, _, err := get.Call(0x52534d42, 0, 0, 0)
	if n < 8 || n > 1<<20 {
		return "", fmt.Errorf("firmware size unavailable: %v", err)
	}
	raw := make([]byte, n)
	got, _, err := get.Call(0x52534d42, 0, uintptr(unsafe.Pointer(&raw[0])), n)
	if got != n {
		return "", fmt.Errorf("firmware read: %v", err)
	}
	return makeDeviceInfo(raw[8:]), nil
}
