package config

import (
	"os"
	"strings"
	"sync"

	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/apppath"
)

var (
	cachedConfig AppConfig
	configMutex  sync.RWMutex
	isLoaded     bool
)

type AppConfig struct {
	BaseRoot          string `json:"base_root"`
	WindowWidth       int    `json:"window_width"`
	WindowHeight      int    `json:"window_height"`
	DisableDiscordRPC bool   `json:"disable_discord_rpc"`
}

func Load() (AppConfig, error) {
	configMutex.RLock()
	if isLoaded {
		c := cachedConfig
		configMutex.RUnlock()
		return c, nil
	}
	configMutex.RUnlock()

	return Reload()
}

func Reload() (AppConfig, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	var c AppConfig
	p := apppath.ConfigPath()
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &c)
		cachedConfig = c
		isLoaded = true
		apppath.SetBaseRootOverride(c.BaseRoot)
		return c, nil
	}

	db, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(p, db, 0o644)
	cachedConfig = c
	isLoaded = true
	apppath.SetBaseRootOverride(c.BaseRoot)
	return c, nil
}

func Save(c AppConfig) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	p := apppath.ConfigPath()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	cachedConfig = c
	isLoaded = true
	apppath.SetBaseRootOverride(c.BaseRoot)
	return os.WriteFile(p, b, 0o644)
}

func ConfigDir() string {
	return apppath.ConfigDir()
}

func GetBaseRootOverride() string {
	configMutex.RLock()
	if isLoaded {
		v := cachedConfig.BaseRoot
		configMutex.RUnlock()
		return strings.TrimSpace(v)
	}
	configMutex.RUnlock()

	c, _ := Load()
	return strings.TrimSpace(c.BaseRoot)
}

func GetDiscordRPCDisabled() bool {
	configMutex.RLock()
	if isLoaded {
		v := cachedConfig.DisableDiscordRPC
		configMutex.RUnlock()
		return v
	}
	configMutex.RUnlock()

	c, _ := Load()
	return c.DisableDiscordRPC
}
