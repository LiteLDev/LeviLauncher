//go:build unix

package install

import (
	"os/exec"
	"syscall"
)

// exitCode is Process.ExitCode as .NET reports it on Unix: the exit status,
// or 128 plus the signal for a process killed by a signal.
func exitCode(e *exec.ExitError) (int, bool) {
	if ws, ok := e.Sys().(syscall.WaitStatus); ok {
		switch {
		case ws.Exited():
			return ws.ExitStatus(), true
		case ws.Signaled():
			return 128 + int(ws.Signal()), true
		}
	}
	if code := e.ExitCode(); code >= 0 {
		return code, true
	}
	return 0, false
}
