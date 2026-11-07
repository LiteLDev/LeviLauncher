package config

import (
	"os"
	"path/filepath"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(dir, "roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "local"))
	configMutex.Lock()
	cachedConfig, isLoaded = AppConfig{}, false
	configMutex.Unlock()
	t.Cleanup(func() {
		configMutex.Lock()
		cachedConfig, isLoaded = AppConfig{}, false
		configMutex.Unlock()
		apppath.SetBaseRootOverride("")
	})
	if err := os.MkdirAll(apppath.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return apppath.ConfigPath()
}

func TestUpdateKeepsChangesSavedByAnotherProcess(t *testing.T) {
	path := useTempConfig(t)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	// Another launcher process writes the file after this one loaded it.
	other := AppConfig{WindowWidth: 1234, MinimizeToTray: true}
	b, _ := json.Marshal(other)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetCurrentInstance("  Survival "); err != nil {
		t.Fatal(err)
	}
	var got AppConfig
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.WindowWidth != 1234 || !got.MinimizeToTray {
		t.Fatalf("the other process's settings were overwritten: %+v", got)
	}
	if got.CurrentInstance != "Survival" || GetCurrentInstance() != "Survival" {
		t.Fatalf("current instance = %q / %q", got.CurrentInstance, GetCurrentInstance())
	}
	if err := SetCurrentInstance(""); err != nil {
		t.Fatal(err)
	}
	if GetCurrentInstance() != "" {
		t.Fatal("current instance not cleared")
	}
}
