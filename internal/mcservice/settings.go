package mcservice

import (
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func GetBaseRoot() string { return utils.BaseRoot() }

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
	br := utils.BaseRoot()
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
