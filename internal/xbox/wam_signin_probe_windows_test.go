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

	"golang.org/x/sys/windows"
)

// This opt-in probe runs the production interactive sign-in against a real
// window, including the Xbox Live exchange. Only lengths and the resolved profile
// are logged; credentials never leave the backend.
func TestInteractiveSignIn(t *testing.T) {
	if os.Getenv("LEVI_WAM_SIGNIN_PROBE") != "1" {
		t.Skip("set LEVI_WAM_SIGNIN_PROBE=1 for a live interactive sign-in")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user32 := syscall.NewLazyDLL("user32.dll")
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("LeviLauncher sign-in verification")
	hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x00cf0000, 240, 240, 900, 620, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal("CreateWindowExW failed")
	}
	defer user32.NewProc("DestroyWindow").Call(hwnd)
	user32.NewProc("ShowWindow").Call(hwnd, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := ResetSession(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	// The probe never persists an identity and never writes the account preference.
	if err := SignIn(ctx, hwnd, func(string) error { return nil }); err != nil {
		t.Fatalf("interactive sign-in failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
	}
	gamertag, err := GetLocalUserGamertag()
	if err != nil {
		t.Fatal(err)
	}
	xuid, err := GetLocalUserId()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sign-in completed in %s gamertag=%q xuid=%d", time.Since(start).Round(time.Millisecond), gamertag, xuid)
}