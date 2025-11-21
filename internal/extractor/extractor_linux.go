//go:build linux

package extractor

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/utils"
)

func Init() {}

func ensureEmbeddedDLL() string {
    if len(embeddedLauncherCoreDLL) == 0 {
        return ""
    }
    base := utils.BaseRoot()
    dir := filepath.Join(base, "bin")
    _ = os.MkdirAll(dir, 0755)
    target := filepath.Join(dir, "launcher_core.dll")
    if fi, err := os.Stat(target); err == nil && fi.Size() > 0 {
        return target
    }
    tmp := target + ".tmp"
    if err := os.WriteFile(tmp, embeddedLauncherCoreDLL, 0644); err == nil {
        if err := os.Rename(tmp, target); err == nil {
            return target
        }
        _ = os.Remove(tmp)
    }
    return ""
}

func ensureEmbeddedWrapper() string {
    if len(embeddedLauncherCoreCLI) == 0 {
        return ""
    }
    base := utils.BaseRoot()
    dir := filepath.Join(base, "bin")
    _ = os.MkdirAll(dir, 0755)
    target := filepath.Join(dir, "launcher_core_cli.exe")
    if fi, err := os.Stat(target); err == nil && fi.Size() > 0 {
        return target
    }
    tmp := target + ".tmp"
    if err := os.WriteFile(tmp, embeddedLauncherCoreCLI, 0644); err == nil {
        if err := os.Rename(tmp, target); err == nil {
            return target
        }
        _ = os.Remove(tmp)
    }
    return ""
}

func MiHoYo(msixvcPath string, outDir string) (int, string) {
	if strings.TrimSpace(msixvcPath) == "" || strings.TrimSpace(outDir) == "" {
		return 1, "ERR_ARGS"
	}
	if _, err := os.Stat(msixvcPath); err != nil {
		return 1, "ERR_MSIXVC_NOT_FOUND"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return 1, "ERR_CREATE_TARGET_DIR"
	}

    dll := strings.TrimSpace(os.Getenv("LAUNCHER_CORE_DLL"))
    if dll == "" {
        if p := ensureEmbeddedDLL(); p != "" {
            dll = p
        } else {
            base := utils.BaseRoot()
            cand := filepath.Join(base, "bin", "launcher_core.dll")
            if _, err := os.Stat(cand); err == nil {
                dll = cand
            }
        }
    }
    if strings.TrimSpace(dll) == "" {
        return 1, "ERR_DLL_NOT_AVAILABLE"
    }
    if _, err := os.Stat(dll); err != nil {
        return 1, "ERR_DLL_NOT_AVAILABLE"
    }
    wrapper := strings.TrimSpace(os.Getenv("LAUNCHER_CORE_WRAPPER"))
    if wrapper == "" {
        if p := ensureEmbeddedWrapper(); p != "" {
            wrapper = p
        } else {
            base := filepath.Dir(dll)
            cand := filepath.Join(base, "launcher_core_cli.exe")
            if _, err := os.Stat(cand); err == nil {
                wrapper = cand
            }
        }
    }
    if strings.TrimSpace(wrapper) == "" {
        return 1, "ERR_WRAPPER_NOT_FOUND"
    }
    if _, err := os.Stat(wrapper); err != nil {
        return 1, "ERR_WRAPPER_NOT_FOUND"
    }
    cmd := exec.Command("wine", wrapper, msixvcPath, outDir)
    if err := cmd.Run(); err != nil {
        return 1, "ERR_APPX_INSTALL_FAILED"
    }
    return 0, ""
}
