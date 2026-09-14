package app

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/liteldev/LeviLauncher/internal/config"
)

const sponsorPromptInterval uint64 = 100

type launcherStartState struct {
	once          sync.Once
	pendingPrompt atomic.Uint64
}

// RecordLauncherStart counts a normal launcher window once per process. Shortcut
// game launches and the update-only window do not call this method.
//
//wails:ignore
func (a *Minecraft) RecordLauncherStart() {
	a.launcherStarts.once.Do(func() {
		var count uint64
		if err := config.Update(func(c *config.AppConfig) {
			c.LauncherLaunchCount++
			count = c.LauncherLaunchCount
		}); err != nil {
			log.Printf("sponsor prompt: failed to record launcher start: %v", err)
			return
		}
		if count > 0 && count%sponsorPromptInterval == 0 {
			a.launcherStarts.pendingPrompt.Store(count)
		}
	})
}

// TakeSponsorPrompt returns the launch milestone once, or zero when no reminder
// is due. Consuming it in Go prevents a WebView reload from showing it again.
func (a *Minecraft) TakeSponsorPrompt() uint64 {
	return a.launcherStarts.pendingPrompt.Swap(0)
}
