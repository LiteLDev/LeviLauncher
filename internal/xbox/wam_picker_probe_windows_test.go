//go:build windows && wamprobe

package xbox

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// This opt-in probe opens a real Windows account panel and dismisses it without
// authenticating, persisting an identity or logging any account identifiers.
func TestNativeAccountPicker(t *testing.T) {
	if os.Getenv("LEVI_WAM_PICKER_PROBE") != "1" {
		t.Skip("set LEVI_WAM_PICKER_PROBE=1 to show the native account panel")
	}
	t.Run("minimum_960x600", func(t *testing.T) { probeNativeAccountPicker(t, 960, 600) })
	t.Run("default_1024x640", func(t *testing.T) { probeNativeAccountPicker(t, 1024, 640) })
}

func probeNativeAccountPicker(t *testing.T, width, height uintptr) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeWAM(0); err != nil {
		t.Fatal(err)
	}
	defer uninitializeWAM()
	user32 := syscall.NewLazyDLL("user32.dll")
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("LeviLauncher account picker verification")
	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10cf0000, 100, 100, width, height, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("DestroyWindow").Call(hwnd)
	type accountsResult struct {
		provider *ole.IUnknown
		err      error
	}
	ready := make(chan accountsResult, 1)
	release := make(chan struct{})
	defer close(release)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := initializeWAM(roInitMultithreaded); err != nil {
			ready <- accountsResult{err: err}
			return
		}
		defer uninitializeWAM()
		provider, err := findMSAProvider(context.Background())
		if err != nil {
			ready <- accountsResult{err: err}
			return
		}
		defer provider.Release()
		ready <- accountsResult{provider: provider}
		<-release
	}()
	result := <-ready
	if result.provider == nil {
		t.Fatalf("account provider unavailable: %v", result.err)
	}
	p := &wamAccountPane{}
	defer p.close()
	if err := p.open(hwnd, result.provider); err != nil {
		t.Fatalf("opening native account panel: %v", err)
	}
	// MSG with pointer-sized padding supplied by Go's normal struct alignment.
	var msg struct {
		HWND    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		X, Y    int32
		Private uint32
	}
	duration := 2 * time.Second
	if os.Getenv("LEVI_WAM_PICKER_INSPECT") == "1" {
		duration = 30 * time.Second
	}
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		for {
			hasMessage, _, _ := user32.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
			if hasMessage == 0 {
				break
			}
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
		}
		if p.err != nil {
			t.Fatalf("populating native account panel: %v", p.err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	wamDelegates.Lock()
	callbacks := len(wamDelegates.live)
	wamDelegates.Unlock()
	if callbacks < 2 {
		t.Fatal("native panel did not request account/provider commands")
	}
	t.Log("native panel populated; no token requested and no account saved")
}
