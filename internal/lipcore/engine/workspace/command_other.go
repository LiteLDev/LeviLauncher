//go:build !windows && !unix

package workspace

import "os/exec"

// processTree is a no-op: only the script process itself is killed on
// cancellation.
type processTree struct{}

// newProcessTree leaves cmd as is: arguments are passed as argv.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	return &processTree{}, nil
}

func (*processTree) started() error { return nil }

func (*processTree) release() {}
