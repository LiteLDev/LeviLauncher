//go:build windows

package procutil

import (
    "os/exec"
    "syscall"
)

func NoWindow(cmd *exec.Cmd) {
    if cmd != nil {
        cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    }
}