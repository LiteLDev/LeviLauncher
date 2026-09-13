//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/xbox"
	"golang.org/x/sys/windows"
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

func lockDeviceCache(dir string) windows.Handle {
	secureCache(dir)
	path, err := windows.UTF16PtrFromString(filepath.Join(dir, "device.lock"))
	must(err)
	lock, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		failCode("ERR_AUTH_DEVICE_BUSY", "the account device cache is in use")
	}
	return lock
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
	defer windows.CloseHandle(lock)
	checkCanceled()
	fn()
	return nil
}

// acquireUserTicket uses the launcher's selected WAM account.
func acquireUserTicket(ctx context.Context, acquire func(context.Context) (string, string, error)) {
	token, reference, err := acquire(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		code := "ERR_AUTH_FAILED"
		switch {
		case errors.Is(err, context.Canceled):
			code = "ERR_CANCELED"
		case errors.Is(err, context.DeadlineExceeded):
			code = "ERR_AUTH_TIMEOUT"
		case errors.Is(err, xbox.ErrInteractionRequired):
			code = "ERR_AUTH_INTERACTION_REQUIRED"
		case errors.Is(err, xbox.ErrAccountChanged):
			code = "ERR_AUTH_ACCOUNT_CHANGED"
		}
		panic(&Error{Code: code, Reason: "launcher account authorization failed", cause: err})
	}
	if token == "" || reference == "" {
		failCode("ERR_AUTH_FAILED", "empty launcher account authorization")
	}
	checkCanceled()
	userTicket, userReference = token, reference
}
