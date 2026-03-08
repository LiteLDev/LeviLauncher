package gdk

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

const forcedRegistrationConfigVersion = "1.26.0.0"

var microsoftGameConfigIdentityVersionPattern = regexp.MustCompile(`(?is)(<Identity\b[^>]*\bVersion\s*=\s*")([^"]*)(")`)

func wdappPath() string {
	return `C:\Program Files (x86)\Microsoft GDK\bin\wdapp.exe`
}

func WdappExists() bool { return utils.FileExists(wdappPath()) }

func logCommandOutput(scope string, output []byte) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return
	}
	log.Printf("%s output: %s", scope, trimmed)
}

func runHiddenCommand(scope string, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	logCommandOutput(scope, output)
	if err != nil {
		log.Printf("%s failed: %v", scope, err)
		return err
	}
	log.Printf("%s succeeded", scope)
	return nil
}

func normalizeInstallPath(path string) string {
	norm := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
	norm = strings.TrimPrefix(norm, `\\?\`)
	norm = strings.TrimPrefix(norm, `\??\`)
	return norm
}

func replaceMicrosoftGameConfigVersion(content []byte, version string) ([]byte, string, error) {
	text := string(content)
	matches := microsoftGameConfigIdentityVersionPattern.FindStringSubmatch(text)
	if len(matches) != 4 {
		return nil, "", fmt.Errorf("identity version attribute not found")
	}
	originalVersion := strings.TrimSpace(matches[2])
	if originalVersion == version {
		return content, originalVersion, nil
	}
	replaced := microsoftGameConfigIdentityVersionPattern.ReplaceAllString(text, "${1}"+version+"${3}")
	return []byte(replaced), originalVersion, nil
}

func overrideMicrosoftGameConfigVersion(folder string, version string) (func() error, error) {
	configPath := filepath.Join(folder, "MicrosoftGame.Config")
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", configPath)
	}
	originalContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	updatedContent, originalVersion, err := replaceMicrosoftGameConfigVersion(originalContent, version)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(configPath, updatedContent, info.Mode().Perm()); err != nil {
		return nil, err
	}
	log.Printf("gdk.overrideMicrosoftGameConfigVersion: temporary override path=%s version=%s original=%s", configPath, version, originalVersion)
	return func() error {
		if err := os.WriteFile(configPath, originalContent, info.Mode().Perm()); err != nil {
			return err
		}
		log.Printf("gdk.overrideMicrosoftGameConfigVersion: restored path=%s version=%s", configPath, originalVersion)
		return nil
	}, nil
}

func RegisterVersionFolder(folder string) (result string) {
	folder = filepath.Clean(strings.TrimSpace(folder))
	log.Printf("gdk.RegisterVersionFolder: start folder=%s", folder)
	if folder == "" || !utils.FileExists(folder) {
		log.Printf("gdk.RegisterVersionFolder: target folder missing or empty: %s", folder)
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	if !WdappExists() {
		log.Printf("gdk.RegisterVersionFolder: wdapp not found at %s", wdappPath())
		return "ERR_MSIXVC_NOT_FOUND"
	}
	if !registry.IsDevModeEnabled() {
		log.Printf("gdk.RegisterVersionFolder: Windows Developer Mode disabled, trying to enable it")
		if !registry.TryEnableDevMode() {
			log.Printf("gdk.RegisterVersionFolder: failed to enable Windows Developer Mode")
			return "ERR_DEV_MODE_REQUIRED"
		}
		log.Printf("gdk.RegisterVersionFolder: Windows Developer Mode enabled")
	}
	restoreConfig, err := overrideMicrosoftGameConfigVersion(folder, forcedRegistrationConfigVersion)
	if err != nil {
		log.Printf("gdk.RegisterVersionFolder: failed to override MicrosoftGame.Config version for folder=%s: %v", folder, err)
		return "ERR_REGISTER_FAILED"
	}
	defer func() {
		if restoreConfig == nil {
			return
		}
		if restoreErr := restoreConfig(); restoreErr != nil {
			log.Printf("gdk.RegisterVersionFolder: failed to restore MicrosoftGame.Config for folder=%s: %v", folder, restoreErr)
			if result == "" {
				result = "ERR_REGISTER_FAILED"
			}
		}
	}()
	cmd := exec.Command(wdappPath(), "register", folder)
	log.Printf("gdk.RegisterVersionFolder: executing %q register %q", wdappPath(), folder)
	if err := runHiddenCommand("gdk.RegisterVersionFolder: wdapp register", cmd); err != nil {
		return "ERR_REGISTER_FAILED"
	}
	log.Printf("gdk.RegisterVersionFolder: completed folder=%s", folder)
	return ""
}

func UnregisterIfExists(isPreview bool) string {
	pkg := "MICROSOFT.MINECRAFTUWP"
	if isPreview {
		pkg = "Microsoft.MinecraftWindowsBeta"
	}
	log.Printf("gdk.UnregisterIfExists: checking existing package=%s", pkg)
	if info, err := registry.GetAppxInfo(pkg); err == nil && info != nil {
		pf := strings.TrimSpace(info.PackageFullName)
		log.Printf("gdk.UnregisterIfExists: found package=%s fullName=%s installLocation=%s", pkg, pf, strings.TrimSpace(info.InstallLocation))
		if pf != "" {
			cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
			log.Printf("gdk.UnregisterIfExists: removing package=%s fullName=%s", pkg, pf)
			if er := runHiddenCommand("gdk.UnregisterIfExists: Remove-AppxPackage", cmd); er != nil {
				return "ERR_UNREGISTER_FAILED"
			}
		}
	} else if err != nil {
		log.Printf("gdk.UnregisterIfExists: GetAppxInfo failed for package=%s: %v", pkg, err)
	} else {
		log.Printf("gdk.UnregisterIfExists: package=%s not found", pkg)
	}
	return ""
}

func UnregisterVersionFolder(folder string) string {
	folder = filepath.Clean(strings.TrimSpace(folder))
	log.Printf("gdk.UnregisterVersionFolder: start folder=%s", folder)
	if folder == "" {
		log.Printf("gdk.UnregisterVersionFolder: target folder is empty")
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	check := func(pkg string) string {
		info, err := registry.GetAppxInfo(pkg)
		if err != nil || info == nil {
			if err != nil {
				log.Printf("gdk.UnregisterVersionFolder: GetAppxInfo failed for package=%s: %v", pkg, err)
			}
			return ""
		}
		loc := normalizeInstallPath(info.InstallLocation)
		f := normalizeInstallPath(folder)
		log.Printf("gdk.UnregisterVersionFolder: compare package=%s installLocation=%s target=%s", pkg, loc, f)
		if loc == f {
			log.Printf("gdk.UnregisterVersionFolder: matched package=%s fullName=%s", pkg, strings.TrimSpace(info.PackageFullName))
			return strings.TrimSpace(info.PackageFullName)
		}
		return ""
	}
	pf := check("MICROSOFT.MINECRAFTUWP")
	if pf == "" {
		pf = check("Microsoft.MinecraftWindowsBeta")
	}
	if pf == "" {
		log.Printf("gdk.UnregisterVersionFolder: folder=%s is not registered to system", folder)
		return "ERR_NOT_REGISTERED_THIS_VERSION"
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
	log.Printf("gdk.UnregisterVersionFolder: removing fullName=%s", pf)
	if er := runHiddenCommand("gdk.UnregisterVersionFolder: Remove-AppxPackage", cmd); er != nil {
		return "ERR_UNREGISTER_FAILED"
	}
	log.Printf("gdk.UnregisterVersionFolder: completed folder=%s", folder)
	return ""
}
