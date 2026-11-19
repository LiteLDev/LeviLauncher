package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/diagnose"
)

func TestWineDiagnosticsUseSelectedContainer(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "wine")
	prefix := filepath.Join(root, "prefix")
	t.Setenv("APPDATA", root)
	t.Setenv("LEVI_WINE_APP", "")
	t.Setenv("LEVI_WINE_EXECUTABLE", engine)
	t.Setenv("LEVI_WINEPREFIX", prefix)

	runChecks := func() map[string]diagnose.Check {
		checks := make(map[string]diagnose.Check)
		for _, check := range wineDiagnosticChecks() {
			checks[check.ID] = check.Run(context.Background())
		}
		return checks
	}

	checks := runChecks()
	if check := checks["wine_runtime"]; check.Status != diagnose.StatusFail || check.Params["reason"] != "missing" {
		t.Fatalf("missing engine: %+v", check)
	}
	if err := os.WriteFile(engine, []byte("#!/bin/sh\necho wine-test\n"), 0700); err != nil {
		t.Fatal(err)
	}
	checks = runChecks()
	if check := checks["wine_runtime"]; check.Status != diagnose.StatusFail || check.Params["reason"] != "prefix" {
		t.Fatalf("uninitialized prefix: %+v", check)
	}
	if check := checks["wine_gameinput"]; check.Status != diagnose.StatusSkip {
		t.Fatalf("GameInput check without a valid prefix: %+v", check)
	}

	system32 := filepath.Join(prefix, "drive_c", "windows", "system32")
	if err := os.MkdirAll(system32, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(system32, "GameInputRedist.dll"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	checks = runChecks()
	if check := checks["wine_graphics"]; check.Status != diagnose.StatusInfo || check.Params["backend"] != "auto" {
		t.Fatalf("configured graphics backend: %+v", check)
	}
	if check := checks["wine_gameinput"]; check.Status != diagnose.StatusOK {
		t.Fatalf("GameInput present: %+v", check)
	}
	if check := checks["wine_vcruntime"]; check.Status != diagnose.StatusInfo {
		t.Fatalf("missing Visual C++ runtime: %+v", check)
	}
	runtimeCheck := checks["wine_runtime"]
	if checks["wine_rosetta"].Status == diagnose.StatusFail {
		if runtimeCheck.Status != diagnose.StatusSkip || runtimeCheck.Params["reason"] != "rosetta" {
			t.Fatalf("engine probe without Rosetta: %+v", runtimeCheck)
		}
	} else if runtimeCheck.Status != diagnose.StatusOK || runtimeCheck.Params["version"] != "wine-test" {
		t.Fatalf("engine version: %+v", runtimeCheck)
	}
}
