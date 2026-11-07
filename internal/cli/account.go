package cli

import (
	"context"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/winhost"
)

// SignIn shows the Microsoft account picker and saves the chosen account as
// the launcher's account, as the window's Sign in button does. The picker is
// owned by a native window of this process (winhost), so no Wails window is
// needed. Errors: ERR_NO_INTERACTIVE_DESKTOP (exit 8) in a service or other
// non-interactive session, ERR_CANCELED (exit 10) when the user closes the
// picker, ERR_AUTH_* (exit 5) otherwise.
func SignIn(ctx context.Context) error {
	if !winhost.InteractiveDesktop() {
		return Backend("ERR_NO_INTERACTIVE_DESKTOP")
	}
	host, err := winhost.New(winhost.Options{Title: "LeviLauncher sign-in"})
	if err != nil {
		return Errorf(ExitFailure, "ERR_AUTH_FAILED", "cannot open the sign-in window: %v", err)
	}
	defer host.Close()
	// A broken saved selection must not block choosing a new account.
	_ = RestoreAccount(ctx)
	if err := nativeinstall.SignInSharedAccount(ctx, apppath.AccountsDir(), host.HWND(), host.Dispatch); err != nil {
		return Backend(nativeinstall.AuthErrorCode(err)).WithDetail(err.Error())
	}
	return nil
}

// SignOut forgets the launcher's Microsoft account and the content keys
// cached for it. Signing out while signed out succeeds.
func SignOut() error {
	if err := nativeinstall.ForgetSharedAccount(apppath.AccountsDir()); err != nil {
		return FromError(err)
	}
	return nil
}
