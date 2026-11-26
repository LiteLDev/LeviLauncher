//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/config"
)

func TestRotateDarwinStartupLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "startup.log")

	if flag := rotateDarwinStartupLogs(logPath); flag != os.O_TRUNC {
		t.Fatalf("first rotation flag = %d, want O_TRUNC", flag)
	}
	if err := os.WriteFile(logPath, []byte("run-1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if flag := rotateDarwinStartupLogs(logPath); flag != os.O_TRUNC {
		t.Fatalf("second rotation flag = %d, want O_TRUNC", flag)
	}
	got, err := os.ReadFile(filepath.Join(dir, "startup.1.log"))
	if err != nil || string(got) != "run-1" {
		t.Fatalf("expected startup.1.log == run-1, got %q (%v)", string(got), err)
	}
}

func TestResolveDarwinWindowSize(t *testing.T) {
	w, h, persist := resolveDarwinWindowSize(config.AppConfig{})
	if w != darwinDefaultWindowWidth || h != darwinDefaultWindowHeight || !persist {
		t.Fatalf("zero config: got %dx%d persist=%v", w, h, persist)
	}
	w, h, persist = resolveDarwinWindowSize(config.AppConfig{WindowWidth: 500, WindowHeight: 400})
	if w != darwinMinWindowWidth || h != darwinMinWindowHeight || persist {
		t.Fatalf("below min: got %dx%d persist=%v", w, h, persist)
	}
	w, h, persist = resolveDarwinWindowSize(config.AppConfig{WindowWidth: 1400, WindowHeight: 900})
	if w != 1400 || h != 900 || persist {
		t.Fatalf("saved size: got %dx%d persist=%v", w, h, persist)
	}
}

func TestHandleDarwinSecondInstanceArgs(t *testing.T) {
	t.Run("focus on empty args", func(t *testing.T) {
		focused := false
		handleDarwinSecondInstanceArgs(nil,
			func(string) error { t.Fatal("unexpected link"); return nil },
			func(string) { t.Fatal("unexpected launch") },
			func() { focused = true },
		)
		if !focused {
			t.Fatal("expected focus callback")
		}
	})

	t.Run("launch arg", func(t *testing.T) {
		var launched string
		handleDarwinSecondInstanceArgs([]string{"--launch=Survival"},
			func(string) error { t.Fatal("unexpected link"); return nil },
			func(name string) { launched = name },
			func() { t.Fatal("unexpected focus") },
		)
		if launched != "Survival" {
			t.Fatalf("launched = %q, want Survival", launched)
		}
	})

	t.Run("deep link delivers and focuses", func(t *testing.T) {
		var delivered string
		focused := false
		handleDarwinSecondInstanceArgs([]string{"levilauncher://mods/example"},
			func(raw string) error { delivered = raw; return nil },
			func(string) { t.Fatal("unexpected launch") },
			func() { focused = true },
		)
		if delivered != "levilauncher://mods/example" || !focused {
			t.Fatalf("delivered=%q focused=%v", delivered, focused)
		}
	})
}
