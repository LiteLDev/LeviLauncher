//go:build darwin

package versionlaunch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionsBackupAndRestoreOnTruncation(t *testing.T) {
	versionDir := t.TempDir()
	peDir := filepath.Join(versionDir, "Minecraft Bedrock", "Users", "12345", "games", "com.mojang", "minecraftpe")
	if err := os.MkdirAll(peDir, 0700); err != nil {
		t.Fatal(err)
	}
	opt := filepath.Join(peDir, "options.txt")
	healthy := strings.Repeat("gfx_viewdistance:16\ngfx_vsync:0\n", 10)
	if err := os.WriteFile(opt, []byte(healthy), 0600); err != nil {
		t.Fatal(err)
	}

	prepareOptionsBackup(versionDir)
	bak, err := os.ReadFile(opt + optionsBackupSuffix)
	if err != nil || string(bak) != healthy {
		t.Fatalf("backup mismatch: %q, %v", bak, err)
	}

	// Simulate crash truncation.
	if err := os.WriteFile(opt, []byte("gfx_"), 0600); err != nil {
		t.Fatal(err)
	}
	restoreDamagedOptions(versionDir)
	got, err := os.ReadFile(opt)
	if err != nil || string(got) != healthy {
		t.Fatalf("truncated options.txt was not restored: %q, %v", got, err)
	}

	// Simulate missing options.txt recovered before next launch.
	if err := os.Remove(opt); err != nil {
		t.Fatal(err)
	}
	prepareOptionsBackup(versionDir)
	got, err = os.ReadFile(opt)
	if err != nil || string(got) != healthy {
		t.Fatalf("missing options.txt was not restored: %q, %v", got, err)
	}
}
