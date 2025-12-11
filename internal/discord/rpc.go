package discord

import (
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/config"

	"github.com/axrona/go-discordrpc/client"
)

var rpc *client.Client

func ensure() *client.Client {
	if config.GetDiscordRPCDisabled() {
		return nil
	}
	if rpc != nil {
		return rpc
	}
	c := client.NewClient("1430869336712544346")
	if err := c.Login(); err != nil {
		return nil
	}
	rpc = c
	return rpc
}

func Close() {
	if rpc != nil {
		_ = rpc.Logout()
		rpc = nil
	}
}

func Init() {
	c := ensure()
	if c == nil {
		return
	}
	now := time.Now()
	if err := c.SetActivity(client.Activity{
		Type:       0,
		State:      "Using LeviLauncher",
		Details:    "A Minecraft Bedrock Launcher",
		LargeImage: "",
		LargeText:  "LeviLauncher",
		SmallImage: "",
		SmallText:  "LeviLauncher",
		Party:      nil,
		Timestamps: &client.Timestamps{Start: &now},
		Buttons: []*client.Button{
			{Label: "GitHub", Url: "https://github.com/LiteLDev/LeviLauncher"},
			{Label: "Discord", Url: "https://discord.gg/v5R5P4vRZk"},
		},
	}); err != nil {
		return
	}
}

func SetLauncherIdle() {
	c := ensure()
	if c == nil {
		return
	}
	now := time.Now()
	_ = c.SetActivity(client.Activity{
		Type:       0,
		State:      "Using LeviLauncher",
		Details:    "A Minecraft Bedrock Launcher",
		LargeImage: "appicon",
		LargeText:  "LeviLauncher",
		SmallImage: "appicon",
		SmallText:  "LeviLauncher",
		Party:      nil,
		Timestamps: &client.Timestamps{Start: &now},
		Buttons: []*client.Button{
			{Label: "GitHub", Url: "https://github.com/LiteLDev/LeviLauncher"},
			{Label: "Discord", Url: "https://discord.gg/v5R5P4vRZk"},
		},
	})
}

func SetPlayingVersion(version string) {
	c := ensure()
	if c == nil {
		return
	}
	now := time.Now()
	v := strings.TrimSpace(version)
	s := "Playing Minecraft"
	if v != "" {
		s = "Playing Minecraft " + v
	}
	_ = c.SetActivity(client.Activity{
		Type:       0,
		State:      s,
		Details:    "In-game",
		LargeImage: "",
		LargeText:  "LeviLauncher",
		SmallImage: "",
		SmallText:  "LeviLauncher",
		Party:      nil,
		Timestamps: &client.Timestamps{Start: &now},
		Buttons: []*client.Button{
			{Label: "GitHub", Url: "https://github.com/LiteLDev/LeviLauncher"},
			{Label: "Discord", Url: "https://discord.gg/v5R5P4vRZk"},
		},
	})
}
