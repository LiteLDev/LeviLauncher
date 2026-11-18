//go:build darwin

package processinfo

import "os/exec"

// Native Unix tools need no Windows console flags.
func ConfigureChild(cmd *exec.Cmd) {}
