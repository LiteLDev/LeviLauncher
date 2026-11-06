//go:build unix

package workspace

import (
	"os/exec"
	"sync"
	"syscall"
)

// killProcessGroup signals a process group; tests replace it.
var killProcessGroup = func(pgid int) { _ = syscall.Kill(-pgid, syscall.SIGKILL) }

// processTree is the process group of a script, so cancellation kills every
// process it started.
type processTree struct {
	cmd *exec.Cmd

	mu       sync.Mutex
	released bool // the leader may be reaped, so its pgid may be reused
}

// newProcessTree prepares cmd to run in its own process group. Arguments
// are passed as argv, so cmdLine is unused.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processTree{cmd: cmd}, nil
}

func (*processTree) started() error { return nil }

// kill kills the process group, unless the tree is released.
func (t *processTree) kill() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.released {
		return
	}
	killProcessGroup(t.cmd.Process.Pid)
}

// wait waits for the script to exit, then for its output pipes. The pgid is
// the leader's pid, which cannot be reused before the leader is reaped, so
// the tree is released before reaping it. Where the leader can be waited
// for without reaping it (waitExited), it is reaped only after the pipes,
// so cancellation still kills background processes holding them;
// elsewhere they are left to the pipe wait's commandWaitDelay.
func (t *processTree) wait(pipes func() error) (waitErr, copyErr error) {
	if waitExited(t.cmd.Process.Pid) == nil {
		copyErr = pipes()
		t.release()
		return t.cmd.Wait(), copyErr
	}
	t.release()
	waitErr = t.cmd.Wait()
	return waitErr, pipes()
}

func (t *processTree) release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.released = true
}
