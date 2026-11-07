//go:build unix

package install

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// withoutWaitid makes processTree.wait take the path of systems where the
// leader cannot be waited for unreaped.
func withoutWaitid(t *testing.T) {
	orig := waitExited
	t.Cleanup(func() { waitExited = orig })
	waitExited = func(int) error { return errors.ErrUnsupported }
}

func TestProcessTreeNoKillAfterReap(t *testing.T) {
	// Once the leader is reaped its pgid may be reused by an unrelated
	// group: a cancellation's late kill must not signal it. Before the
	// leader is reaped, kill still reaches the group.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	for _, waitid := range []bool{true, false} {
		name := "waitid"
		if !waitid {
			name = "no waitid"
		}
		t.Run(name, func(t *testing.T) {
			if waitid && errors.Is(waitExited(os.Getpid()), errors.ErrUnsupported) {
				t.Skip("no waitid")
			}
			if !waitid {
				withoutWaitid(t)
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
			count := func() int {
				mu.Lock()
				defer mu.Unlock()
				return len(killed)
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
			tree.kill()
			if count() != 1 {
				t.Fatal("kill of a running script did not signal the group")
			}
			var killedInPipes bool
			waitErr, _ := tree.wait(func() error {
				tree.kill()
				killedInPipes = count() == 2
				return nil
			})
			// The fake kill left the script to exit on its own.
			if waitErr != nil {
				t.Fatal(waitErr)
			}
			// With waitid the pipes are waited for before the reap, so kill
			// still reaches background processes holding them; without it
			// the leader is already reaped by then.
			if killedInPipes != waitid {
				t.Errorf("kill while waiting for the pipes signalled the group: %v", killedInPipes)
			}
			n := count()
			tree.kill()
			if count() != n {
				t.Errorf("kill after the leader was reaped signalled pgid %d", pid)
			}
		})
	}
}

func TestDefaultCommandRunnerCancelKillsTreeWithoutWaitid(t *testing.T) {
	// Where the leader cannot be waited for unreaped (darwin, the BSDs),
	// cancellation must still kill the whole tree while the script runs.
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no sh")
	}
	withoutWaitid(t)
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := DefaultCommandRunner(ctx, dir, `sh -c "sleep 3; echo late > late.txt"; true`, io.Discard)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("returned after %v", elapsed)
	}
	time.Sleep(4*time.Second - elapsed)
	if exists(filepath.Join(dir, "late.txt")) {
		t.Error("grandchild survived cancellation")
	}
}
