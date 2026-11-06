//go:build !windows

package workspace

import "os/exec"

// setCommandLine is a no-op: on Unix arguments are passed as argv.
func setCommandLine(cmd *exec.Cmd, cmdLine string) {}
