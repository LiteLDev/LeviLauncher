//go:build darwin

package versionlaunch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/gameruntime"
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

func TestApplyOptionsFramePacing(t *testing.T) {
	versionDir := t.TempDir()
	peDir := filepath.Join(versionDir, "Minecraft Bedrock", "Users", "12345", "games", "com.mojang", "minecraftpe")
	if err := os.MkdirAll(peDir, 0700); err != nil {
		t.Fatal(err)
	}
	opt := filepath.Join(peDir, "options.txt")
	base := strings.Repeat("gfx_viewdistance:16\r\n", 8) +
		"gfx_vsync:1\r\n" +
		"gfx_max_framerate:60\r\n" +
		"gfx_max_framerate_advanced:60\r\n"
	if err := os.WriteFile(opt, []byte(base), 0600); err != nil {
		t.Fatal(err)
	}

	// Auto VSync + unlimited (0) should leave options.txt untouched.
	applyOptionsFramePacing(versionDir, gameruntime.Profile{VSync: "auto", MaxFPS: 0})
	got, err := os.ReadFile(opt)
	if err != nil || string(got) != base {
		t.Fatalf("auto/0 modified options.txt unexpectedly: %q, %v", got, err)
	}

	// VSync off + 240 FPS should disable gfx_vsync and uncap both in-game limiters while preserving CRLF.
	applyOptionsFramePacing(versionDir, gameruntime.Profile{VSync: "off", MaxFPS: 240})
	got, err = os.ReadFile(opt)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "gfx_vsync:0\r\n") ||
		!strings.Contains(text, "gfx_max_framerate:0\r\n") ||
		!strings.Contains(text, "gfx_max_framerate_advanced:0\r\n") {
		t.Fatalf("unexpected rewritten options.txt: %q", text)
	}

	// VSync on should set gfx_vsync:1 and keep limiters at 0.
	applyOptionsFramePacing(versionDir, gameruntime.Profile{VSync: "on", MaxFPS: 0})
	got, err = os.ReadFile(opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "gfx_vsync:1\r\n") {
		t.Fatalf("expected gfx_vsync:1, got: %q", string(got))
	}

	// Only DLSS without RTX should not touch gfx_upscaling or gfx_raytracing.
	withRTX := strings.Repeat("gfx_viewdistance:16\r\n", 8) +
		"gfx_upscaling:0\r\n" +
		"gfx_raytracing:0\r\n"
	if err := os.WriteFile(opt, []byte(withRTX), 0600); err != nil {
		t.Fatal(err)
	}
	applyOptionsFramePacing(versionDir, gameruntime.Profile{Graphics: "mullion", VSync: "auto", EnableDLSS: true})
	got, err = os.ReadFile(opt)
	if err != nil || string(got) != withRTX {
		t.Fatalf("DLSS-only unexpectedly modified options.txt: %q, %v", got, err)
	}

	// RTX + DLSS should ensure gfx_upscaling:1 while never forcing gfx_raytracing:1.
	applyOptionsFramePacing(versionDir, gameruntime.Profile{Graphics: "mullion", VSync: "auto", EnableRayTracing: true, EnableDLSS: true})
	got, err = os.ReadFile(opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "gfx_upscaling:1\r\n") || !strings.Contains(string(got), "gfx_raytracing:0\r\n") {
		t.Fatalf("RTX+DLSS options.txt mismatch: %q", string(got))
	}

	// When gfx_upscaling is missing from options.txt, RTX + DLSS appends gfx_upscaling:1.
	noUpscaling := strings.Repeat("gfx_viewdistance:16\n", 10)
	if err := os.WriteFile(opt, []byte(noUpscaling), 0600); err != nil {
		t.Fatal(err)
	}
	applyOptionsFramePacing(versionDir, gameruntime.Profile{Graphics: "d3dmetal", VSync: "auto", EnableRayTracing: true, EnableDLSS: true})
	got, err = os.ReadFile(opt)
	if err != nil || !strings.Contains(string(got), "gfx_upscaling:1\n") {
		t.Fatalf("missing gfx_upscaling was not appended: %q, %v", got, err)
	}
}
