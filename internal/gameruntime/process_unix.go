//go:build darwin

package gameruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Process struct {
	PID        int
	Executable string
}

func processCommand(tool string, args ...string) *exec.Cmd {
	cmd := exec.Command(tool, args...)
	// Finder may launch the app with the C locale, which causes both
	// tools to escape Unicode paths before returning them.
	cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")
	return cmd
}

func Processes() []Process {
	data, err := processCommand("/bin/ps", "-axww", "-o", "pid=,comm=").Output()
	if err != nil {
		return nil
	}
	var result []Process
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			continue
		}
		command := strings.TrimSpace(line[i:])
		if !strings.Contains(strings.ToLower(command), "minecraft.windows.exe") {
			continue
		}
		pidString := line[:i]
		pid, err := strconv.Atoi(pidString)
		if err != nil {
			continue
		}
		// With Wine's native process title, ps can identify the instance
		// even when lsof cannot see the executable's mapping.
		if filepath.IsAbs(command) && strings.EqualFold(filepath.Base(command), "Minecraft.Windows.exe") {
			if info, err := os.Stat(command); err == nil && info.Mode().IsRegular() {
				result = append(result, Process{pid, command})
				continue
			}
		}
		// Finder launches can omit /usr/sbin from PATH. lsof can also return
		// useful mappings with a nonzero status when another file is inaccessible.
		files, _ := processCommand("/usr/sbin/lsof", "-p", pidString, "-Fn").Output()
		if len(files) == 0 {
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
