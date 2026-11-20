//go:build darwin

package gameruntime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunHook preserves Windows cmd semantics for client packages on native Unix hosts.
func RunHook(ctx context.Context, dir, command string, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := Default()
	if err != nil {
		return err
	}
	return p.RunHook(ctx, dir, command, output)
}

func (p Profile) RunHook(ctx context.Context, dir, command string, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "levi-hook-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	name := filepath.Join(temp, "hook.cmd")
	// The wrapper may inherit the launcher's macOS PATH. Make Wine's
	// system tools available before running the Windows commands.
	text := "@echo off\r\nset \"PATH=%SystemRoot%\\system32;%SystemRoot%;%PATH%\"\r\nchcp 65001 >nul\r\n" + strings.ReplaceAll(strings.ReplaceAll(command, "\r\n", "\n"), "\n", "\r\n") + "\r\n"
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		return err
	}
	launcher := filepath.Join(temp, "run.cmd")
	cmd, detached, err := p.GameCommand(ctx, launcher)
	if err != nil {
		return err
	}
	if detached {
		guestDir, err := p.WindowsPath(dir)
		if err != nil {
			return err
		}
		cmd.Env = append(cmd.Env, "LEVI_HOOK_DIRECTORY="+guestDir)
		// The Sikarugir wrapper returns before cmd.exe exits. Publish the
		// guest exit code only after its output is closed.
		const supervisor = "@echo off\r\nsetlocal DisableDelayedExpansion\r\n" +
			"mkdir \"%~dp0started\" 2>nul\r\n" +
			"if errorlevel 1 exit /b 0\r\n" +
			"cd /d \"%LEVI_HOOK_DIRECTORY%\" > \"%~dp0hook.log\" 2>&1\r\n" +
			"if not errorlevel 1 call \"%~dp0hook.cmd\" >> \"%~dp0hook.log\" 2>&1\r\n" +
			" >\"%~dp0exit.tmp\" echo %errorlevel%\r\n" +
			"move /y \"%~dp0exit.tmp\" \"%~dp0exit.txt\" >nul\r\n"
		if err := os.WriteFile(launcher, []byte(supervisor), 0600); err != nil {
			return err
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start Windows package hook: %w", err)
		}
		// The SDK can return without forwarding the command while it is
		// winding down a previous launch. Retry only if the guest hasn't
		// started the script. The guest's mkdir prevents duplicate work.
		started, err := waitForHookStart(ctx, temp)
		if err != nil {
			return err
		}
		if !started {
			retry := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
			retry.Env = cmd.Env
			if err := retry.Run(); err != nil {
				return fmt.Errorf("restart Windows package hook: %w", err)
			}
		}
		return waitForHook(ctx, temp, output)
	}
	path, err := p.WindowsPath(name)
	if err != nil {
		return err
	}
	cmd = p.Command(ctx, "cmd.exe", "/d", "/c", path)
	cmd.Dir = dir
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.WaitDelay = 2 * time.Second
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Windows package hook failed: %w", err)
	}
	return nil
}

func waitForHookStart(ctx context.Context, dir string) (bool, error) {
	started := filepath.Join(dir, "started")
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(started); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			return false, nil
		case <-ticker.C:
		}
	}
}

func waitForHook(ctx context.Context, dir string, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(filepath.Join(dir, "exit.txt"))
		if err == nil {
			logFile, logErr := os.Open(filepath.Join(dir, "hook.log"))
			if logErr == nil {
				if output != nil {
					_, _ = io.Copy(output, logFile)
				}
				_ = logFile.Close()
			}
			code, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return fmt.Errorf("read Windows package hook exit code: %w", err)
			}
			if code != 0 {
				return fmt.Errorf("Windows package hook exited with code %d", code)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
