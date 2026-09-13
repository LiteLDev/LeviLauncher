package launch

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestGameProcessHelper(t *testing.T) {
	if os.Getenv("LEVI_TEST_GAME_PROCESS") != "1" {
		return
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestGameRunningThroughLinkedVersion(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	for _, exeName := range []string{"Minecraft.Windows.exe", "Minecraft.Win10.DX11.exe"} {
		t.Run(exeName, func(t *testing.T) {
			root := t.TempDir()
			realDir := filepath.Join(root, "external game")
			if err := os.Mkdir(realDir, 0755); err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(realDir, exeName)
			if err := os.WriteFile(exe, image, 0755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "-test.run=^TestGameProcessHelper$")
			cmd.Env = append(os.Environ(), "LEVI_TEST_GAME_PROCESS=1")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})

			for _, kind := range []string{"directory", "junction", "symlink"} {
				t.Run(kind, func(t *testing.T) {
					dir := realDir
					if kind != "directory" {
						dir = filepath.Join(root, kind)
						createVersionLink(t, kind, dir, realDir)
					}
					if !isGameRunning(dir) {
						t.Errorf("running game was not detected through %s", kind)
					}
					// Protocol launches have no PID; direct launches provide one.
					for _, pid := range []uint32{0, uint32(cmd.Process.Pid)} {
						found, exited, canceled := waitForGameProcess(context.Background(), dir, pid, time.Second)
						if !found || exited || canceled {
							t.Errorf("waitForGameProcess(%s, pid=%d) = (%v, %v, %v)", kind, pid, found, exited, canceled)
						}
					}
				})
			}
			for _, dir := range []string{"", "  ", filepath.Join(root, "external"), filepath.Join(root, "other version")} {
				if isGameRunning(dir) {
					t.Errorf("game incorrectly matched unrelated directory %q", dir)
				}
			}
			_ = stdin.Close()
			if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{realDir, filepath.Join(root, "junction"), filepath.Join(root, "symlink")} {
				if isGameRunning(dir) {
					t.Errorf("exited game still reported as running through %q", dir)
				}
			}
		})
	}
}

func createVersionLink(t *testing.T, kind, link, target string) {
	t.Helper()
	if kind == "symlink" {
		if err := os.Symlink(target, link); err != nil {
			if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
				t.Skip("directory symlinks require Developer Mode or symlink privilege")
			}
			t.Fatal(err)
		}
		return
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:LEVI_TEST_LINK -Target $env:LEVI_TEST_TARGET | Out-Null")
	cmd.Env = append(os.Environ(), "LEVI_TEST_LINK="+link, "LEVI_TEST_TARGET="+target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
}
