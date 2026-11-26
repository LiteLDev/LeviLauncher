//go:build !windows

package utils

import (
	"errors"
	"testing"
)

func TestIsMainlandChinaLocale(t *testing.T) {
	cases := map[string]bool{
		"zh_CN":                  true,
		"zh_CN.UTF-8":            true,
		"en_CN":                  true,
		"zh-Hans-CN":             true,
		"zh_CN@calendar=chinese": true,
		"":                       false,
		"zh":                     false,
		"zh_HK":                  false,
		"zh-Hant-TW":             false,
		"zh_US":                  false,
		"en_US":                  false,
		"en_US.UTF-8":            false,
	}
	for locale, want := range cases {
		if got := IsMainlandChinaLocale(locale); got != want {
			t.Errorf("IsMainlandChinaLocale(%q) = %v, want %v", locale, got, want)
		}
	}
}

func TestDetectsChinaUserUnix(t *testing.T) {
	prevTzFile := readUnixTimezoneFile
	prevLocaltime := readUnixLocaltimeLink
	prevAppleLocale := readDarwinAppleLocale
	t.Cleanup(func() {
		readUnixTimezoneFile = prevTzFile
		readUnixLocaltimeLink = prevLocaltime
		readDarwinAppleLocale = prevAppleLocale
	})

	clearEnv := func(t *testing.T) {
		t.Helper()
		for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
			t.Setenv(k, "")
		}
	}

	t.Run("AppleLocale CN without LANG (Finder launch)", func(t *testing.T) {
		clearEnv(t)
		readUnixTimezoneFile = func() (string, error) { return "", errors.New("missing") }
		readUnixLocaltimeLink = func() (string, error) { return "/var/db/timezone/zoneinfo/America/Los_Angeles", nil }
		readDarwinAppleLocale = func() string { return "zh_CN" }
		if !detectsChinaUser() {
			t.Fatal("expected detectsChinaUser() == true for AppleLocale=zh_CN")
		}
	})

	t.Run("Asia/Shanghai timezone with English locale", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LANG", "en_US.UTF-8")
		readUnixTimezoneFile = func() (string, error) { return "", errors.New("missing") }
		readUnixLocaltimeLink = func() (string, error) { return "/var/db/timezone/zoneinfo/Asia/Shanghai", nil }
		readDarwinAppleLocale = func() string { return "en_US" }
		if !detectsChinaUser() {
			t.Fatal("expected detectsChinaUser() == true for Asia/Shanghai")
		}
	})

	t.Run("Taiwan timezone and zh_TW locale", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LANG", "zh_TW.UTF-8")
		readUnixTimezoneFile = func() (string, error) { return "", errors.New("missing") }
		readUnixLocaltimeLink = func() (string, error) { return "/var/db/timezone/zoneinfo/Asia/Taipei", nil }
		readDarwinAppleLocale = func() string { return "zh_TW" }
		if detectsChinaUser() {
			t.Fatal("expected detectsChinaUser() == false for Taiwan")
		}
	})
}
