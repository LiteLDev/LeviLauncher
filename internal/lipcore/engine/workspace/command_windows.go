//go:build windows

package workspace

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procNtResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// processTree is a job object holding a script process and every process it
// starts, so cancellation kills them all. The job is created with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so the tree also dies with the
// launcher.
type processTree struct {
	cmd *exec.Cmd
	job windows.Handle
}

// newProcessTree prepares cmd. The command line is passed to CreateProcess
// verbatim, so cmd.exe sees the same text as under lip (Go's own escaping
// differs from CliWrap's). The process gets CREATE_NO_WINDOW like CliWrap
// (.NET CreateNoWindow) and starts suspended, so it joins the job before it
// can start children. Cancelling terminates the job.
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
	cmd.Cancel = func() error {
		// Kill the process too, in case it has not joined the job yet.
		_ = windows.TerminateJobObject(job, 1)
		return cmd.Process.Kill()
	}
	return &processTree{cmd: cmd, job: job}, nil
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
	_ = setKillOnJobClose(t.job, false)
	windows.CloseHandle(t.job)
}

func setKillOnJobClose(job windows.Handle, kill bool) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if kill {
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	}
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return err
}
