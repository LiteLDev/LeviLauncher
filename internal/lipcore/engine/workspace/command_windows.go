//go:build windows

package workspace

import (
	"os/exec"
	"syscall"
)

// setCommandLine passes the command line to CreateProcess verbatim, so
// cmd.exe sees the same text as under lip (Go's own escaping differs from
// CliWrap's).
func setCommandLine(cmd *exec.Cmd, cmdLine string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine}
}
