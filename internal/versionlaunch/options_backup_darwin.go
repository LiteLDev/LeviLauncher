//go:build darwin

package versionlaunch

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/fsx"
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
