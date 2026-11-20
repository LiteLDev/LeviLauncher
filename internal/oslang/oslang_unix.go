//go:build darwin

package oslang

import (
	"os"
	"os/exec"
	"strings"
)

func IsChineseUI() bool {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(k); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "zh")
		}
	}
	b, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
	return err == nil && strings.Contains(string(b), "zh-")
}
