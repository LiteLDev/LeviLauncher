package processinfo

import (
	"os/exec"
	"syscall"
)

func ConfigureChild(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
