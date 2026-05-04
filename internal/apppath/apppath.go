package apppath

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	BaseRootOverrideProvider func() string

	baseRootOverride string
)

func SetBaseRootOverride(path string) {
	baseRootOverride = path
}

func executableStorageRoot() string {
	exeName := "levilauncher.exe"
	if exe, err := os.Executable(); err == nil {
		base := strings.TrimSpace(filepath.Base(exe))
		if base != "" {
			exeName = strings.ToLower(base)
		}
	}

	root := filepath.Join(AppData(), exeName)
	_ = os.MkdirAll(root, 0o755)
	return root
}

func LauncherDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

func AppData() string {
	if v := os.Getenv("APPDATA"); strings.TrimSpace(v) != "" {
		return v
	}
	if v, _ := os.UserCacheDir(); strings.TrimSpace(v) != "" {
		return v
	}
	return LauncherDir()
}

func ConfigPath() string {
	return filepath.Join(executableStorageRoot(), "config.json")
}

func ConfigDir() string {
	return filepath.Dir(ConfigPath())
}

func LogsDir() string {
	dir := filepath.Join(executableStorageRoot(), "logs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func StartupLogPath() string {
	return filepath.Join(LogsDir(), "startup.log")
}

func BaseRoot() string {
	if v := strings.TrimSpace(baseRootOverride); v != "" {
		_ = os.MkdirAll(v, 0o755)
		return v
	}

	if BaseRootOverrideProvider != nil {
		if v := strings.TrimSpace(BaseRootOverrideProvider()); v != "" {
			_ = os.MkdirAll(v, 0o755)
			return v
		}
	}

	return executableStorageRoot()
}

func InstallersDir() (string, error) {
	base := BaseRoot()
	dir := filepath.Join(base, "installers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}
	return dir, nil
}

func VersionsDir() (string, error) {
	base := BaseRoot()
	dir := filepath.Join(base, "versions")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}
	return dir, nil
}

func MinecraftData(isPreview bool) string {
	suffix := "Minecraft Bedrock"
	if isPreview {
		suffix = "Minecraft Bedrock Preview"
	}
	return filepath.Join(AppData(), suffix)
}
