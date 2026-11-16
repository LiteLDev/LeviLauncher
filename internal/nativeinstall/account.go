// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"context"
	"errors"

	"strings"

	"github.com/liteldev/LeviLauncher/internal/xbox"
)

var deviceLegacy string
var deviceProof []byte

func recoverFailure(err *error) {
	if v := recover(); v != nil {
		if failure, ok := v.(*Error); ok {
			*err = failure
		} else {
			panic(v)
		}
	}
}

func clearOperation() {
	clear(contentKey)
	contentKey = nil
	clear(ownKey)
	ownKey = nil
	clear(ownDeviceID)
	ownDeviceID = nil
	clear(deviceProof)
	deviceProof = nil
	deviceLegacy = ""
	userTicket, userReference = "", ""
	active = nil
}

func accountOperation(ctx context.Context, dir string, fn func()) (err error) {
	if ctx == nil || dir == "" {
		return errors.New("invalid account operation")
	}
	select {
	case installGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-installGate }()
	defer recoverFailure(&err)
	active = &installation{ctx: ctx, options: Options{CacheDir: dir}}
	defer clearOperation()
	lock := lockDeviceCache(dir)
	defer closeCacheLock(lock)
	checkCanceled()
	fn()
	return nil
}

// acquireUserTicket uses the launcher's selected account.
func acquireUserTicket(ctx context.Context, acquire func(context.Context) (string, string, error)) {
	token, reference, err := acquire(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		accountAuthorizationFailed(err)
	}
	if token == "" || reference == "" {
		failCode("ERR_AUTH_FAILED", "empty launcher account authorization")
	}
	checkCanceled()
	if active != nil && active.account != nil {
		// Browser selection keys are namespaced locally; Microsoft expects the
		// original account reference in the license request.
		expectedReference := strings.TrimPrefix(active.account.id, "browser:")
		if reference != expectedReference {
			accountAuthorizationFailed(xbox.ErrAccountChanged)
		}
	}
	userTicket, userReference = token, reference
}

// AuthErrorCode maps an authentication failure to the code reported to the UI.
func AuthErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "ERR_CANCELED"
	case errors.Is(err, context.DeadlineExceeded):
		return "ERR_AUTH_TIMEOUT"
	case errors.Is(err, xbox.ErrInteractionRequired):
		return "ERR_AUTH_INTERACTION_REQUIRED"
	case errors.Is(err, xbox.ErrAccountChanged):
		return "ERR_AUTH_ACCOUNT_CHANGED"
	case errors.Is(err, xbox.ErrBrowserStoreAuthorization):
		return "ERR_AUTH_BROWSER_STORE_UNAVAILABLE"
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code
	}
	return "ERR_AUTH_FAILED"
}

func accountAuthorizationFailed(err error) {
	panic(&Error{Code: AuthErrorCode(err), Reason: "launcher account authorization failed", cause: err})
}
