package launch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	testCreateWindow    = user32.NewProc("CreateWindowExW")
	testDestroyWindow   = user32.NewProc("DestroyWindow")
	testShowWindow      = user32.NewProc("ShowWindow")
	testDefWindowProc   = user32.NewProc("DefWindowProcW")
	testRegisterClass   = user32.NewProc("RegisterClassW")
	testUnregisterClass = user32.NewProc("UnregisterClassW")
	testWindowProc      = syscall.NewCallback(func(hwnd, message, wparam, lparam uintptr) uintptr {
		result, _, _ := testDefWindowProc.Call(hwnd, message, wparam, lparam)
		return result
	})
)

func registerTestWindowClass(t *testing.T, name string) {
	t.Helper()
	className := windows.StringToUTF16Ptr(name)
	class := struct {
		style                              uint32
		windowProc                         uintptr
		classExtra, windowExtra            int32
		instance, icon, cursor, background uintptr
		menuName, className                *uint16
	}{windowProc: testWindowProc, className: className}
	atom, _, err := testRegisterClass.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		t.Fatalf("register %s: %v", name, err)
	}
	t.Cleanup(func() { testUnregisterClass.Call(uintptr(unsafe.Pointer(className)), 0) })
}

func createTestWindow(t *testing.T, class string, parent windows.HWND, visible bool) windows.HWND {
	t.Helper()
	style := uintptr(0x80000000) // WS_POPUP
	x := int32(-32000)
	if parent != 0 {
		style = 0x40000000 // WS_CHILD
		x = 0
	}
	if visible {
		style |= 0x10000000 // WS_VISIBLE
	}
	// Off-screen, no activation, no taskbar button. These native fixtures do
	// not cover the user's desktop or take focus during the test run.
	hwnd, _, err := testCreateWindow.Call(0x08000084,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(class))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Unrelated or localized game title"))),
		style, uintptr(x), uintptr(x), 32, 32, uintptr(parent), 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("create %s: %v", class, err)
	}
	t.Cleanup(func() {
		if ok, _, err := testDestroyWindow.Call(hwnd); ok == 0 {
			t.Errorf("destroy fixture window: %v", err)
		}
	})
	return windows.HWND(hwnd)
}

func TestGameWindowUsesProcessIdentityAndPresentation(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	pids := map[uint32]struct{}{uint32(os.Getpid()): {}}
	hwnd := createTestWindow(t, "STATIC", 0, true)
	// DWM can briefly cloak a newly created window until its first frame.
	deadline := time.Now().Add(2 * time.Second)
	for !isGameWindowVisible(pids) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !isGameWindowVisible(pids) {
		var cloaked uint32
		err := windows.DwmGetWindowAttribute(hwnd, windows.DWMWA_CLOAKED, unsafe.Pointer(&cloaked), 4)
		search := gameWindowSearch{pids: pids}
		t.Fatalf("visible game window with a different title was missed (visible=%t presented=%t owns=%t class=%s cloaked=%d dwm=%v)", windowIsVisible(hwnd), windowIsPresented(hwnd), search.owns(hwnd), windowClass(hwnd), cloaked, err)
	}
	if isGameWindowVisible(map[uint32]struct{}{0: {}}) || isGameWindowVisible(nil) {
		t.Fatal("unrelated instance matched a visible window")
	}
	testShowWindow.Call(uintptr(hwnd), 0) // SW_HIDE
	if isGameWindowVisible(pids) {
		t.Fatal("hidden window counted as ready")
	}
	testShowWindow.Call(uintptr(hwnd), 4) // SW_SHOWNOACTIVATE
	testShowWindow.Call(uintptr(hwnd), 6) // SW_MINIMIZE
	if isGameWindowVisible(pids) {
		t.Fatal("minimized window counted as ready")
	}
	testShowWindow.Call(uintptr(hwnd), 4)
	cloaked := uint32(1)
	if err := windows.DwmSetWindowAttribute(hwnd, windows.DWMWA_CLOAK, unsafe.Pointer(&cloaked), 4); err != nil {
		t.Fatalf("cloak fixture: %v", err)
	}
	if isGameWindowVisible(pids) {
		t.Fatal("DWM-cloaked window counted as ready")
	}
	cloaked = 0
	if err := windows.DwmSetWindowAttribute(hwnd, windows.DWMWA_CLOAK, unsafe.Pointer(&cloaked), 4); err != nil {
		t.Fatal(err)
	}
	if !isGameWindowVisible(pids) {
		t.Fatal("uncloaked game window was not detected")
	}
}

func TestConsoleWindowDoesNotSignalGameReady(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	registerTestWindowClass(t, "ConsoleWindowClass")
	createTestWindow(t, "ConsoleWindowClass", 0, true)
	if isGameWindowVisible(map[uint32]struct{}{uint32(os.Getpid()): {}}) {
		t.Fatal("console window counted as the game window")
	}
}

// This helper represents ApplicationFrameHost: its frame has a different PID
// from the game-owned child that the parent test creates inside it.
func TestUWPFrameHostHelper(t *testing.T) {
	if os.Getenv("LEVI_TEST_FRAME_HOST") != "1" {
		return
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	registerTestWindowClass(t, "ApplicationFrameWindow")
	hwnd := createTestWindow(t, "ApplicationFrameWindow", 0, true)
	fmt.Printf("FRAME=%d\n", hwnd)
	threadID := windows.GetCurrentThreadId()
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		user32.NewProc("PostThreadMessageW").Call(uintptr(threadID), 0x0012, 0, 0) // WM_QUIT
	}()
	var msg struct {
		hwnd           uintptr
		message        uint32
		wparam, lparam uintptr
		time           uint32
		x, y           int32
		private        uint32
	}
	getMessage := user32.NewProc("GetMessageW")
	dispatchMessage := user32.NewProc("DispatchMessageW")
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func TestUWPFrameMatchesGameOwnedChild(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestUWPFrameHostHelper$")
	cmd.Env = append(os.Environ(), "LEVI_TEST_FRAME_HOST=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("frame helper: %v", err)
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "FRAME=")), 10, 64)
	if err != nil {
		t.Fatalf("frame helper output %q: %v", line, err)
	}
	frame := windows.HWND(value)
	pids := map[uint32]struct{}{uint32(os.Getpid()): {}}
	if isGameWindowVisible(pids) {
		t.Fatal("unrelated UWP frame matched without a game child")
	}
	child := createTestWindow(t, "STATIC", frame, true)
	if !isGameWindowVisible(pids) {
		t.Fatal("visible UWP frame with game-owned child was missed")
	}
	testShowWindow.Call(uintptr(child), 0)
	if !isGameWindowVisible(pids) {
		t.Fatal("visible UWP splash frame lost its association with a hidden game child")
	}
	testShowWindow.Call(uintptr(child), 4)
	testShowWindow.Call(uintptr(frame), 0)
	if isGameWindowVisible(pids) {
		t.Fatal("hidden UWP host counted as ready")
	}
}
