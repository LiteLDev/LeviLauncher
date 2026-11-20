package launch

import (
	"context"

	"golang.org/x/sys/windows"
)

// WaitPIDExit blocks until process pid exits and returns its exit code, or
// returns ctx.Err() when ctx ends first.
func WaitPIDExit(ctx context.Context, pid uint32) (uint32, error) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	for {
		ev, err := windows.WaitForSingleObject(h, 250)
		if err != nil {
			return 0, err
		}
		if ev == windows.WAIT_OBJECT_0 {
			var code uint32
			if err := windows.GetExitCodeProcess(h, &code); err != nil {
				return 0, err
			}
			return code, nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}
