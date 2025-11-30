package gdk

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func wdappPath() string {
	return `C:\Program Files (x86)\Microsoft GDK\bin\wdapp.exe`
}

func WdappExists() bool { return utils.FileExists(wdappPath()) }

func RegisterVersionFolder(folder string) string {
	if strings.TrimSpace(folder) == "" || !utils.FileExists(folder) {
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	if !WdappExists() {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	cmd := exec.Command(wdappPath(), "register", filepath.Clean(folder))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return "ERR_REGISTER_FAILED"
	}
	return ""
}

func UnregisterIfExists(isPreview bool) string {
	pkg := "MICROSOFT.MINECRAFTUWP"
	if isPreview {
		pkg = "Microsoft.MinecraftWindowsBeta"
	}
	if info, err := registry.GetAppxInfo(pkg); err == nil && info != nil {
		pf := strings.TrimSpace(info.PackageFullName)
		if pf != "" {
			cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if er := cmd.Run(); er != nil {
				return "ERR_UNREGISTER_FAILED"
			}
		}
	}
	return ""
}

func UnregisterVersionFolder(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	check := func(pkg string) string {
		info, err := registry.GetAppxInfo(pkg)
		if err != nil || info == nil {
			return ""
		}
		loc := strings.ToLower(strings.TrimSpace(info.InstallLocation))
		f := strings.ToLower(strings.TrimSpace(filepath.Clean(folder)))
		if loc == f {
			return strings.TrimSpace(info.PackageFullName)
		}
		return ""
	}
	pf := check("MICROSOFT.MINECRAFTUWP")
	if pf == "" {
		pf = check("Microsoft.MinecraftWindowsBeta")
	}
	if pf == "" {
		return "ERR_NOT_REGISTERED_THIS_VERSION"
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if er := cmd.Run(); er != nil {
		return "ERR_UNREGISTER_FAILED"
	}
	return ""
}
