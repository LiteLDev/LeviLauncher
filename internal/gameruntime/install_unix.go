//go:build darwin || linux

package gameruntime

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func homebrewPath() string {
	for _, path := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return path
		}
	}
	path, _ := exec.LookPath("brew")
	return path
}

func InstallPlan() types.WineInstallPlan {
	if runtime.GOOS != "darwin" {
		return types.WineInstallPlan{URL: "https://gitlab.winehq.org/wine/wine/-/wikis/Download"}
	}
	return types.WineInstallPlan{
		CanInstall: homebrewPath() != "",
		Command:    "brew install --cask Sikarugir-App/sikarugir/sikarugir",
		URL:        "https://github.com/Sikarugir-App/Sikarugir#readme",
	}
}

func installerScript(brew string) string {
	quoted := "'" + strings.ReplaceAll(brew, "'", "'\\''") + "'"
	return "#!/bin/zsh\n" +
		"echo 'Installing Sikarugir from its official Homebrew tap.'\n" +
		quoted + " install --cask Sikarugir-App/sikarugir/sikarugir\n" +
		"result=$?\n" +
		"if [ $result -eq 0 ]; then\n" +
		"  echo 'Sikarugir Creator is installed. Create and initialize a Wine wrapper, then return to Levi and scan again.'\n" +
		"  open -a 'Sikarugir Creator' || echo 'Open Sikarugir Creator from Applications to continue.'\n" +
		"else\n" +
		"  echo 'Installation did not complete. Review the Homebrew message above and the official Sikarugir instructions.'\n" +
		"fi\n" +
		"read -r '?Press Return to close this installer…'\nexit $result\n"
}

// StartInstaller opens a visible installer terminal, allowing Homebrew to ask
// for any required input. A successful return means only that it was opened.
func StartInstaller() error {
	if runtime.GOOS != "darwin" {
		return errors.New("use the WineHQ installation instructions for your Linux distribution")
	}
	brew := homebrewPath()
	if brew == "" {
		return errors.New("Homebrew is not installed; use the official installation instructions")
	}
	path := filepath.Join(apppath.ConfigDir(), "install-wine.command")
	if err := fsx.WriteFileAtomic(path, []byte(installerScript(brew)), 0700); err != nil {
		return err
	}
	return exec.Command("open", "-a", "Terminal", path).Run()
}
