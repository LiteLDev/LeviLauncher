package mcservice

import (
	"fmt"

	"path/filepath"
	"strings"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"

	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

func ListMinecraftProcesses() []types.ProcessInfo {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return []types.ProcessInfo{}
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return []types.ProcessInfo{}
	}

	versionsDir, _ := apppath.InstancesDir()
	linkedRoots := linkedVersionRoots(versionsDir)
	if versionsDir != "" {
		versionsDir = normalizePath(versionsDir)
	}

	var processes []types.ProcessInfo
	for {
		if versions.IsMinecraftExecutable(windows.UTF16ToString(entry.ExeFile[:])) {
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					exePath := windows.UTF16ToString(buf[:size])
					cleanPath := normalizePath(exePath)

					isLauncher := false
					versionName := ""
					if versionsDir != "" && strings.HasPrefix(cleanPath, versionsDir+string(filepath.Separator)) {
						isLauncher = true
						rel, err := filepath.Rel(versionsDir, cleanPath)
						if err == nil {
							parts := strings.Split(rel, string(filepath.Separator))
							if len(parts) > 0 {
								versionName = parts[0]
							}
						}
					} else if name, ok := matchLinkedVersion(linkedRoots, cleanPath); ok {
						isLauncher = true
						versionName = name
					}

					processes = append(processes, types.ProcessInfo{
						Pid:         int(entry.ProcessID),
						ExePath:     exePath,
						IsLauncher:  isLauncher,
						VersionName: versionName,
					})
				}
				_ = windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return processes
}

func KillProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
