//go:build !unix

package install

import "os/exec"

// exitCode is Process.ExitCode: the exit code of the process.
func exitCode(e *exec.ExitError) (int, bool) {
	if code := e.ExitCode(); code >= 0 {
		return code, true
	}
	return 0, false
}
