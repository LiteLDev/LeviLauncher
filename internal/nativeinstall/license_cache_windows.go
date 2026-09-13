//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/xbox"
	"golang.org/x/sys/windows"
)

type cachedContentKey struct {
	ContentID string
	KeyID     string
	ProductID string `json:",omitempty"`
	Key       []byte
}

type licenseAccount struct {
	id, revision string
	epoch        uint64
}

func clearCachedKeys(keys []cachedContentKey) {
	for i := range keys {
		clear(keys[i].Key)
	}
}

func beginLicenseAccount(ctx context.Context, expectedXUID string) {
	id, err := xbox.GetAccountIDForUser(ctx, expectedXUID)
	if err != nil {
		accountAuthorizationFailed(err)
	}
	selectionMu.Lock()
	defer selectionMu.Unlock()
	lock := lockAccountSelection(active.options.CacheDir)
	defer windows.CloseHandle(lock)
	saved := readLicenseAccount()
	defer clearCachedKeys(saved.Keys)
	if saved.AccountID != id {
		accountAuthorizationFailed(xbox.ErrAccountChanged)
	}
	active.account = &licenseAccount{id: id, revision: saved.Revision, epoch: selectionEpoch.Load()}
}

func readLicenseAccount() wamAccountSelection {
	var record struct {
		AccountID, Revision string
		Keys                json.RawMessage
	}
	defer func() { clear(record.Keys) }()
	if err := loadProtected(filepath.Join(active.options.CacheDir, "wam-account.dpapi"), &record); err != nil {
		failCode("ERR_AUTH_ACCOUNT_SELECTION", "cannot read selected account")
	}
	saved := wamAccountSelection{AccountID: record.AccountID, Revision: record.Revision}
	if saved.AccountID == "" || len(saved.AccountID) > 128 {
		clearCachedKeys(saved.Keys)
		failCode("ERR_AUTH_ACCOUNT_SELECTION", "invalid selected account identity")
	}
	valid := true
	if len(record.Keys) != 0 {
		valid = json.Unmarshal(record.Keys, &saved.Keys) == nil
	}
	valid = valid && len(saved.Keys) <= 256
	for _, key := range saved.Keys {
		valid = valid && validContentID(key.ContentID) && validContentID(key.KeyID) && len(key.Key) == 32
	}
	if !valid {
		clearCachedKeys(saved.Keys)
		saved.Keys = nil
	}
	return saved
}

func withLicenseCache(update func(*wamAccountSelection) bool) {
	selectionMu.Lock()
	defer selectionMu.Unlock()
	checkCanceled()
	lock := lockAccountSelection(active.options.CacheDir)
	defer windows.CloseHandle(lock)
	saved := readLicenseAccount()
	defer func() { clearCachedKeys(saved.Keys) }()
	if active.account == nil || saved.AccountID != active.account.id || saved.Revision != active.account.revision {
		accountAuthorizationFailed(xbox.ErrAccountChanged)
	}
	if update(&saved) {
		checkCanceled()
		must(saveProtected(filepath.Join(active.options.CacheDir, "wam-account.dpapi"), saved))
	}
}

func restoreContentKey() (found bool) {
	withLicenseCache(func(saved *wamAccountSelection) bool {
		for _, key := range saved.Keys {
			if strings.EqualFold(key.KeyID, active.report.KeyID) {
				clear(contentKey)
				contentKey = append([]byte(nil), key.Key...)
				active.report.LicenseType = "Full"
				found = true
				break
			}
		}
		return false
	})
	return found
}

func cachedGameLicenses() (release, preview string) {
	withLicenseCache(func(saved *wamAccountSelection) bool {
		for _, key := range saved.Keys {
			switch key.ProductID {
			case releaseProductID:
				release = "authorized"
			case previewProductID:
				preview = "authorized"
			}
		}
		return false
	})
	return
}

func cacheLicenseKeys(contentID, productID string, keys []licensedContentKey) {
	if active.account == nil {
		return
	}
	withLicenseCache(func(saved *wamAccountSelection) bool {
		changed := false
		for _, key := range keys {
			if key.LicenseType != "Full" {
				continue
			}
			if !validContentID(contentID) || !validContentID(key.KeyID) || len(key.Key) != 32 {
				fail("invalid content key cache entry")
			}
			replaced := false
			for i := range saved.Keys {
				old := &saved.Keys[i]
				if strings.EqualFold(old.KeyID, key.KeyID) && (productID == "" || old.ProductID == "" || old.ProductID == productID) {
					if productID != "" {
						old.ProductID = productID
					}
					clear(old.Key)
					old.Key = append([]byte(nil), key.Key...)
					old.ContentID = contentID
					replaced = true
				}
			}
			if !replaced {
				if len(saved.Keys) >= 256 {
					clear(saved.Keys[0].Key)
					saved.Keys = saved.Keys[1:]
				}
				saved.Keys = append(saved.Keys, cachedContentKey{ContentID: contentID, KeyID: key.KeyID, ProductID: productID, Key: append([]byte(nil), key.Key...)})
			}
			changed = true
		}
		return changed
	})
}

func invalidateChannelKeys(productID string) {
	if active.account == nil {
		return
	}
	withLicenseCache(func(saved *wamAccountSelection) bool {
		kept := make([]cachedContentKey, 0, len(saved.Keys))
		for _, key := range saved.Keys {
			if key.ProductID == productID || key.ProductID == "" {
				clear(key.Key)
			} else {
				kept = append(kept, key)
			}
		}
		changed := len(kept) != len(saved.Keys)
		saved.Keys = kept
		return changed
	})
}
