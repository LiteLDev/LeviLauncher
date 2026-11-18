//go:build darwin

package gameruntime

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Process struct {
	PID        int
	Executable string
}

func Processes() []Process {
	data, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil
	}
	var result []Process
	for _, line := range strings.Split(string(data), "\n") {
		words := strings.Fields(line)
		if len(words) < 2 || !strings.Contains(strings.ToLower(line), "minecraft.windows.exe") {
			continue
		}
		pid, err := strconv.Atoi(words[0])
		if err != nil {
			continue
		}
		files, err := exec.Command("lsof", "-p", words[0], "-Fn").Output()
		if err != nil {
			continue
		}
		for _, f := range strings.Split(string(files), "\n") {
			if strings.HasPrefix(f, "n/") && strings.EqualFold(filepath.Base(f[1:]), "Minecraft.Windows.exe") {
				result = append(result, Process{pid, f[1:]})
				break
			}
		}
	}
	return result
}
