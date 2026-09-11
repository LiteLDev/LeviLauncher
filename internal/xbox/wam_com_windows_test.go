//go:build windows

package xbox

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
)

func TestWAMDelegateABIAndLifetime(t *testing.T) {
	const iid = "b7de5527-4c8f-42dd-84da-5ec493abdb9a"
	for _, one := range []bool{true, false} {
		calls := 0
		delegate := newWAMDelegate(iid, one, func(a, b unsafe.Pointer) uintptr {
			if a == nil || (one && b != nil) || (!one && b == nil) {
				t.Error("incorrect native callback arguments")
			}
			calls++
			return ole.S_OK
		})
		queried, err := delegate.QueryInterface(ole.NewGUID(iid))
		if err != nil {
			t.Fatal(err)
		}
		queried.Release()
		if _, err := delegate.QueryInterface(ole.IID_IInspectable); err == nil {
			t.Fatal("delegate advertised an unimplemented IInspectable vtable")
		}
		var a, b byte
		args := []uintptr{uintptr(unsafe.Pointer(delegate)), uintptr(unsafe.Pointer(&a))}
		if !one {
			args = append(args, uintptr(unsafe.Pointer(&b)))
		}
		syscall.SyscallN(vtblFn(unsafe.Pointer(delegate), 3), args...)
		if calls != 1 {
			t.Fatal("native callback did not run")
		}
		if refs := delegate.Release(); refs != 0 {
			t.Fatalf("leaked delegate references: %d", refs)
		}
		wamDelegates.Lock()
		_, retained := wamDelegates.live[(*wamDelegate)(unsafe.Pointer(delegate))]
		wamDelegates.Unlock()
		if retained {
			t.Fatal("released delegate remained rooted")
		}
	}
}
