package mcservice

import (
	"testing"

	"github.com/liteldev/LeviLauncher/internal/config"
)

func TestGameBehaviorSettings(t *testing.T) {
	// Registered before t.Setenv so that cleanups run in the right order:
	// APPDATA is back to its real value by the time this reload rebuilds the
	// cached config and the base-root override the rest of the package reads.
	t.Cleanup(func() {
		_, _ = config.Reload()
	})
	t.Setenv("APPDATA", t.TempDir())

	if _, err := config.Reload(); err != nil {
		t.Fatalf("config.Reload() = %v", err)
	}

	// 1. Defaults
	defaultLaunch := GetGameLaunchBehavior()
	if defaultLaunch != config.OnGameLaunchMinimize {
		t.Errorf("default GetGameLaunchBehavior() = %q; want %q", defaultLaunch, config.OnGameLaunchMinimize)
	}

	defaultExit := GetGameExitBehavior()
	if defaultExit != config.OnGameExitReopen {
		t.Errorf("default GetGameExitBehavior() = %q; want %q", defaultExit, config.OnGameExitReopen)
	}

	// 2. Set valid launch behaviors
	validLaunch := []string{
		config.OnGameLaunchMinimize,
		config.OnGameLaunchHide,
		config.OnGameLaunchClose,
		config.OnGameLaunchKeep,
	}
	for _, b := range validLaunch {
		errCode := SetGameLaunchBehavior(b)
		if errCode != "" {
			t.Errorf("SetGameLaunchBehavior(%q) failed with error code: %s", b, errCode)
		}
		got := GetGameLaunchBehavior()
		if got != b {
			t.Errorf("GetGameLaunchBehavior() = %q; want %q", got, b)
		}
	}

	// 3. Reject invalid launch behavior
	invalidLaunchErr := SetGameLaunchBehavior("invalid_behavior")
	if invalidLaunchErr != "ERR_INVALID_PARAM" {
		t.Errorf("SetGameLaunchBehavior(\"invalid_behavior\") = %q; want \"ERR_INVALID_PARAM\"", invalidLaunchErr)
	}

	// 4. Set valid exit behaviors
	validExit := []string{
		config.OnGameExitReopen,
		config.OnGameExitKeep,
		config.OnGameExitClose,
	}
	for _, b := range validExit {
		errCode := SetGameExitBehavior(b)
		if errCode != "" {
			t.Errorf("SetGameExitBehavior(%q) failed with error code: %s", b, errCode)
		}
		got := GetGameExitBehavior()
		if got != b {
			t.Errorf("GetGameExitBehavior() = %q; want %q", got, b)
		}
	}

	// 5. Reject invalid exit behavior
	invalidExitErr := SetGameExitBehavior("destroy_world")
	if invalidExitErr != "ERR_INVALID_PARAM" {
		t.Errorf("SetGameExitBehavior(\"destroy_world\") = %q; want \"ERR_INVALID_PARAM\"", invalidExitErr)
	}

	// 6. Minimize to tray settings
	if GetMinimizeToTray() {
		t.Errorf("default GetMinimizeToTray() = true; want false")
	}
	if err := SetMinimizeToTray(true); err != "" {
		t.Errorf("SetMinimizeToTray(true) failed: %s", err)
	}
	if !GetMinimizeToTray() {
		t.Errorf("GetMinimizeToTray() = false; want true")
	}
	if err := SetMinimizeToTray(false); err != "" {
		t.Errorf("SetMinimizeToTray(false) failed: %s", err)
	}
	if GetMinimizeToTray() {
		t.Errorf("GetMinimizeToTray() = true; want false")
	}
}
