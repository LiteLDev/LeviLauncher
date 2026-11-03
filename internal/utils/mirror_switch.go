package utils

import (
	"os"
	"runtime"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	winreg "golang.org/x/sys/windows/registry"
)

// IsMirrorEnabled reports whether mirrors should be used. A value saved from
// the global mirror switch takes precedence; without one, it falls back to the
// time zone and locale based China-user detection.
func IsMirrorEnabled() bool {
	if enabled, ok := config.GetMirrorSourceOverride(); ok {
		return enabled
	}
	return detectsChinaUser()
}

// detectsChinaUser reports whether the current session appears to be used from
// mainland China, based on time zone and locale settings.
func detectsChinaUser() bool {
	if runtime.GOOS == "windows" {
		return isWindowsTimeZoneChina() && isWindowsLocaleZhCN()
	}

	isTz := false
	isLang := false
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		s := strings.ToLower(strings.TrimSpace(string(b)))
		if s == "asia/shanghai" || s == "asia/urumqi" {
			isTz = true
		}
	} else if p, err := os.Readlink("/etc/localtime"); err == nil {
		low := strings.ToLower(p)
		if strings.Contains(low, "asia/shanghai") || strings.Contains(low, "asia/urumqi") {
			isTz = true
		}
	}

	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "zh_cn") || strings.HasPrefix(v, "zh-cn") || v == "zh" {
			isLang = true
			break
		}
	}

	return isTz || isLang
}

func isZhCnLocaleValue(v string) bool {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.TrimPrefix(s, "0x")
	if strings.HasPrefix(s, "zh-cn") || s == "zh" || s == "zh-hans" {
		return true
	}
	if strings.Contains(s, "chinese (simplified") {
		return true
	}
	return s == "804" || s == "0804" || s == "00000804"
}

func isWindowsTimeZoneChina() bool {
	k, err := winreg.OpenKey(winreg.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\TimeZoneInformation`, winreg.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	for _, field := range []string{"TimeZoneKeyName", "StandardName"} {
		if val, _, err := k.GetStringValue(field); err == nil {
			if strings.EqualFold(strings.TrimSpace(val), "China Standard Time") {
				return true
			}
		}
	}
	return false
}

func isWindowsLocaleZhCN() bool {
	k, err := winreg.OpenKey(winreg.CURRENT_USER, `Control Panel\International`, winreg.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		for _, field := range []string{"LocaleName", "Locale", "sLanguage"} {
			if val, _, err := k.GetStringValue(field); err == nil && isZhCnLocaleValue(val) {
				return true
			}
		}
	}
	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if isZhCnLocaleValue(os.Getenv(key)) {
			return true
		}
	}
	return false
}
