//go:build windows

package xbox

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var wamCombase = syscall.NewLazyDLL("combase.dll")

func initializeWAM(apartment uint32) error {
	hr, _, _ := wamCombase.NewProc("RoInitialize").Call(uintptr(apartment))
	// S_FALSE is successful initialization of an already-initialized apartment
	// and still needs a matching RoUninitialize. go-ole v1.3.0 rejects S_FALSE.
	if int32(hr) < 0 {
		return ole.NewError(hr)
	}
	return nil
}

func uninitializeWAM() { wamCombase.NewProc("RoUninitialize").Call() }

// wamCall invokes a WinRT method through the ABI, whose slots include IUnknown
// and IInspectable. Interface IDs and slots come from Windows SDK 10.0.26100.0.
// Keep pointer-valued arguments alive across slice allocation and stack growth,
// just as syscall.SyscallN does for direct calls.
//
//go:uintptrescapes
func wamCall(object *ole.IUnknown, slot int, args ...uintptr) error {
	if object == nil {
		return ErrAuthenticationFailed
	}
	args = append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	hr, _, _ := syscall.SyscallN(vtblFn(unsafe.Pointer(object), slot), args...)
	if int32(hr) < 0 {
		return ole.NewError(hr)
	}
	return nil
}

func wamAccountID(account *ole.IUnknown) (string, error) {
	account2, err := account.QueryInterface(ole.NewGUID(account2IID))
	if err != nil {
		return "", err
	}
	defer account2.Release()
	var value ole.HString
	if err := wamCall(&account2.IUnknown, accountIDSlot, uintptr(unsafe.Pointer(&value))); err != nil {
		return "", err
	}
	defer ole.DeleteHString(value)
	if value.String() == "" {
		return "", ErrAuthenticationFailed
	}
	return value.String(), nil
}
