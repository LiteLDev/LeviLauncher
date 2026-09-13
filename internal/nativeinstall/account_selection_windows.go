//go:build windows

package nativeinstall

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/xbox"
)

var selectionMu sync.Mutex

type wamAccountSelection struct{ AccountID string }

// RestoreSharedAccount loads only the current WAM account preference.
func RestoreSharedAccount(ctx context.Context, dir string) error {
	return xbox.RestoreAccountSelection(ctx, func() (id string, err error) {
		selectionMu.Lock()
		defer selectionMu.Unlock()
		defer recoverFailure(&err)
		var saved wamAccountSelection
		if e := loadProtected(filepath.Join(dir, "wam-account.dpapi"), &saved); e == nil {
			if saved.AccountID == "" || len(saved.AccountID) > 128 {
				failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid saved account identity")
			}
			return saved.AccountID, nil
		} else if !os.IsNotExist(e) {
			failCode("ERR_AUTH_ACCOUNT_SELECTION", "cannot restore saved account identity")
		}
		return "", nil
	})
}

func SignInSharedAccount(ctx context.Context, dir string, hwnd uintptr, dispatch func(func())) error {
	return xbox.SignIn(ctx, hwnd, func(id string) (err error) {
		selectionMu.Lock()
		defer selectionMu.Unlock()
		defer recoverFailure(&err)
		if id == "" || len(id) > 128 {
			failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid selected account identity")
		}
		secureCache(dir)
		if e := saveProtected(filepath.Join(dir, "wam-account.dpapi"), wamAccountSelection{AccountID: id}); e != nil {
			return &Error{Code: "ERR_AUTH_ACCOUNT_SELECTION", Reason: "cannot save selected account", cause: e}
		}
		return nil
	}, dispatch)
}
