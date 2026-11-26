//go:build !windows

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// setupConsole is a no-op on Unix: stdin, stdout and stderr are already
// attached to the parent terminal.
func setupConsole() {}

// isConsole reports whether f is connected to an interactive terminal.
func isConsole(f *os.File) bool {
	if f == nil {
		return false
	}
	_, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	return err == nil
}

// consoleWidth returns the width of the terminal behind stderr, or 0 when
// unknown.
func consoleWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stderr.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(ws.Col)
}

// GUIRunning reports whether a LeviLauncher GUI window is currently running.
// Wails' macOS single-instance manager holds an exclusive flock on
// <TMPDIR>/org.levimc.levilauncher.native.lock.
func GUIRunning() bool {
	lockPath := filepath.Join(os.TempDir(), "org.levimc.levilauncher.native.lock")
	f, err := os.OpenFile(lockPath, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}
