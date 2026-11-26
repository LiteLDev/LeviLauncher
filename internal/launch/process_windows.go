package launch

import (
	"path/filepath"
	"strings"

	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"golang.org/x/sys/windows"
)

func gameProcessIDs(versionDir string) map[uint32]struct{} {
	cleanVerDir := utils.CanonicalWindowsPath(versionDir)
	if cleanVerDir == "" {
		return nil
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}

	pids := make(map[uint32]struct{})
	for {
		if versions.IsMinecraftExecutable(windows.UTF16ToString(entry.ExeFile[:])) {
			h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buf := make([]uint16, 1024)
				size := uint32(len(buf))
				if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err == nil && size > 0 {
					// The image path keeps the names the process was started
					// with (8.3 short names too); canonicalize it like the
					// version directory so both compare.
					p := utils.CanonicalWindowsPath(windows.UTF16ToString(buf[:size]))
					_ = windows.CloseHandle(h)
					if strings.HasPrefix(p, cleanVerDir+string(filepath.Separator)) {
						pids[entry.ProcessID] = struct{}{}
					}
				} else {
					_ = windows.CloseHandle(h)
				}
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return pids
}

func isProcessAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(h, &exitCode); err != nil {
		return false
	}

	const stillActive = 259
	return exitCode == stillActive
}
