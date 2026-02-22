package mcservice

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"golang.org/x/sys/windows"
)

func normalizeProcPath(p string) string {
	s := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	s = strings.TrimPrefix(s, `\\?\`)
	s = strings.TrimPrefix(s, `\??\`)
	return s
}

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

	versionsDir, _ := apppath.VersionsDir()
	if versionsDir != "" {
		versionsDir = normalizeProcPath(versionsDir)
	}

	var processes []types.ProcessInfo
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "Minecraft.Windows.exe") {
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					exePath := windows.UTF16ToString(buf[:size])
					cleanPath := normalizeProcPath(exePath)

					isLauncher := false
					versionName := ""
					if versionsDir != "" && strings.HasPrefix(cleanPath, versionsDir) {
						isLauncher = true
						rel, err := filepath.Rel(versionsDir, cleanPath)
						if err == nil {
							parts := strings.Split(rel, string(filepath.Separator))
							if len(parts) > 0 {
								versionName = parts[0]
							}
						}
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

func KillAllMinecraftProcesses() error {
	var firstErr error
	found := false
	for _, p := range ListMinecraftProcesses() {
		found = true
		if err := KillProcess(p.Pid); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if !found {
		return fmt.Errorf("no Minecraft process found")
	}
	return nil
}
