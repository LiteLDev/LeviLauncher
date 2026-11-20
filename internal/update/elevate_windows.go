package update

import (
	"fmt"

	"strings"
	"syscall"

	"unsafe"
)

func startSelfUpdateElevated(exePath string, curVer string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	arg := "--self-update=" + strings.TrimPrefix(strings.TrimSpace(curVer), "v")
	args, err := syscall.UTF16PtrFromString(arg)
	if err != nil {
		return err
	}
	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		swShowNormal,
	)
	if ret <= shellExecuteErrCut {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}
	return nil
}

var (
	shell32            = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW  = shell32.NewProc("ShellExecuteW")
	swShowNormal       = uintptr(1)
	shellExecuteErrCut = uintptr(32)
)
