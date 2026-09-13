package mcservice

import (
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/uwp"
)

func versionExecutable(dir string) string {
	if manifest, err := uwp.ReadManifest(dir); err == nil {
		return filepath.Join(dir, manifest.Applications[0].Executable)
	}
	return filepath.Join(dir, "Minecraft.Windows.exe")
}
