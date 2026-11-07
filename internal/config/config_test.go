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

func readConfigFile(t *testing.T, path string) AppConfig {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c AppConfig
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestUpdateKeepsTheRunningDataRoot(t *testing.T) {
	path := useTempConfig(t)
	started := filepath.Join(t.TempDir(), "started")
	b, _ := json.Marshal(AppConfig{BaseRoot: started})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if apppath.BaseRoot() != started {
		t.Fatalf("base root = %q, want %q", apppath.BaseRoot(), started)
	}

	// Another process (the command line) saves a new data root.
	external := filepath.Join(t.TempDir(), "external")
	b, _ = json.Marshal(AppConfig{BaseRoot: external})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	// This process then changes an unrelated setting.
	if err := Update(func(c *AppConfig) { c.MinimizeToTray = true }); err != nil {
		t.Fatal(err)
	}
	if apppath.BaseRoot() != started || GetBaseRootOverride() != started {
		t.Fatalf("running data root moved to %q / %q", apppath.BaseRoot(), GetBaseRootOverride())
	}
	if got := readConfigFile(t, path); got.BaseRoot != external || !got.MinimizeToTray {
		t.Fatalf("config.json = %+v, want base_root %q kept", got, external)
	}

	// A mutation that sets the data root itself moves the process.
	moved := filepath.Join(t.TempDir(), "moved")
	if err := Update(func(c *AppConfig) { c.BaseRoot = moved }); err != nil {
		t.Fatal(err)
	}
	if apppath.BaseRoot() != moved || GetBaseRootOverride() != moved {
		t.Fatalf("base root = %q / %q, want %q", apppath.BaseRoot(), GetBaseRootOverride(), moved)
	}
	if got := readConfigFile(t, path); got.BaseRoot != moved {
		t.Fatalf("config.json base_root = %q, want %q", got.BaseRoot, moved)
	}

	// Setting the root this process already runs on replaces a different
	// one another process saved meanwhile.
	b, _ = json.Marshal(AppConfig{BaseRoot: external, MinimizeToTray: true})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetBaseRoot(moved); err != nil {
		t.Fatal(err)
	}
	if got := readConfigFile(t, path); got.BaseRoot != moved || !got.MinimizeToTray || apppath.BaseRoot() != moved {
		t.Fatalf("config.json = %+v, base root %q; want %q", got, apppath.BaseRoot(), moved)
	}
}
