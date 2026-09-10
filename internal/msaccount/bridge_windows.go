//go:build windows

package msaccount

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/combridge"
)

// WebView2's typed Go wrappers do not expose all registration methods. These
// narrow callbacks use the public COM bridge for reference counting and native
// allocation; no Go pointer is retained by COM. Slots follow WebView2's base ABI.
type callback interface {
	combridge.IUnknown
	Invoke(uintptr, unsafe.Pointer)
}
type controllerCallback interface{ callback }
type messageCallback interface{ callback }
type resourceCallback interface{ callback }
type navigationCallback interface{ callback }
type navigationStartingCallback interface{ callback }
type newWindowCallback interface{ callback }
type permissionCallback interface{ callback }
type scriptCallback interface{ callback }
type executeCallback interface{ callback }
type processCallback interface{ callback }

type callbackFunc struct {
	combridge.IUnknown
	fn func(uintptr, unsafe.Pointer)
}

func (c *callbackFunc) Invoke(a uintptr, b unsafe.Pointer) { c.fn(a, b) }
func invokeCallback[T callback](this uintptr, a uintptr, b unsafe.Pointer) uintptr {
	combridge.Resolve[T](this).Invoke(a, b)
	return 0
}
func init() {
	combridge.RegisterVTable[combridge.IUnknown, controllerCallback]("{6c4819f3-c9b7-4260-8127-c9f5bde7f68c}", invokeCallback[controllerCallback])
	combridge.RegisterVTable[combridge.IUnknown, messageCallback]("{57213f19-00e6-49fa-8e07-898ea01ecbd2}", invokeCallback[messageCallback])
	combridge.RegisterVTable[combridge.IUnknown, resourceCallback]("{ab00b74c-15f1-4646-80e8-e76341d25d71}", invokeCallback[resourceCallback])
	combridge.RegisterVTable[combridge.IUnknown, navigationCallback]("{d33a35bf-1c49-4f98-93ab-006e0533fe1c}", invokeCallback[navigationCallback])
	combridge.RegisterVTable[combridge.IUnknown, navigationStartingCallback]("{9adbe429-f36d-432b-9ddc-f8881fbd76e3}", invokeCallback[navigationStartingCallback])
	combridge.RegisterVTable[combridge.IUnknown, newWindowCallback]("{d4c185fe-c81c-4989-97af-2d3fa7ab5651}", invokeCallback[newWindowCallback])
	combridge.RegisterVTable[combridge.IUnknown, permissionCallback]("{15e1c6a3-c72a-4df3-91d7-d097fbec6bfd}", invokeCallback[permissionCallback])
	combridge.RegisterVTable[combridge.IUnknown, scriptCallback]("{b99369f3-9b11-47b5-bc6f-8e7895fcea17}", invokeCallback[scriptCallback])
	combridge.RegisterVTable[combridge.IUnknown, executeCallback]("{49511172-cc67-4bca-9923-137112f4c4cc}", invokeCallback[executeCallback])
	combridge.RegisterVTable[combridge.IUnknown, processCallback]("{79e0aea4-990b-42d9-aa1d-0fcc2e5bc7f1}", invokeCallback[processCallback])
}

func withCallback[T callback](fn func(uintptr, unsafe.Pointer), use func(uintptr) error) error {
	c := combridge.New[T](any(&callbackFunc{fn: fn}).(T))
	defer c.Close()
	return use(c.Ref())
}

//go:uintptrescapes
func comCall(object unsafe.Pointer, slot int, args ...uintptr) error {
	if object == nil {
		return fmt.Errorf("WebView2 interface unavailable")
	}
	vtable := *(**[64]uintptr)(object)
	result, _, _ := syscall.SyscallN(vtable[slot], append([]uintptr{uintptr(object)}, args...)...)
	runtime.KeepAlive(object)
	if int32(result) < 0 {
		return fmt.Errorf("WebView2 HRESULT 0x%08x", uint32(result))
	}
	return nil
}

func registerCallback[T callback](object unsafe.Pointer, slot int, fn func(uintptr, unsafe.Pointer)) error {
	return withCallback[T](fn, func(ref uintptr) error {
		var token int64
		return comCall(object, slot, ref, uintptr(unsafe.Pointer(&token)))
	})
}
