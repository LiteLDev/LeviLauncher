//go:build !windows

package cli

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// ProgramName is how help and hints name the program.
const ProgramName = "levilauncher"

const overviewShellNote = ""

var (
	testErrProcessAccessDenied error = syscall.EPERM
	testErrProcessGone         error = syscall.ESRCH
)

func isFileSharingError(err error) bool {
	return errors.Is(err, syscall.EBUSY)
}

func isProcessAccessDenied(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrPermission)
}

func isProcessGone(err error) bool {
	return errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone)
}

func isReparsePoint(os.FileInfo) bool {
	return false
}

func openDirInExplorer(dir string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("/usr/bin/open", dir).Start()
	}
	return exec.Command("xdg-open", dir).Start()
}
