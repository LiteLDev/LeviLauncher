package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type AppConfig struct {
    BaseRoot     string `json:"base_root"`
    WindowWidth  int    `json:"window_width"`
    WindowHeight int    `json:"window_height"`
    DisableDiscordRPC bool `json:"disable_discord_rpc"`
}

func localAppData() string {
	if v := os.Getenv("APPDATA"); strings.TrimSpace(v) != "" {
		return v
	}
	if v, _ := os.UserCacheDir(); strings.TrimSpace(v) != "" {
		return v
	}
	return "."
}

func configPath() string {
	exeName := func() string {
		if exe, err := os.Executable(); err == nil {
			b := strings.TrimSpace(filepath.Base(exe))
			if b != "" {
				return strings.ToLower(b)
			}
		}
		return "levilauncher.exe"
	}()
	base := filepath.Join(localAppData(), exeName)
	_ = os.MkdirAll(base, 0o755)
	return filepath.Join(base, "config.json")
}

func oldConfigPath() string {
	la := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if la == "" {
		return ""
	}
	p := filepath.Join(la, "levilauncher", "config.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	p2 := filepath.Join(la, "LeviLauncher", "config.json")
	if _, err := os.Stat(p2); err == nil {
		return p2
	}
	return ""
}

func Load() (AppConfig, error) {
	var c AppConfig
	p := configPath()
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &c)
		return c, nil
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if old := oldConfigPath(); old != "" {
		if ob, err := os.ReadFile(old); err == nil {
			_ = json.Unmarshal(ob, &c)
			if err2 := os.WriteFile(p, ob, 0o644); err2 == nil {
				_ = os.Remove(old)
			}
			return c, nil
		}
	}
	db, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(p, db, 0o644)
	return c, nil
}

func Save(c AppConfig) error {
	p := configPath()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func GetBaseRootOverride() string {
    c, _ := Load()
    return strings.TrimSpace(c.BaseRoot)
}

func GetDiscordRPCDisabled() bool {
    c, _ := Load()
    return c.DisableDiscordRPC
}
