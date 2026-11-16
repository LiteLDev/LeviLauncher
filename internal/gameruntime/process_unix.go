//go:build darwin || linux

package gameruntime

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		if runtime.GOOS == "darwin" {
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
		} else {
			f, err := os.Open(filepath.Join("/proc", words[0], "maps"))
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				index := strings.Index(line, "/")
				if index >= 0 && strings.EqualFold(filepath.Base(line[index:]), "Minecraft.Windows.exe") {
					result = append(result, Process{pid, line[index:]})
					break
				}
			}
			f.Close()
		}
	}
	return result
}
