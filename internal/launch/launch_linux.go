//go:build linux

package launch

import (
    "context"
    "github.com/wailsapp/wails/v3/pkg/application"
)

const (
    EventMcLaunchStart         = "mc.launch.start"
    EventMcLaunchDone          = "mc.launch.done"
    EventMcLaunchFailed        = "mc.launch.failed"
    EventGamingServicesMissing = "gamingservices.missing"
)

func FindWindowByTitleExact(title string) bool { return false }

func EnsureGamingServicesInstalled(ctx context.Context) bool { return true }

func MonitorMinecraftWindow(ctx context.Context) {
    application.Get().Event.Emit(EventMcLaunchStart, struct{}{})
    application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
}