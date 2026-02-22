package registry

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	shell32DevMode           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteWDevMode = shell32DevMode.NewProc("ShellExecuteW")
)

func IsDevModeEnabled() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}
	return val == 1
}

func TryEnableDevMode() bool {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return false
	}
	file, err := syscall.UTF16PtrFromString("reg.exe")
	if err != nil {
		return false
	}
	args, err := syscall.UTF16PtrFromString(`add HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock /v AllowDevelopmentWithoutDevLicense /t REG_DWORD /d 1 /f`)
	if err != nil {
		return false
	}
	ret, _, _ := procShellExecuteWDevMode.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		1,
	)
	if ret <= 32 {
		return false
	}

	for i := 0; i < 25; i++ {
		if IsDevModeEnabled() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return IsDevModeEnabled()
}
