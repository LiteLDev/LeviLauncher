//go:build darwin || linux

package processinfo

import "os/exec"

// Native Unix tools need no Windows console flags.
func ConfigureChild(cmd *exec.Cmd) {}
