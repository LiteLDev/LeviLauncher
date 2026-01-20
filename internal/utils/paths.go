package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
)

func LauncherDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

func BaseRoot() string {
	if v := strings.TrimSpace(config.GetBaseRootOverride()); v != "" {
		_ = os.MkdirAll(v, 0o755)
		return v
	}
	exeName := "levilauncher.exe"
	if exe, err := os.Executable(); err == nil {
		base := strings.TrimSpace(filepath.Base(exe))
		if base != "" {
			exeName = strings.ToLower(base)
		}
	}
	if ap := strings.TrimSpace(GetAppDataPath()); ap != "" {
		root := filepath.Join(ap, exeName)
		_ = os.MkdirAll(root, 0o755)
		return root
	}
	if d, _ := os.UserCacheDir(); strings.TrimSpace(d) != "" {
		root := filepath.Join(d, exeName)
		_ = os.MkdirAll(root, 0o755)
		return root
	}
	root := filepath.Join(LauncherDir(), exeName)
	_ = os.MkdirAll(root, 0o755)
	return root
}

func GetInstallerDir() (string, error) {
	base := BaseRoot()
	dir := filepath.Join(base, "installers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}
	return dir, nil
}

func GetVersionsDir() (string, error) {
	base := BaseRoot()
	dir := filepath.Join(base, "versions")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}
	return dir, nil
}

func CanWriteDir(p string) bool {
	v := strings.TrimSpace(p)
	if v == "" {
		return false
	}
	if !filepath.IsAbs(v) {
		return false
	}

	isWritable := func(dir string) bool {
		tf := filepath.Join(dir, ".ll_write_test.tmp")
		f, err := os.Create(tf)
		if err != nil {
			return false
		}
		_, werr := f.Write([]byte("ok"))
		cerr := f.Close()
		_ = os.Remove(tf)
		return werr == nil && cerr == nil
	}

	curr := v
	for {
		info, err := os.Stat(curr)
		if err == nil {
			if info.IsDir() {
				return isWritable(curr)
			}
			return false
		}
		if !os.IsNotExist(err) {
			return false
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return false
		}
		curr = parent
	}
}
