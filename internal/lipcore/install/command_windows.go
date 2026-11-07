//go:build windows

package install

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procNtResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// terminateJobObject terminates a job; tests replace it.
var terminateJobObject = windows.TerminateJobObject

// processTree is a job object holding a script process and every process it
// starts, so cancellation kills them all. The job is created with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so the tree also dies with the
// launcher. It also has JOB_OBJECT_LIMIT_BREAKAWAY_OK, so a process may
// create children with CREATE_BREAKAWAY_FROM_JOB as it can without a job
// (under lip); such children escape cancellation.
type processTree struct {
	cmd *exec.Cmd
	job windows.Handle

	mu       sync.Mutex
	released bool // the job handle is closed; kill must not use it
}

// newProcessTree prepares cmd. The command line is passed to CreateProcess
// verbatim, so cmd.exe sees the same text as under lip (Go's own escaping
// differs from CliWrap's). The process gets CREATE_NO_WINDOW like CliWrap
// (.NET CreateNoWindow) and starts suspended, so it joins the job before it
// can start children.
func newProcessTree(cmd *exec.Cmd, cmdLine string) (*processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	if err := setKillOnJobClose(job, true); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       cmdLine,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED,
	}
	return &processTree{cmd: cmd, job: job}, nil
}

// kill terminates the job, and the process too in case it is not in it. It
// does nothing once the tree is released: the handle value may by then
// belong to another job.
func (t *processTree) kill() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.released {
		return
	}
	_ = terminateJobObject(t.job, 1)
	_ = t.cmd.Process.Kill()
}

// started puts the started (suspended) process into the job and resumes it.
// If the process cannot join the job (a job that forbids nesting), it runs
// anyway and cancellation kills only the process itself.
func (t *processTree) started() error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(t.cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(t.job, h)
	if r, _, _ := procNtResumeProcess.Call(uintptr(h)); r != 0 {
		return windows.NTStatus(r)
	}
	return nil
}

// release closes the job. Processes a finished script left running in the
// background keep running, as under lip, so the kill-on-close limit is
// lifted first; a cancelled tree was already terminated.
func (t *processTree) release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.released = true
	_ = setKillOnJobClose(t.job, false)
	windows.CloseHandle(t.job)
}

// wait waits for the script to exit, then for its output pipes.
func (t *processTree) wait(pipes func() error) (waitErr, copyErr error) {
	waitErr = t.cmd.Wait()
	return waitErr, pipes()
}

func setKillOnJobClose(job windows.Handle, kill bool) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	if kill {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	}
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return err
}
