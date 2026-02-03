package extractor

import (
	"github.com/liteldev/LeviLauncher/internal/launchercore"
)

func Init() {
	launchercore.Init()
}

func Get(msixvcPath string, outDir string) (int, string) {
	return launchercore.Extract(msixvcPath, outDir)
}

func GetWithPipe(msixvcPath string, outDir string, pipeName string) (int, string) {
	return launchercore.ExtractWithPipe(msixvcPath, outDir, pipeName)
}
