//go:build darwin || linux

package mcservice

import (
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/types"
	"os"
	"path/filepath"
	"syscall"
)

func ListMinecraftProcesses() []types.ProcessInfo {
	root, _ := apppath.InstancesDir()
	var result []types.ProcessInfo
	for _, p := range gameruntime.Processes() {
		name := ""
		relative, err := filepath.Rel(root, p.Executable)
		if err == nil && filepath.Dir(relative) != "." && relative != ".." && len(relative) > 0 {
			first := relative
			for filepath.Dir(first) != "." {
				first = filepath.Dir(first)
			}
			if first != ".." {
				name = first
			}
		}
		result = append(result, types.ProcessInfo{Pid: p.PID, ExePath: p.Executable, IsLauncher: name != "", VersionName: name})
	}
	return result
}
func KillProcess(pid int) error {
	for _, p := range gameruntime.Processes() {
		if p.PID == pid {
			process, err := os.FindProcess(pid)
			if err != nil {
				return err
			}
			return process.Signal(syscall.SIGTERM)
		}
	}
	return fmt.Errorf("Minecraft process %d is no longer running", pid)
}
