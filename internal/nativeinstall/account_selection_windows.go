//go:build windows

package nativeinstall

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/xbox"
	"golang.org/x/sys/windows"
)

var selectionMu sync.Mutex
var selectionEpoch atomic.Uint64

// Identity and keys are committed together: selecting another account replaces
// the entire protected record, including all of the previous account's keys.
type wamAccountSelection struct {
	AccountID string
	Revision  string             `json:",omitempty"`
	Keys      []cachedContentKey `json:",omitempty"`
}

// RestoreSharedAccount loads only the current WAM account preference.
func RestoreSharedAccount(ctx context.Context, dir string) error {
	return xbox.RestoreAccountSelection(ctx, func() (id string, err error) {
		selectionMu.Lock()
		defer selectionMu.Unlock()
		defer recoverFailure(&err)
		var saved struct{ AccountID string }
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

func SignInSharedAccount(ctx context.Context, dir string, hwnd uintptr) error {
	return xbox.SignIn(ctx, hwnd, func(id string) (err error) {
		return saveSharedAccountSelection(dir, id)
	})
}

func saveSharedAccountSelection(dir, id string) (err error) {
	selectionMu.Lock()
	defer selectionMu.Unlock()
	defer recoverFailure(&err)
	if id == "" || len(id) > 128 {
		failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid selected account identity")
	}
	secureCache(dir)
	lock := lockAccountSelection(dir)
	defer windows.CloseHandle(lock)
	path := filepath.Join(dir, "wam-account.dpapi")
	var saved wamAccountSelection
	loadErr := loadProtected(path, &saved)
	defer func() { clearCachedKeys(saved.Keys) }()
	changed := loadErr != nil || saved.AccountID != id || saved.Revision == ""
	if loadErr != nil || saved.AccountID != id {
		clearCachedKeys(saved.Keys)
		saved = wamAccountSelection{AccountID: id}
	}
	if changed {
		saved.Revision = rand.Text()
	}
	if e := saveProtected(path, saved); e != nil {
		return &Error{Code: "ERR_AUTH_ACCOUNT_SELECTION", Reason: "cannot save selected account", cause: e}
	}
	if changed {
		selectionEpoch.Add(1)
	}
	return nil
}

// Cache operations already hold device.lock, but sign-in must be able to replace
// the account during a long installation. This separate, short-lived lock keeps
// another launcher process from overwriting a concurrent account switch.
func lockAccountSelection(dir string) windows.Handle {
	path, err := windows.UTF16PtrFromString(filepath.Join(dir, "account.lock"))
	must(err)
	lock, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		failCode("ERR_AUTH_DEVICE_BUSY", "another process is updating the selected account")
	}
	return lock
}
