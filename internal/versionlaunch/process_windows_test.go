package versionlaunch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRunningExecutableThroughLinkedVersion(t *testing.T) {
	// The test process itself supplies a live image for the real Win32 query.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"directory", "junction", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			target := exe
			if kind != "directory" {
				link := filepath.Join(t.TempDir(), "linked version")
				if kind == "symlink" {
					if err := os.Symlink(filepath.Dir(exe), link); err != nil {
						if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
							t.Skip("directory symlinks require Developer Mode or symlink privilege")
						}
						t.Fatal(err)
					}
				} else {
					cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
						"$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:LEVI_TEST_LINK -Target $env:LEVI_TEST_TARGET | Out-Null")
					cmd.Env = append(os.Environ(), "LEVI_TEST_LINK="+link, "LEVI_TEST_TARGET="+filepath.Dir(exe))
					cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("create junction: %v: %s", err, output)
					}
				}
				target = filepath.Join(link, filepath.Base(exe))
			}
			for _, path := range []string{target, strings.ToUpper(target), `\\?\` + target} {
				if !isProcessRunningAtPath(path) {
					t.Errorf("running executable was not detected through %q", path)
				}
			}
			if isProcessRunningAtPath(target + ".other.exe") {
				t.Error("different executable incorrectly reported as running")
			}
		})
	}
}
