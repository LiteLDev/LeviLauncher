//go:build windows

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// ProgramName is how help and hints name the program.
const ProgramName = "LeviLauncher.exe"

const overviewShellNote = "\nThe program does not wait in cmd.exe and some shells; use\n'start /wait LeviLauncher.exe ...' or pipe its output to see results and\n%ERRORLEVEL%.\n"

var (
	testErrProcessAccessDenied error = windows.ERROR_ACCESS_DENIED
	testErrProcessGone         error = windows.ERROR_INVALID_PARAMETER
)

func isFileSharingError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

func isProcessAccessDenied(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

func isProcessGone(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_PARAMETER)
}

func isReparsePoint(fi os.FileInfo) bool {
	if d, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

func openDirInExplorer(dir string) error {
	cmd := exec.Command("explorer.exe", dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // explorer.exe exits 1 even on success
	return nil
}
