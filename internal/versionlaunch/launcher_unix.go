//go:build darwin || linux

package versionlaunch

import (
	"context"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func (l *Launcher) Launch(ctx context.Context, name string, checkRunning bool) string {
	dir, code := resolveVersionDir(name)
	if code != "" {
		return code
	}
	meta, err := versions.ReadMeta(dir)
	if err != nil {
		return "ERR_INVALID_VERSION_META"
	}
	if versions.DetectPackageType(dir, meta) == versions.PackageTypeUWP {
		return "ERR_UWP_UNSUPPORTED_ON_HOST"
	}
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	if _, err = os.Stat(exe); err != nil {
		return "ERR_NOT_FOUND_EXE"
	}
	if isProcessRunningAtPath(exe) {
		return "ERR_GAME_ALREADY_RUNNING"
	}
	p, err := gameruntime.Default()
	if err != nil {
		return "ERR_WINE_RUNTIME: " + err.Error()
	}
	if _, err = leviloader.PatchAndActivateForWine(ctx, dir); err != nil {
		return "ERR_LOADER_PREPARE: " + err.Error()
	}
	prepared, err := peeditor.PrepareExecutableForLaunch(ctx, dir, meta.EnableConsole)
	if err != nil {
		return "ERR_LAUNCH_PREPARE: " + err.Error()
	}
	if prepared != "" {
		exe = prepared
	}
	args := parseCommandLineArgs(meta.LaunchArgs)
	if meta.EnableEditorMode {
		args = append(args, "-Editor", "true")
	}
	// Game lifetime is independent of the launcher's context once started.
	cmd, detached, err := p.GameCommand(context.Background(), exe, args...)
	if err != nil {
		return "ERR_WINE_RUNTIME: " + err.Error()
	}
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	env := map[string]string{}
	for _, e := range cmd.Env {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			env[k] = v
		}
	}
	for _, e := range strings.Split(meta.EnvVars, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(e), "=")
		if ok && k != "" {
			env[k] = v
		}
	}
	cmd.Env = nil
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	logFile, err := os.OpenFile(filepath.Join(dir, ".levilauncher", "wine-launch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "ERR_LAUNCH_GAME: " + err.Error()
	}
	fmt.Fprintln(logFile, time.Now().Format(time.RFC3339), "Starting", filepath.Base(exe), "via", filepath.Base(cmd.Path))
	cmd.Stdout, cmd.Stderr = logFile, logFile
	eventsink.From(ctx).Emit(launch.EventMcLaunchStart, struct{}{})
	if err = cmd.Start(); err != nil {
		logFile.Close()
		return "ERR_LAUNCH_GAME: " + err.Error()
	}
	pid := cmd.Process.Pid
	if detached {
		// Sikarugir delegates to its SDK and exits before the game's window.
		// Monitor the actual instance, not the short-lived wrapper's PID.
		pid = 0
	}
	go func() {
		defer logFile.Close()
		if err := cmd.Wait(); err != nil {
			fmt.Fprintln(logFile, "Wine launcher exited:", err)
		}
	}()
	go launch.MonitorGameProcess(ctx, dir, pid)
	return ""
}
func isProcessRunningAtPath(path string) bool {
	want, _ := filepath.EvalSymlinks(path)
	for _, p := range gameruntime.Processes() {
		got, _ := filepath.EvalSymlinks(p.Executable)
		if got == want && want != "" {
			return true
		}
	}
	return false
}
