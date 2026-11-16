//go:build darwin || linux

package oslang

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func IsChineseUI() bool {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(k); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "zh")
		}
	}
	if runtime.GOOS == "darwin" {
		b, e := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
		return e == nil && strings.Contains(string(b), "zh-")
	}
	return false
}
