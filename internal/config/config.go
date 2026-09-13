package config

import (
	"fmt"
	"log"
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
	EnableBetaUpdates bool   `json:"enable_beta_updates"`
	LoaderMigratedV1  bool   `json:"loader_migrated_v1"`
	OnGameLaunch      string `json:"on_game_launch,omitempty"`
	OnGameExit        string `json:"on_game_exit,omitempty"`
	MinimizeToTray    bool   `json:"minimize_to_tray,omitempty"`
}

const (
	OnGameLaunchMinimize = "minimize"
	OnGameLaunchHide     = "hide"
	OnGameLaunchClose    = "close"
	OnGameLaunchKeep     = "keep"

	OnGameExitReopen = "reopen"
	OnGameExitKeep   = "keep"
	OnGameExitClose  = "close"
)

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

	return reloadLocked()
}

// Update applies mutate to the current config and writes the result back,
// holding the write lock throughout. Read-modify-write through Load and Save
// leaves a window in which a concurrent writer's change is read back stale and
// then overwritten.
func Update(mutate func(*AppConfig)) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	c := cachedConfig
	if !isLoaded {
		var err error
		if c, err = reloadLocked(); err != nil {
			return err
		}
	}
	mutate(&c)
	return saveLocked(c)
}

func reloadLocked() (AppConfig, error) {
	var c AppConfig
	p := apppath.ConfigPath()
	if b, err := os.ReadFile(p); err == nil {
		if err := json.Unmarshal(b, &c); err != nil {
			log.Printf("config.Reload: invalid config at %s: %v", p, err)
			return AppConfig{}, fmt.Errorf("ERR_CONFIG_CORRUPTED: %w", err)
		}
		cachedConfig = c
		isLoaded = true
		apppath.SetBaseRootOverride(c.BaseRoot)
		return c, nil
	} else if !os.IsNotExist(err) {
		log.Printf("config.Reload: read config failed at %s: %v", p, err)
		return AppConfig{}, fmt.Errorf("ERR_CONFIG_READ_FAILED: %w", err)
	}

	db, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("config.Reload: marshal default config failed: %v", err)
		return AppConfig{}, err
	}
	if err := os.WriteFile(p, db, 0o644); err != nil {
		log.Printf("config.Reload: write default config failed at %s: %v", p, err)
		return c, fmt.Errorf("ERR_CONFIG_WRITE_FAILED: %w", err)
	}
	cachedConfig = c
	isLoaded = true
	apppath.SetBaseRootOverride(c.BaseRoot)
	return c, nil
}

func Save(c AppConfig) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	return saveLocked(c)
}

func saveLocked(c AppConfig) error {
	p := apppath.ConfigPath()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return err
	}
	cachedConfig = c
	isLoaded = true
	apppath.SetBaseRootOverride(c.BaseRoot)
	return nil
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

func GetOnGameLaunch() string {
	configMutex.RLock()
	if isLoaded {
		v := cachedConfig.OnGameLaunch
		configMutex.RUnlock()
		if v == "" {
			return OnGameLaunchMinimize
		}
		return v
	}
	configMutex.RUnlock()

	c, _ := Load()
	if c.OnGameLaunch == "" {
		return OnGameLaunchMinimize
	}
	return c.OnGameLaunch
}

func GetOnGameExit() string {
	configMutex.RLock()
	if isLoaded {
		v := cachedConfig.OnGameExit
		configMutex.RUnlock()
		if v == "" {
			return OnGameExitReopen
		}
		return v
	}
	configMutex.RUnlock()

	c, _ := Load()
	if c.OnGameExit == "" {
		return OnGameExitReopen
	}
	return c.OnGameExit
}

func GetMinimizeToTray() bool {
	configMutex.RLock()
	if isLoaded {
		v := cachedConfig.MinimizeToTray
		configMutex.RUnlock()
		return v
	}
	configMutex.RUnlock()

	c, _ := Load()
	return c.MinimizeToTray
}
