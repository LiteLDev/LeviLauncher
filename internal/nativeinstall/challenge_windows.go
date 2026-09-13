// SPDX-License-Identifier: GPL-3.0-only
// Experimental adaptation of Xodus 0670e25aeb0e0e9f800f8f2f4968ae3b681842a7.
// Adapted under GPL-3.0-only; see THIRD_PARTY_NOTICES.
package nativeinstall

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"math/bits"
	"unsafe"
)

func challengeRound(i int, x uint32) uint32 {
	r := bits.RotateLeft32
	switch i {
	case 1:
		return 0x3243*r(x^0x24181621, -22) - r(x, -8)
	case 2:
		return 0x3243 * (r(x, -15) ^ 0x2418)
	case 3:
		return (x >> 9) + 0x1621*r(x^0x4139, 3)
	case 4:
		return r(x, -28) ^ (0x4139 * r(x^0x24181621, -9))
	case 5:
		return r(x, -12) + 0x3243*r(x-0x24181621, -14)
	case 6:
		return r(x, -11) ^ (0x2418 * r(x^0x1621, 2))
	case 7:
		return x - 0x41393243 - 0x1621
	case 8:
		return 0x4139*r(x^0x2418, 2) - r(x, -18)
	case 0:
		return 0x3243*r(x-0x24181621, -18) - r(x, -9)
	default:
		return 0x4139*r(x+0x24181621, -10) - r(x, -29)
	}
}
func obfuscate(b []byte) {
	u := binary.LittleEndian
	const magic uint32 = 0x24181621
	var a uint32
	c := ^(uint32(0x4139) * bits.RotateLeft32(magic, -10))
	for i := 1; i <= 8; i++ {
		a, c = c, a^challengeRound(i, c)
	}
	iv := u.Uint32(b[4:])
	lo, hi, previousLo, previousHi := c^iv, uint32(0), iv, uint32(0)
	u.PutUint32(b[4:], lo)
	for p := 8; p < len(b); p += 8 {
		x, y := u.Uint32(b[p:]), u.Uint32(b[p+4:])
		a, c = lo^x, hi^y^challengeRound(0, lo^x)
		for i := 8; i >= 1; i-- {
			a, c = c^challengeRound(i, a), a
		}
		lo, hi = previousLo^a^challengeRound(9, c), previousHi^c
		previousLo, previousHi = x, y
		u.PutUint32(b[p:], lo)
		u.PutUint32(b[p+4:], hi)
	}
}
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
	s := "<DeviceInfo Id=\"DeviceInfo\">"
	for _, version := range []uint32{2, 4} {
		b := make([]byte, 2048)
		binary.LittleEndian.PutUint32(b, version)
		copy(b[4:260], raw[8:])
		if version == 2 {
			b[328] = 1
		} else {
			binary.LittleEndian.PutUint32(b[1231:], 1)
		}
		obfuscate(b)
		id := 8196
		if version == 4 {
			id = 8197
		}
		s += fmt.Sprintf("<Component name=\"%d\">%s</Component>", id, base64.StdEncoding.EncodeToString(b))
	}
	s += "<Component name=\"4113\">AA==</Component><Component name=\"4145\">AQAAAA==</Component>"
	for _, id := range []int{4100, 4101, 4102, 4160, 4161} {
		s += fmt.Sprintf("<Component name=\"%d\" error=\"-2147024894\"/>", id)
	}
	return s + "</DeviceInfo>", nil
}
