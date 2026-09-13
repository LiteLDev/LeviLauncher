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

var (
	shell32                           = windows.NewLazySystemDLL("shell32.dll")
	shParseDisplayName                = shell32.NewProc("SHParseDisplayName")
	shCreateShellItemArrayFromIDLists = shell32.NewProc("SHCreateShellItemArrayFromIDLists")
)

func activate(appUserModelID string) (int, error) {
	return activateWithURI(appUserModelID, "")
}

func activateWithURI(appUserModelID, uri string) (int, error) {
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
	var hr uintptr
	method := "ActivateApplication"
	if uri == "" {
		hr, _, _ = syscall.SyscallN(vtbl.ActivateApplication, uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(id)), 0, 0, uintptr(unsafe.Pointer(&pid)))
	} else {
		items, err := protocolItems(uri)
		if err != nil {
			return 0, err
		}
		defer items.Release()
		// Target this AUMID explicitly; the default URI handler could be a GDK
		// installation instead of the selected UWP instance.
		method = "ActivateForProtocol"
		hr, _, _ = syscall.SyscallN(vtbl.ActivateForProtocol, uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(items)), uintptr(unsafe.Pointer(&pid)))
	}
	if int32(hr) < 0 {
		return 0, fmt.Errorf("%s failed: HRESULT 0x%08X", method, uint32(hr))
	}
	return int(pid), nil
}

// protocolItems builds the single URI shell item required by ActivateForProtocol.
// The caller must initialize COM and release the returned IShellItemArray.
func protocolItems(uri string) (*ole.IUnknown, error) {
	name, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return nil, err
	}
	var pidl unsafe.Pointer
	hr, _, _ := shParseDisplayName.Call(uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("SHParseDisplayName failed: HRESULT 0x%08X", uint32(hr))
	}
	defer ole.CoTaskMemFree(uintptr(pidl))
	var items *ole.IUnknown
	hr, _, _ = shCreateShellItemArrayFromIDLists.Call(1, uintptr(unsafe.Pointer(&pidl)), uintptr(unsafe.Pointer(&items)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("SHCreateShellItemArrayFromIDLists failed: HRESULT 0x%08X", uint32(hr))
	}
	return items, nil
}
