package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole      = kernel32.NewProc("AttachConsole")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
	procGetConsoleWindow   = kernel32.NewProc("GetConsoleWindow")
)

const (
	attachParentProcess = ^uint32(0)
	cpUTF8              = 65001
)

// setupConsole points stdout, stderr and stdin at the parent console. The
// release build is a GUI-subsystem program, so it starts without one;
// streams the caller redirected to a file or pipe are kept. Validity is
// checked before attaching: AttachConsole fills in empty standard handles
// itself, but the Go os.Std* files were bound at startup and still wrap the
// empty ones. No console is allocated: without a parent console (started
// from Explorer) output is discarded and the exit code still reports the
// outcome.
func setupConsole() {
	type stream struct {
		which uint32
		name  string
		flag  int
		file  **os.File
	}
	var missing []stream
	for _, s := range []stream{
		{windows.STD_OUTPUT_HANDLE, "CONOUT$", os.O_WRONLY, &os.Stdout},
		{windows.STD_ERROR_HANDLE, "CONOUT$", os.O_WRONLY, &os.Stderr},
		{windows.STD_INPUT_HANDLE, "CONIN$", os.O_RDONLY, &os.Stdin},
	} {
		if !stdHandleValid(s.which) {
			missing = append(missing, s)
		}
	}
	attached := false
	if len(missing) > 0 {
		r, _, err := procAttachConsole.Call(uintptr(attachParentProcess))
		attached = r != 0 || err == windows.ERROR_ACCESS_DENIED // already attached
	}
	if attached || hasConsole() {
		_, _, _ = procSetConsoleOutputCP.Call(cpUTF8)
		_, _, _ = procSetConsoleCP.Call(cpUTF8)
	}
	if !attached {
		return
	}
	for _, s := range missing {
		if f, err := os.OpenFile(s.name, s.flag, 0); err == nil {
			*s.file = f
			_ = windows.SetStdHandle(s.which, windows.Handle(f.Fd()))
		}
	}
}

func stdHandleValid(which uint32) bool {
	h, err := windows.GetStdHandle(which)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return false
	}
	t, err := windows.GetFileType(h)
	return err == nil && t != windows.FILE_TYPE_UNKNOWN
}

// hasConsole reports whether the process is attached to a console.
func hasConsole() bool {
	r, _, _ := procGetConsoleWindow.Call()
	return r != 0
}

// isConsole reports whether f is an interactive console (not redirected).
func isConsole(f *os.File) bool {
	if f == nil {
		return false
	}
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil
}

// consoleWidth returns the width of the console behind stderr, 0 when
// unknown.
func consoleWidth() int {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stderr.Fd()), &info); err != nil {
		return 0
	}
	return int(info.Window.Right-info.Window.Left) + 1
}

// GUIRunning reports whether a launcher window is running: it holds the
// single-instance mutex. Commands that must not run next to the window
// (self-update) check it.
func GUIRunning() bool {
	name, _ := windows.UTF16PtrFromString(`Global\LeviLauncher_SingleInstance`)
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return err == windows.ERROR_ACCESS_DENIED
	}
	_ = windows.CloseHandle(h)
	return true
}
