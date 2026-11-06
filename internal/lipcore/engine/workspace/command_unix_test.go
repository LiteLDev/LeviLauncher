//go:build unix

package workspace

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"testing"
)

func TestProcessTreeNoKillAfterReap(t *testing.T) {
	// Once the leader is reaped its pgid may be reused by an unrelated
	// group: a cancellation's late kill must not signal it. Before the
	// leader is reaped, kill still reaches the group.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	orig := killProcessGroup
	t.Cleanup(func() { killProcessGroup = orig })
	var mu sync.Mutex
	var killed []int
	killProcessGroup = func(pgid int) {
		mu.Lock()
		killed = append(killed, pgid)
		mu.Unlock()
	}

	cmd := exec.Command("sh", "-c", "exit 0")
	tree, err := newProcessTree(cmd, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := startCommand(cmd); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	var killedBeforeReap bool
	waitErr, _ := tree.wait(func() error {
		tree.kill()
		mu.Lock()
		killedBeforeReap = len(killed) == 1
		mu.Unlock()
		return nil
	})
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	// Where the leader cannot be waited for unreaped, pipes runs after the
	// reap and kill is already a no-op.
	if !errors.Is(waitExited(os.Getpid()), errors.ErrUnsupported) && !killedBeforeReap {
		t.Error("kill before the leader was reaped did not signal the group")
	}
	mu.Lock()
	n := len(killed)
	mu.Unlock()
	tree.kill()
	mu.Lock()
	defer mu.Unlock()
	if len(killed) != n {
		t.Errorf("kill after the leader was reaped signalled pgid %d", pid)
	}
}
