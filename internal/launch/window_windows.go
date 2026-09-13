package launch

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
	procIsIconic        = user32.NewProc("IsIconic")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
)

type gameWindowSearch struct {
	pids  map[uint32]struct{}
	found bool
}

func windowClass(hwnd windows.HWND) string {
	var name [256]uint16
	n, err := windows.GetClassName(hwnd, &name[0], int32(len(name)))
	if err != nil || n == 0 {
		return ""
	}
	return windows.UTF16ToString(name[:n])
}

func windowIsVisible(hwnd windows.HWND) bool {
	visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return visible != 0
}

func windowIsPresented(hwnd windows.HWND) bool {
	if !windowIsVisible(hwnd) {
		return false
	}
	minimized, _, _ := procIsIconic.Call(uintptr(hwnd))
	if minimized != 0 {
		return false
	}
	var rect struct{ left, top, right, bottom int32 }
	ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ok == 0 || rect.right <= rect.left || rect.bottom <= rect.top {
		return false
	}
	// UWP and windows on other virtual desktops can have WS_VISIBLE while
	// DWM keeps them off screen. Such a window must not hide the launcher.
	var cloaked uint32
	err := windows.DwmGetWindowAttribute(hwnd, windows.DWMWA_CLOAKED, unsafe.Pointer(&cloaked), uint32(unsafe.Sizeof(cloaked)))
	return err != nil || cloaked == 0
}

func (s *gameWindowSearch) owns(hwnd windows.HWND) bool {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return false
	}
	_, matches := s.pids[pid]
	return matches
}

// Allocate callbacks once: syscall.NewCallback allocations are never freed.
var gameChildWindowCallback = syscall.NewCallback(func(hwnd windows.HWND, param unsafe.Pointer) uintptr {
	s := (*gameWindowSearch)(param)
	// The frame determines presentation. Its game-owned child identifies the
	// instance even while the frame is showing a splash instead of that child.
	if s.owns(hwnd) {
		s.found = true
		return 0
	}
	return 1
})

var gameTopWindowCallback = syscall.NewCallback(func(hwnd windows.HWND, param unsafe.Pointer) uintptr {
	s := (*gameWindowSearch)(param)
	if !windowIsPresented(hwnd) {
		return 1
	}
	class := windowClass(hwnd)
	if class == "ConsoleWindowClass" || class == "CASCADIA_HOSTING_WINDOW_CLASS" {
		return 1
	}
	if s.owns(hwnd) {
		s.found = true
	} else if class == "ApplicationFrameWindow" {
		// On Windows 10 the visible UWP frame can belong to
		// ApplicationFrameHost.exe; its CoreWindow belongs to the game.
		windows.EnumChildWindows(hwnd, gameChildWindowCallback, param)
	}
	if s.found {
		return 0
	}
	return 1
})

func isGameWindowVisible(pids map[uint32]struct{}) bool {
	if len(pids) == 0 {
		return false
	}
	s := gameWindowSearch{pids: pids}
	_ = windows.EnumWindows(gameTopWindowCallback, unsafe.Pointer(&s))
	return s.found
}
