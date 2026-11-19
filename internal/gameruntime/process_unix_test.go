//go:build darwin

package gameruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessesWithChinesePathInDesktopEnvironment(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文实例 with spaces")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile("/bin/sleep")
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "Minecraft.Windows.exe")
	if err := os.WriteFile(executable, binary, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("LC_ALL", "C")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, process := range Processes() {
			if process.PID == command.Process.Pid {
				expected, err := filepath.EvalSymlinks(executable)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := filepath.EvalSymlinks(process.Executable)
				if err != nil || actual != expected {
					t.Fatalf("wrong executable: %q (%v)", process.Executable, err)
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("game process was not detected with the desktop application's PATH and locale")
}
