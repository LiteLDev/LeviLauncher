//go:build darwin

package versionlaunch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
)

const optionsBackupSuffix = ".levibak"

// optionsFiles finds every Minecraft Bedrock options.txt under an instance's
// isolated Users tree (e.g. "Minecraft Bedrock/Users/<id>/games/com.mojang/minecraftpe/options.txt").
func optionsFiles(versionDir string) []string {
	var paths []string
	for _, root := range []string{"Minecraft Bedrock", "Minecraft Bedrock Preview"} {
		usersDir := filepath.Join(versionDir, root, "Users")
		entries, err := os.ReadDir(usersDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			peDir := filepath.Join(usersDir, entry.Name(), "games", "com.mojang", "minecraftpe")
			opt := filepath.Join(peDir, "options.txt")
			bak := opt + optionsBackupSuffix
			if _, err := os.Stat(opt); err == nil {
				paths = append(paths, opt)
			} else if _, err := os.Stat(bak); err == nil {
				paths = append(paths, opt)
			}
		}
	}
	return paths
}

func isHealthyOptionsData(data []byte) bool {
	if len(data) < 128 {
		return false
	}
	return bytes.Contains(data, []byte("gfx_")) && bytes.Contains(data, []byte(":"))
}

// prepareOptionsBackup repairs any truncated options.txt from a previous crash,
// then saves a fresh backup of each healthy options.txt before launch.
func prepareOptionsBackup(versionDir string) {
	for _, opt := range optionsFiles(versionDir) {
		bak := opt + optionsBackupSuffix
		data, err := os.ReadFile(opt)
		if err == nil && isHealthyOptionsData(data) {
			_ = fsx.WriteFileAtomic(bak, data, 0600)
			continue
		}
		if bakData, bakErr := os.ReadFile(bak); bakErr == nil && isHealthyOptionsData(bakData) {
			_ = fsx.WriteFileAtomic(opt, bakData, 0600)
		}
	}
}

// restoreDamagedOptions checks options.txt after the game exits and restores
// the pre-launch backup if a crash or force-quit truncated the file.
func restoreDamagedOptions(versionDir string) {
	for _, opt := range optionsFiles(versionDir) {
		bak := opt + optionsBackupSuffix
		data, err := os.ReadFile(opt)
		if err == nil && isHealthyOptionsData(data) {
			_ = fsx.WriteFileAtomic(bak, data, 0600)
			continue
		}
		if bakData, bakErr := os.ReadFile(bak); bakErr == nil && isHealthyOptionsData(bakData) {
			_ = fsx.WriteFileAtomic(opt, bakData, 0600)
		}
	}
}

// applyOptionsFramePacing synchronizes launcher VSync, FPS pacing, and DLSS
// upscaling overrides into Minecraft Bedrock's options.txt before launch.
// Keeping gfx_vsync aligned prevents the game's DX12 swap chain from requesting
// VSync / SyncInterval=1 when native VSync is forced off, keeping
// gfx_max_framerate at 0 prevents Bedrock's in-game slider from resetting >60
// FPS limits back to 60, and ensuring gfx_upscaling:1 when both RTX and DLSS
// are enabled prevents Bedrock from reverting the in-game upscaling toggle.
// gfx_raytracing is intentionally never forced because worlds without an RTX
// PBR resource pack can misbehave if it is turned on externally.
func applyOptionsFramePacing(versionDir string, p gameruntime.Profile) {
	enableUpscaling := p.EffectiveRayTracing() && p.EffectiveDLSS()
	if p.VSync != "off" && p.VSync != "on" && p.MaxFPS <= 0 && !enableUpscaling {
		return
	}
	for _, opt := range optionsFiles(versionDir) {
		data, err := os.ReadFile(opt)
		if err != nil || !isHealthyOptionsData(data) {
			continue
		}
		updated, changed := rewriteOptionsLaunchOverrides(data, p.VSync, p.MaxFPS, enableUpscaling)
		if !changed {
			continue
		}
		if err := fsx.WriteFileAtomic(opt, updated, 0600); err == nil {
			_ = fsx.WriteFileAtomic(opt+optionsBackupSuffix, updated, 0600)
		}
	}
}

func rewriteOptionsLaunchOverrides(original []byte, vsync string, maxFPS int, enableUpscaling bool) ([]byte, bool) {
	pacingActive := vsync == "off" || vsync == "on" || maxFPS > 0
	if !pacingActive && !enableUpscaling {
		return original, false
	}
	changes := map[string]string{}
	if pacingActive {
		changes["gfx_max_framerate"] = "0"
		changes["gfx_max_framerate_advanced"] = "0"
		switch vsync {
		case "off":
			changes["gfx_vsync"] = "0"
		case "on":
			changes["gfx_vsync"] = "1"
		}
	}
	if enableUpscaling {
		changes["gfx_upscaling"] = "1"
	}

	newline := "\n"
	if bytes.Contains(original, []byte("\r\n")) {
		newline = "\r\n"
	}
	text := strings.ReplaceAll(string(original), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	changed := false
	foundUpscaling := false
	for i, line := range lines {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if key == "gfx_upscaling" {
			foundUpscaling = true
		}
		want, exists := changes[key]
		if !exists || val == want {
			continue
		}
		lines[i] = key + ":" + want
		changed = true
	}
	if enableUpscaling && !foundUpscaling {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = "gfx_upscaling:1"
			lines = append(lines, "")
		} else {
			lines = append(lines, "gfx_upscaling:1")
		}
		changed = true
	}
	if !changed {
		return original, false
	}
	return []byte(strings.Join(lines, newline)), true
}
