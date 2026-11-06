//go:build unix

package workspace

import (
	"os/exec"
	"syscall"
)

// processTree is the process group of a script, so cancellation kills every
// process it started.
type processTree struct{}

// newProcessTree prepares cmd to run in its own process group, killed as a
// whole on cancellation. Arguments are passed as argv, so cmdLine is unused.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return &processTree{}, nil
}

func (*processTree) started() error { return nil }

func (*processTree) release() {}
