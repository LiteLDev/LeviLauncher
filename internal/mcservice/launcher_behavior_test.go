package mcservice

import (
	"os"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
)

func TestGameBehaviorSettings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "levilauncher-behavior-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origBaseRoot := apppath.BaseRoot()
	apppath.SetBaseRootOverride(tempDir)
	defer apppath.SetBaseRootOverride(origBaseRoot)
	t.Setenv("APPDATA", tempDir)

	// Reload config to ensure clean state
	_, _ = config.Reload()

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
	if GetMinimizeToTray() != false {
		t.Errorf("default GetMinimizeToTray() = true; want false")
	}
	if err := SetMinimizeToTray(true); err != "" {
		t.Errorf("SetMinimizeToTray(true) failed: %s", err)
	}
	if GetMinimizeToTray() != true {
		t.Errorf("GetMinimizeToTray() = false; want true")
	}
	if err := SetMinimizeToTray(false); err != "" {
		t.Errorf("SetMinimizeToTray(false) failed: %s", err)
	}
	if GetMinimizeToTray() != false {
		t.Errorf("GetMinimizeToTray() = true; want false")
	}
}
