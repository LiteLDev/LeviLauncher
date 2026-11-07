// Package winhost provides a native owner window with its own UI thread for
// processes that have no Wails window, such as the command line. The
// Microsoft account picker (xbox.SignIn) needs an HWND owned by this process
// and a dispatcher that runs functions synchronously on that window's thread;
// Host supplies both.
package winhost

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procGetProcessWindowStation  = user32.NewProc("GetProcessWindowStation")
	procGetUserObjectInformation = user32.NewProc("GetUserObjectInformationW")
	procOpenInputDesktop         = user32.NewProc("OpenInputDesktop")
	procCloseDesktop             = user32.NewProc("CloseDesktop")
)

const (
	wmDestroy  = 0x0002
	wmClose    = 0x0010
	wmApp      = 0x8000
	wmDispatch = wmApp + 1

	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopMost    = 0x00000008

	swShow         = 5
	smCxScreen     = 0
	smCyScreen     = 1
	uoiFlags       = 1
	wsfVisible     = 0x0001
	desktopReadObj = 0x0001

	className = "LeviLauncherOwnerWindow"
)

// Options configure New.
type Options struct {
	// Title is the window title (default "LeviLauncher").
	Title string
	// Visible shows a small captioned window centred on the screen instead
	// of keeping it hidden. Dialogs owned by a visible window come to the
	// foreground more reliably.
	Visible bool
}

// Host is a native window running a message loop on its own locked OS
// thread.
type Host struct {
	hwnd uintptr
	tid  uint32

	mu      sync.Mutex
	queue   []func()
	closing bool
	done    chan struct{}
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

var (
	hosts       sync.Map // hwnd -> *Host
	registerErr error
	registerMu  sync.Once
	// Allocated once: callbacks made by syscall.NewCallback are never freed.
	wndProcCallback = syscall.NewCallback(wndProc)
)

func registerClass() error {
	registerMu.Do(func() {
		name, _ := windows.UTF16PtrFromString(className)
		var instance windows.Handle
		_ = windows.GetModuleHandleEx(0, nil, &instance)
		wc := wndClassEx{wndProc: wndProcCallback, instance: instance, className: name}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			registerErr = fmt.Errorf("RegisterClassEx: %w", err)
		}
	})
	return registerErr
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmDispatch:
		if v, ok := hosts.Load(hwnd); ok {
			v.(*Host).drain()
		}
		return 0
	case wmClose:
		// Only Close ends the host: a user closing a visible window must not
		// strand a dialog that still dispatches to it.
		if v, ok := hosts.Load(hwnd); ok && !v.(*Host).isClosing() {
			return 0
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// New creates the window on a new locked OS thread and starts its message
// loop. Close releases it.
func New(o Options) (*Host, error) {
	if o.Title == "" {
		o.Title = "LeviLauncher"
	}
	type result struct {
		h   *Host
		err error
	}
	ready := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := registerClass(); err != nil {
			ready <- result{err: err}
			return
		}
		h := &Host{tid: windows.GetCurrentThreadId(), done: make(chan struct{})}
		defer close(h.done)
		cls, _ := windows.UTF16PtrFromString(className)
		title, _ := windows.UTF16PtrFromString(o.Title)
		style, exStyle := uintptr(wsPopup), uintptr(wsExToolWindow)
		x, y, w, ht := uintptr(0), uintptr(0), uintptr(0), uintptr(0)
		if o.Visible {
			style = wsOverlapped | wsCaption | wsSysMenu
			exStyle = wsExTopMost
			sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
			sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
			w, ht = 420, 120
			if sw > w {
				x = (sw - w) / 2
			}
			if sh > ht {
				y = (sh - ht) / 2
			}
		}
		hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
			style, x, y, w, ht, 0, 0, 0, 0)
		if hwnd == 0 {
			ready <- result{err: fmt.Errorf("CreateWindowEx: %w", err)}
			return
		}
		h.hwnd = hwnd
		hosts.Store(hwnd, h)
		defer hosts.Delete(hwnd)
		if o.Visible {
			procShowWindow.Call(hwnd, swShow)
			procSetForegroundWindow.Call(hwnd)
		}
		ready <- result{h: h}
		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		// Run whatever was queued after the loop stopped, so no Dispatch
		// caller waits forever.
		h.mu.Lock()
		h.closing = true
		h.mu.Unlock()
		h.drain()
	}()
	r := <-ready
	return r.h, r.err
}

// HWND returns the window handle.
func (h *Host) HWND() uintptr { return h.hwnd }

// ErrClosed is returned by Dispatch after Close.
var ErrClosed = errors.New("owner window closed")

// Dispatch runs fn on the window's thread and waits for it to return. Called
// from the window's own thread (from inside a dispatched function), it runs
// fn directly. It is the dispatcher xbox.SignIn expects.
func (h *Host) Dispatch(fn func()) {
	_ = h.DispatchErr(fn)
}

// DispatchErr is Dispatch reporting ErrClosed when fn could not run.
func (h *Host) DispatchErr(fn func()) error {
	select {
	case <-h.done:
		return ErrClosed
	default:
	}
	// Until done is closed the window goroutine holds its OS thread, so
	// code running on that thread is a dispatched function.
	if windows.GetCurrentThreadId() == h.tid {
		fn()
		return nil
	}
	finished := make(chan struct{})
	h.mu.Lock()
	select {
	case <-h.done:
		h.mu.Unlock()
		return ErrClosed
	default:
	}
	h.queue = append(h.queue, func() {
		defer close(finished)
		fn()
	})
	h.mu.Unlock()
	// When posting fails the window is being destroyed; the loop's final
	// drain runs the queue or done reports the closure.
	procPostMessageW.Call(h.hwnd, wmDispatch, 0, 0)
	select {
	case <-finished:
		return nil
	case <-h.done:
		select {
		case <-finished:
			return nil
		default:
			return ErrClosed
		}
	}
}

func (h *Host) drain() {
	for {
		h.mu.Lock()
		if len(h.queue) == 0 {
			h.mu.Unlock()
			return
		}
		fn := h.queue[0]
		h.queue = h.queue[1:]
		h.mu.Unlock()
		fn()
	}
}

func (h *Host) isClosing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closing
}

// Close destroys the window and waits for its thread to finish.
func (h *Host) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.closing = true
	h.mu.Unlock()
	procPostMessageW.Call(h.hwnd, wmClose, 0, 0)
	<-h.done
}

// InteractiveDesktop reports whether this process can show windows to a
// user: its window station is visible and it can open the input desktop.
// It is false in services, scheduled tasks without a logged-on session and
// over some remote shells, where sign-in cannot work.
func InteractiveDesktop() bool {
	station, _, _ := procGetProcessWindowStation.Call()
	if station != 0 {
		var flags struct {
			inherit  int32
			reserved int32
			flags    uint32
		}
		var needed uint32
		r, _, _ := procGetUserObjectInformation.Call(station, uoiFlags, uintptr(unsafe.Pointer(&flags)), unsafe.Sizeof(flags), uintptr(unsafe.Pointer(&needed)))
		if r != 0 && flags.flags&wsfVisible == 0 {
			return false
		}
	}
	desk, _, _ := procOpenInputDesktop.Call(0, 0, desktopReadObj)
	if desk == 0 {
		return false
	}
	procCloseDesktop.Call(desk)
	return true
}
