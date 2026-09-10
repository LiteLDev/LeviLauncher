//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"context"
	"crypto/hmac"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
	"golang.org/x/sys/windows"
)

const LoginURL = "https://login.live.com/ppsecure/InlineLogin.srf?id=80604&scid=3&mkt=en-US&Platform=Windows10&clientid=000000004424da1f&hosted=1"

type Account struct {
	Name     string `json:"name"`
	SignedIn bool   `json:"signedIn"`
}

// LoginPrompt belongs to the native host. Credentials never cross Wails bindings.
type LoginPrompt func(context.Context, string) (map[string]string, error)

type userSTSState struct {
	Token, Username, PUID string
	DeviceID              []byte
	ExpiresAt             time.Time
}

func IsLoginURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == "login.live.com" &&
		(u.Port() == "" || u.Port() == "443") && u.User == nil && u.Fragment == ""
}

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

// RestoreAccount only restores this launcher's DPAPI state. The next installation
// exchanges a fresh Store ticket and checks the package entitlement online.
func RestoreAccount(ctx context.Context, dir string) (account Account, err error) {
	err = accountOperation(ctx, dir, func() {
		var state userSTSState
		if err := loadProtected(filepath.Join(dir, "account.dpapi"), &state); os.IsNotExist(err) {
			return
		} else if err != nil {
			failCode("ERR_AUTH_ACCOUNT_CACHE", "account state cannot be decrypted")
		}
		var device ownDevice
		if loadProtected(cachePath(), &device) != nil {
			failCode("ERR_AUTH_ACCOUNT_CACHE", "account device is unavailable")
		}
		validateDevice(device)
		blocks := parseSP(decode64(device.License))
		id := blocks[2]
		if len(id) != 10 || !hmac.Equal(id[2:], state.DeviceID) {
			failCode("ERR_AUTH_ACCOUNT_CACHE", "account device does not match")
		}
		if validUserState(state) {
			account = Account{Name: state.Username, SignedIn: true}
		}
	})
	return
}

func validUserState(s userSTSState) bool {
	return s.Username != "" && len(s.Username) <= 320 && s.PUID != "" && len(s.PUID) <= 128 &&
		len(s.Token) > 0 && len(s.Token) <= 256<<10 && len(s.DeviceID) == 8 && time.Now().Before(s.ExpiresAt)
}

func SignOut(ctx context.Context, dir string) error {
	return accountOperation(ctx, dir, func() {
		if err := os.Remove(filepath.Join(dir, "account.dpapi")); err != nil && !os.IsNotExist(err) {
			must(err)
		}
	})
}

func Authenticate(ctx context.Context, dir string, prompt LoginPrompt) (account Account, err error) {
	if prompt == nil {
		return account, errors.New("missing login host")
	}
	err = accountOperation(ctx, dir, func() {
		ensureDevice()
		ownDeviceTicket()
		next := LoginURL
		var firstIdentity string
		for stage := 0; stage < 2; stage++ {
			checkCanceled()
			da, err := prompt(ctx, next)
			must(err)
			for _, key := range []string{"sDAToken", "sSigninName", "K"} {
				if da[key] == "" {
					failCode("ERR_AUTH_FAILED", "incomplete Microsoft login result")
				}
			}
			if stage == 0 {
				firstIdentity = da["K"]
			} else if !strings.EqualFold(firstIdentity, da["K"]) {
				failCode("ERR_AUTH_ACCOUNT_CHANGED", "account changed during authorization")
			}
			response := postRST(makeUserRST(da, stage == 1, false), deviceProof)
			if len(walk(response.Root(), "Fault")) != 0 {
				links := walk(response.Root(), "inlineauthurl")
				if stage != 0 || len(links) != 1 || !IsLoginURL(links[0].Text()) {
					failCode("ERR_AUTH_FAILED", "Microsoft authorization was not completed")
				}
				next = links[0].Text()
				clear(da)
				continue
			}
			state := userStateFromResponse(response, da)
			clear(da)
			storeUserTicket(state)
			checkCanceled()
			must(saveProtected(filepath.Join(dir, "account.dpapi"), state))
			account = Account{Name: state.Username, SignedIn: true}
			return
		}
		failCode("ERR_AUTH_FAILED", "Microsoft authorization needs a new login")
	})
	return
}

func userStateFromResponse(response *etree.Document, da map[string]string) userSTSState {
	var legacy *etree.Element
	var expires time.Time
	for _, r := range walk(response.Root(), "RequestSecurityTokenResponse") {
		if only(r, "TokenType").Text() != "urn:passport:legacy" {
			continue
		}
		if legacy != nil {
			fail("ambiguous user STS")
		}
		legacy = only(r, "RequestedSecurityToken")
		if lifetime := walk(r, "Lifetime"); len(lifetime) == 1 {
			if end := walk(lifetime[0], "Expires"); len(end) == 1 {
				expires, _ = time.Parse(time.RFC3339, end[0].Text())
			}
		}
	}
	if legacy == nil || len(legacy.ChildElements()) != 1 {
		failCode("ERR_AUTH_FAILED", "Microsoft did not issue a user STS")
	}
	doc := etree.NewDocument()
	doc.SetRoot(legacy.ChildElements()[0].Copy())
	token, err := doc.WriteToString()
	must(err)
	// If the service omits lifetime metadata, require fresh interactive auth after
	// one day instead of treating a cached credential as indefinitely valid.
	if expires.IsZero() {
		expires = time.Now().Add(24 * time.Hour)
	}
	state := userSTSState{Token: token, Username: da["sSigninName"], PUID: da["K"], DeviceID: append([]byte(nil), ownDeviceID...), ExpiresAt: expires}
	if !validUserState(state) {
		failCode("ERR_AUTH_FAILED", "Microsoft issued invalid account state")
	}
	return state
}

func acquireUserTicket() {
	var state userSTSState
	if loadProtected(filepath.Join(active.options.CacheDir, "account.dpapi"), &state) != nil || !validUserState(state) {
		failCode("ERR_AUTH_INTERACTION_REQUIRED", "sign in on the download page")
	}
	storeUserTicket(state)
}

func storeUserTicket(state userSTSState) {
	if !hmac.Equal(state.DeviceID, ownDeviceID) {
		failCode("ERR_AUTH_ACCOUNT_CACHE", "account belongs to another device")
	}
	da := map[string]string{"sDAToken": state.Token, "sSigninName": state.Username, "K": state.PUID}
	response := postRST(makeUserRST(da, false, true), deviceProof)
	if len(walk(response.Root(), "Fault")) != 0 {
		failCode("ERR_AUTH_INTERACTION_REQUIRED", "Microsoft account authorization has expired")
	}
	userTicket = only(only(response.Root(), "RequestedSecurityToken"), "BinarySecurityToken").Text()
	userReference = state.PUID
	if userTicket == "" {
		failCode("ERR_AUTH_FAILED", "Microsoft returned an empty Store ticket")
	}
}
