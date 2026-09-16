package main

import (
	"time"

	win "golang.org/x/sys/windows"
)

// The named object's lifetime is the instance guard; thread ownership is not
// needed. Request only SYNCHRONIZE so detecting an elevated instance does not
// require write access to its mutex. The name also remains visible to Setup.
func acquireSingleInstanceMutex(name string, wait time.Duration) (win.Handle, error) {
	ptr, err := win.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	deadline := time.Now().Add(wait)
	for {
		h, err := win.CreateMutexEx(nil, ptr, 0, win.SYNCHRONIZE)
		if err == nil {
			return h, nil
		}
		// An existing object's handle must be closed before waiting, otherwise
		// this process itself keeps the old instance's guard alive after exit.
		if h != 0 {
			_ = win.CloseHandle(h)
		}
		if err != win.ERROR_ALREADY_EXISTS {
			return 0, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, err
		}
		time.Sleep(min(100*time.Millisecond, remaining))
	}
}
