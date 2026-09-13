package mcservice

import (
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/tray"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func GetBaseRoot() string { return apppath.BaseRoot() }

func SetBaseRoot(root string) string {
	r := strings.TrimSpace(root)
	if r == "" {
		return "ERR_INVALID_PATH"
	}
	if err := utils.CreateDir(r); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	if err := config.Update(func(c *config.AppConfig) { c.BaseRoot = r }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func ResetBaseRoot() string {
	if err := config.Update(func(c *config.AppConfig) { c.BaseRoot = "" }); err != nil {
		return "ERR_WRITE_FILE"
	}
	br := strings.TrimSpace(apppath.BaseRoot())
	if err := config.Update(func(c *config.AppConfig) { c.BaseRoot = br }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func CanWriteToDir(path string) bool { return utils.CanWriteDir(path) }

func GetDisableDiscordRPC() bool {
	return config.GetDiscordRPCDisabled()
}

func SetDisableDiscordRPC(disable bool) string {
	if err := config.Update(func(c *config.AppConfig) { c.DisableDiscordRPC = disable }); err != nil {
		return "ERR_WRITE_FILE"
	}
	if disable {
		discord.Close()
	} else {
		discord.SetLauncherIdle()
	}
	return ""
}

func GetEnableBetaUpdates() bool {
	c, _ := config.Load()
	return c.EnableBetaUpdates
}

func SetEnableBetaUpdates(enable bool) string {
	if err := config.Update(func(c *config.AppConfig) { c.EnableBetaUpdates = enable }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func GetGameLaunchBehavior() string {
	return config.GetOnGameLaunch()
}

func SetGameLaunchBehavior(behavior string) string {
	switch behavior {
	case config.OnGameLaunchMinimize, config.OnGameLaunchHide, config.OnGameLaunchClose, config.OnGameLaunchKeep:
		// valid
	default:
		return "ERR_INVALID_PARAM"
	}
	if err := config.Update(func(c *config.AppConfig) { c.OnGameLaunch = behavior }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func GetGameExitBehavior() string {
	return config.GetOnGameExit()
}

func SetGameExitBehavior(behavior string) string {
	switch behavior {
	case config.OnGameExitReopen, config.OnGameExitKeep, config.OnGameExitClose:
		// valid
	default:
		return "ERR_INVALID_PARAM"
	}
	if err := config.Update(func(c *config.AppConfig) { c.OnGameExit = behavior }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

// SetTrayLabels retranslates the tray menu. The launcher language is resolved
// in the frontend, which pushes the rendered labels down whenever it changes.
func SetTrayLabels(show, exit string) string {
	show = strings.TrimSpace(show)
	exit = strings.TrimSpace(exit)
	if show == "" || exit == "" {
		return "ERR_INVALID_PARAM"
	}
	tray.SetLabels(show, exit)
	return ""
}

func GetMinimizeToTray() bool {
	return config.GetMinimizeToTray()
}

func SetMinimizeToTray(enable bool) string {
	if err := config.Update(func(c *config.AppConfig) { c.MinimizeToTray = enable }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}
