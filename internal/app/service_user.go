package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/launchercore"
)

type UserService struct{}

func NewUserService(_ *Minecraft) *UserService {
	return &UserService{}
}

func (s *UserService) GetGamertagByXuid(xuidStr string) string {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return ""
	}
	tag, err := launchercore.GetGamertagByXuid(xuid)
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserId() string {
	id, err := launchercore.GetLocalUserId()
	if err != nil {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

func (s *UserService) GetLocalUserGamertag() string {
	tag, err := launchercore.GetLocalUserGamertag()
	if err != nil {
		return ""
	}
	return tag
}

func (s *UserService) GetLocalUserGamerPicture(size int) string {
	bin, err := launchercore.GetLocalUserGamerPicture(size)
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

		raw, err := launchercore.GetGamertagByXuid(xuid)
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
	if err := launchercore.ResetSession(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *UserService) XUserGetState() int {
	state, err := launchercore.XUserGetState()
	if err != nil {
		return 1
	}
	return int(state)
}

type UserStatistics struct {
	MinutesPlayed     int64   `json:"minutesPlayed"`
	BlockBroken       int64   `json:"blockBroken"`
	MobKilled         int64   `json:"mobKilled"`
	DistanceTravelled float64 `json:"distanceTravelled"`
}

func (s *UserService) GetAggregatedUserStatistics(xuidStr string) UserStatistics {
	xuid, err := strconv.ParseUint(xuidStr, 10, 64)
	if err != nil {
		return UserStatistics{}
	}
	mp, bb, mk, dt, err := launchercore.GetAggregatedUserStatisticsByXuid(xuid)
	if err != nil {
		return UserStatistics{}
	}
	return UserStatistics{
		MinutesPlayed:     mp,
		BlockBroken:       bb,
		MobKilled:         mk,
		DistanceTravelled: dt,
	}
}
