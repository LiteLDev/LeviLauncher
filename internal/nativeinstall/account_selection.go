package nativeinstall

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/xbox"
)

var selectionMu sync.Mutex
var selectionEpoch atomic.Uint64

// Identity and keys are committed together: selecting another account replaces
// the entire protected record, including all of the previous account's keys.
type wamAccountSelection struct {
	AccountID      string
	BrowserRefresh string             `json:",omitempty"`
	Revision       string             `json:",omitempty"`
	Keys           []cachedContentKey `json:",omitempty"`
}

// RestoreSharedAccount loads the selected account and its protected browser refresh token on macOS.
func RestoreSharedAccount(ctx context.Context, dir string) error {
	return xbox.RestoreAccountSelection(ctx, func() (id, refresh string, err error) {
		selectionMu.Lock()
		defer selectionMu.Unlock()
		defer recoverFailure(&err)
		var saved struct{ AccountID, BrowserRefresh string }
		if e := loadProtected(filepath.Join(dir, "wam-account.dpapi"), &saved); e == nil {
			if saved.AccountID == "" || len(saved.AccountID) > 128 {
				failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid saved account identity")
			}
			return saved.AccountID, saved.BrowserRefresh, nil
		} else if !os.IsNotExist(e) {
			failCode("ERR_AUTH_ACCOUNT_SELECTION", "cannot restore saved account identity")
		}
		return "", "", nil
	}, browserRefreshSaver(dir))
}

func SignInSharedAccount(ctx context.Context, dir string, hwnd uintptr, dispatch func(func())) error {
	return xbox.SignIn(ctx, hwnd, func(id string) (err error) {
		return saveSharedAccountSelection(dir, id, "", "")
	}, dispatch)
}

// ForgetSharedAccount signs the launcher out: it deletes the saved account
// selection together with the content keys cached for it, and drops the
// in-memory Xbox session so nothing falls back to the old account. Signing
// out when no account is saved is not an error. The Microsoft account itself
// stays known to Windows.
func ForgetSharedAccount(dir string) (err error) {
	if dir == "" {
		return errors.New("invalid account directory")
	}
	return xbox.ForgetAccountSelection(func() (err error) {
		selectionMu.Lock()
		defer selectionMu.Unlock()
		defer recoverFailure(&err)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			return nil
		}
		lock := lockAccountSelection(dir)
		defer closeCacheLock(lock)
		if e := os.Remove(filepath.Join(dir, "wam-account.dpapi")); e != nil && !os.IsNotExist(e) {
			return &Error{Code: "ERR_AUTH_ACCOUNT_SELECTION", Reason: "cannot remove saved account", cause: e}
		}
		selectionEpoch.Add(1)
		return nil
	})
}

func SignInBrowserAccount(ctx context.Context, dir string, notify func(xbox.BrowserSignInCode)) error {
	persist := browserRefreshSaver(dir)
	if persist == nil {
		persist = func(id, previous, _ string) error {
			if previous != "" {
				return nil
			}
			return saveSharedAccountSelection(dir, id, "", "")
		}
	}
	return xbox.SignInBrowser(ctx, notify, persist)
}

func browserRefreshSaver(dir string) xbox.BrowserRefreshSaver {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return func(id, previous, next string) error {
		return saveSharedAccountSelection(dir, id, previous, next)
	}
}

func saveSharedAccountSelection(dir, id, previousRefresh, refresh string) (err error) {
	selectionMu.Lock()
	defer selectionMu.Unlock()
	defer recoverFailure(&err)
	if id == "" || len(id) > 128 {
		failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid selected account identity")
	}
	secureCache(dir)
	lock := lockAccountSelection(dir)
	defer closeCacheLock(lock)
	path := filepath.Join(dir, "wam-account.dpapi")
	var saved wamAccountSelection
	loadErr := loadProtected(path, &saved)
	defer func() { clearCachedKeys(saved.Keys) }()
	if previousRefresh != "" && (loadErr != nil || saved.AccountID != id || saved.BrowserRefresh != previousRefresh) {
		return xbox.ErrAccountChanged
	}
	changed := loadErr != nil || saved.AccountID != id || saved.Revision == ""
	if loadErr != nil || saved.AccountID != id {
		clearCachedKeys(saved.Keys)
		saved = wamAccountSelection{AccountID: id}
	}
	if changed {
		saved.Revision = rand.Text()
	}
	saved.BrowserRefresh = refresh
	if e := saveProtected(path, saved); e != nil {
		return &Error{Code: "ERR_AUTH_ACCOUNT_SELECTION", Reason: "cannot save selected account", cause: e}
	}
	if changed {
		selectionEpoch.Add(1)
	}
	return nil
}
