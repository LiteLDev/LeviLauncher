package launchercore

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL       = windows.NewLazySystemDLL("kernel32.dll")
	procLstrlenA      = kernel32DLL.NewProc("lstrlenA")
	procRtlMoveMemory = kernel32DLL.NewProc("RtlMoveMemory")
)

func GetGamertagByXuid(xuid uint64) (string, error) {
	if err := EnsureApiLoaded(); err != nil {
		return "", err
	}
	r1, _, _ := pGetGamertagByXuid.Call(uintptr(xuid))
	if r1 == 0 {
		return "", errors.New("failed to get gamertag")
	}

	n, _, _ := procLstrlenA.Call(r1)
	if n == 0 {
		return "", nil
	}
	b := make([]byte, n)
	procRtlMoveMemory.Call(
		uintptr(unsafe.Pointer(&b[0])),
		r1,
		n,
	)
	return AcpToString(b)
}

func GetLocalUserId() (uint64, error) {
	if err := EnsureApiLoaded(); err != nil {
		return 0, err
	}
	var xuid uint64
	r1, _, _ := pGetLocalUserId.Call(uintptr(unsafe.Pointer(&xuid)))
	if int32(r1) != 0 {
		return 0, errors.New("failed to get local user id")
	}
	return xuid, nil
}

func GetLocalUserGamertag() (string, error) {
	if err := EnsureApiLoaded(); err != nil {
		return "", err
	}
	buf := make([]byte, 256)
	var used uintptr
	r1, _, _ := pGetLocalUserGamertag.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&used)),
	)
	if int32(r1) != 0 {
		return "", errors.New("failed to get local user gamertag")
	}
	return AcpToString(buf[:used])
}

func GetLocalUserGamerPictureSize(size int) (int, error) {
	if err := EnsureApiLoaded(); err != nil {
		return 0, err
	}
	var picSize uintptr
	r1, _, _ := pGetLocalUserGamerPictureSize.Call(
		uintptr(size),
		uintptr(unsafe.Pointer(&picSize)),
	)
	if int32(r1) != 0 {
		return 0, errors.New("failed to get gamer picture size")
	}
	return int(picSize), nil
}

func GetLocalUserGamerPicture(size int) ([]byte, error) {
	if err := EnsureApiLoaded(); err != nil {
		return nil, err
	}
	reqSize, err := GetLocalUserGamerPictureSize(size)
	if err != nil {
		return nil, err
	}
	if reqSize == 0 {
		return nil, nil
	}

	buf := make([]byte, reqSize)
	var used uintptr
	r1, _, _ := pGetLocalUserGamerPicture.Call(
		uintptr(size),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&used)),
	)
	if int32(r1) != 0 {
		return nil, errors.New("failed to get gamer picture")
	}
	return buf[:used], nil
}

func ResetSession() error {
	if err := EnsureApiLoaded(); err != nil {
		return err
	}
	r1, _, _ := pResetSession.Call()
	if int32(r1) != 0 {
		return errors.New("ERR_XBL_RESET_FAILED")
	}
	return nil
}

func XUserGetState() (uint32, error) {
	if err := EnsureApiLoaded(); err != nil {
		return 0, err
	}
	var state uint32
	r1, _, _ := pXUserGetState.Call(uintptr(unsafe.Pointer(&state)))
	if int32(r1) != 0 {
		return 0, errors.New("ERR_XUSER_GET_STATE_FAILED")
	}
	return state, nil
}
