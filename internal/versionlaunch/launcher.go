package versionlaunch

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/discord"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/preloader"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

type Launcher struct {
	enablePreloader bool
}

func New(enablePreloader bool) *Launcher {
	return &Launcher{enablePreloader: enablePreloader}
}

func (l *Launcher) Launch(ctx context.Context, name string, checkRunning bool) string {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	dir := filepath.Join(vdir, strings.TrimSpace(name))
	exe := filepath.Join(dir, "Minecraft.Windows.exe")
	if !utils.FileExists(exe) {
		return "ERR_NOT_FOUND_EXE"
	}
	application.Get().Event.Emit(launch.EventMcLaunchStart, struct{}{})
	_ = vcruntime.EnsureForVersion(ctx, dir)
	if l.enablePreloader {
		_ = preloader.EnsureForVersion(ctx, dir)
		_ = peeditor.EnsureForVersion(ctx, dir)
		_ = peeditor.RunForVersion(ctx, dir)
	}

	var args []string
	var envs []string
	toRun := exe
	var gameVer string
	var enableConsole bool
	if m, err := versions.ReadMeta(dir); err == nil {
		if m.EnvVars != "" {
			for _, line := range strings.Split(m.EnvVars, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					envs = append(envs, trimmed)
				}
			}
		}
		if m.LaunchArgs != "" {
			args = append(args, parseCommandLineArgs(m.LaunchArgs)...)
		}
		enableConsole = m.EnableConsole
		preparedExe, err := peeditor.PrepareExecutableForLaunch(ctx, dir, m.EnableConsole)
		if err != nil {
			log.Printf("Failed to prepare executable: %v", err)
		} else if strings.TrimSpace(preparedExe) != "" {
			toRun = preparedExe
		}
		if m.Registered {
			isPreview := strings.EqualFold(strings.TrimSpace(m.Type), "preview")
			protocol := "minecraft://"
			if isPreview {
				protocol = "minecraft-preview://"
			}
			url := protocol
			if m.EnableEditorMode {
				url = protocol + "creator/?Editor=true"
			}

			cmd := exec.Command("cmd", "/c", "start", "", url)
			if len(envs) > 0 {
				cmd.Env = append(os.Environ(), envs...)
			}
			if err := cmd.Start(); err != nil {
				return "ERR_LAUNCH_GAME"
			}
			gameVer = strings.TrimSpace(m.GameVersion)
			discord.SetPlayingVersion(gameVer)
			go launch.MonitorGameProcess(ctx, dir)
			return ""
		}
		if m.EnableEditorMode {
			args = []string{"-Editor", "true"}
		}
		gameVer = strings.TrimSpace(m.GameVersion)
	}

	if checkRunning {
		if isProcessRunningAtPath(toRun) {
			return "ERR_GAME_ALREADY_RUNNING"
		}
	}
	cmd := exec.Command(toRun, args...)
	if len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	}
	cmd.Dir = filepath.Dir(toRun)
	if enableConsole {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:       false,
			NoInheritHandles: true,
		}
	}
	if err := cmd.Start(); err != nil {
		return "ERR_LAUNCH_GAME"
	}
	discord.SetPlayingVersion(gameVer)
	go launch.MonitorGameProcess(ctx, dir)
	return ""
}

func parseCommandLineArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false

	for _, r := range input {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t', '\n', '\r':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func isProcessRunningAtPath(exePath string) bool {
	normalizedTarget := strings.ToLower(filepath.Clean(strings.TrimSpace(exePath)))
	if normalizedTarget == "" {
		return false
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return false
	}
	for {
		pid := pe.ProcessID
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err == nil {
			buf := make([]uint16, 1024)
			size := uint32(len(buf))
			if e := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e == nil && size > 0 {
				processPath := windows.UTF16ToString(buf[:size])
				_ = windows.CloseHandle(h)
				normalizedProcessPath := strings.ToLower(filepath.Clean(strings.TrimSpace(processPath)))
				normalizedProcessPath = strings.TrimPrefix(normalizedProcessPath, `\\?\`)
				normalizedProcessPath = strings.TrimPrefix(normalizedProcessPath, `\??\`)
				if normalizedProcessPath == normalizedTarget {
					return true
				}
			} else {
				_ = windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return false
}
