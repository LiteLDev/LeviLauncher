package uwp

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

type shellItemArrayVtbl struct {
	ole.IUnknownVtbl
	BindToHandler              uintptr
	GetPropertyStore           uintptr
	GetPropertyDescriptionList uintptr
	GetAttributes              uintptr
	GetCount                   uintptr
	GetItemAt                  uintptr
	EnumItems                  uintptr
}

type shellItemVtbl struct {
	ole.IUnknownVtbl
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
}

// Exercise Windows URI marshalling without activating a game or changing any
// registration. This catches URI parsing and COM array/vtable mistakes that
// the mocked deployment tests cannot detect.
func TestProtocolItemsPreserveEditorURI(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if e, ok := err.(*ole.OleError); !ok || e.Code() != 1 {
			t.Fatal(err)
		}
	}
	defer ole.CoUninitialize()
	for _, uri := range []string{
		"minecraft:?Editor=true", "minecraft-preview:?Editor=true",
		"minecraft://creator/?Editor=true", "minecraft-preview://creator/?Editor=true",
	} {
		items, err := protocolItems(uri)
		if err != nil {
			t.Fatalf("%s: %v", uri, err)
		}
		func() {
			defer items.Release()
			vtbl := (*shellItemArrayVtbl)(unsafe.Pointer(items.RawVTable))
			var count uint32
			hr, _, _ := syscall.SyscallN(vtbl.GetCount, uintptr(unsafe.Pointer(items)), uintptr(unsafe.Pointer(&count)))
			if int32(hr) < 0 || count != 1 {
				t.Fatalf("URI array count: HRESULT 0x%08X, count=%d", uint32(hr), count)
			}
			var item *ole.IUnknown
			hr, _, _ = syscall.SyscallN(vtbl.GetItemAt, uintptr(unsafe.Pointer(items)), 0, uintptr(unsafe.Pointer(&item)))
			if int32(hr) < 0 {
				t.Fatalf("GetItemAt: HRESULT 0x%08X", uint32(hr))
			}
			defer item.Release()
			itemVtbl := (*shellItemVtbl)(unsafe.Pointer(item.RawVTable))
			var name *uint16
			const sigdnURL = 0x80068000
			hr, _, _ = syscall.SyscallN(itemVtbl.GetDisplayName, uintptr(unsafe.Pointer(item)), sigdnURL, uintptr(unsafe.Pointer(&name)))
			if int32(hr) < 0 {
				t.Fatalf("GetDisplayName: HRESULT 0x%08X", uint32(hr))
			}
			defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(name)))
			if got := windows.UTF16PtrToString(name); got != uri {
				t.Fatalf("URI changed: got %q, want %q", got, uri)
			}
		}()
	}
	if items, err := protocolItems("minecraft://creator/\x00?Editor=true"); err == nil || items != nil {
		t.Fatalf("embedded NUL accepted: %v", err)
	}
}
