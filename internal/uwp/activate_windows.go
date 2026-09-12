package uwp

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

type activationManagerVtbl struct {
	ole.IUnknownVtbl
	ActivateApplication uintptr
	ActivateForFile     uintptr
	ActivateForProtocol uintptr
}

func activate(appUserModelID string) (int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		if e, ok := err.(*ole.OleError); !ok || e.Code() != 1 {
			return 0, err
		}
	}
	defer ole.CoUninitialize()
	manager, err := ole.CreateInstance(ole.NewGUID("{45BA127D-10A8-46EA-8AB7-56EA9078943C}"), ole.NewGUID("{2E941141-7F97-4756-BA1D-9DECDE894A3D}"))
	if err != nil {
		return 0, err
	}
	defer manager.Release()
	id, err := windows.UTF16PtrFromString(appUserModelID)
	if err != nil {
		return 0, err
	}
	var pid uint32
	vtbl := (*activationManagerVtbl)(unsafe.Pointer(manager.RawVTable))
	hr, _, _ := syscall.SyscallN(vtbl.ActivateApplication, uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(id)), 0, 0, uintptr(unsafe.Pointer(&pid)))
	if int32(hr) < 0 {
		return 0, fmt.Errorf("ActivateApplication failed: HRESULT 0x%08X", uint32(hr))
	}
	return int(pid), nil
}
