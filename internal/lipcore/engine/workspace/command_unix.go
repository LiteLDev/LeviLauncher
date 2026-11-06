//go:build unix

package workspace

import (
	"os/exec"
	"syscall"
)

// processTree is the process group of a script, so cancellation kills every
// process it started.
type processTree struct {
	cmd *exec.Cmd
}

// newProcessTree prepares cmd to run in its own process group. Arguments
// are passed as argv, so cmdLine is unused.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processTree{cmd: cmd}, nil
}

func (*processTree) started() error { return nil }

// kill kills the process group.
func (t *processTree) kill() {
	_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
}

func (*processTree) release() {}
