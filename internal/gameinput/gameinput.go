package gameinput

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	winreg "golang.org/x/sys/windows/registry"
)

var (
	mu       sync.Mutex
	ensuring bool
)

func IsInstalled() bool {
	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInputRedist`, winreg.READ); err == nil {
		return true
	}
	if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInput`, winreg.READ); err == nil {
		return true
	}
	return false
}

func EnsureInteractive(ctx context.Context) {
	mu.Lock()
	if ensuring {
		mu.Unlock()
		return
	}
	ensuring = true
	mu.Unlock()
	defer func() {
		mu.Lock()
		ensuring = false
		mu.Unlock()
	}()

	if IsInstalled() {
		return
	}

	eventsink.From(ctx).Emit("gameinput.ensure.start", struct{}{})

	if len(msiBytes) == 0 {
		eventsink.From(ctx).Emit("gameinput.download.error", "MSI not embedded")
		log.Println("gameinput msi not embedded")
		eventsink.From(ctx).Emit("gameinput.ensure.done", false)
		return
	}

	dir, _ := apppath.InstallersDir()
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		eventsink.From(ctx).Emit("gameinput.download.error", err.Error())
		log.Println("mkdir installers error:", err)
		eventsink.From(ctx).Emit("gameinput.ensure.done", false)
		return
	}
	dlPath := filepath.Join(dir, "GameInputRedist.msi")
	tmpPath := fsx.PartialPath(dlPath)

	eventsink.From(ctx).Emit("gameinput.download.start", int64(len(msiBytes)))
	if err := os.WriteFile(tmpPath, msiBytes, 0644); err != nil {
		eventsink.From(ctx).Emit("gameinput.download.error", err.Error())
		log.Println("gameinput write error:", err)
		eventsink.From(ctx).Emit("gameinput.ensure.done", false)
		return
	}
	if _, stErr := os.Stat(dlPath); stErr == nil {
		_ = os.Remove(dlPath)
	}
	if err := os.Rename(tmpPath, dlPath); err != nil {
		log.Println("gameinput rename error:", err)
		eventsink.From(ctx).Emit("gameinput.download.error", err.Error())
		eventsink.From(ctx).Emit("gameinput.ensure.done", false)
		return
	}
	eventsink.From(ctx).Emit("gameinput.download.done", struct{}{})
	log.Println("GameInputRedist prepared:", dlPath)

	cmd := exec.Command("msiexec", "/i", dlPath, "/passive")
	if err := cmd.Run(); err != nil {
		log.Println("failed to run GameInput installer:", err)
	}
	installed := false
	for i := 0; i < 30; i++ {
		if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInput`, winreg.READ); err == nil {
			installed = true
			break
		}
		if _, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\GameInputRedist`, winreg.READ); err == nil {
			installed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Println("GameInput installed:", installed)
	eventsink.From(ctx).Emit("gameinput.ensure.done", installed)
}
