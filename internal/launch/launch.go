package launch

import (
	"context"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/registry"
)

const (
	EventMcLaunchStart         = "mc.launch.start"
	EventMcLaunchDone          = "mc.launch.done"
	EventMcLaunchFailed        = "mc.launch.failed"
	EventGamingServicesMissing = "gamingservices.missing"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procFindWindowW     = user32.NewProc("FindWindowW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

func FindWindowByTitleExact(title string) bool {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
	if hwnd == 0 {
		return false
	}

	isVisible, _, _ := procIsWindowVisible.Call(hwnd)
	return isVisible != 0
}

func EnsureGamingServicesInstalled(ctx context.Context) bool {
	if _, err := registry.GetAppxInfo("Microsoft.GamingServices"); err != nil {
		application.Get().Event.Emit(EventGamingServicesMissing, struct{}{})
		return false
	}
	return true
}

func MonitorMinecraftWindow(ctx context.Context) {
    application.Get().Event.Emit(EventMcLaunchStart, struct{}{})
    const maxWait = 60
    for i := 0; i < maxWait; i++ {
        if FindWindowByTitleExact("Minecraft") || FindWindowByTitleExact("Minecraft Preview") {
            application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
            w := application.Get().Window.Current()
            if w != nil {
                w.Minimise()
            }
            return
        }
        time.Sleep(1 * time.Second)
    }
    application.Get().Event.Emit(EventMcLaunchDone, struct{}{})
}
