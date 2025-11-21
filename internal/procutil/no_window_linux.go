//go:build linux

package procutil

import "os/exec"

func NoWindow(cmd *exec.Cmd) {}