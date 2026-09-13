package mcservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

func normalizePath(p string) string {
	s := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	s = strings.TrimPrefix(s, `\\?\`)
	s = strings.TrimPrefix(s, `\??\`)
	return s
}

// linkedVersionRoots maps the resolved location of every junction-backed
// version folder to its version name. Windows reports the resolved image path
// for a running process, so a linked version's game does not appear under the
// versions directory.
func linkedVersionRoots(versionsDir string) map[string]string {
	if strings.TrimSpace(versionsDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil
	}
	roots := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dir := filepath.Join(versionsDir, e.Name())
		if !utils.ResolvesToDir(dir) {
			continue
		}
		if resolved := canonicalPath(dir); resolved != "" && resolved != normalizePath(dir) {
			roots[resolved] = e.Name()
		}
	}
	return roots
}

func matchLinkedVersion(roots map[string]string, cleanPath string) (string, bool) {
	for root, name := range roots {
		if strings.HasPrefix(cleanPath, root+string(filepath.Separator)) {
			return name, true
		}
	}
	return "", false
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
