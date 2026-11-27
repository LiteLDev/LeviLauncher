//go:build darwin

package versionlaunch

import (
	"bytes"
	"context"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	"github.com/liteldev/LeviLauncher/internal/gameauth"
	"github.com/liteldev/LeviLauncher/internal/gameconsole"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/gameshell"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxWineLaunchLogBytes = 16 << 20

var prepareWineMu sync.Mutex

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
	if checkRunning && isProcessRunningAtPath(exe) {
		return "ERR_GAME_ALREADY_RUNNING"
	}
	p, err := gameruntime.Default()
	if err != nil {
		return "ERR_WINE_RUNTIME: " + err.Error()
	}
	if err := PrepareWine(ctx, p); err != nil {
		return err.Error()
	}
	accountSession := gameauth.Register(name, p.Prefix)
	shellSession := gameshell.Register(name, p.Prefix)
	var consoleSession *gameconsole.Session
	if meta.EnableConsole {
		consoleSession = gameconsole.Register(name)
	}
	launched := false
	defer func() {
		if !launched {
			if accountSession != nil {
				accountSession.Close()
			}
			if shellSession != nil {
				shellSession.Close()
			}
			if consoleSession != nil {
				consoleSession.Close()
			}
		}
	}()
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
	if err := p.PrepareGameDirectory(exe); err != nil {
		return "ERR_LAUNCH_PREPARE: " + err.Error()
	}
	prepareOptionsBackup(dir)
	applyOptionsFramePacing(dir, p)
	args := parseCommandLineArgs(meta.LaunchArgs)
	if meta.EnableEditorMode {
		args = append(args, "-Editor", "true")
	}
	extraEnv := map[string]string{
		"LEVI_WINE_INPUT_FIX": "1",
	}
	if title := strings.TrimSpace(name); title != "" {
		extraEnv["LEVI_GAME_TITLE"] = title
	}
	if iconPath := resolveInstanceIconPath(dir); iconPath != "" {
		extraEnv["LEVI_GAME_ICON_PATH"] = iconPath
	}
	for _, e := range strings.Split(meta.EnvVars, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(e), "=")
		if ok && k != "" {
			extraEnv[k] = v
		}
	}
	if accountSession != nil {
		for k, v := range accountSession.Environment() {
			extraEnv[k] = v
		}
		// The client suppresses only prompts acknowledged by this launcher;
		// when it is unavailable the original game prompt remains usable.
		extraEnv["XGR_SIGNIN_NOMSGBOX"] = "0"
	}
	if consoleSession != nil {
		for k, v := range consoleSession.Environment() {
			extraEnv[k] = v
		}
	}
	if shellSession != nil {
		for k, v := range shellSession.Environment() {
			extraEnv[k] = v
		}
	}
	logPath := filepath.Join(dir, ".levilauncher", "wine-launch.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return "ERR_LAUNCH_GAME: " + err.Error()
	}
	openFlags := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > maxWineLaunchLogBytes {
		openFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	}
	// Game lifetime is independent of the launcher's context once started.
	cmd, detached, err := p.GameCommandWithOptions(context.Background(), exe, gameruntime.GameLaunchOptions{
		WorkDir:       dir,
		LogPath:       logPath,
		ExtraEnv:      extraEnv,
		HasWineBridge: hasLeviWineBridge(dir),
	}, args...)
	if err != nil {
		return "ERR_WINE_RUNTIME: " + err.Error()
	}
	logFile, err := os.OpenFile(logPath, openFlags, 0600)
	if err != nil {
		return "ERR_LAUNCH_GAME: " + err.Error()
	}
	fmt.Fprintln(logFile, time.Now().Format(time.RFC3339), "Starting", filepath.Base(exe), "via", filepath.Base(cmd.Path))
	usesLaunchServices := cmd.Path == "/usr/bin/open"
	if usesLaunchServices {
		// /usr/bin/open redirects the launched app's stdout/stderr directly to logPath.
		logFile.Close()
		var openOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &openOut, &openOut
		eventsink.From(ctx).Emit(launch.EventMcLaunchStart, struct{}{})
		if err = cmd.Run(); err != nil {
			detail := strings.TrimSpace(openOut.String())
			if detail != "" {
				return fmt.Sprintf("ERR_LAUNCH_GAME: %v: %s", err, detail)
			}
			return "ERR_LAUNCH_GAME: " + err.Error()
		}
	} else {
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Stdout, cmd.Stderr = logFile, logFile
		eventsink.From(ctx).Emit(launch.EventMcLaunchStart, struct{}{})
		if err = cmd.Start(); err != nil {
			logFile.Close()
			return "ERR_LAUNCH_GAME: " + err.Error()
		}
	}
	launched = true
	if consoleSession != nil {
		if err := gameconsole.OpenDefaultTerminal(name, consoleSession.Address(), consoleSession.AuthToken()); err != nil {
			if f, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); openErr == nil {
				fmt.Fprintln(f, "Failed to open console terminal:", err)
				f.Close()
			}
		}
	}
	pid := 0
	if !usesLaunchServices {
		pid = cmd.Process.Pid
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
	}
	go func() {
		launch.MonitorGameProcess(ctx, dir, pid)
		restoreDamagedOptions(dir)
		if accountSession != nil {
			accountSession.Close()
		}
		if shellSession != nil {
			shellSession.Close()
		}
		if consoleSession != nil {
			consoleSession.Close()
		}
	}()
	return ""
}

// PrepareWine installs the same dependencies for managed setup and on-demand launch.
func PrepareWine(ctx context.Context, p gameruntime.Profile) error {
	prepareWineMu.Lock()
	defer prepareWineMu.Unlock()
	sink := eventsink.From(ctx)
	if !vcruntime.IsInstalledIn(p) {
		sink.Emit(launch.EventMcLaunchPrepare, "vcruntime")
		if err := vcruntime.Install(ctx, p); err != nil {
			return fmt.Errorf("ERR_VCRUNTIME_MISSING: %w", err)
		}
	}
	if !gameinput.IsInstalledIn(p) {
		sink.Emit(launch.EventMcLaunchPrepare, "gameinput")
		if err := gameinput.Install(ctx, p); err != nil {
			return fmt.Errorf("ERR_GAMEINPUT_INSTALL: %w", err)
		}
	}
	sink.Emit(launch.EventMcLaunchPrepare, "runtime")
	if err := p.PrepareMinecraft(ctx); err != nil {
		return fmt.Errorf("ERR_WINE_RUNTIME: %w", err)
	}
	if err := p.ApplyMacDriverSettings(ctx); err != nil {
		return fmt.Errorf("ERR_WINE_RUNTIME: %w", err)
	}
	return nil
}

func resolveInstanceIconPath(instanceDir string) string {
	if instanceDir == "" {
		return ""
	}
	for _, filename := range []string{"LargeLogo.png", "Logo.png", "StoreLogo.png", "minecraftIcon.ico"} {
		candidate := filepath.Join(instanceDir, filename)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return candidate
		}
	}
	return ""
}

func hasLeviWineBridge(instanceDir string) bool {
	if instanceDir == "" {
		return false
	}
	direct := filepath.Join(instanceDir, "mods", "LeviClient", "LeviWineBridge.dll")
	if info, err := os.Stat(direct); err == nil && info.Mode().IsRegular() {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(instanceDir, "mods"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(instanceDir, "mods", entry.Name(), "LeviWineBridge.dll")
		if info, statErr := os.Stat(candidate); statErr == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
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
