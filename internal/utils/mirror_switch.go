package utils

import (
	"runtime"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
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
	return isUnixChinaTimeZone() || isUnixChinaLocale()
}

// IsMainlandChinaLocale reports whether a POSIX/BCP-47/AppleLocale tag targets
// mainland China (region subtag CN), e.g. "zh_CN", "zh_CN.UTF-8", "en_CN",
// "zh-Hans-CN", or "zh_CN@calendar=chinese".
func IsMainlandChinaLocale(locale string) bool {
	tag := strings.TrimSpace(locale)
	tag, _, _ = strings.Cut(tag, "@")
	tag, _, _ = strings.Cut(tag, ".")
	parts := strings.FieldsFunc(tag, func(c rune) bool { return c == '_' || c == '-' })
	for i, part := range parts {
		if i == 0 {
			continue
		}
		if strings.EqualFold(part, "CN") {
			return true
		}
	}
	return false
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
