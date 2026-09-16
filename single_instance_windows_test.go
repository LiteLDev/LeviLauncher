package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
	"unsafe"

	win "golang.org/x/sys/windows"
)

func testInstanceMutexName(t *testing.T) string {
	return fmt.Sprintf("Local\\LeviLauncher_Test_%d_%s", os.Getpid(), t.Name())
}

func TestSingleInstanceMutexDuplicateDoesNotKeepGuardAlive(t *testing.T) {
	name := testInstanceMutexName(t)
	h, err := acquireSingleInstanceMutex(name, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if h != 0 {
			win.CloseHandle(h)
		}
	}()
	for _, wait := range []time.Duration{0, 30 * time.Millisecond} {
		duplicate, err := acquireSingleInstanceMutex(name, wait)
		if duplicate != 0 {
			win.CloseHandle(duplicate)
			t.Fatal("duplicate acquisition retained a handle")
		}
		if err != win.ERROR_ALREADY_EXISTS {
			t.Fatalf("duplicate acquisition = %v, want ERROR_ALREADY_EXISTS", err)
		}
	}
	if err := win.CloseHandle(h); err != nil {
		t.Fatal(err)
	}
	h = 0
	next, err := acquireSingleInstanceMutex(name, 0)
	if err != nil {
		t.Fatalf("guard survived its last owner: %v", err)
	}
	win.CloseHandle(next)
}

func TestSingleInstanceMutexDetectsReadOnlyExistingGuard(t *testing.T) {
	// UAC handoffs cross integrity levels. Detecting the existing guard must
	// not request MUTEX_ALL_ACCESS (including write permissions).
	sd, err := win.SecurityDescriptorFromString("D:P(A;;0x00100000;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	sa := win.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(win.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	name := testInstanceMutexName(t)
	ptr, err := win.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	h, err := win.CreateMutexEx(&sa, ptr, 0, win.SYNCHRONIZE)
	if err != nil {
		t.Fatal(err)
	}
	defer win.CloseHandle(h)
	fullAccess, err := win.CreateMutex(nil, false, ptr)
	if fullAccess != 0 {
		win.CloseHandle(fullAccess)
	}
	if err != win.ERROR_ACCESS_DENIED {
		t.Fatalf("test guard must reject full access, got %v", err)
	}
	duplicate, err := acquireSingleInstanceMutex(name, 0)
	if duplicate != 0 {
		win.CloseHandle(duplicate)
		t.Fatal("duplicate acquisition retained a handle")
	}
	if err != win.ERROR_ALREADY_EXISTS {
		t.Fatalf("read-only existing guard = %v, want ERROR_ALREADY_EXISTS", err)
	}
}

func TestSingleInstanceMutexWaitsForPreviousProcessExit(t *testing.T) {
	name := testInstanceMutexName(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSingleInstanceMutexChild$")
	cmd.Env = append(os.Environ(), "LEVI_INSTANCE_TEST_CHILD=1", "LEVI_INSTANCE_TEST_MUTEX="+name)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		stdin.Close()
		if !waited {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		ready <- line
	}()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("helper did not acquire its mutex: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not become ready")
	}
	// Keep the old process alive beyond the former three-second restart
	// budget. Closing stdin then lets Windows release its handle on exit.
	timer := time.AfterFunc(3200*time.Millisecond, func() { stdin.Close() })
	defer timer.Stop()
	h, err := acquireSingleInstanceMutex(name, 10*time.Second)
	if err != nil {
		t.Fatalf("handoff failed while previous process exited: %v", err)
	}
	defer win.CloseHandle(h)
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("previous process failed: %v", err)
	}
}

func TestSingleInstanceMutexChild(t *testing.T) {
	if os.Getenv("LEVI_INSTANCE_TEST_CHILD") != "1" {
		return
	}
	_, err := acquireSingleInstanceMutex(os.Getenv("LEVI_INSTANCE_TEST_MUTEX"), 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("ready")
	io.Copy(io.Discard, os.Stdin)
	// Match the updater's os.Exit path instead of explicitly closing the
	// mutex, to verify kernel handle cleanup across a process handoff.
	os.Exit(0)
}
