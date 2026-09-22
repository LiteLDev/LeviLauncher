//go:build windows

package xbox

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	"github.com/liteldev/LeviLauncher/internal/xbox/winrt/core"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

var (
	ErrInteractionRequired  = errors.New("ERR_AUTH_INTERACTION_REQUIRED")
	ErrAccountChanged       = errors.New("ERR_AUTH_ACCOUNT_CHANGED")
	ErrAuthenticationFailed = errors.New("ERR_AUTH_FAILED")
)

// errSignInUIRequired marks a window-scoped request that needs the user but was
// answered without a sign-in UI. The forced prompt is what presents it, so such a
// request is retried once with that prompt.
var errSignInUIRequired = errors.New("WAM sign-in UI required")

// ABI slot numbers include IUnknown (3) and IInspectable (3) methods.
const (
	wamSilent            = 6
	wamSilentWithAccount = 7
	wamFindAccount       = 10
	responseAccount      = 8
	accountIDSlot        = 6
	account2IID          = "7b56d6f8-990b-4eb5-94a7-5621f3a8b824"

	// IWebTokenRequestFactory: Create and CreateWithPromptType.
	wamRequest           = 6
	wamRequestWithPrompt = 7
	// IWebAuthenticationCoreManagerInterop: RequestTokenForWindowAsync.
	wamRequestForWindow                    = 6
	iidWebAuthenticationCoreManagerInterop = "f4b8e804-811e-4436-b69c-44cb67b72084"
	iidTokenRequestResultOperation         = "0a815852-7c44-5674-b3d2-fa2e4c1e46c9"

	// The user drives the sign-in UI, so it may stay open for a while.
	interactiveTimeout = 10 * time.Minute
)

func requestWAMTicket(ctx context.Context, scope, expectedAccount string) (string, string, error) {
	return requestWAMToken(ctx, scope, expectedAccount, 0, nil)
}

// requestWAMToken obtains a token together with the identity it belongs to. A
// non-zero hwnd uses the account picker for normal processes. Elevated processes
// use the window-scoped WAM request directly because AccountsSettingsPane can
// close immediately across the elevation boundary. Silent renewal is unchanged.
func requestWAMToken(ctx context.Context, scope, expectedAccount string, hwnd uintptr, dispatch func(func())) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeWAM(roInitMultithreaded); err != nil {
		return "", "", err
	}
	defer uninitializeWAM()
	manager, err := ole.RoGetActivationFactory("Windows.Security.Authentication.Web.Core.WebAuthenticationCoreManager", ole.NewGUID(core.GUIDiWebAuthenticationCoreManagerStatics))
	if err != nil {
		return "", "", err
	}
	defer manager.Release()
	var provider *ole.IUnknown
	if hwnd != 0 && !processinfo.IsElevated() {
		provider, err = selectWAMProvider(ctx, hwnd, dispatch)
	} else {
		provider, err = findMSAProvider(ctx)
	}
	if err != nil {
		return "", "", err
	}
	defer provider.Release()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	var account *ole.IUnknown
	if expectedAccount != "" {
		id, err := ole.NewHString(expectedAccount)
		if err != nil {
			return "", "", err
		}
		defer ole.DeleteHString(id)
		var findOp *foundation.IAsyncOperation
		hr, _, _ := syscall.SyscallN(vtblFn(unsafe.Pointer(manager), wamFindAccount), uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(provider)), uintptr(id), uintptr(unsafe.Pointer(&findOp)))
		if hr != 0 {
			return "", "", ole.NewError(hr)
		}
		defer findOp.Release()
		ptr, err := awaitContext(ctx, findOp)
		if err != nil {
			return "", "", err
		}
		if ptr == nil {
			return "", "", ErrInteractionRequired
		}
		account = (*ole.IUnknown)(ptr)
		defer account.Release()
	}

	factory, err := ole.RoGetActivationFactory("Windows.Security.Authentication.Web.Core.WebTokenRequest", ole.NewGUID(core.GUIDiWebTokenRequestFactory))
	if err != nil {
		return "", "", err
	}
	defer factory.Release()
	// The default prompt reuses an existing Windows session and keeps single
	// sign-on available.
	token, reference, err := requestWAMTokenWithPrompt(ctx, manager, factory, provider, account, scope, hwnd, core.WebTokenRequestPromptTypeDefault)
	if wamShouldForceSignInUI(hwnd, err) {
		token, reference, err = requestWAMTokenWithPrompt(ctx, manager, factory, provider, account, scope, hwnd, core.WebTokenRequestPromptTypeForceAuthentication)
	}
	if errors.Is(err, errSignInUIRequired) {
		err = ErrInteractionRequired
	}
	if err != nil {
		return "", "", err
	}
	if err := validateWAMIdentity(expectedAccount, reference, token); err != nil {
		return "", "", err
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	return token, reference, nil
}

// wamShouldForceSignInUI reports whether a failed request has to be repeated with
// the forced prompt. Only a window-scoped request that asked for the user without
// presenting the sign-in UI is repeated: a silent request (hwnd == 0) must never
// open a dialog, dismissing the UI is final, and a provider failure is not worth
// repeating.
func wamShouldForceSignInUI(hwnd uintptr, err error) bool {
	return hwnd != 0 && errors.Is(err, errSignInUIRequired)
}

func requestWAMTokenWithPrompt(ctx context.Context, manager, factory *ole.IInspectable, provider, account *ole.IUnknown, scope string, hwnd uintptr, prompt core.WebTokenRequestPromptType) (string, string, error) {
	request, err := createWAMRequest(factory, provider, scope, prompt)
	if err != nil {
		return "", "", err
	}
	defer request.Release()
	var tokenOp *foundation.IAsyncOperation
	var hr uintptr
	if hwnd != 0 {
		interop, err := manager.QueryInterface(ole.NewGUID(iidWebAuthenticationCoreManagerInterop))
		if err != nil {
			return "", "", err
		}
		defer interop.Release()
		// SDK: RequestTokenForWindowAsync returns
		// IAsyncOperation<WebTokenRequestResult>, Windows SDK 10.0.26100.
		iid := ole.NewGUID(iidTokenRequestResultOperation)
		hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(interop), wamRequestForWindow), uintptr(unsafe.Pointer(interop)), hwnd, uintptr(unsafe.Pointer(request)), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&tokenOp)))
	} else if account == nil {
		hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(manager), wamSilent), uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(request)), uintptr(unsafe.Pointer(&tokenOp)))
	} else {
		hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(manager), wamSilentWithAccount), uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(request)), uintptr(unsafe.Pointer(account)), uintptr(unsafe.Pointer(&tokenOp)))
	}
	if hr != 0 {
		return "", "", ole.NewError(hr)
	}
	if tokenOp == nil {
		return "", "", fmt.Errorf("WAM token request returned no operation: %w", ErrAuthenticationFailed)
	}
	defer tokenOp.Release()
	waitTimeout := asyncTimeout
	if hwnd != 0 {
		waitTimeout = interactiveTimeout
	}
	resultPtr, err := awaitWithTimeout(ctx, tokenOp, waitTimeout)
	if err != nil {
		return "", "", err
	}
	if resultPtr == nil {
		return "", "", ErrAuthenticationFailed
	}
	result := (*core.WebTokenRequestResult)(resultPtr)
	defer result.Release()
	status, err := result.GetResponseStatus()
	if err != nil {
		return "", "", err
	}
	if err := wamStatusError(status, hwnd != 0); err != nil {
		return "", "", wamProviderFailure(result, err)
	}
	data, err := result.GetResponseData()
	if err != nil {
		return "", "", err
	}
	defer data.Release()
	size, err := data.GetSize()
	if err != nil {
		return "", "", err
	}
	if size != 1 {
		return "", "", ErrAuthenticationFailed
	}
	ptr, err := data.GetAt(0)
	if err != nil {
		return "", "", err
	}
	if ptr == nil {
		return "", "", ErrAuthenticationFailed
	}
	response := (*core.WebTokenResponse)(ptr)
	defer response.Release()
	token, err := response.GetToken()
	if err != nil {
		return "", "", err
	}
	responseItf, err := response.QueryInterface(ole.NewGUID(core.GUIDiWebTokenResponse))
	if err != nil {
		return "", "", err
	}
	defer responseItf.Release()
	var actualAccount *ole.IUnknown
	hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(responseItf), responseAccount), uintptr(unsafe.Pointer(responseItf)), uintptr(unsafe.Pointer(&actualAccount)))
	if hr != 0 {
		return "", "", ole.NewError(hr)
	}
	if actualAccount == nil {
		return "", "", ErrAuthenticationFailed
	}
	defer actualAccount.Release()
	reference, err := wamAccountID(actualAccount)
	if err != nil {
		return "", "", err
	}
	return token, reference, nil
}

// createWAMRequest builds a token request. The forced prompt is the one that
// guarantees WAM presents its sign-in UI to the window.
func createWAMRequest(factory *ole.IInspectable, provider *ole.IUnknown, scope string, prompt core.WebTokenRequestPromptType) (*core.WebTokenRequest, error) {
	scopeString, err := ole.NewHString(scope)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(scopeString)
	clientString, err := ole.NewHString(clientID)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(clientString)
	var request *core.WebTokenRequest
	var hr uintptr
	if prompt == core.WebTokenRequestPromptTypeDefault {
		hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(factory), wamRequest), uintptr(unsafe.Pointer(factory)), uintptr(unsafe.Pointer(provider)), uintptr(scopeString), uintptr(clientString), uintptr(unsafe.Pointer(&request)))
	} else {
		hr, _, _ = syscall.SyscallN(vtblFn(unsafe.Pointer(factory), wamRequestWithPrompt), uintptr(unsafe.Pointer(factory)), uintptr(unsafe.Pointer(provider)), uintptr(scopeString), uintptr(clientString), uintptr(prompt), uintptr(unsafe.Pointer(&request)))
	}
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	if request == nil {
		return nil, ErrAuthenticationFailed
	}
	return request, nil
}

func validateWAMIdentity(expected, actual, token string) error {
	if actual == "" || token == "" {
		return ErrAuthenticationFailed
	}
	if expected != "" && expected != actual {
		return ErrAccountChanged
	}
	return nil
}

func wamStatusError(status core.WebTokenRequestStatus, interactive bool) error {
	switch status {
	case core.WebTokenRequestStatusSuccess:
		return nil
	case core.WebTokenRequestStatusUserCancel:
		if interactive {
			// The user dismissed the sign-in UI that WAM presented for this window.
			return context.Canceled
		}
		return ErrInteractionRequired
	case core.WebTokenRequestStatusUserInteractionRequired:
		return errSignInUIRequired
	case core.WebTokenRequestStatusAccountProviderNotAvailable:
		return ErrInteractionRequired
	case core.WebTokenRequestStatusAccountSwitch:
		return ErrAccountChanged
	default:
		return ErrAuthenticationFailed
	}
}

// wamProviderFailure keeps the provider's own diagnosis of a failed response
// together with the sentinel, so callers can still match it with errors.Is.
func wamProviderFailure(result *core.WebTokenRequestResult, statusErr error) error {
	providerError, err := result.GetResponseError()
	if err != nil || providerError == nil {
		return statusErr
	}
	defer providerError.Release()
	code, err := providerError.GetErrorCode()
	if err != nil {
		return statusErr
	}
	message, err := providerError.GetErrorMessage()
	if err != nil {
		return statusErr
	}
	return fmt.Errorf("%w: WAM provider 0x%08x %s", statusErr, code, message)
}

func awaitContext(ctx context.Context, op *foundation.IAsyncOperation) (unsafe.Pointer, error) {
	return awaitWithTimeout(ctx, op, asyncTimeout)
}

func awaitWithTimeout(ctx context.Context, op *foundation.IAsyncOperation, timeout time.Duration) (unsafe.Pointer, error) {
	if op == nil {
		return nil, ErrAuthenticationFailed
	}
	info, err := op.QueryInterface(ole.NewGUID(iidIAsyncInfo))
	if err != nil {
		return nil, err
	}
	defer info.Release()
	this := unsafe.Pointer(info)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			syscall.SyscallN(vtblFn(this, 9), uintptr(this)) // IAsyncInfo.Cancel
			return nil, err
		}
		var status int32
		hr, _, _ := syscall.SyscallN(vtblFn(this, slotGetStatus), uintptr(this), uintptr(unsafe.Pointer(&status)))
		if hr != 0 {
			return nil, ole.NewError(hr)
		}
		if status == asyncCompleted {
			return op.GetResults()
		}
		if status != asyncStarted {
			var code int32
			syscall.SyscallN(vtblFn(this, slotGetErrorCode), uintptr(this), uintptr(unsafe.Pointer(&code)))
			return nil, fmt.Errorf("WAM async status=%d HRESULT=0x%08x", status, uint32(code))
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

func vtblFn(obj unsafe.Pointer, index int) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	return *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))
}
