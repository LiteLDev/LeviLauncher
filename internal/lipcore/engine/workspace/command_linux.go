//go:build linux

package workspace

import (
	"errors"

	"golang.org/x/sys/unix"
)

// waitExited waits for process pid to exit without reaping it.
func waitExited(pid int) error {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
