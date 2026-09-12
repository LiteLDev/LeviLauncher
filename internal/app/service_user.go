package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

type UserService struct{ window *application.WebviewWindow }

func NewUserService(_ *Minecraft) *UserService {
	// Restore identity before any avatar or installation request can select a default account.
	_ = nativeinstall.RestoreSharedAccount(context.Background(), filepath.Join(config.ConfigDir(), "microsoft-account"))
	return &UserService{}
}

func (s *UserService) GetGamertagByXuid(xuidStr string) string {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return ""
	}
	tag, err := xbox.GetGamertagByXuid(xuid)
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserId() string {
	id, err := xbox.GetLocalUserId()
	if err != nil {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

func (s *UserService) GetLocalUserGamertag() string {
	tag, err := xbox.GetLocalUserGamertag()
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserGamerPicture(size int) string {
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

	cachePath := filepath.Join(config.ConfigDir(), "user_gamertag_map.json")
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
			_ = os.WriteFile(cachePath, b, 0644)
		}
	}

	return out
}

func (s *UserService) ResetSession() string {
	if err := xbox.ResetSession(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *UserService) XUserGetState() int {
	state, err := xbox.XUserGetState()
	if err != nil {
		return 1
	}
	return int(state)
}

func (s *UserService) CheckGameLicenses(ctx context.Context, xuid string) nativeinstall.GameLicenses {
	return nativeinstall.CheckGameLicenses(ctx, filepath.Join(config.ConfigDir(), "microsoft-account"), xuid)
}

// Attach connects native authentication dialogs to the main window.
//
//wails:ignore
func (s *UserService) Attach(window *application.WebviewWindow) { s.window = window }

func (s *UserService) SignIn(ctx context.Context) string {
	if s.window == nil {
		return "ERR_AUTH_FAILED"
	}
	err := nativeinstall.SignInSharedAccount(ctx, filepath.Join(config.ConfigDir(), "microsoft-account"), uintptr(s.window.NativeWindow()), application.InvokeSync)
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "ERR_CANCELED"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "ERR_AUTH_TIMEOUT"
	}
	var failure *nativeinstall.Error
	if errors.As(err, &failure) {
		return failure.Code
	}
	return "ERR_AUTH_FAILED"
}
