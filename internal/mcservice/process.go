package mcservice

import (
	"encoding/csv"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func ListMinecraftProcesses() []types.ProcessInfo {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_Process -Filter \"Name='Minecraft.Windows.exe'\" | Select-Object ProcessId,ExecutablePath | ConvertTo-Csv -NoTypeInformation")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return []types.ProcessInfo{}
	}

	reader := csv.NewReader(strings.NewReader(string(out)))
	records, err := reader.ReadAll()
	if err != nil {
		return []types.ProcessInfo{}
	}

	if len(records) < 2 {
		return []types.ProcessInfo{}
	}

	exeIdx := -1
	pidIdx := -1
	for i, col := range records[0] {
		if strings.EqualFold(col, "ExecutablePath") {
			exeIdx = i
		} else if strings.EqualFold(col, "ProcessId") {
			pidIdx = i
		}
	}

	if exeIdx == -1 || pidIdx == -1 {
		return []types.ProcessInfo{}
	}

	versionsDir, _ := utils.GetVersionsDir()
	versionsDir = strings.ToLower(filepath.Clean(versionsDir))

	var processes []types.ProcessInfo
	for _, record := range records[1:] {
		if len(record) <= exeIdx || len(record) <= pidIdx {
			continue
		}

		pidStr := record[pidIdx]
		exePath := record[exeIdx]

		if strings.TrimSpace(pidStr) == "" {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		cleanPath := strings.ToLower(filepath.Clean(exePath))
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
			Pid:         pid,
			ExePath:     exePath,
			IsLauncher:  isLauncher,
			VersionName: versionName,
		})
	}

	return processes
}

func KillProcess(pid int) error {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func KillAllMinecraftProcesses() error {
	cmd := exec.Command("taskkill", "/IM", "Minecraft.Windows.exe", "/F")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
