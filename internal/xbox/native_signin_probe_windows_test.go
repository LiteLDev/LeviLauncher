//go:build windows && wamprobe

package xbox

import (
	"context"
	"github.com/go-ole/go-ole"
	"github.com/liteldev/LeviLauncher/internal/xbox/winrt/core"
	"github.com/liteldev/LeviLauncher/internal/xbox/winrt/credentials"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// Checks the desktop interop and generic result IID without opening a login dialog.
func TestNativeSignInContract(t *testing.T) {
	if os.Getenv("LEVI_WAM_PROBE") != "1" {
		t.Skip("explicit probe required")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if e := ole.RoInitialize(roInitMultithreaded); e != nil {
		t.Fatal(e)
	}
	defer syscall.NewLazyDLL("combase.dll").NewProc("RoUninitialize").Call()
	manager, e := ole.RoGetActivationFactory("Windows.Security.Authentication.Web.Core.WebAuthenticationCoreManager", ole.NewGUID(core.GUIDiWebAuthenticationCoreManagerStatics))
	if e != nil {
		t.Fatal(e)
	}
	defer manager.Release()
	interop, e := manager.QueryInterface(ole.NewGUID("f4b8e804-811e-4436-b69c-44cb67b72084"))
	if e != nil {
		t.Fatal(e)
	}
	interop.Release()
	op, e := core.WebAuthenticationCoreManagerFindAccountProviderAsync(msaProvider)
	if e != nil {
		t.Fatal(e)
	}
	defer op.Release()
	ptr, e := awaitContext(context.Background(), op)
	if e != nil {
		t.Fatal(e)
	}
	if ptr == nil {
		t.Fatal("MSA provider missing")
	}
	provider := (*credentials.WebAccountProvider)(ptr)
	defer provider.Release()
	request, e := core.WebTokenRequestCreate(provider, xblScope, clientID)
	if e != nil {
		t.Fatal(e)
	}
	defer request.Release()
	prompt, e := request.GetPromptType()
	if e != nil || prompt != core.WebTokenRequestPromptTypeDefault {
		t.Fatal("incorrect interactive prompt type")
	}
	silentRequest, e := core.WebTokenRequestCreate(provider, xblScope, clientID)
	if e != nil {
		t.Fatal(e)
	}
	defer silentRequest.Release()
	tokenOp, e := core.WebAuthenticationCoreManagerGetTokenSilentlyAsync(silentRequest)
	if e != nil {
		t.Fatal(e)
	}
	defer tokenOp.Release()
	typed, e := tokenOp.QueryInterface(ole.NewGUID("0a815852-7c44-5674-b3d2-fa2e4c1e46c9"))
	if e != nil {
		t.Fatal(e)
	}
	typed.Release()
	info, e := tokenOp.QueryInterface(ole.NewGUID(iidIAsyncInfo))
	if e != nil {
		t.Fatal(e)
	}
	defer info.Release()
	syscall.SyscallN(vtblFn(unsafe.Pointer(info), 9), uintptr(unsafe.Pointer(info)))
	t.Log("desktop_interop=true default_prompt=true result_iid_verified=true")
}
