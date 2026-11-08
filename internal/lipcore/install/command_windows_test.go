//go:build windows

package install

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// startTree starts command in a new process tree like DefaultCommandRunner.
func startTree(t *testing.T, command string) (*exec.Cmd, *processTree) {
	t.Helper()
	path, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "/c", command)
	tree, err := newProcessTree(cmd, `"`+path+`" /s /c "`+command+`"`)
	if err != nil {
		t.Fatal(err)
	}
	if err := startCommand(cmd); err != nil {
		tree.release()
		t.Fatal(err)
	}
	if err := tree.started(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		tree.release()
		t.Fatal(err)
	}
	return cmd, tree
}

func TestProcessTreeKillAfterRelease(t *testing.T) {
	// A cancellation's kill can run after the runner has released the tree
	// (the script exited as ctx was cancelled). The closed job handle value
	// may then belong to another script's job: the late kill must not use
	// it.
	orig := terminateJobObject
	t.Cleanup(func() { terminateJobObject = orig })
	var terminated []windows.Handle
	terminateJobObject = func(job windows.Handle, code uint32) error {
		terminated = append(terminated, job)
		return orig(job, code)
	}

	cmd, tree := startTree(t, "exit 0")
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	tree.release()
	tree.kill()
	if len(terminated) != 0 {
		t.Errorf("a released tree's kill terminated job %v", terminated)
	}
}

func TestProcessTreeReleaseWaitsForKill(t *testing.T) {
	// A kill already running when the runner releases the tree finishes
	// with the job handle before release closes it.
	orig := terminateJobObject
	t.Cleanup(func() { terminateJobObject = orig })
	entered, proceed := make(chan struct{}), make(chan struct{})
	var handleValid bool
	terminateJobObject = func(job windows.Handle, code uint32) error {
		close(entered)
		<-proceed
		var info windows.JOBOBJECT_BASIC_LIMIT_INFORMATION
		handleValid = windows.QueryInformationJobObject(job, windows.JobObjectBasicLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil) == nil
		return orig(job, code)
	}

	cmd, tree := startTree(t, "ping -n 30 127.0.0.1 >nul")
	killed := make(chan struct{})
	go func() {
		tree.kill()
		close(killed)
	}()
	<-entered
	released := make(chan struct{})
	go func() {
		tree.release()
		close(released)
	}()
	select {
	case <-released:
		t.Error("release returned while kill was using the job")
	case <-time.After(200 * time.Millisecond):
	}
	close(proceed)
	<-killed
	<-released
	_ = cmd.Wait()
	if !handleValid {
		t.Error("kill used a closed job handle")
	}
}

func jobLimitFlags(t *testing.T, job windows.Handle) uint32 {
	t.Helper()
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		t.Fatal(err)
	}
	return info.BasicLimitInformation.LimitFlags
}

func TestProcessTreeJobAllowsBreakaway(t *testing.T) {
	tree, err := newProcessTree(exec.Command("cmd.exe"), "cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	defer tree.release()
	want := uint32(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK)
	if got := jobLimitFlags(t, tree.job); got != want {
		t.Errorf("limit flags %#x, want %#x", got, want)
	}
	// Background processes of a finished script may still break away.
	if err := setKillOnJobClose(tree.job, false); err != nil {
		t.Fatal(err)
	}
	if got := jobLimitFlags(t, tree.job); got != windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK {
		t.Errorf("released limit flags %#x", got)
	}
}

// breakawayHelper starts cmd.exe with CREATE_BREAKAWAY_FROM_JOB and exits 3
// if that fails.
const breakawayHelper = `package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command(os.Getenv("ComSpec"), "/c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x01000000} // CREATE_BREAKAWAY_FROM_JOB
	if err := cmd.Run(); err != nil {
		fmt.Println("breakaway failed:", err)
		os.Exit(3)
	}
}
`

func TestDefaultCommandRunnerBreakawayChild(t *testing.T) {
	// Without a job (as under lip) a script's tools may start processes
	// with CREATE_BREAKAWAY_FROM_JOB; the runner's job must allow it.
	if testing.Short() {
		t.Skip("builds a helper")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "breakaway.go")
	if err := os.WriteFile(src, []byte(breakawayHelper), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "breakaway.exe")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", exe, src)
	build.Dir = dir
	build.Env = append(os.Environ(), "GO111MODULE=off", "GOFLAGS=", "GOOS=windows", "GOARCH="+runtime.GOARCH)
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build helper: %v\n%s", err, out)
	}
	// Breakaway needs every enclosing job to allow it, so this cannot run
	// in a job that forbids it (common for CI and sandboxes);
	// TestProcessTreeJobAllowsBreakaway still checks the job's limit there.
	if out, err := exec.Command(exe).CombinedOutput(); err != nil {
		t.Skipf("this test runs in a job that forbids breakaway: %v\n%s", err, out)
	}

	var out strings.Builder
	if err := DefaultCommandRunner(context.Background(), dir, `.\breakaway.exe`, &out); err != nil {
		t.Errorf("err = %v, output %q", err, out.String())
	}
}
