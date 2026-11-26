package cli

import (
	"context"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
)

type signInNoticeKey struct{}

// withSignInNotice attaches a notice callback for non-Windows browser sign-in
// flows so the verification URL and device code are written to stderr.
func withSignInNotice(ctx context.Context, fn func(format string, args ...any)) context.Context {
	return context.WithValue(ctx, signInNoticeKey{}, fn)
}

func signInNoticeFrom(ctx context.Context) func(format string, args ...any) {
	if fn, ok := ctx.Value(signInNoticeKey{}).(func(format string, args ...any)); ok && fn != nil {
		return fn
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
