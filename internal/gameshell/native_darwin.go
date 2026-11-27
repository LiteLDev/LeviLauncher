//go:build darwin

package gameshell

import (
	"os/exec"
)

func DefaultHandlers() Handlers {
	return Handlers{
		OpenDir:     openDirInFinder,
		RevealPath:  revealPathInFinder,
		ShowDialog:  showNativeDialogPlatform,
		FocusDialog: focusNativeDialogPlatform,
	}
}

func StartDefault() (*Broker, error) {
	return Start(DefaultHandlers())
}

func openDirInFinder(dir string) error {
	return exec.Command("/usr/bin/open", "-a", "Finder", dir).Run()
}

func revealPathInFinder(path string) error {
	return exec.Command("/usr/bin/open", "-R", path).Run()
}
