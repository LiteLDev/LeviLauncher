//go:build !windows && !unix

package workspace

import "os/exec"

// processTree is the script process alone: only it is killed on
// cancellation.
type processTree struct {
	cmd *exec.Cmd
}

// newProcessTree leaves cmd as is: arguments are passed as argv.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	return &processTree{cmd: cmd}, nil
}

func (*processTree) started() error { return nil }

func (t *processTree) kill() { _ = t.cmd.Process.Kill() }

func (*processTree) release() {}
