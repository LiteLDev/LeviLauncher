//go:build linux

package install

import (
	"errors"

	"golang.org/x/sys/unix"
)

// waitExitedUnreaped waits for process pid to exit without reaping it.
func waitExitedUnreaped(pid int) error {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
