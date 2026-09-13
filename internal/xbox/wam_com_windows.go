//go:build windows

package xbox

import (
	"sync"
	"sync/atomic"
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

// These helpers operate on the WinRT ABI, whose slots include IUnknown and
// IInspectable. Interface IDs and slots come from Windows SDK 10.0.26100.0.
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

func wamObject(object *ole.IUnknown, slot int) (*ole.IUnknown, error) {
	var value *ole.IUnknown
	err := wamCall(object, slot, uintptr(unsafe.Pointer(&value)))
	if err == nil && value == nil {
		err = ErrAuthenticationFailed
	}
	return value, err
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

// Native code owns references to these delegates beyond the initiating syscall.
// Keep each Go object and its closure rooted until the last COM Release. The
// shared vtables also avoid exhausting syscall.NewCallback's process-wide limit.
type wamDelegate struct {
	vtable *[4]uintptr
	refs   atomic.Int32
	iid    ole.GUID
	invoke func(unsafe.Pointer, unsafe.Pointer) uintptr
}

var wamDelegates = struct {
	sync.Mutex
	live map[*wamDelegate]struct{}
}{live: make(map[*wamDelegate]struct{})}

var (
	wamDelegateQI      = syscall.NewCallback(wamDelegateQueryInterface)
	wamDelegateAddRef  = syscall.NewCallback(func(d *wamDelegate) uintptr { return uintptr(d.refs.Add(1)) })
	wamDelegateRelease = syscall.NewCallback(func(d *wamDelegate) uintptr {
		refs := d.refs.Add(-1)
		if refs == 0 {
			wamDelegates.Lock()
			delete(wamDelegates.live, d)
			wamDelegates.Unlock()
		}
		return uintptr(refs)
	})
	wamDelegateOne = [4]uintptr{wamDelegateQI, wamDelegateAddRef, wamDelegateRelease,
		syscall.NewCallback(func(d *wamDelegate, arg unsafe.Pointer) uintptr { return d.invoke(arg, nil) })}
	wamDelegateTwo = [4]uintptr{wamDelegateQI, wamDelegateAddRef, wamDelegateRelease,
		syscall.NewCallback(func(d *wamDelegate, sender, args unsafe.Pointer) uintptr { return d.invoke(sender, args) })}
)

func newWAMDelegate(iid string, oneArgument bool, invoke func(unsafe.Pointer, unsafe.Pointer) uintptr) *ole.IUnknown {
	d := &wamDelegate{vtable: &wamDelegateTwo, iid: *ole.NewGUID(iid), invoke: invoke}
	if oneArgument {
		d.vtable = &wamDelegateOne
	}
	d.refs.Store(1)
	wamDelegates.Lock()
	wamDelegates.live[d] = struct{}{}
	wamDelegates.Unlock()
	return (*ole.IUnknown)(unsafe.Pointer(d))
}

func wamDelegateQueryInterface(d *wamDelegate, iid *ole.GUID, out *unsafe.Pointer) uintptr {
	if out == nil || iid == nil {
		return ole.E_POINTER
	}
	*out = nil
	if !ole.IsEqualGUID(iid, &d.iid) && !ole.IsEqualGUID(iid, ole.IID_IUnknown) &&
		!ole.IsEqualGUID(iid, ole.NewGUID("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")) { // IAgileObject
		return ole.E_NOINTERFACE
	}
	d.refs.Add(1)
	*out = unsafe.Pointer(d)
	return ole.S_OK
}
