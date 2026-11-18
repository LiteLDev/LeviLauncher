package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

type UserService struct {
	window             *application.WebviewWindow
	restoreAccountOnce sync.Once
}

func (s *UserService) IsElevated() bool {
	return processinfo.IsElevated()
}

func NewUserService(_ *Minecraft) *UserService {
	s := &UserService{}
	if runtime.GOOS == "windows" {
		s.restoreAccount()
	}
	return s
}

func (s *UserService) restoreAccount() {
	// Native credential access may ask for Keychain permission. Defer it to
	// account requests so it cannot block the main thread before Wails opens.
	// Every account request still waits for the same restoration to finish.
	s.restoreAccountOnce.Do(func() {
		_ = nativeinstall.RestoreSharedAccount(context.Background(), apppath.AccountsDir())
	})
}

func (s *UserService) GetGamertagByXuid(xuidStr string) string {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return ""
	}
	s.restoreAccount()
	tag, err := xbox.GetGamertagByXuid(xuid)
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserId() string {
	s.restoreAccount()
	id, err := xbox.GetLocalUserId()
	if err != nil {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

// LocalUserStatus is the signed-in account, or why there is none.
type LocalUserStatus struct {
	Xuid string `json:"xuid"`
	// Unreachable is true when an account is selected but Xbox Live could
	// not be reached to confirm it, so the user is not actually signed out.
	Unreachable bool `json:"unreachable"`
}

func (s *UserService) GetLocalUserStatus() LocalUserStatus {
	id, err := xbox.GetLocalUserId()
	if err != nil || id == 0 {
		return LocalUserStatus{Unreachable: xbox.IsUnreachable(err)}
	}
	return LocalUserStatus{Xuid: strconv.FormatUint(id, 10)}
}

func (s *UserService) GetLocalUserGamertag() string {
	s.restoreAccount()
	tag, err := xbox.GetLocalUserGamertag()
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserGamerPicture(size int) string {
	s.restoreAccount()
	bin, err := xbox.GetLocalUserGamerPicture(size)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(bin)
}

func (s *UserService) GetUserGamertagMap(usersRoot string) map[string]string {
	usersRoot = strings.TrimSpace(usersRoot)
	if usersRoot == "" {
		return map[string]string{}
	}
	s.restoreAccount()

	cachePath := apppath.GamertagsPath()
	out := map[string]string{}
	loadMap := func(p string) {
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			return
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			return
		}
		for k, v := range m {
			kk := strings.TrimSpace(k)
			vv := strings.TrimSpace(v)
			if kk == "" || vv == "" {
				continue
			}
			if strings.TrimSpace(out[kk]) == "" {
				out[kk] = vv
			}
		}
	}

	loadMap(cachePath)

	entries, err := os.ReadDir(usersRoot)
	if err != nil {
		return out
	}

	changed := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		obf := strings.TrimSpace(e.Name())
		if obf == "" || strings.EqualFold(obf, "shared") || obf == "9556213259376595538" {
			continue
		}
		if v := strings.TrimSpace(out[obf]); v != "" {
			continue
		}

		ss := filepath.Join(usersRoot, obf, "games", "com.mojang", "Screenshots")
		xuidDirs, err := os.ReadDir(ss)
		if err != nil {
			continue
		}

		var xuidStr string
		for _, xd := range xuidDirs {
			if xd.IsDir() {
				xuidStr = strings.TrimSpace(xd.Name())
				if xuidStr != "" {
					break
				}
			}
		}
		if xuidStr == "" {
			continue
		}

		xuid, err := strconv.ParseUint(xuidStr, 10, 64)
		if err != nil {
			continue
		}

		raw, err := xbox.GetGamertagByXuid(xuid)
		if err != nil {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		parts := strings.SplitN(raw, "|", 2)
		gamertag := strings.TrimSpace(parts[0])
		if gamertag == "" {
			continue
		}

		key := obf
		if len(parts) == 2 {
			if k := strings.TrimSpace(parts[1]); k != "" {
				key = k
			}
		}

		if strings.TrimSpace(out[key]) != gamertag {
			out[key] = gamertag
			changed = true
		}
	}

	if changed {
		if b, err := json.Marshal(out); err == nil {
			_ = fsx.WriteFileAtomic(cachePath, b, 0o644)
		}
	}

	return out
}

// SignOut removes the saved browser account from the launcher on macOS.
func (s *UserService) SignOut() string {
	if runtime.GOOS != "darwin" {
		return "ERR_AUTH_FAILED"
	}
	s.restoreAccount()
	if err := nativeinstall.ForgetSharedAccount(apppath.AccountsDir()); err != nil {
		return nativeinstall.AuthErrorCode(err)
	}
	return ""
}

func (s *UserService) ResetSession() string {
	s.restoreAccount()
	if err := xbox.ResetSession(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *UserService) XUserGetState() int {
	s.restoreAccount()
	state, err := xbox.XUserGetState()
	if err != nil {
		return 1
	}
	return int(state)
}

func (s *UserService) CheckGameLicenses(ctx context.Context, xuid string) nativeinstall.GameLicenses {
	s.restoreAccount()
	return nativeinstall.CheckGameLicenses(ctx, apppath.AccountsDir(), xuid)
}

func (s *UserService) RefreshGameLicenses(ctx context.Context, xuid string) nativeinstall.GameLicenses {
	s.restoreAccount()
	return nativeinstall.RefreshGameLicenses(ctx, apppath.AccountsDir(), xuid)
}

// Attach connects native authentication dialogs to the main window.
//
//wails:ignore
func (s *UserService) Attach(window *application.WebviewWindow) { s.window = window }

func (s *UserService) SignIn(ctx context.Context, requestID string) string {
	if s.window == nil {
		return "ERR_AUTH_FAILED"
	}
	s.restoreAccount()
	dir := apppath.AccountsDir()
	hwnd := uintptr(s.window.NativeWindow())
	var err error
	if runtime.GOOS != "windows" || hwnd == 0 {
		err = nativeinstall.SignInBrowserAccount(ctx, dir, func(code xbox.BrowserSignInCode) {
			application.Get().Event.Emit("microsoft.browser.signin", map[string]any{
				"requestID":       requestID,
				"userCode":        code.UserCode,
				"verificationURL": code.VerificationURL,
				"expiresIn":       code.ExpiresIn,
			})
		})
	} else {
		err = nativeinstall.SignInSharedAccount(ctx, dir, hwnd, application.InvokeSync)
	}
	if err == nil {
		return ""
	}
	log.Printf("UserService.SignIn: %v", err)
	return nativeinstall.AuthErrorCode(err)
}
