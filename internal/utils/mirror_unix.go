//go:build !windows

package utils

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func isWindowsTimeZoneChina() bool { return false }
func isWindowsLocaleZhCN() bool    { return false }

var (
	readUnixTimezoneFile = func() (string, error) {
		b, err := os.ReadFile("/etc/timezone")
		return string(b), err
	}
	readUnixLocaltimeLink = func() (string, error) {
		return os.Readlink("/etc/localtime")
	}
	readDarwinAppleLocale = func() string {
		if runtime.GOOS != "darwin" {
			return ""
		}
		out, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLocale").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
)

func isChinaTimezoneName(raw string) bool {
	low := strings.ToLower(strings.TrimSpace(raw))
	return strings.Contains(low, "asia/shanghai") ||
		strings.Contains(low, "asia/urumqi") ||
		strings.Contains(low, "asia/chongqing") ||
		strings.Contains(low, "asia/harbin")
}

func isUnixChinaTimeZone() bool {
	if s, err := readUnixTimezoneFile(); err == nil {
		return isChinaTimezoneName(s)
	}
	if p, err := readUnixLocaltimeLink(); err == nil {
		return isChinaTimezoneName(p)
	}
	return false
}

func isUnixChinaLocale() bool {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			continue
		}
		if IsMainlandChinaLocale(v) {
			return true
		}
	}
	return IsMainlandChinaLocale(readDarwinAppleLocale())
}
