//go:build windows

package xbox

import (
	"context"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/liteldev/LeviLauncher/internal/xbox/winrt/core"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

const wamManagerClass = "Windows.Security.Authentication.Web.Core.WebAuthenticationCoreManager"

// The caller initializes WinRT and owns the returned reference. Account
// discovery for interactive sign-in is performed by AccountsSettingsPane, not
// FindAllAccountsAsync (which may reject enumeration by unpackaged apps).
func findMSAProvider(ctx context.Context) (*ole.IUnknown, error) {
	manager, err := ole.RoGetActivationFactory(wamManagerClass, ole.NewGUID(core.GUIDiWebAuthenticationCoreManagerStatics))
	if err != nil {
		return nil, err
	}
	defer manager.Release()
	name, err := ole.NewHString(msaProvider)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(name)
	authority, err := ole.NewHString("consumers")
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(authority)
	var op *foundation.IAsyncOperation
	if err = wamCall(&manager.IUnknown, 12, uintptr(name), uintptr(authority), uintptr(unsafe.Pointer(&op))); err != nil {
		return nil, err
	}
	if op == nil {
		return nil, ErrInteractionRequired
	}
	defer op.Release()
	ptr, err := awaitContext(ctx, op)
	if err != nil {
		return nil, err
	}
	if ptr == nil {
		return nil, ErrInteractionRequired
	}
	return (*ole.IUnknown)(ptr), nil
}
