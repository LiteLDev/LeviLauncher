package app

import (
	"github.com/liteldev/LeviLauncher/internal/deeplink"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DeepLinkService hands levilauncher:// links to the frontend. Links are
// kept in deeplink.Default until the frontend takes them, so one that
// arrives before the window loaded is not lost.
type DeepLinkService struct{}

func NewDeepLinkService() *DeepLinkService {
	deeplink.Default.SetNotify(func(r deeplink.Request) {
		go launch.RestoreLauncherWindow()
		if a := application.Get(); a != nil {
			a.Event.Emit(deeplink.EventOpen, r)
		}
	})
	return &DeepLinkService{}
}

// TakePending returns the link waiting to be opened and clears it; nil when
// there is none. The frontend calls it once it can navigate, and again on
// every deeplink.open event.
func (s *DeepLinkService) TakePending() *deeplink.Request {
	return deeplink.Default.Take()
}
