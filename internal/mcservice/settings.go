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
	c, _ := config.Load()
	c.BaseRoot = r
	if err := config.Save(c); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func ResetBaseRoot() string {
	c, _ := config.Load()
	c.BaseRoot = ""
	if err := config.Save(c); err != nil {
		return "ERR_WRITE_FILE"
	}
	br := apppath.BaseRoot()
	c.BaseRoot = strings.TrimSpace(br)
	if err := config.Save(c); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func CanWriteToDir(path string) bool { return utils.CanWriteDir(path) }

func GetDisableDiscordRPC() bool {
	return config.GetDiscordRPCDisabled()
}

func SetDisableDiscordRPC(disable bool) string {
	c, _ := config.Load()
	c.DisableDiscordRPC = disable
	if err := config.Save(c); err != nil {
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
	c, _ := config.Load()
	c.EnableBetaUpdates = enable
	if err := config.Save(c); err != nil {
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
	c, _ := config.Load()
	c.OnGameLaunch = behavior
	if err := config.Save(c); err != nil {
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
	c, _ := config.Load()
	c.OnGameExit = behavior
	if err := config.Save(c); err != nil {
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
	c, _ := config.Load()
	c.MinimizeToTray = enable
	if err := config.Save(c); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}
